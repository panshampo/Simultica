package lark

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"path"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/events"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

// CardStatus mirrors lark_outbound_card_message.status. Kept as a typed
// alias so callers can't pass arbitrary strings into the status column.
type CardStatus string

const (
	CardStatusPending   CardStatus = "pending"
	CardStatusStreaming CardStatus = "streaming"
	CardStatusFinal     CardStatus = "final"
	CardStatusError     CardStatus = "error"
)

// CardKind enumerates the small set of card variants the patcher
// renders. The Renderer is plug-replaceable so the on-wire card
// template can evolve without touching the patcher's transport / DB
// logic.
type CardKind string

const (
	CardKindThinking CardKind = "thinking"
	CardKindRunning  CardKind = "running"
	CardKindFinal    CardKind = "final"
	CardKindError    CardKind = "error"
)

// CardRender is the rendered card body the Renderer produces. The
// patcher serializes the JSON before handing it to APIClient.
type CardRender struct {
	JSON string
}

// RenderInput is the (typed) snapshot the Renderer sees when building
// or patching a card. Fields are populated as they become available
// during a task lifecycle — IssueNumber is set for `/issue` flows,
// Content is set for completed chat tasks, ErrorMessage for failed.
type RenderInput struct {
	Kind         CardKind
	AgentName    string
	IssueNumber  int32
	IssueID      pgtype.UUID
	TaskID       pgtype.UUID
	Content      string
	ErrorMessage string
}

// Renderer turns a typed RenderInput into the actual Lark card JSON.
// Centralizing this lets us swap card templates (or A/B them) without
// touching event subscription or persistence code.
type Renderer interface {
	Render(in RenderInput) (CardRender, error)
}

// defaultRenderer produces minimal text-only cards that work against
// Lark's generic interactive-card schema. The exact JSON layout will
// be refined when the real product card design lands; this default
// keeps the wiring real (the JSON deserializes against Lark's schema)
// without committing the product to a particular template.
type defaultRenderer struct{}

// NewDefaultRenderer returns the production-default Renderer. Override
// via PatcherConfig.Renderer when a custom template is needed.
func NewDefaultRenderer() Renderer { return &defaultRenderer{} }

func (defaultRenderer) Render(in RenderInput) (CardRender, error) {
	header := "Multica"
	if in.AgentName != "" {
		header = in.AgentName
	}
	var body string
	switch in.Kind {
	case CardKindThinking:
		body = "Thinking…"
	case CardKindRunning:
		body = "Working on it…"
	case CardKindFinal:
		body = in.Content
		if body == "" {
			body = "Done."
		}
	case CardKindError:
		body = "Run failed."
		if in.ErrorMessage != "" {
			body = "Run failed: " + in.ErrorMessage
		}
	default:
		return CardRender{}, fmt.Errorf("unknown card kind %q", in.Kind)
	}
	// update_multi MUST be true on every render: Lark refuses to apply
	// PatchInteractiveCard to a card whose config does not declare it
	// a "shared, updatable" card. Since this renderer drives the
	// thinking → streaming → final/error lifecycle (the card is sent
	// once and patched multiple times), an absent update_multi causes
	// every patch after the first send to silently no-op on the
	// Lark side while the local outbound status row still flips to
	// streaming/final. Keep this on every kind — including thinking
	// and error — because that initial JSON IS the body Lark stores
	// and consults for subsequent patches.
	doc := map[string]any{
		"config": map[string]any{
			"wide_screen_mode": true,
			"update_multi":     true,
		},
		"header": map[string]any{
			"template": "blue",
			"title":    map[string]any{"tag": "plain_text", "content": header},
		},
		"elements": []any{
			map[string]any{
				"tag": "div",
				"text": map[string]any{
					"tag":     "plain_text",
					"content": body,
				},
			},
		},
	}
	raw, err := json.Marshal(doc)
	if err != nil {
		return CardRender{}, err
	}
	return CardRender{JSON: string(raw)}, nil
}

