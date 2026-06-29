package lark

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/events"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

type fakePatcherQueries struct {
	mu               sync.Mutex
	binding          db.LarkChatSessionBinding
	bindingErr       error
	installation     db.LarkInstallation
	installationErr  error
	agent            db.Agent
	agentErr         error
	task             db.AgentTaskQueue
	taskErr          error
	card             db.LarkOutboundCardMessage
	cardErr          error
	created          []db.CreateLarkOutboundCardMessageParams
	createReturn     db.LarkOutboundCardMessage
	statusUpdates    []db.UpdateLarkOutboundCardStatusParams
	commentsSince    []db.Comment
	commentsSinceErr error
	taskMessages     []db.TaskMessage
	imageAttachments []db.TaskMessageAttachment
	imageAttachErr   error
	commentsCreated  []db.CreateCommentParams
	createCommentErr error
}

func (f *fakePatcherQueries) GetAgentTask(ctx context.Context, id pgtype.UUID) (db.AgentTaskQueue, error) {
	return f.task, f.taskErr
}
func (f *fakePatcherQueries) GetChatSession(ctx context.Context, id pgtype.UUID) (db.ChatSession, error) {
	return db.ChatSession{}, nil
}
func (f *fakePatcherQueries) GetAgent(ctx context.Context, id pgtype.UUID) (db.Agent, error) {
	return f.agent, f.agentErr
}
func (f *fakePatcherQueries) GetLarkInstallation(ctx context.Context, id pgtype.UUID) (db.LarkInstallation, error) {
	return f.installation, f.installationErr
}
func (f *fakePatcherQueries) GetLarkChatSessionBindingBySession(ctx context.Context, sessID pgtype.UUID) (db.LarkChatSessionBinding, error) {
	return f.binding, f.bindingErr
}
func (f *fakePatcherQueries) GetLarkOutboundCardByTask(ctx context.Context, taskID pgtype.UUID) (db.LarkOutboundCardMessage, error) {
	return f.card, f.cardErr
}
func (f *fakePatcherQueries) CreateLarkOutboundCardMessage(ctx context.Context, arg db.CreateLarkOutboundCardMessageParams) (db.LarkOutboundCardMessage, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.created = append(f.created, arg)
	return f.createReturn, nil
}
func (f *fakePatcherQueries) UpdateLarkOutboundCardStatus(ctx context.Context, arg db.UpdateLarkOutboundCardStatusParams) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.statusUpdates = append(f.statusUpdates, arg)
	return nil
}
func (f *fakePatcherQueries) ListCommentsSinceForIssue(ctx context.Context, arg db.ListCommentsSinceForIssueParams) ([]db.Comment, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.commentsSince, f.commentsSinceErr
}
func (f *fakePatcherQueries) ListTaskMessages(ctx context.Context, taskID pgtype.UUID) ([]db.TaskMessage, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.taskMessages, nil
}
func (f *fakePatcherQueries) ListTaskMessageImageAttachments(ctx context.Context, taskID pgtype.UUID) ([]db.TaskMessageAttachment, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.imageAttachments, f.imageAttachErr
}
func (f *fakePatcherQueries) CreateComment(ctx context.Context, arg db.CreateCommentParams) (db.Comment, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.commentsCreated = append(f.commentsCreated, arg)
	if f.createCommentErr != nil {
		return db.Comment{}, f.createCommentErr
	}
	return db.Comment{
		IssueID:     arg.IssueID,
		AuthorType:  arg.AuthorType,
		AuthorID:    arg.AuthorID,
		Content:     arg.Content,
		Type:        arg.Type,
		WorkspaceID: arg.WorkspaceID,
	}, nil
}

type fakeCredentials struct{ secret string }

func (f fakeCredentials) DecryptAppSecret(inst db.LarkInstallation) (string, error) {
	return f.secret, nil
}

type fakeAPIClient struct {
	mu             sync.Mutex
	sent           []SendCardParams
	patched        []PatchCardParams
	textSent       []SendTextParams
	mdCardSent     []SendMarkdownCardParams
	sendReturn     string
	sendErr        error
	patchErr       error
	textSendErr    error
	textSendReturn string
	mdCardErr      error
	mdCardReturn   string
	bindingSent    []BindingPromptParams
	uploadCalls    []UploadImageParams
	uploadBodies   [][]byte
	uploadKeys     []string
	uploadErrs     []error
	uploadIdx      int
}

func (f *fakeAPIClient) IsConfigured() bool { return true }

func (f *fakeAPIClient) SendInteractiveCard(ctx context.Context, p SendCardParams) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.sent = append(f.sent, p)
	return f.sendReturn, f.sendErr
}
func (f *fakeAPIClient) PatchInteractiveCard(ctx context.Context, p PatchCardParams) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.patched = append(f.patched, p)
	return f.patchErr
}
func (f *fakeAPIClient) SendTextMessage(ctx context.Context, p SendTextParams) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.textSent = append(f.textSent, p)
	return f.textSendReturn, f.textSendErr
}
func (f *fakeAPIClient) SendMarkdownCard(ctx context.Context, p SendMarkdownCardParams) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.mdCardSent = append(f.mdCardSent, p)
	return f.mdCardReturn, f.mdCardErr
}
func (f *fakeAPIClient) SendBindingPromptCard(ctx context.Context, p BindingPromptParams) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.bindingSent = append(f.bindingSent, p)
	return nil
}
func (f *fakeAPIClient) SendMessageReaction(ctx context.Context, p SendReactionParams) (string, error) {
	return "", nil
}
func (f *fakeAPIClient) DeleteMessageReaction(ctx context.Context, p DeleteReactionParams) error {
	return nil
}
func (f *fakeAPIClient) GetBotInfo(ctx context.Context, creds InstallationCredentials) (BotInfo, error) {
	return BotInfo{}, nil
}
func (f *fakeAPIClient) GetMessage(ctx context.Context, creds InstallationCredentials, messageID string) ([]LarkMessage, error) {
	return nil, nil
}
func (f *fakeAPIClient) ListChatMessages(ctx context.Context, creds InstallationCredentials, p ListMessagesParams) ([]LarkMessage, error) {
	return nil, nil
}
func (f *fakeAPIClient) BatchGetUsers(ctx context.Context, creds InstallationCredentials, openIDs []string) (map[string]string, error) {
	return nil, nil
}
func (f *fakeAPIClient) UploadImage(ctx context.Context, creds InstallationCredentials, p UploadImageParams) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	body, _ := io.ReadAll(p.Reader)
	f.uploadBodies = append(f.uploadBodies, body)
	f.uploadCalls = append(f.uploadCalls, p)
	idx := f.uploadIdx
	f.uploadIdx++
	var key string
	var err error
	if idx < len(f.uploadKeys) {
		key = f.uploadKeys[idx]
	}
	if idx < len(f.uploadErrs) {
		err = f.uploadErrs[idx]
	}
	return key, err
}

