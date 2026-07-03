CREATE TABLE task_message_attachment (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    task_id     UUID NOT NULL REFERENCES agent_task_queue(id) ON DELETE CASCADE,
    message_id  UUID REFERENCES task_message(id) ON DELETE CASCADE,
    kind        TEXT NOT NULL,
    mime_type   TEXT NOT NULL,
    byte_size   BIGINT NOT NULL,
    storage_key TEXT NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_task_message_attachment_task ON task_message_attachment(task_id);
CREATE INDEX idx_task_message_attachment_message ON task_message_attachment(message_id) WHERE message_id IS NOT NULL;
