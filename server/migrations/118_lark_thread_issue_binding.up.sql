-- Lark group @-mentions auto-create per-thread Issues.
--
-- Three coupled changes land here:
--
--   1. agent.lark_issue_project_id — per-agent target project for the
--      auto-created Issue. NULL means "do not auto-create"; the agent
--      keeps the legacy chat-only behavior.
--
--   2. lark_chat_session_binding.lark_thread_root_id — the Lark thread
--      root id (RootID, falling back to MessageID for the first message
--      in a new thread). p2p messages use the empty string. Combined
--      with (installation_id, lark_chat_id) it becomes the unique key
--      for a binding row, so a single group chat can host many parallel
--      threads, each with its own chat_session and Issue.
--
--   3. lark_chat_session_binding.issue_id — the Issue auto-created on
--      the first @-mention of a thread. NULL when the agent had no
--      lark_issue_project_id at session-creation time. ON DELETE SET
--      NULL so cleaning up a stale Issue does not also nuke the
--      chat_session that mirrored into it.

ALTER TABLE agent
    ADD COLUMN lark_issue_project_id UUID NULL
        REFERENCES project(id) ON DELETE SET NULL;

CREATE INDEX idx_agent_lark_issue_project
    ON agent(lark_issue_project_id)
    WHERE lark_issue_project_id IS NOT NULL;

ALTER TABLE lark_chat_session_binding
    ADD COLUMN lark_thread_root_id TEXT NOT NULL DEFAULT '';

ALTER TABLE lark_chat_session_binding
    ADD COLUMN issue_id UUID NULL
        REFERENCES issue(id) ON DELETE SET NULL;

-- Replace the legacy (installation_id, lark_chat_id) UNIQUE with the
-- thread-aware triple. The legacy UNIQUE is what GetLarkChatSessionBinding
-- relied on; the triple keeps that lookup deterministic while letting
-- multiple threads coexist inside a single Lark group chat.
ALTER TABLE lark_chat_session_binding
    DROP CONSTRAINT lark_chat_session_binding_installation_id_lark_chat_id_key;

ALTER TABLE lark_chat_session_binding
    ADD CONSTRAINT lark_chat_session_binding_thread_key
        UNIQUE (installation_id, lark_chat_id, lark_thread_root_id);

CREATE INDEX idx_lark_chat_session_binding_issue
    ON lark_chat_session_binding(issue_id)
    WHERE issue_id IS NOT NULL;