func newTestPatcher(t *testing.T) (*Patcher, *fakePatcherQueries, *fakeAPIClient) {
	t.Helper()
	q := &fakePatcherQueries{
		binding: db.LarkChatSessionBinding{
			ChatSessionID:  uuidFromString(t, "cccccccc-cccc-cccc-cccc-cccccccccccc"),
			InstallationID: uuidFromString(t, "1111aaaa-1111-1111-1111-111111111111"),
			LarkChatID:     "oc_test_chat",
			LarkChatType:   "p2p",
		},
		installation: db.LarkInstallation{
			ID:                 uuidFromString(t, "1111aaaa-1111-1111-1111-111111111111"),
			AppID:              "cli_test_app",
			AppSecretEncrypted: []byte("ciphertext"),
			Status:             string(InstallationActive),
			AgentID:            uuidFromString(t, "aaaa1111-aaaa-aaaa-aaaa-aaaaaaaaaaaa"),
		},
		task: db.AgentTaskQueue{
			ID:            uuidFromString(t, "ee000000-ee00-ee00-ee00-eeeeeeeeeeee"),
			ChatSessionID: uuidFromString(t, "cccccccc-cccc-cccc-cccc-cccccccccccc"),
		},
		agent:   db.Agent{Name: "TestAgent"},
		cardErr: pgx.ErrNoRows,
	}
	api := &fakeAPIClient{sendReturn: "lark_card_msg_1", textSendReturn: "lark_text_msg_1"}
	p := NewPatcher(q, fakeCredentials{secret: "shh"}, api, PatcherConfig{
		Logger: newDiscardLogger(),
		Now:    time.Now,
	})
	return p, q, api
}

func pgText(s string) pgtype.Text {
	return pgtype.Text{String: s, Valid: s != ""}
}

func timestamptzFromTime(t *testing.T, ts time.Time) pgtype.Timestamptz {
	t.Helper()
	return pgtype.Timestamptz{Time: ts, Valid: true}
}

// TestPatcherSendsPlainTextOnChatDone pins the new behaviour Bohan asked
// for: when the agent finishes replying, the Patcher posts the reply as
// a plain Lark IM text message (msg_type=text), not nested inside an
// interactive card. This is the load-bearing UX call — the prior card
// chrome made every reply look like a system notification.
func TestPatcherSendsPlainTextOnChatDone(t *testing.T) {
	p, q, api := newTestPatcher(t)
	taskID := uuidFromString(t, "ee333333-ee33-ee33-ee33-eeeeeeeeeeee")

	p.handleEvent(events.Event{
		Type:          protocol.EventChatDone,
		TaskID:        uuidString(taskID),
		ChatSessionID: uuidString(q.binding.ChatSessionID),
		Payload: protocol.ChatDonePayload{
			TaskID:        uuidString(taskID),
			ChatSessionID: uuidString(q.binding.ChatSessionID),
			Content:       "Hello! I'm cc, a coding agent…",
		},
	})

	api.mu.Lock()
	defer api.mu.Unlock()
	if len(api.textSent) != 1 {
		t.Fatalf("expected one SendTextMessage call on ChatDone; got %d", len(api.textSent))
	}
	got := api.textSent[0]
	if got.Text != "Hello! I'm cc, a coding agent…" {
		t.Errorf("text mismatch: got %q", got.Text)
	}
	if got.ChatID != ChatID(q.binding.LarkChatID) {
		t.Errorf("chat_id mismatch: got %q want %q", got.ChatID, q.binding.LarkChatID)
	}
	if got.InstallationID.AppID != "cli_test_app" {
		t.Errorf("expected installation app_id propagated; got %q", got.InstallationID.AppID)
	}
	if len(api.sent) != 0 || len(api.patched) != 0 {
		t.Errorf("ChatDone must NOT send / patch any card; got sent=%d patched=%d",
			len(api.sent), len(api.patched))
	}
}

func TestPatcherMirrorsAgentReplyToAutoIssue(t *testing.T) {
	p, q, _ := newTestPatcher(t)
	taskID := uuidFromString(t, "ee303030-ee30-ee30-ee30-eeeeeeeeeeee")
	issueID := uuidFromString(t, "99999999-9999-9999-9999-999999999999")
	workspaceID := uuidFromString(t, "22222222-2222-2222-2222-222222222222")
	q.binding.IssueID = issueID
	q.installation.WorkspaceID = workspaceID

	p.handleEvent(events.Event{
		Type:          protocol.EventChatDone,
		TaskID:        uuidString(taskID),
		ChatSessionID: uuidString(q.binding.ChatSessionID),
		Payload: protocol.ChatDonePayload{
			TaskID:        uuidString(taskID),
			ChatSessionID: uuidString(q.binding.ChatSessionID),
			Content:       "Agent answer",
		},
	})

	q.mu.Lock()
	defer q.mu.Unlock()
	if len(q.commentsCreated) != 1 {
		t.Fatalf("expected one mirrored agent comment, got %+v", q.commentsCreated)
	}
	got := q.commentsCreated[0]
	if got.IssueID != issueID || got.WorkspaceID != workspaceID || got.AuthorType != "agent" || got.AuthorID != q.installation.AgentID || got.Content != "Agent answer" {
		t.Fatalf("mirrored agent comment mismatch: %+v", got)
	}
}