// PatcherQueries is the narrow subset of *db.Queries the Patcher
// needs. Declared as an interface so the patcher is unit-testable
// without a real Postgres connection.
type PatcherQueries interface {
	GetAgentTask(ctx context.Context, id pgtype.UUID) (db.AgentTaskQueue, error)
	GetChatSession(ctx context.Context, id pgtype.UUID) (db.ChatSession, error)
	GetAgent(ctx context.Context, id pgtype.UUID) (db.Agent, error)
	GetLarkInstallation(ctx context.Context, id pgtype.UUID) (db.LarkInstallation, error)
	GetLarkChatSessionBindingBySession(ctx context.Context, chatSessionID pgtype.UUID) (db.LarkChatSessionBinding, error)
	GetLarkOutboundCardByTask(ctx context.Context, taskID pgtype.UUID) (db.LarkOutboundCardMessage, error)
	CreateLarkOutboundCardMessage(ctx context.Context, arg db.CreateLarkOutboundCardMessageParams) (db.LarkOutboundCardMessage, error)
	UpdateLarkOutboundCardStatus(ctx context.Context, arg db.UpdateLarkOutboundCardStatusParams) error
	ListCommentsSinceForIssue(ctx context.Context, arg db.ListCommentsSinceForIssueParams) ([]db.Comment, error)
	ListTaskMessages(ctx context.Context, taskID pgtype.UUID) ([]db.TaskMessage, error)
	ListTaskMessageImageAttachments(ctx context.Context, taskID pgtype.UUID) ([]db.TaskMessageAttachment, error)
	CreateComment(ctx context.Context, arg db.CreateCommentParams) (db.Comment, error)
}

// CredentialsResolver decrypts an installation's app_secret for the
// transport layer. *InstallationService satisfies it directly; tests
// substitute a fake.
type CredentialsResolver interface {
	DecryptAppSecret(inst db.LarkInstallation) (string, error)
}

// PatcherConfig tunes the outbound Patcher. Defaults via withDefaults;
// tests typically override Renderer / Now / Logger.
type PatcherConfig struct {
	// Renderer drives the error card template used on the EventTaskFailed
	// path. The success path (EventChatDone) bypasses the renderer
	// entirely — it sends the raw assistant reply as a plain text IM
	// message — so this only matters for the failure branch.
	Renderer Renderer
	Now      func() time.Time
	Logger   *slog.Logger
}

func (c PatcherConfig) withDefaults() PatcherConfig {
	if c.Renderer == nil {
		c.Renderer = NewDefaultRenderer()
	}
	if c.Now == nil {
		c.Now = time.Now
	}
	if c.Logger == nil {
		c.Logger = slog.Default()
	}
	return c
}

// AttachmentReader is the narrow surface the Patcher needs from the
// storage backend to stream a previously-uploaded image (recorded in
// task_message_attachment) back into the Lark image-upload pipeline.
// It mirrors *storage.Storage.GetReader as a single-method interface
// so tests can inject an in-memory fake without dragging the full
// storage abstraction in.
type AttachmentReader interface {
	Open(ctx context.Context, key string) (io.ReadCloser, error)
}

// StorageReaderAdapter bridges a storage.Storage value (which exposes
// GetReader) to the narrower AttachmentReader interface the Patcher
// consumes. Wiring is done in cmd/server/router.go so the Patcher
// itself stays free of any storage-package import.
type StorageReaderAdapter struct {
	Storage interface {
		GetReader(ctx context.Context, key string) (io.ReadCloser, error)
	}
}

func (a StorageReaderAdapter) Open(ctx context.Context, key string) (io.ReadCloser, error) {
	if a.Storage == nil {
		return nil, errors.New("lark patcher: storage reader not configured")
	}
	return a.Storage.GetReader(ctx, key)
}

// Patcher reacts to task-lifecycle events on the event bus and forwards
// chat replies to Lark as plain text IM messages. It is the outbound
// side of §4.5 — but the original "thinking → streaming → final card"
// lifecycle was reduced to a single plain-text reply on EventChatDone
// after Bohan reported the card chrome made replies feel like system
// notifications. The error path is the one survivor of card rendering:
// failed runs surface as a short error card on EventTaskFailed because
// the visual distinction from a normal reply is genuinely useful.
//
// Scope:
//
//   - Only tasks whose chat_session has a lark_chat_session_binding
//     produce outbound. Tasks born from the web UI or autopilot pass
//     through unchanged.
//
//   - Each EventChatDone yields one Lark text message; there is no
//     streaming, no throttling, no DB row to track card-state.
//
//   - Multi-replica safety is inherited from the inbound WS lease: at
//     most one replica holds the installation lease at a time, the
//     event bus is per-process, so exactly one Patcher reacts per run.
type Patcher struct {
	queries       PatcherQueries
	credentials   CredentialsResolver
	client        APIClient
	tracker       ReactionTracker
	threadAnchors ThreadAnchorTracker
	storage       AttachmentReader
	cfg           PatcherConfig
}

