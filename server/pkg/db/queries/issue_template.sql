-- =====================
-- Issue Template CRUD
-- =====================

-- name: CreateIssueTemplate :one
INSERT INTO issue_template (
    workspace_id, project_id, title, issue_title_template, issue_body_template,
    assignee_type, assignee_id, priority, labels, default_metadata, execution_spec,
    created_by_type, created_by_id
) VALUES (
    $1, sqlc.narg('project_id'), $2, $3, sqlc.narg('issue_body_template'),
    $4, $5, $6, sqlc.narg('labels'), sqlc.narg('default_metadata'), sqlc.narg('execution_spec'),
    $7, $8
) RETURNING *;

-- name: ListIssueTemplates :many
SELECT it.*,
       COUNT(a.id)::bigint AS automation_ref_count
FROM issue_template it
LEFT JOIN automation a ON a.template_id = it.id
WHERE it.workspace_id = $1
  AND (sqlc.narg('project_id')::uuid IS NULL OR it.project_id = sqlc.narg('project_id'))
GROUP BY it.id
ORDER BY it.updated_at DESC;

-- name: GetIssueTemplate :one
SELECT * FROM issue_template
WHERE id = $1;

-- name: GetIssueTemplateInWorkspace :one
SELECT * FROM issue_template
WHERE id = $1 AND workspace_id = $2;

-- name: UpdateIssueTemplate :one
UPDATE issue_template SET
    project_id = sqlc.narg('project_id'),
    title = COALESCE(sqlc.narg('title'), title),
    issue_title_template = COALESCE(sqlc.narg('issue_title_template'), issue_title_template),
    issue_body_template = sqlc.narg('issue_body_template'),
    assignee_type = COALESCE(sqlc.narg('assignee_type'), assignee_type),
    assignee_id = COALESCE(sqlc.narg('assignee_id')::uuid, assignee_id),
    priority = COALESCE(sqlc.narg('priority'), priority),
    labels = sqlc.narg('labels'),
    default_metadata = sqlc.narg('default_metadata'),
    execution_spec = sqlc.narg('execution_spec'),
    updated_at = now()
WHERE id = sqlc.arg('id') AND workspace_id = sqlc.arg('workspace_id')
RETURNING *;

-- name: DeleteIssueTemplate :exec
DELETE FROM issue_template
WHERE id = sqlc.arg('id') AND workspace_id = sqlc.arg('workspace_id');

-- name: CountIssueTemplateAutomationReferences :one
SELECT count(*) FROM automation
WHERE template_id = $1;

-- name: CountIssueTemplateIssueReferences :one
SELECT count(*) FROM issue
WHERE issue_template_id = $1;

-- name: ListIssueTemplateIssues :many
SELECT i.id, i.workspace_id, i.title, i.description, i.status, i.priority,
       i.assignee_type, i.assignee_id, i.creator_type, i.creator_id,
       i.parent_issue_id, i.position, i.start_date, i.due_date, i.created_at, i.updated_at, i.number, i.project_id, i.metadata
FROM issue i
WHERE i.workspace_id = sqlc.arg('workspace_id')
  AND i.issue_template_id = sqlc.arg('issue_template_id')
ORDER BY i.created_at DESC
LIMIT sqlc.arg('limit') OFFSET sqlc.arg('offset');
