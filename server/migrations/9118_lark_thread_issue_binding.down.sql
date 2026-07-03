-- Reverse of 9118_lark_thread_issue_binding.up.sql. Restores the
-- pre-thread schema: drops the triple UNIQUE / new columns, restores
-- the legacy (installation_id, lark_chat_id) UNIQUE, removes
-- agent.lark_issue_project_id.

DROP INDEX IF EXISTS idx_lark_chat_session_binding_issue;

ALTER TABLE lark_chat_session_binding
    DROP CONSTRAINT IF EXISTS lark_chat_session_binding_thread_key;

ALTER TABLE lark_chat_session_binding
    DROP COLUMN IF EXISTS issue_id;

ALTER TABLE lark_chat_session_binding
    DROP COLUMN IF EXISTS lark_thread_root_id;

ALTER TABLE lark_chat_session_binding
    ADD CONSTRAINT lark_chat_session_binding_installation_id_lark_chat_id_key
        UNIQUE (installation_id, lark_chat_id);

DROP INDEX IF EXISTS idx_agent_lark_issue_project;

ALTER TABLE agent
    DROP COLUMN IF EXISTS lark_issue_project_id;