// NewPatcher constructs a Patcher bound to its dependencies. The
// patcher does not subscribe to the bus until Register is called.
func NewPatcher(queries PatcherQueries, credentials CredentialsResolver, client APIClient, cfg PatcherConfig) *Patcher {
	cfg = cfg.withDefaults()
	return &Patcher{
		queries:     queries,
		credentials: credentials,
		client:      client,
		cfg:         cfg,
	}
}

// SetReactionTracker wires the in-memory tracker shared with the
// inbound OutcomeReplier so the Patcher can retract the receipt-style
// reaction once the agent's actual reply has been delivered. Optional;
// nil tracker means reactions stick around (the prior behavior).
func (p *Patcher) SetReactionTracker(t ReactionTracker) {
	p.tracker = t
}

// SetThreadAnchorTracker wires the in-memory tracker shared with the
// inbound OutcomeReplier. When set, the Patcher routes outbounds for a
// chat session through Lark's reply-in-thread endpoint so group-chat
// replies group under the user's @-mention. The anchor is cleared on
// terminal events (EventChatDone success, EventTaskFailed) so a stale
// entry can never redirect a future, unrelated reply into an old thread.
func (p *Patcher) SetThreadAnchorTracker(t ThreadAnchorTracker) {
	p.threadAnchors = t
}

// SetAttachmentReader wires the storage reader the Patcher uses to
// stream task_message_attachment rows (chart PNGs, screenshots …)
// back into the Lark image-upload pipeline so they can be embedded
// in the schema-2.0 mixed card. nil keeps the prior text/markdown-only
// behaviour: sendChatReply silently skips the image branch.
func (p *Patcher) SetAttachmentReader(r AttachmentReader) {
	p.storage = r
}

// Register subscribes the patcher to the task-lifecycle events it
// cares about on the supplied bus. Idempotent only if you call it
// against a fresh bus; call sites should invoke it exactly once
// during server boot (after the bus + patcher are constructed and
// before HTTP traffic starts).
//
// Subscriptions are deliberately minimal:
//
//   - EventChatDone — the agent finished replying. The Patcher posts
//     the final reply as a reply-in-thread under the user's original
//     message, then retracts the inbound "OneSecond" receipt reaction.
//     Intermediate task messages (reasoning / partial text) are NOT
//     forwarded — the reaction is the only signal the user sees while
//     the agent is thinking.
//
//   - EventTaskFailed — the run failed; surface a short error card
//     so the failure is visually distinct from a successful reply.
func (p *Patcher) Register(bus *events.Bus) {
	bus.Subscribe(protocol.EventTaskFailed, p.handleEvent)
	bus.Subscribe(protocol.EventChatDone, p.handleEvent)
}

func (p *Patcher) handleEvent(e events.Event) {
	// Use a fresh background ctx with a tight timeout: bus delivery is
	// synchronous so a stuck Lark HTTP call would otherwise wedge the
	// whole publish call site.
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := p.processEvent(ctx, e); err != nil {
		p.cfg.Logger.Warn("lark patcher: event handling failed",
			"event_type", e.Type,
			"task_id", e.TaskID,
			"chat_session_id", e.ChatSessionID,
			"error", err,
		)
	}
}

func (p *Patcher) processEvent(ctx context.Context, e events.Event) error {
	taskID, chatSessionID, ok := taskAndSessionFromEvent(e)
	if !ok {
		return nil
	}
	var task db.AgentTaskQueue
	taskLoaded := false
	if taskID.Valid {
		if row, err := p.queries.GetAgentTask(ctx, taskID); err == nil {
			task = row
			taskLoaded = true
		}
	}
	if !chatSessionID.Valid && taskID.Valid {
		// EventChatDone normally carries chat_session_id; the failure
		// path (EventTaskFailed) does not, so recover it from
		// agent_task_queue. Issue / autopilot tasks still come back
		// with NULL ChatSessionID and fall through to the early-return
		// below.
		if taskLoaded && task.ChatSessionID.Valid {
			chatSessionID = task.ChatSessionID
		}
	}
	if !chatSessionID.Valid {
		// Issue / autopilot tasks have no chat_session.
		return nil
	}

	binding, err := p.queries.GetLarkChatSessionBindingBySession(ctx, chatSessionID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			// Web-only chat session — not a Lark target.
			return nil
		}
		return fmt.Errorf("lookup chat session binding: %w", err)
	}

	inst, err := p.queries.GetLarkInstallation(ctx, binding.InstallationID)
	if err != nil {
		return fmt.Errorf("load installation: %w", err)
	}
	if InstallationStatus(inst.Status) != InstallationActive {
		// Revoked between trigger and event; nothing to patch.
		return nil
	}
	creds, err := p.installationCredentials(inst)
	if err != nil {
		return err
	}

	agent, agentErr := p.queries.GetAgent(ctx, inst.AgentID)
	agentName := ""
	if agentErr == nil {
		agentName = agent.Name
	}
	mirrorToIssue := !taskLoaded || !task.IssueID.Valid

	switch e.Type {
	case protocol.EventChatDone:
		var taskRef *db.AgentTaskQueue
		if taskLoaded {
			taskRef = &task
		}
		return p.sendChatReply(ctx, creds, inst, binding, taskID, taskRef, agentName, e.Payload, mirrorToIssue)
	case protocol.EventTaskFailed:
		return p.fail(ctx, creds, inst, binding, taskID, agentName, e.Payload, mirrorToIssue)
	}
	return nil
}