func TestPatcherPrefersIssueCommentForIssueBackedChatReply(t *testing.T) {
	p, q, api := newTestPatcher(t)
	taskID := uuidFromString(t, "ee313131-ee31-ee31-ee31-eeeeeeeeeeee")
	issueID := uuidFromString(t, "98989898-9898-9898-9898-989898989898")
	workspaceID := uuidFromString(t, "23232323-2323-2323-2323-232323232323")
	startedAt := timestamptzFromTime(t, time.Date(2026, 6, 11, 9, 0, 0, 0, time.UTC))

	q.installation.WorkspaceID = workspaceID
	q.task = db.AgentTaskQueue{
		ID:            taskID,
		IssueID:       issueID,
		AgentID:       q.installation.AgentID,
		ChatSessionID: q.binding.ChatSessionID,
		StartedAt:     startedAt,
	}
	q.commentsSince = []db.Comment{
		{
			IssueID:    issueID,
			AuthorType: "agent",
			AuthorID:   q.installation.AgentID,
			Content:    "已在 M-62 里回复了 BOE 操作清单，按指标/维度表达式、TCC DATASET_CONFIG、指标单元/DAG、报告视频章节和下一步验证拆开整理。",
		},
	}
	q.taskMessages = []db.TaskMessage{
		{Type: "tool_result"},
		{Type: "text", Content: pgText("当前结论是：指标/维度和指标单元已同步并通过 review；还需要确认并发布 3361039 的 TCC 配置。")},
	}

	p.handleEvent(events.Event{
		Type:          protocol.EventChatDone,
		TaskID:        uuidString(taskID),
		ChatSessionID: uuidString(q.binding.ChatSessionID),
		Payload: protocol.ChatDonePayload{
			TaskID:        uuidString(taskID),
			ChatSessionID: uuidString(q.binding.ChatSessionID),
			Content:       "fallback payload",
		},
	})

	api.mu.Lock()
	defer api.mu.Unlock()
	if len(api.textSent) != 1 {
		t.Fatalf("expected one SendTextMessage call on ChatDone; got %d", len(api.textSent))
	}
	if got := api.textSent[0].Text; got != q.commentsSince[0].Content {
		t.Fatalf("issue-backed chat reply must prefer mirrored issue comment; got %q want %q", got, q.commentsSince[0].Content)
	}
}

func TestPatcherFallsBackWhenIssueCommentUnavailable(t *testing.T) {
	p, q, api := newTestPatcher(t)
	taskID := uuidFromString(t, "ee323232-ee32-ee32-ee32-eeeeeeeeeeee")
	issueID := uuidFromString(t, "97979797-9797-9797-9797-979797979797")
	workspaceID := uuidFromString(t, "24242424-2424-2424-2424-242424242424")
	startedAt := timestamptzFromTime(t, time.Date(2026, 6, 11, 9, 0, 0, 0, time.UTC))

	q.installation.WorkspaceID = workspaceID
	q.task = db.AgentTaskQueue{
		ID:            taskID,
		IssueID:       issueID,
		AgentID:       q.installation.AgentID,
		ChatSessionID: q.binding.ChatSessionID,
		StartedAt:     startedAt,
	}
	q.taskMessages = []db.TaskMessage{
		{Type: "tool_result"},
		{Type: "text", Content: pgText("tail answer")},
	}

	p.handleEvent(events.Event{
		Type:          protocol.EventChatDone,
		TaskID:        uuidString(taskID),
		ChatSessionID: uuidString(q.binding.ChatSessionID),
		Payload: protocol.ChatDonePayload{
			TaskID:        uuidString(taskID),
			ChatSessionID: uuidString(q.binding.ChatSessionID),
			Content:       "fallback payload",
		},
	})

	api.mu.Lock()
	defer api.mu.Unlock()
	if len(api.textSent) != 1 {
		t.Fatalf("expected one SendTextMessage call on ChatDone; got %d", len(api.textSent))
	}
	if got := api.textSent[0].Text; got != "tail answer" {
		t.Fatalf("when no issue comment is found, reply should fall back to finalAnswer; got %q", got)
	}
}

// TestPatcherRoutesMarkdownReplyToCard pins the two-path chat reply:
// when the agent's body contains markdown syntax, the Patcher MUST
// route to SendMarkdownCard (schema-2.0 interactive card with a
// `tag: "markdown"` body element) so Lark renders the formatting
// instead of leaving raw `**bold**` / `# heading` characters in the
// transcript. Plain prose continues to go through SendTextMessage.
func TestPatcherRoutesMarkdownReplyToCard(t *testing.T) {
	p, q, api := newTestPatcher(t)
	taskID := uuidFromString(t, "ee444444-ee44-ee44-ee44-eeeeeeeeeeee")

	body := "# Summary\n\n- bullet one\n- bullet two\n\n```go\nfunc f() {}\n```\n"
	p.handleEvent(events.Event{
		Type:          protocol.EventChatDone,
		TaskID:        uuidString(taskID),
		ChatSessionID: uuidString(q.binding.ChatSessionID),
		Payload: protocol.ChatDonePayload{
			TaskID:        uuidString(taskID),
			ChatSessionID: uuidString(q.binding.ChatSessionID),
			Content:       body,
		},
	})

	api.mu.Lock()
	defer api.mu.Unlock()
	if len(api.mdCardSent) != 1 {
		t.Fatalf("expected one SendMarkdownCard call; got %d", len(api.mdCardSent))
	}
	got := api.mdCardSent[0]
	if got.Markdown != body {
		t.Errorf("markdown body must be forwarded verbatim; got %q", got.Markdown)
	}
	if got.ChatID != ChatID(q.binding.LarkChatID) {
		t.Errorf("chat_id mismatch: got %q want %q", got.ChatID, q.binding.LarkChatID)
	}
	if len(api.textSent) != 0 {
		t.Errorf("markdown body must NOT also fire SendTextMessage; got %d", len(api.textSent))
	}
	if len(api.sent) != 0 || len(api.patched) != 0 {
		t.Errorf("ChatDone must NOT use legacy card paths; sent=%d patched=%d", len(api.sent), len(api.patched))
	}
}

