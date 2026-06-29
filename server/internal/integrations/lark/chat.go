package lark

import (
	"context"

	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

// ChatSessionService is the channel-aware chat-session entry point for
// Lark. It exists deliberately apart from the HTTP `SendChatMessage`
// handler because that handler's single-creator semantics
// (chat_session.creator_id == request user_id) make sense for the
// browser/desktop client — one human, one session — but break for
// group chat_sessions where many Lark users converse with one Bot.
//
// Concrete implementation lands in a follow-up PR (MUL-2671). The
// interface is declared here so the migration + service boundary PR
// can establish the architectural cut without dragging in OAuth, WS,
// and card-patching code.
//
// Inbound contract (enforced by the implementation):
//
//   - EnsureChatSession is the ONLY way Lark code creates / looks up a
//     chat_session. Identity check MUST run before this call — the
//     service treats every successful return as "the sender is a
//     verified, workspace-bound user".
//
//   - AppendUserMessage trusts that the caller has gated the message
//     through identity + group-mention filters. Unbound users and
//     non-addressed group messages do NOT come through here; they go
//     to AuditDrop instead.
type ChatSessionService interface {
	// EnsureChatSession returns the chat_session bound to the given
	// (installation, lark_chat_id, thread_root_id) triple, creating it
	// on first contact. `sender` must already be a verified
	// lark_user_binding row — see the contract note above. The result
	// carries the chat_session.id (always) and, when the binding has
	// an attached auto-issue, the issue id.
	EnsureChatSession(ctx context.Context, p EnsureChatSessionParams) (EnsureChatSessionResult, error)

	// AppendUserMessage appends the message to chat_session, dedups
	// via lark_inbound_message_dedup, and (when the message starts
	// with `/issue`) returns the parsed command so the caller can
	// dispatch through service.IssueService.Create.
	AppendUserMessage(ctx context.Context, p AppendUserMessageParams) (AppendResult, error)
}

// EnsureChatSessionParams carries the inputs for ChatSessionService.EnsureChatSession.
// Note `Sender` is the resolved Multica user UUID — the caller has
// already mapped lark_open_id → user via lark_user_binding.
//
// ThreadRootID identifies the Lark thread inside ChatID. For group chats
// it is the message_id of the thread anchor (the first message that
// @-mentioned the bot in this thread); follow-ups in the same thread
// pass the same value. For p2p chats it is the empty string — every
// message in a p2p collapses to one session.
type EnsureChatSessionParams struct {
	WorkspaceID    pgtype.UUID
	InstallationID pgtype.UUID
	AgentID        pgtype.UUID
	ChatID         ChatID
	ChatType       ChatType
	ThreadRootID   string
	Sender         pgtype.UUID
}

// EnsureChatSessionResult is the structured return from
// EnsureChatSession. It carries both the chat_session id (the legacy
// shape) and, when one is already attached, the issue id the binding
// row points to. Created reports whether this call created the binding
// (vs. found an existing one) — the dispatcher uses it to gate
// auto-issue creation.
type EnsureChatSessionResult struct {
	ChatSessionID pgtype.UUID
	BindingID     pgtype.UUID
	IssueID       pgtype.UUID
	Issue         db.Issue
	Created       bool
}

// AppendUserMessageParams carries the inputs for ChatSessionService.AppendUserMessage.
// Body is the (already-decoded) user-facing text. LarkMessageID is the
// Lark-side message id used for idempotency dedup.
//
// ClaimToken is the owner-fencing token returned by the dispatcher's
// ClaimLarkInboundDedup call. When ClaimToken.Valid is true,
// AppendUserMessage runs MarkLarkInboundDedupProcessed INSIDE its own
// chat_message+session transaction, gated on this token. A mismatched
// token (another worker re-claimed the row while we were running)
// returns ErrClaimLost and rolls back the entire transaction, so no
// second chat_message can land for the same Lark message_id. Pass an
// invalid (zero) UUID to skip the in-tx Mark — useful for tests and
// for callers that have already finalized dedup outside the
// transaction.
type AppendUserMessageParams struct {
	ChatSessionID pgtype.UUID
	Sender        pgtype.UUID
	// Body is the full text stored as the chat_message — including any
	// quoted-reply / forwarded context the enricher inlined.
	Body string
	// CommandBody is the user's own typed text, used as the `/issue`
	// command source. It is the un-enriched Body; when empty (callers
	// that don't set it), `/issue` parsing falls back to Body so
	// behavior is unchanged for the non-enriched path.
	CommandBody    string
	InstallationID pgtype.UUID
	LarkMessageID  string
	ClaimToken     pgtype.UUID
}

// AppendResult reports what AppendUserMessage decided.
//
// Dedup is enforced by the Dispatcher's top-level dedup gate before
// AppendUserMessage runs, so a returned AppendResult always
// represents a freshly-stored message. Callers may safely act on
// IssueCommand without re-checking idempotency.
type AppendResult struct {
	// IssueCommand is non-nil when the first non-empty line begins
	// with `/issue`. The caller passes this to
	// service.IssueService.Create.
	IssueCommand *IssueCommand
	// DedupMarked is true when AppendUserMessage finalized the dedup
	// claim in its own transaction (i.e. ClaimToken was supplied and
	// the Mark succeeded). The dispatcher uses this to skip the
	// post-pipeline finalize, since the row is already in its
	// terminal state.
	DedupMarked bool
}

// IssueCommand is the parsed shape of a user-typed `/issue ...`
// command. Title is required; Description is the joined remainder of
// the message body (empty when only a title was given).
type IssueCommand struct {
	Title       string
	Description string
}

// AuditLogger records dropped inbound events to lark_inbound_audit.
// The interface deliberately does not accept a message body — see the
// drop-audit policy in MUL-2671 §4.7.
type AuditLogger interface {
	RecordDrop(ctx context.Context, p AuditDropParams) error
}

type AuditDropParams struct {
	InstallationID pgtype.UUID // may be invalid for installation-less events
	ChatID         ChatID
	EventType      string
	LarkEventID    string
	LarkMessageID  string
	Reason         DropReason
}