// maxMixedCardImages caps how many images a single mixed card embeds.
// Lark's card schema allows more, but readability collapses past a
// handful — when the agent emits more images than this we keep the
// most recent maxMixedCardImages and call out the truncation in a
// trailing note element so the user knows there were extras.
const maxMixedCardImages = 5

// sendChatReply turns ChatDonePayload.Content into a Lark message. The
// reply is always posted as a thread-reply under the user's original
// trigger message (recorded by the inbound OutcomeReplier as a thread
// anchor) so the answer lands inside a thread alongside the
// "OneSecond" receipt reaction. After the message is delivered the
// receipt reaction is retracted and the anchor cleared.
//
// Empty content is silently dropped: we'd rather show nothing than
// "Done." (the prior fallback that confused Bohan in the live env).
// In practice empty Content means the daemon completed without
// producing visible output — the reaction stays in place rather than
// posting a blank bubble.
//
// When the task has any task_message_attachment rows of kind=image,
// each one is streamed through Lark's image upload endpoint to mint
// an `image_key`, and the reply is composed as a schema-2.0 mixed
// card carrying both the markdown body (when non-empty) and the
// image elements. Per-image upload failures are warn-logged and
// skipped; if every upload fails we degrade to the markdown card
// path so the user still sees the textual answer.
// imageSlot represents the post-upload state of one image attachment:
// either Lark returned an image_key (success) or the upload failed and
// we keep a placeholder so the slot stays visible in the card. Per
// the parent issue's soft-failure contract, single-image failures
// should NOT collapse the whole card — instead we render a `note`
// element that surfaces the missing image so the user understands
// something was supposed to be there.
type imageSlot struct {
	imageKey    string
	placeholder string
}

func (s imageSlot) ok() bool { return s.imageKey != "" }

func (p *Patcher) sendChatReply(ctx context.Context, creds InstallationCredentials, inst db.LarkInstallation, binding db.LarkChatSessionBinding, taskID pgtype.UUID, task *db.AgentTaskQueue, agentName string, payload any, mirrorToIssue bool) error {
	content := p.authoritativeReplyContent(ctx, inst.WorkspaceID, task, taskID, payload)
	images := p.collectImageAttachments(ctx, taskID)
	if content == "" && len(images) == 0 {
		return nil
	}
	if sanitized, changed := stripLocalArtifactImages(content); changed {
		content = sanitized
	}
	anchor := p.replyAnchor(binding.ChatSessionID)
	if len(images) > 0 {
		slots, truncated, anyOK := p.uploadImageAttachments(ctx, creds, images)
		if anyOK {
			cardJSON, err := buildMixedCard(content, slots, truncated)
			if err != nil {
				return fmt.Errorf("build mixed card: %w", err)
			}
			if _, err := p.client.SendInteractiveCard(ctx, SendCardParams{
				InstallationID:   creds,
				ChatID:           ChatID(binding.LarkChatID),
				CardJSON:         cardJSON,
				ReplyToMessageID: anchor,
				ReplyInThread:    anchor != "",
			}); err != nil {
				return fmt.Errorf("send mixed card: %w", err)
			}
			p.retractPendingReactions(ctx, creds, binding.ChatSessionID)
			p.clearThreadAnchor(binding.ChatSessionID)
			if mirrorToIssue {
				p.mirrorAgentReplyToIssue(ctx, inst, binding, content)
			}
			return nil
		}
		// Every upload failed — fall through to text/markdown so the
		// user still gets the textual answer. If the answer is also
		// empty we drop silently rather than send a blank bubble.
		if content == "" {
			return nil
		}
	}
	if containsMarkdown(content) {
		if _, err := p.client.SendMarkdownCard(ctx, SendMarkdownCardParams{
			InstallationID:   creds,
			ChatID:           ChatID(binding.LarkChatID),
			Markdown:         content,
			ReplyToMessageID: anchor,
			ReplyInThread:    anchor != "",
		}); err != nil {
			return fmt.Errorf("send markdown card: %w", err)
		}
	} else {
		if _, err := p.client.SendTextMessage(ctx, SendTextParams{
			InstallationID:   creds,
			ChatID:           ChatID(binding.LarkChatID),
			Text:             content,
			ReplyToMessageID: anchor,
			ReplyInThread:    anchor != "",
		}); err != nil {
			return fmt.Errorf("send text message: %w", err)
		}
	}
	p.retractPendingReactions(ctx, creds, binding.ChatSessionID)
	p.clearThreadAnchor(binding.ChatSessionID)
	if mirrorToIssue {
		p.mirrorAgentReplyToIssue(ctx, inst, binding, content)
	}
	return nil
}