func TestPatcherMarkdownReplyStripsLocalArtifactImages(t *testing.T) {
	p, q, api := newTestPatcher(t)
	taskID := uuidFromString(t, "ee444444-ee44-ee44-ee44-eeeeeeeeeeef")

	body := "已查询完成。\n\n![图表](./artifacts/vn-tts-gmv-20260501-20260606.png)\n\n| 日期 | 数值 |\n|---|---:|\n| 2026-06-06 | 56532821.97 |"
	p.handleEvent(events.Event{
		Type:          protocol.EventChatDone,
		TaskID:        uuidString(taskID),
		ChatSessionID: uuidString(q.binding.ChatSessionID),
		Payload: protocol.ChatDonePayload{
			TaskID:        uuidString(taskID),
			ChatSessionID: uuidString(q.binding.ChatSessionID),
			Content:       body,
		},
	})

	api.mu.Lock()
	defer api.mu.Unlock()
	if len(api.mdCardSent) != 1 {
		t.Fatalf("expected one SendMarkdownCard call; got %d", len(api.mdCardSent))
	}
	if strings.Contains(api.mdCardSent[0].Markdown, "./artifacts/") {
		t.Fatalf("local artifact image markdown must be stripped before send: %q", api.mdCardSent[0].Markdown)
	}
	if !strings.Contains(api.mdCardSent[0].Markdown, "| 日期 | 数值 |") {
		t.Fatalf("table content should remain after stripping local image markdown: %q", api.mdCardSent[0].Markdown)
	}
}

// TestPatcherRoutesPlainReplyToText is the inverse: a short prose
// reply without any markdown syntax should stay on the cheap
// msg_type=text path so the user sees a normal IM bubble.
func TestPatcherRoutesPlainReplyToText(t *testing.T) {
	p, q, api := newTestPatcher(t)
	taskID := uuidFromString(t, "ee555555-ee55-ee55-ee55-eeeeeeeeeeee")

	p.handleEvent(events.Event{
		Type:          protocol.EventChatDone,
		TaskID:        uuidString(taskID),
		ChatSessionID: uuidString(q.binding.ChatSessionID),
		Payload: protocol.ChatDonePayload{
			TaskID:        uuidString(taskID),
			ChatSessionID: uuidString(q.binding.ChatSessionID),
			Content:       "Sure, on it.",
		},
	})

	api.mu.Lock()
	defer api.mu.Unlock()
	if len(api.textSent) != 1 {
		t.Fatalf("plain prose must take the text path; got %d text sends", len(api.textSent))
	}
	if len(api.mdCardSent) != 0 {
		t.Errorf("plain prose must NOT wrap in a markdown card; got %d card sends", len(api.mdCardSent))
	}
}

// TestPatcherDropsEmptyChatReply guards the fallback we deliberately
// removed: the previous design rendered "Done." when content was
// empty. Now an empty Content is silently dropped (no text message
// sent at all). Showing nothing is better than showing the misleading
// "Done." fallback, which Bohan reported confused him in the live env.
func TestPatcherDropsEmptyChatReply(t *testing.T) {
	p, q, api := newTestPatcher(t)
	taskID := uuidFromString(t, "ee777777-ee77-ee77-ee77-eeeeeeeeeeee")

	p.handleEvent(events.Event{
		Type:          protocol.EventChatDone,
		TaskID:        uuidString(taskID),
		ChatSessionID: uuidString(q.binding.ChatSessionID),
		Payload: protocol.ChatDonePayload{
			TaskID:        uuidString(taskID),
			ChatSessionID: uuidString(q.binding.ChatSessionID),
			Content:       "",
		},
	})

	api.mu.Lock()
	defer api.mu.Unlock()
	if len(api.textSent) != 0 {
		t.Errorf("empty content must drop, not render the Done. fallback; got %d text sends", len(api.textSent))
	}
}

func TestPatcherSkipsWhenNoChatSessionBinding(t *testing.T) {
	p, q, api := newTestPatcher(t)
	q.bindingErr = pgx.ErrNoRows

	p.handleEvent(events.Event{
		Type:          protocol.EventChatDone,
		TaskID:        uuidString(uuidFromString(t, "ee222222-ee22-ee22-ee22-eeeeeeeeeeee")),
		ChatSessionID: uuidString(uuidFromString(t, "cc222222-cc22-cc22-cc22-cccccccccccc")),
		Payload: protocol.ChatDonePayload{
			Content: "irrelevant — no binding",
		},
	})

	api.mu.Lock()
	defer api.mu.Unlock()
	if len(api.textSent) != 0 || len(api.sent) != 0 {
		t.Fatalf("web-only chat sessions must produce no outbound; got text=%d cards=%d",
			len(api.textSent), len(api.sent))
	}
}

