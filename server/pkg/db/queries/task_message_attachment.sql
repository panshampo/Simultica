-- name: CreateTaskMessageAttachment :one
INSERT INTO task_message_attachment (
    task_id, message_id, kind, mime_type, byte_size, storage_key
) VALUES ($1, $2, $3, $4, $5, $6)
RETURNING *;

-- name: ListTaskMessageImageAttachments :many
SELECT * FROM task_message_attachment
WHERE task_id = $1 AND kind = 'image'
ORDER BY created_at ASC;

-- name: ListTaskMessageAttachmentsByTask :many
SELECT * FROM task_message_attachment
WHERE task_id = $1
ORDER BY created_at ASC;

-- name: GetTaskMessageAttachment :one
SELECT * FROM task_message_attachment
WHERE id = $1;