// stripLocalArtifactImages removes markdown image embeds that point at local
// `./artifacts/...` files. Lark markdown cards interpret image syntax as
// image_key references, so local paths cause a 400 "invalid image keys"
// response and prevent the whole reply from being delivered.
func stripLocalArtifactImages(s string) (string, bool) {
	lines := strings.Split(s, "\n")
	out := make([]string, 0, len(lines))
	changed := false
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "![") && strings.Contains(trimmed, "](./artifacts/") {
			changed = true
			continue
		}
		out = append(out, line)
	}
	if !changed {
		return s, false
	}
	return strings.TrimSpace(strings.Join(out, "\n")), true
}

// collectImageAttachments loads the task's image attachments. A nil
// reader (no storage wired) or DB error degrades to the legacy
// no-image path — chart rendering is a soft feature and a transport
// hiccup must never block the textual reply.
func (p *Patcher) collectImageAttachments(ctx context.Context, taskID pgtype.UUID) []db.TaskMessageAttachment {
	if p.storage == nil || !taskID.Valid {
		return nil
	}
	rows, err := p.queries.ListTaskMessageImageAttachments(ctx, taskID)
	if err != nil {
		p.cfg.Logger.Warn("lark patcher: list image attachments failed",
			"task_id", taskID,
			"error", err,
		)
		return nil
	}
	return rows
}

// uploadImageAttachments streams each attachment through Lark's image
// upload endpoint and returns one slot per input row plus a flag
// indicating whether the input was truncated to maxMixedCardImages.
// Per-row failures are warn-logged and turned into placeholder slots
// so the caller can render a "image failed to upload" note instead
// of dropping the slot silently. anyOK reports whether at least one
// slot resolved to a real image_key, which is the trigger for the
// mixed-card path; when every upload fails the caller falls back to
// text/markdown.
func (p *Patcher) uploadImageAttachments(ctx context.Context, creds InstallationCredentials, rows []db.TaskMessageAttachment) (slots []imageSlot, truncated, anyOK bool) {
	limit := len(rows)
	if limit > maxMixedCardImages {
		limit = maxMixedCardImages
		truncated = true
	}
	slots = make([]imageSlot, 0, limit)
	for _, row := range rows[:limit] {
		key, err := p.uploadOneImage(ctx, creds, row)
		if err != nil {
			p.cfg.Logger.Warn("lark patcher: image upload failed",
				"attachment_id", row.ID,
				"storage_key", row.StorageKey,
				"error", err,
			)
			slots = append(slots, imageSlot{placeholder: imagePlaceholderText(row)})
			continue
		}
		slots = append(slots, imageSlot{imageKey: key})
		anyOK = true
	}
	return slots, truncated, anyOK
}

// imagePlaceholderText is the user-facing fallback shown in the card
// when an image attachment failed to upload. It identifies the file
// (by storage key tail) so the user can tell which chart is missing
// without exposing the full storage path or a stack trace.
func imagePlaceholderText(row db.TaskMessageAttachment) string {
	name := path.Base(row.StorageKey)
	if name == "" || name == "." || name == "/" {
		name = "image"
	}
	return fmt.Sprintf("[图片 %s 上传失败]", name)
}

func (p *Patcher) uploadOneImage(ctx context.Context, creds InstallationCredentials, row db.TaskMessageAttachment) (string, error) {
	reader, err := p.storage.Open(ctx, row.StorageKey)
	if err != nil {
		return "", fmt.Errorf("open storage: %w", err)
	}
	defer reader.Close()
	filename := path.Base(row.StorageKey)
	if filename == "" || filename == "." || filename == "/" {
		filename = "image"
	}
	return p.client.UploadImage(ctx, creds, UploadImageParams{
		Reader:    reader,
		ImageType: "message",
		Filename:  filename,
	})
}