// TestPatcherFailEventSendsErrorCard verifies the failure path still
// surfaces a card. The visual distinction between a successful reply
// (plain text bubble) and a failure (red header card) is genuinely
// useful — and failures are rare enough that the card chrome isn't
// noisy. One-shot send (no patching of any prior thinking card,
// because there isn't one anymore).
func TestPatcherFailEventSendsErrorCard(t *testing.T) {
	p, q, api := newTestPatcher(t)
	taskID := uuidFromString(t, "ee444444-ee44-ee44-ee44-eeeeeeeeeeee")

	p.handleEvent(events.Event{
		Type:          protocol.EventTaskFailed,
		TaskID:        uuidString(taskID),
		ChatSessionID: uuidString(q.binding.ChatSessionID),
		Payload: map[string]any{
			"task_id":         uuidString(taskID),
			"chat_session_id": uuidString(q.binding.ChatSessionID),
			"error":           "boom",
		},
	})

	api.mu.Lock()
	defer api.mu.Unlock()
	if len(api.sent) != 1 {
		t.Fatalf("fail event must send an error card; got %d card sends", len(api.sent))
	}
	if len(api.patched) != 0 {
		t.Errorf("fail event must NOT patch any card (no prior card lifecycle); got %d patches", len(api.patched))
	}
	if !strings.Contains(api.sent[0].CardJSON, "boom") {
		t.Errorf("error card body should embed the error message; got %s", api.sent[0].CardJSON)
	}
}

func TestPatcherSwallowsInstallationLoadErrors(t *testing.T) {
	p, q, api := newTestPatcher(t)
	q.installationErr = errors.New("db down")

	p.handleEvent(events.Event{
		Type:          protocol.EventChatDone,
		TaskID:        uuidString(uuidFromString(t, "ee555555-ee55-ee55-ee55-eeeeeeeeeeee")),
		ChatSessionID: uuidString(q.binding.ChatSessionID),
		Payload: protocol.ChatDonePayload{
			Content: "would-be reply",
		},
	})

	// The patcher logs but never panics; no outbound.
	api.mu.Lock()
	defer api.mu.Unlock()
	if len(api.textSent) != 0 || len(api.sent) != 0 {
		t.Fatalf("DB failure must not produce outbound; got text=%d cards=%d",
			len(api.textSent), len(api.sent))
	}
}

// TestPatcherIgnoresEventTaskCompletedForChatTasks pins the no-extra-send
// invariant. TaskService publishes ChatDone (with content) immediately
// before TaskCompleted (without content) for every chat task. The
// Patcher must NOT react to TaskCompleted — doing so would either
// re-send the same text reply (duplicate bubble) or send the "Done."
// fallback (the original bug Bohan reported). The fix is to leave
// EventTaskCompleted unsubscribed; this test asserts exactly one
// outbound text message from the sequence.
func TestPatcherIgnoresEventTaskCompletedForChatTasks(t *testing.T) {
	p, q, api := newTestPatcher(t)
	taskID := uuidFromString(t, "ee666666-ee66-ee66-ee66-eeeeeeeeeeee")

	// Step 1: ChatDone arrives with the real agent reply. Plain text
	// is sent to Lark.
	p.handleEvent(events.Event{
		Type:          protocol.EventChatDone,
		TaskID:        uuidString(taskID),
		ChatSessionID: uuidString(q.binding.ChatSessionID),
		Payload: protocol.ChatDonePayload{
			TaskID:        uuidString(taskID),
			ChatSessionID: uuidString(q.binding.ChatSessionID),
			Content:       "Hello! I'm cc, a coding agent…",
		},
	})

	// Step 2: TaskCompleted fires immediately after with no content.
	// The Patcher MUST NOT send a second message — neither a
	// duplicate of the reply nor the "Done." fallback.
	p.handleEvent(events.Event{
		Type:          protocol.EventTaskCompleted,
		TaskID:        uuidString(taskID),
		ChatSessionID: uuidString(q.binding.ChatSessionID),
		Payload: map[string]any{
			"task_id":         uuidString(taskID),
			"chat_session_id": uuidString(q.binding.ChatSessionID),
			"status":          "completed",
		},
	})

	api.mu.Lock()
	defer api.mu.Unlock()
	if len(api.textSent) != 1 {
		t.Fatalf("exactly one text send expected (ChatDone); EventTaskCompleted must be ignored. Got %d sends", len(api.textSent))
	}
	if api.textSent[0].Text != "Hello! I'm cc, a coding agent…" {
		t.Errorf("text content mismatch; got %q", api.textSent[0].Text)
	}
	if len(api.sent) != 0 || len(api.patched) != 0 {
		t.Errorf("no card outbound expected on the success path; got sent=%d patched=%d",
			len(api.sent), len(api.patched))
	}
}

// TestDefaultRendererConfigCarriesUpdateMulti pins the streaming-card
// contract: Lark refuses PatchInteractiveCard on a card whose config
// does not declare update_multi=true. Since the Patcher's whole
// raison d'être is to send a thinking card and then patch it forward
// to streaming/final/error, ANY kind missing update_multi would make
// the patch silently no-op against Lark while the local DB row still
// flips. Hence the assertion covers every kind, not just the final
// patched kinds.
func TestDefaultRendererConfigCarriesUpdateMulti(t *testing.T) {
	r := NewDefaultRenderer()
	for _, kind := range []CardKind{CardKindThinking, CardKindRunning, CardKindFinal, CardKindError} {
		t.Run(string(kind), func(t *testing.T) {
			out, err := r.Render(RenderInput{Kind: kind, Content: "x", ErrorMessage: "y"})
			if err != nil {
				t.Fatalf("render: %v", err)
			}
			var doc map[string]any
			if err := json.Unmarshal([]byte(out.JSON), &doc); err != nil {
				t.Fatalf("decode card json: %v", err)
			}
			cfg, ok := doc["config"].(map[string]any)
			if !ok {
				t.Fatalf("missing config block: %v", doc)
			}
			if v, _ := cfg["update_multi"].(bool); !v {
				t.Errorf("config.update_multi must be true so subsequent patches apply; got %v", cfg)
			}
			if v, _ := cfg["wide_screen_mode"].(bool); !v {
				t.Errorf("config.wide_screen_mode regression: %v", cfg)
			}
		})
	}
}

// fakeAttachmentReader is an in-memory AttachmentReader: storage_key →
// raw bytes. Missing keys return ErrNotFound to simulate a stale row.
type fakeAttachmentReader struct {
	files map[string][]byte
	err   error
}

