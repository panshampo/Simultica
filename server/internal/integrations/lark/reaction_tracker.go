package lark

import (
	"sync"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/util"
)

// ReactionTracker bridges the inbound OutcomeReplier (which posts a
// receipt-style reaction the moment a user message lands) and the
// outbound Patcher (which delivers the agent's reply). The two run on
// independent goroutines and have no shared state via the DB — the
// reaction's identity (message_id + reaction_id) is only known to the
// inbound side at send time, and only the outbound side knows when the
// agent has finished replying. The tracker keeps the (message_id,
// reaction_id) pairs in memory keyed by chat_session_id so the Patcher
// can retract them once EventChatDone fires.
//
// Scope: best-effort, in-memory, single-replica. The reaction is itself
// best-effort (a stale one is just visual noise, never a correctness
// problem); persisting these rows to DB would buy nothing in exchange
// for migration churn. On process restart any not-yet-retracted
// reactions stay in place; the next user message triggers a new reaction
// + reply cycle that retracts the new one.
type ReactionTracker interface {
	// Track registers a reaction the OutcomeReplier just posted on a
	// user's Lark message. The Patcher drains these via Drain when the
	// agent's reply for the same chat_session lands.
	Track(chatSessionID pgtype.UUID, messageID, reactionID string)

	// Drain returns and removes every pending reaction for a chat_session.
	// Called by the Patcher right before / after sending the agent reply
	// so each entry's DeleteMessageReaction call can run.
	Drain(chatSessionID pgtype.UUID) []ReactionEntry
}

// ReactionEntry is one (Lark message, reaction id) pair the Patcher needs
// in order to call DeleteMessageReaction.
type ReactionEntry struct {
	MessageID  string
	ReactionID string
}

// NewReactionTracker returns the production in-memory tracker. Safe for
// concurrent use across the Hub's per-installation goroutines and the
// Patcher's event-bus subscription.
func NewReactionTracker() ReactionTracker {
	return &memoryReactionTracker{entries: make(map[string][]ReactionEntry)}
}

type memoryReactionTracker struct {
	mu      sync.Mutex
	entries map[string][]ReactionEntry
}

func (t *memoryReactionTracker) Track(chatSessionID pgtype.UUID, messageID, reactionID string) {
	if !chatSessionID.Valid || messageID == "" || reactionID == "" {
		return
	}
	key := util.UUIDToString(chatSessionID)
	t.mu.Lock()
	t.entries[key] = append(t.entries[key], ReactionEntry{MessageID: messageID, ReactionID: reactionID})
	t.mu.Unlock()
}

func (t *memoryReactionTracker) Drain(chatSessionID pgtype.UUID) []ReactionEntry {
	if !chatSessionID.Valid {
		return nil
	}
	key := util.UUIDToString(chatSessionID)
	t.mu.Lock()
	out := t.entries[key]
	delete(t.entries, key)
	t.mu.Unlock()
	return out
}