// buildMixedCard composes the schema-2.0 interactive card that mixes
// optional markdown text with one or more image elements. The shape
// follows the parent issue's spec:
//
//	{
//	  "schema": "2.0",
//	  "config": {"summary": {"content": "[图片回复]"}},
//	  "body":   {"elements": [<markdown?>, <img>...]}
//	}
//
// markdown is omitted entirely when the body is empty (pure-image
// reply); when truncated is true a trailing `note` element calls out
// that extras were dropped so the user is not left guessing why a
// chart they asked for is missing.
func buildMixedCard(markdown string, slots []imageSlot, truncated bool) (string, error) {
	if len(slots) == 0 {
		return "", errors.New("buildMixedCard: at least one image slot required")
	}
	anyOK := false
	for _, s := range slots {
		if s.ok() {
			anyOK = true
			break
		}
	}
	if !anyOK {
		return "", errors.New("buildMixedCard: every image slot is a placeholder")
	}
	elements := make([]any, 0, len(slots)+2)
	if strings.TrimSpace(markdown) != "" {
		elements = append(elements, map[string]any{
			"tag":     "markdown",
			"content": markdown,
		})
	}
	for _, slot := range slots {
		if slot.ok() {
			elements = append(elements, map[string]any{
				"tag":         "img",
				"img_key":     slot.imageKey,
				"alt":         map[string]any{"tag": "plain_text", "content": ""},
				"mode":        "fit_horizontal",
				"transparent": false,
			})
			continue
		}
		elements = append(elements, map[string]any{
			"tag": "note",
			"elements": []any{
				map[string]any{
					"tag":     "plain_text",
					"content": slot.placeholder,
				},
			},
		})
	}
	if truncated {
		elements = append(elements, map[string]any{
			"tag": "note",
			"elements": []any{
				map[string]any{
					"tag":     "plain_text",
					"content": fmt.Sprintf("仅展示前 %d 张图片，更多图片已省略。", maxMixedCardImages),
				},
			},
		})
	}
	summary := "[图片回复]"
	if strings.TrimSpace(markdown) != "" {
		summary = summarize(markdown)
	}
	doc := map[string]any{
		"schema": "2.0",
		"config": map[string]any{
			"summary": map[string]any{"content": summary},
		},
		"body": map[string]any{
			"elements": elements,
		},
	}
	raw, err := json.Marshal(doc)
	if err != nil {
		return "", fmt.Errorf("encode mixed card: %w", err)
	}
	return string(raw), nil
}

// summarize trims a markdown body into a single-line preview Lark can
// render in chat lists / desktop notifications. It keeps the first
// non-blank line and clamps to ~40 runes — long enough to be useful,
// short enough to fit Lark's preview chrome.
func summarize(markdown string) string {
	for _, line := range strings.Split(markdown, "\n") {
		s := strings.TrimSpace(line)
		if s == "" {
			continue
		}
		runes := []rune(s)
		if len(runes) > 40 {
			return string(runes[:40]) + "…"
		}
		return s
	}
	return "[图片回复]"
}

// retractPendingReactions deletes the receipt-style reactions the
// inbound OutcomeReplier posted on the user's messages for this
// chat_session. Called after a successful agent reply so the user no
// longer sees the "OneSecond" hourglass alongside the answer.
// Best-effort: a Lark failure here just leaves the reaction in place,
// which is visual noise but never a correctness problem.
func (p *Patcher) retractPendingReactions(ctx context.Context, creds InstallationCredentials, chatSessionID pgtype.UUID) {
	if p.tracker == nil {
		return
	}
	for _, entry := range p.tracker.Drain(chatSessionID) {
		if err := p.client.DeleteMessageReaction(ctx, DeleteReactionParams{
			InstallationID: creds,
			MessageID:      entry.MessageID,
			ReactionID:     entry.ReactionID,
		}); err != nil {
			p.cfg.Logger.Warn("lark patcher: delete reaction failed",
				"message_id", entry.MessageID,
				"reaction_id", entry.ReactionID,
				"error", err,
			)
		}
	}
}