func (f *fakeAttachmentReader) Open(ctx context.Context, key string) (io.ReadCloser, error) {
	if f.err != nil {
		return nil, f.err
	}
	body, ok := f.files[key]
	if !ok {
		return nil, errors.New("not found: " + key)
	}
	return io.NopCloser(strings.NewReader(string(body))), nil
}

func attachWithKey(t *testing.T, taskID pgtype.UUID, suffix, key string) db.TaskMessageAttachment {
	t.Helper()
	return db.TaskMessageAttachment{
		ID:         uuidFromString(t, "abcd0000-0000-0000-0000-0000000000"+suffix),
		TaskID:     taskID,
		Kind:       "image",
		MimeType:   "image/png",
		ByteSize:   int64(8),
		StorageKey: key,
	}
}

// TestPatcherSendChatReplyNoImages pins the legacy text/markdown path
// when the task has no image attachments — the schema-2.0 mixed card
// must not fire and no Lark image upload may be triggered.
func TestPatcherSendChatReplyNoImages(t *testing.T) {
	p, q, api := newTestPatcher(t)
	p.SetAttachmentReader(&fakeAttachmentReader{files: map[string][]byte{}})
	taskID := uuidFromString(t, "ee100000-ee10-ee10-ee10-eeeeeeeeeeee")
	q.imageAttachments = nil

	p.handleEvent(events.Event{
		Type:          protocol.EventChatDone,
		TaskID:        uuidString(taskID),
		ChatSessionID: uuidString(q.binding.ChatSessionID),
		Payload: protocol.ChatDonePayload{
			TaskID:        uuidString(taskID),
			ChatSessionID: uuidString(q.binding.ChatSessionID),
			Content:       "Sure, on it.",
		},
	})

	api.mu.Lock()
	defer api.mu.Unlock()
	if len(api.uploadCalls) != 0 {
		t.Errorf("no images attached, must not call UploadImage; got %d", len(api.uploadCalls))
	}
	if len(api.sent) != 0 {
		t.Errorf("no mixed card expected; got %d card sends", len(api.sent))
	}
	if len(api.textSent) != 1 {
		t.Errorf("plain prose must take the text path; got %d", len(api.textSent))
	}
}

// TestPatcherSendChatReplySingleImage exercises the mixed-card happy
// path with one image attachment. The reply body is wrapped in a
// schema-2.0 interactive card carrying a markdown element + one img
// element keyed off the Lark-returned image_key.
func TestPatcherSendChatReplySingleImage(t *testing.T) {
	p, q, api := newTestPatcher(t)
	taskID := uuidFromString(t, "ee200000-ee20-ee20-ee20-eeeeeeeeeeee")
	q.imageAttachments = []db.TaskMessageAttachment{
		attachWithKey(t, taskID, "01", "tasks/x/chart-1.png"),
	}
	p.SetAttachmentReader(&fakeAttachmentReader{files: map[string][]byte{
		"tasks/x/chart-1.png": []byte("PNG-BYTES-1"),
	}})
	api.uploadKeys = []string{"img_key_1"}

	p.handleEvent(events.Event{
		Type:          protocol.EventChatDone,
		TaskID:        uuidString(taskID),
		ChatSessionID: uuidString(q.binding.ChatSessionID),
		Payload: protocol.ChatDonePayload{
			TaskID:        uuidString(taskID),
			ChatSessionID: uuidString(q.binding.ChatSessionID),
			Content:       "Here is your chart.",
		},
	})

	api.mu.Lock()
	defer api.mu.Unlock()
	if len(api.uploadCalls) != 1 {
		t.Fatalf("expected one UploadImage call; got %d", len(api.uploadCalls))
	}
	if string(api.uploadBodies[0]) != "PNG-BYTES-1" {
		t.Errorf("upload body mismatch; got %q", api.uploadBodies[0])
	}
	if len(api.sent) != 1 {
		t.Fatalf("expected one mixed card send; got %d", len(api.sent))
	}
	if len(api.textSent) != 0 || len(api.mdCardSent) != 0 {
		t.Errorf("mixed card path must NOT also fire text/markdown; got text=%d md=%d",
			len(api.textSent), len(api.mdCardSent))
	}
	var doc map[string]any
	if err := json.Unmarshal([]byte(api.sent[0].CardJSON), &doc); err != nil {
		t.Fatalf("decode card json: %v", err)
	}
	if doc["schema"] != "2.0" {
		t.Errorf("schema must be 2.0; got %v", doc["schema"])
	}
	body := doc["body"].(map[string]any)
	elements := body["elements"].([]any)
	if len(elements) != 2 {
		t.Fatalf("expected markdown + 1 img = 2 elements; got %d", len(elements))
	}
	if elements[0].(map[string]any)["tag"] != "markdown" {
		t.Errorf("first element must be markdown; got %v", elements[0])
	}
	imgEl := elements[1].(map[string]any)
	if imgEl["tag"] != "img" {
		t.Errorf("second element must be img; got %v", imgEl)
	}
	if imgEl["img_key"] != "img_key_1" {
		t.Errorf("img_key mismatch; got %v", imgEl["img_key"])
	}
}

