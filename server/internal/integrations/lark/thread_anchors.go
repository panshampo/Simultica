package lark

import (
	"sync"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/util"
)

// ThreadAnchorTracker remembers, per chat_session, the Lark message_id
// the next outbound from the bot should reply to with reply_in_thread=true.
// In group chats with several coworkers using the bot at once, threading
// every reply onto the user's @-mention keeps each conversation grouped
// instead of interleaving raw messages in the main timeline.
//
// Lifecycle:
//
//   - The Dispatcher writes an anchor for every group-chat message that
//     is addressed to the bot (the message_id of the user's @-mention).
//     The latest message in a debounce window wins — the agent run
//     reads the whole session, but the visible reply lands under the
//     most recent prompt.
//
//   - The Patcher reads the anchor when an outbound is about to be
//     sent and clears it on EventChatDone / EventTaskFailed so a stale
//     anchor cannot redirect a future, unrelated reply into an old
//     thread.
//
//   - The OutcomeReplier reads the anchor for synchronous notices
//     (issue-created confirmation, agent-offline notice) but does NOT
//     clear it; the Patcher's async final reply owns that.
//
// In-memory, single-replica. The WS lease guarantees one process per
// installation, and a stale anchor on crash is harmless — the next
// inbound message overwrites it.
type ThreadAnchorTracker interface {
	Set(chatSessionID pgtype.UUID, messageID string)
	Get(chatSessionID pgtype.UUID) string
	Clear(chatSessionID pgtype.UUID)
}

// NewThreadAnchorTracker returns the production in-memory tracker.
func NewThreadAnchorTracker() ThreadAnchorTracker {
	return &memoryThreadAnchorTracker{anchors: make(map[string]string)}
}

type memoryThreadAnchorTracker struct {
	mu      sync.Mutex
	anchors map[string]string
}

func (t *memoryThreadAnchorTracker) Set(chatSessionID pgtype.UUID, messageID string) {
	if !chatSessionID.Valid || messageID == "" {
		return
	}
	key := util.UUIDToString(chatSessionID)
	t.mu.Lock()
	t.anchors[key] = messageID
	t.mu.Unlock()
}

func (t *memoryThreadAnchorTracker) Get(chatSessionID pgtype.UUID) string {
	if !chatSessionID.Valid {
		return ""
	}
	key := util.UUIDToString(chatSessionID)
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.anchors[key]
}

func (t *memoryThreadAnchorTracker) Clear(chatSessionID pgtype.UUID) {
	if !chatSessionID.Valid {
		return
	}
	key := util.UUIDToString(chatSessionID)
	t.mu.Lock()
	delete(t.anchors, key)
	t.mu.Unlock()
}