func (p *Patcher) installationCredentials(inst db.LarkInstallation) (InstallationCredentials, error) {
	if p.credentials == nil {
		return InstallationCredentials{}, errors.New("lark patcher: credentials resolver missing")
	}
	secret, err := p.credentials.DecryptAppSecret(inst)
	if err != nil {
		return InstallationCredentials{}, fmt.Errorf("decrypt app_secret: %w", err)
	}
	creds := InstallationCredentials{
		AppID:     inst.AppID,
		AppSecret: secret,
		Region:    RegionOrDefault(inst.Region),
	}
	if inst.TenantKey.Valid {
		creds.TenantKey = inst.TenantKey.String
	}
	return creds, nil
}

// fail surfaces a short error card on task failure. Unlike the
// success path (plain text via sendChatReply), failures stay as cards
// because the user benefits from the visual distinction — a red /
// header-styled card is much harder to miss than a regular bubble,
// and these are rare enough that the card chrome isn't noisy.
//
// One-shot send (no patching, no DB row): if the task fails a second
// time we'd just send a second card, which is fine — failure is
// usually a single terminal event.
func (p *Patcher) fail(ctx context.Context, creds InstallationCredentials, inst db.LarkInstallation, binding db.LarkChatSessionBinding, taskID pgtype.UUID, agentName string, payload any, mirrorToIssue bool) error {
	render, err := p.cfg.Renderer.Render(RenderInput{
		Kind:         CardKindError,
		AgentName:    agentName,
		TaskID:       taskID,
		ErrorMessage: errorMessageFromPayload(payload),
	})
	if err != nil {
		return fmt.Errorf("render error card: %w", err)
	}
	anchor := p.replyAnchor(binding.ChatSessionID)
	if _, err := p.client.SendInteractiveCard(ctx, SendCardParams{
		InstallationID:   creds,
		ChatID:           ChatID(binding.LarkChatID),
		CardJSON:         render.JSON,
		ReplyToMessageID: anchor,
		ReplyInThread:    anchor != "",
	}); err != nil {
		return fmt.Errorf("send error card: %w", err)
	}
	p.clearThreadAnchor(binding.ChatSessionID)
	if msg := errorMessageFromPayload(payload); mirrorToIssue && msg != "" {
		p.mirrorAgentReplyToIssue(ctx, inst, binding, "Run failed: "+msg)
	}
	return nil
}

// mirrorAgentReplyToIssue posts the agent's reply (text-only) as a
// comment on the auto-created Issue tied to this Lark thread. Best
// effort: errors are warn-logged, never propagated. Skipped when the
// binding has no associated issue or when there's no textual content
// (image-only replies are not mirrored — the chart attachment lives on
// the Lark side and would surface as a noisy "(no content)" comment).
func (p *Patcher) mirrorAgentReplyToIssue(ctx context.Context, inst db.LarkInstallation, binding db.LarkChatSessionBinding, content string) {
	if !binding.IssueID.Valid {
		return
	}
	if strings.TrimSpace(content) == "" {
		return
	}
	if _, err := p.queries.CreateComment(ctx, db.CreateCommentParams{
		IssueID:     binding.IssueID,
		WorkspaceID: inst.WorkspaceID,
		AuthorType:  "agent",
		AuthorID:    inst.AgentID,
		Content:     content,
		Type:        "comment",
	}); err != nil {
		p.cfg.Logger.Warn("lark patcher: mirror agent reply to issue failed",
			"issue_id", binding.IssueID,
			"error", err,
		)
	}
}

// replyAnchor reads the trigger message_id stashed by the inbound
// OutcomeReplier for this chat_session, if any. A non-empty result
// tells the transport layer to route this outbound through Lark's
// reply endpoint with reply_in_thread=true so the bot's reply lands
// inside a thread under the user's prompt — preventing interleaved
// replies in busy group chats and grouping the answer with the inbound
// receipt reaction. p2p chats and any session without a recorded anchor
// fall back to a normal send.
func (p *Patcher) replyAnchor(chatSessionID pgtype.UUID) string {
	if p.threadAnchors == nil {
		return ""
	}
	return p.threadAnchors.Get(chatSessionID)
}

func (p *Patcher) clearThreadAnchor(chatSessionID pgtype.UUID) {
	if p.threadAnchors == nil {
		return
	}
	p.threadAnchors.Clear(chatSessionID)
}