// TestPatcherSendChatReplyMultipleImages pins the multi-image path
// and the 5-image hard cap: a 6th attachment must be dropped and a
// trailing note element must call out the truncation.
func TestPatcherSendChatReplyMultipleImages(t *testing.T) {
	p, q, api := newTestPatcher(t)
	taskID := uuidFromString(t, "ee300000-ee30-ee30-ee30-eeeeeeeeeeee")
	files := map[string][]byte{}
	rows := make([]db.TaskMessageAttachment, 0, 6)
	keys := make([]string, 0, 6)
	for i := 0; i < 6; i++ {
		key := "tasks/x/chart-" + string(rune('1'+i)) + ".png"
		files[key] = []byte("BYTES-" + string(rune('1'+i)))
		suffix := "1" + string(rune('0'+i))
		rows = append(rows, attachWithKey(t, taskID, suffix, key))
		keys = append(keys, "img_key_"+string(rune('1'+i)))
	}
	q.imageAttachments = rows
	p.SetAttachmentReader(&fakeAttachmentReader{files: files})
	api.uploadKeys = keys

	p.handleEvent(events.Event{
		Type:          protocol.EventChatDone,
		TaskID:        uuidString(taskID),
		ChatSessionID: uuidString(q.binding.ChatSessionID),
		Payload: protocol.ChatDonePayload{
			TaskID:        uuidString(taskID),
			ChatSessionID: uuidString(q.binding.ChatSessionID),
			Content:       "Six charts attached.",
		},
	})

	api.mu.Lock()
	defer api.mu.Unlock()
	if len(api.uploadCalls) != maxMixedCardImages {
		t.Fatalf("expected upload calls capped at %d; got %d", maxMixedCardImages, len(api.uploadCalls))
	}
	if len(api.sent) != 1 {
		t.Fatalf("expected one mixed card send; got %d", len(api.sent))
	}
	var doc map[string]any
	if err := json.Unmarshal([]byte(api.sent[0].CardJSON), &doc); err != nil {
		t.Fatalf("decode card json: %v", err)
	}
	body := doc["body"].(map[string]any)
	elements := body["elements"].([]any)
	// 1 markdown + 5 img + 1 note = 7
	if len(elements) != 1+maxMixedCardImages+1 {
		t.Fatalf("expected %d elements; got %d", 1+maxMixedCardImages+1, len(elements))
	}
	tail := elements[len(elements)-1].(map[string]any)
	if tail["tag"] != "note" {
		t.Errorf("trailing element must be a note; got %v", tail)
	}
}

// TestPatcherSendChatReplyAllUploadsFailDegrades verifies the soft-
// degrade contract: when every UploadImage call errors, the Patcher
// must NOT send a mixed card and instead fall back to the legacy
// text/markdown path so the user still gets the textual answer.
func TestPatcherSendChatReplyAllUploadsFailDegrades(t *testing.T) {
	p, q, api := newTestPatcher(t)
	taskID := uuidFromString(t, "ee400000-ee40-ee40-ee40-eeeeeeeeeeee")
	q.imageAttachments = []db.TaskMessageAttachment{
		attachWithKey(t, taskID, "21", "tasks/y/chart-1.png"),
		attachWithKey(t, taskID, "22", "tasks/y/chart-2.png"),
	}
	p.SetAttachmentReader(&fakeAttachmentReader{files: map[string][]byte{
		"tasks/y/chart-1.png": []byte("a"),
		"tasks/y/chart-2.png": []byte("b"),
	}})
	api.uploadKeys = []string{"", ""}
	api.uploadErrs = []error{errors.New("upload boom 1"), errors.New("upload boom 2")}

	p.handleEvent(events.Event{
		Type:          protocol.EventChatDone,
		TaskID:        uuidString(taskID),
		ChatSessionID: uuidString(q.binding.ChatSessionID),
		Payload: protocol.ChatDonePayload{
			TaskID:        uuidString(taskID),
			ChatSessionID: uuidString(q.binding.ChatSessionID),
			Content:       "Plain answer remains.",
		},
	})

	api.mu.Lock()
	defer api.mu.Unlock()
	if len(api.uploadCalls) != 2 {
		t.Errorf("expected two UploadImage attempts; got %d", len(api.uploadCalls))
	}
	if len(api.sent) != 0 {
		t.Errorf("no mixed card expected on full upload failure; got %d", len(api.sent))
	}
	if len(api.textSent) != 1 {
		t.Errorf("expected text fallback; got %d text sends", len(api.textSent))
	}
	if api.textSent[0].Text != "Plain answer remains." {
		t.Errorf("fallback text mismatch; got %q", api.textSent[0].Text)
	}
}

// TestPatcherSendChatReplyPartialUploadFailureKeepsCard verifies the
// per-image soft-failure contract: when only some uploads fail, the
// card is still sent — the failed slot becomes a `note` placeholder
// element so the user knows an image was supposed to be there. The
// successful slots stay as img elements with the real image_key.
func TestPatcherSendChatReplyPartialUploadFailureKeepsCard(t *testing.T) {
	p, q, api := newTestPatcher(t)
	taskID := uuidFromString(t, "ee500000-ee50-ee50-ee50-eeeeeeeeeeee")
	q.imageAttachments = []db.TaskMessageAttachment{
		attachWithKey(t, taskID, "31", "tasks/z/chart-1.png"),
		attachWithKey(t, taskID, "32", "tasks/z/chart-2.png"),
		attachWithKey(t, taskID, "33", "tasks/z/chart-3.png"),
	}
	p.SetAttachmentReader(&fakeAttachmentReader{files: map[string][]byte{
		"tasks/z/chart-1.png": []byte("a"),
		"tasks/z/chart-2.png": []byte("b"),
		"tasks/z/chart-3.png": []byte("c"),
	}})
	// First upload OK, second fails, third OK.
	api.uploadKeys = []string{"img_key_a", "", "img_key_c"}
	api.uploadErrs = []error{nil, errors.New("upload boom"), nil}

	p.handleEvent(events.Event{
		Type:          protocol.EventChatDone,
		TaskID:        uuidString(taskID),
		ChatSessionID: uuidString(q.binding.ChatSessionID),
		Payload: protocol.ChatDonePayload{
			TaskID:        uuidString(taskID),
			ChatSessionID: uuidString(q.binding.ChatSessionID),
			Content:       "Three charts.",
		},
	})

	api.mu.Lock()
	defer api.mu.Unlock()
	if len(api.uploadCalls) != 3 {
		t.Fatalf("expected three UploadImage attempts; got %d", len(api.uploadCalls))
	}
	if len(api.sent) != 1 {
		t.Fatalf("expected one mixed card send (partial failure must not degrade); got %d", len(api.sent))
	}
	if len(api.textSent) != 0 || len(api.mdCardSent) != 0 {
		t.Errorf("partial failure must NOT also fire text/markdown; got text=%d md=%d",
			len(api.textSent), len(api.mdCardSent))
	}
	var doc map[string]any
	if err := json.Unmarshal([]byte(api.sent[0].CardJSON), &doc); err != nil {
		t.Fatalf("decode card json: %v", err)
	}
	els := doc["body"].(map[string]any)["elements"].([]any)
	// 1 markdown + 3 slot elements
	if len(els) != 4 {
		t.Fatalf("expected 4 elements (md + img + note + img); got %d", len(els))
	}
	if els[1].(map[string]any)["tag"] != "img" {
		t.Errorf("element 1 must be img; got %v", els[1])
	}
	if els[2].(map[string]any)["tag"] != "note" {
		t.Errorf("element 2 must be a placeholder note; got %v", els[2])
	}
	if els[3].(map[string]any)["tag"] != "img" {
		t.Errorf("element 3 must be img; got %v", els[3])
	}
	noteText := els[2].(map[string]any)["elements"].([]any)[0].(map[string]any)["content"].(string)
	if !strings.Contains(noteText, "chart-2.png") || !strings.Contains(noteText, "上传失败") {
		t.Errorf("placeholder text must identify the failing image; got %q", noteText)
	}
}

func okSlot(key string) imageSlot    { return imageSlot{imageKey: key} }
func failSlot(text string) imageSlot { return imageSlot{placeholder: text} }

// TestBuildMixedCardSnapshots pins the schema-2.0 JSON layout for the
// three composition cases the parent issue calls out: pure text+image
// happy path, text-only (caller never builds a mixed card here, but we
// still pin behaviour), and the soft-failure placeholder case where a
// failed upload becomes a `note` element instead of dropping the slot.
func TestBuildMixedCardSnapshots(t *testing.T) {
	t.Run("text+image", func(t *testing.T) {
		raw, err := buildMixedCard("# Title\nbody text", []imageSlot{okSlot("k1"), okSlot("k2")}, false)
		if err != nil {
			t.Fatalf("build: %v", err)
		}
		var doc map[string]any
		if err := json.Unmarshal([]byte(raw), &doc); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if doc["schema"] != "2.0" {
			t.Errorf("schema must be 2.0; got %v", doc["schema"])
		}
		body := doc["body"].(map[string]any)
		els := body["elements"].([]any)
		if len(els) != 3 {
			t.Fatalf("expected 1 markdown + 2 images = 3 elements; got %d", len(els))
		}
		if els[0].(map[string]any)["tag"] != "markdown" {
			t.Errorf("first element must be markdown; got %v", els[0])
		}
		for i := 1; i < 3; i++ {
			img := els[i].(map[string]any)
			if img["tag"] != "img" {
				t.Errorf("element %d must be img; got %v", i, img)
			}
		}
		cfg := doc["config"].(map[string]any)
		if _, ok := cfg["summary"]; !ok {
			t.Errorf("config must include summary; got %v", cfg)
		}
	})
	t.Run("pure-image-only", func(t *testing.T) {
		// Empty markdown body + at least one ok slot must still render.
		raw, err := buildMixedCard("", []imageSlot{okSlot("only")}, false)
		if err != nil {
			t.Fatalf("build: %v", err)
		}
		var doc map[string]any
		if err := json.Unmarshal([]byte(raw), &doc); err != nil {
			t.Fatalf("decode: %v", err)
		}
		els := doc["body"].(map[string]any)["elements"].([]any)
		if len(els) != 1 {
			t.Fatalf("empty body must omit markdown; got %d elements", len(els))
		}
		if els[0].(map[string]any)["tag"] != "img" {
			t.Errorf("only element must be img; got %v", els[0])
		}
	})
	t.Run("partial-upload-failure-placeholder", func(t *testing.T) {
		// Mixed slots: ok + failed + ok. The failed slot must render
		// inline as a `note` element so the user sees something is
		// missing without breaking the whole card.
		raw, err := buildMixedCard("Body", []imageSlot{
			okSlot("k1"),
			failSlot("[图片 chart-2.png 上传失败]"),
			okSlot("k3"),
		}, false)
		if err != nil {
			t.Fatalf("build: %v", err)
		}
		var doc map[string]any
		if err := json.Unmarshal([]byte(raw), &doc); err != nil {
			t.Fatalf("decode: %v", err)
		}
		els := doc["body"].(map[string]any)["elements"].([]any)
		// 1 markdown + 3 slot elements (img, note, img) = 4
		if len(els) != 4 {
			t.Fatalf("expected 4 elements (md + 3 slots); got %d", len(els))
		}
		mid := els[2].(map[string]any)
		if mid["tag"] != "note" {
			t.Errorf("failed slot must render as note; got %v", mid)
		}
		inner := mid["elements"].([]any)[0].(map[string]any)
		if inner["content"] != "[图片 chart-2.png 上传失败]" {
			t.Errorf("placeholder text mismatch; got %v", inner)
		}
		if els[1].(map[string]any)["tag"] != "img" || els[3].(map[string]any)["tag"] != "img" {
			t.Errorf("ok slots must remain img elements; got %v / %v", els[1], els[3])
		}
	})
}

// TestBuildMixedCardRejectsAllFailed pins the precondition: callers
// must not invoke buildMixedCard when every slot is a placeholder —
// the contract is that we degrade to text/markdown in that case. The
// function returns an error so a programming bug surfaces loudly
// instead of producing a card with zero img elements.
func TestBuildMixedCardRejectsAllFailed(t *testing.T) {
	if _, err := buildMixedCard("body", []imageSlot{failSlot("[a]"), failSlot("[b]")}, false); err == nil {
		t.Fatalf("expected error when every slot is a placeholder")
	}
	if _, err := buildMixedCard("body", nil, false); err == nil {
		t.Fatalf("expected error on empty slot list")
	}
}