// taskAndSessionFromEvent parses the typed-ish payload broadcastTaskEvent
// publishes — a map[string]any with `task_id` (always) and
// `chat_session_id` (chat tasks only). EventChatDone carries a
// ChatDonePayload struct instead.
func taskAndSessionFromEvent(e events.Event) (taskID, chatSessionID pgtype.UUID, ok bool) {
	if e.TaskID != "" {
		if err := taskID.Scan(e.TaskID); err != nil {
			taskID = pgtype.UUID{}
		}
	}
	if e.ChatSessionID != "" {
		if err := chatSessionID.Scan(e.ChatSessionID); err != nil {
			chatSessionID = pgtype.UUID{}
		}
	}
	switch p := e.Payload.(type) {
	case map[string]any:
		if !taskID.Valid {
			if s, _ := p["task_id"].(string); s != "" {
				_ = taskID.Scan(s)
			}
		}
		if !chatSessionID.Valid {
			if s, _ := p["chat_session_id"].(string); s != "" {
				_ = chatSessionID.Scan(s)
			}
		}
	case protocol.ChatDonePayload:
		if !taskID.Valid {
			_ = taskID.Scan(p.TaskID)
		}
		if !chatSessionID.Valid {
			_ = chatSessionID.Scan(p.ChatSessionID)
		}
	}
	return taskID, chatSessionID, taskID.Valid
}

func chatDoneContent(payload any) string {
	switch p := payload.(type) {
	case protocol.ChatDonePayload:
		return p.Content
	case map[string]any:
		if s, ok := p["content"].(string); ok {
			return s
		}
	}
	return ""
}

// finalAnswer derives the user-visible final answer by reading the task's
// per-message stream and taking only the text emitted after the last
// non-text event (tool_use / tool_result / thinking / error). This mirrors
// the desktop chat UI's "preface + final" split: the conductor-style fold
// hides everything sandwiched between the first and last non-text item, and
// only what comes after the last non-text item is rendered as the answer.
//
// Falls back to ChatDonePayload.Content when the task_message rows are
// missing or contain no text-after-tool segment (e.g. a pure-text run with
// no tool calls — the whole content IS the answer).
func (p *Patcher) finalAnswer(ctx context.Context, taskID pgtype.UUID, payload any) string {
	full := chatDoneContent(payload)
	if !taskID.Valid {
		return full
	}
	rows, err := p.queries.ListTaskMessages(ctx, taskID)
	if err != nil || len(rows) == 0 {
		return full
	}
	lastNonText := -1
	for i, r := range rows {
		if r.Type != "text" {
			lastNonText = i
		}
	}
	if lastNonText < 0 {
		// Pure-text run; full content is the answer.
		return full
	}
	var b strings.Builder
	for _, r := range rows[lastNonText+1:] {
		if r.Type != "text" || !r.Content.Valid {
			continue
		}
		s := r.Content.String
		if s == "" {
			continue
		}
		if b.Len() > 0 {
			b.WriteString("\n\n")
		}
		b.WriteString(s)
	}
	if b.Len() == 0 {
		return full
	}
	return b.String()
}

// authoritativeReplyContent prefers the Issue comment written by the task
// completion path for issue-backed chat tasks. That comment is the user-visible
// source of truth in Multica; using it here keeps the Lark-side reply aligned
// with the Issue thread instead of collapsing to the shorter task-message tail.
func (p *Patcher) authoritativeReplyContent(ctx context.Context, workspaceID pgtype.UUID, task *db.AgentTaskQueue, taskID pgtype.UUID, payload any) string {
	if s := p.issueReplyContent(ctx, workspaceID, task); s != "" {
		return s
	}
	return p.finalAnswer(ctx, taskID, payload)
}

func (p *Patcher) issueReplyContent(ctx context.Context, workspaceID pgtype.UUID, task *db.AgentTaskQueue) string {
	if task == nil || !task.IssueID.Valid || !task.StartedAt.Valid {
		return ""
	}
	rows, err := p.queries.ListCommentsSinceForIssue(ctx, db.ListCommentsSinceForIssueParams{
		IssueID:     task.IssueID,
		WorkspaceID: workspaceID,
		CreatedAt:   task.StartedAt,
		Limit:       200,
	})
	if err != nil || len(rows) == 0 {
		return ""
	}
	for i := len(rows) - 1; i >= 0; i-- {
		row := rows[i]
		if row.AuthorType != "agent" || row.AuthorID != task.AgentID {
			continue
		}
		if strings.TrimSpace(row.Content) == "" {
			continue
		}
		return row.Content
	}
	return ""
}

func errorMessageFromPayload(payload any) string {
	if m, ok := payload.(map[string]any); ok {
		if s, ok := m["error"].(string); ok {
			return s
		}
		if s, ok := m["error_message"].(string); ok {
			return s
		}
	}
	return ""
}
