-- name: CreateWorkflowCase :one
INSERT INTO workflow_case (
    workspace_id, title, description, source_issue_id, owner_agent_id, status, created_by, updated_by
) VALUES (
    $1, $2, $3, sqlc.narg('source_issue_id'), sqlc.narg('owner_agent_id'), $4, sqlc.narg('created_by'), sqlc.narg('updated_by')
)
RETURNING *;

-- name: GetWorkflowCase :one
SELECT * FROM workflow_case WHERE id = $1;

-- name: GetWorkflowCaseBySourceIssue :one
SELECT * FROM workflow_case
WHERE source_issue_id = $1
ORDER BY created_at DESC
LIMIT 1;

-- name: GetActiveWorkflowCaseByEntryIssue :one
SELECT * FROM workflow_case
WHERE source_issue_id = $1
  AND status <> 'archived'
ORDER BY created_at DESC
LIMIT 1;

-- name: ListWorkflowCases :many
SELECT * FROM workflow_case
WHERE workspace_id = $1
ORDER BY updated_at DESC
LIMIT sqlc.arg('limit') OFFSET sqlc.arg('offset');

-- name: UpdateWorkflowCase :one
UPDATE workflow_case SET
    title = COALESCE(sqlc.narg('title'), title),
    description = COALESCE(sqlc.narg('description'), description),
    owner_agent_id = COALESCE(sqlc.narg('owner_agent_id'), owner_agent_id),
    status = COALESCE(sqlc.narg('status'), status),
    current_run_id = COALESCE(sqlc.narg('current_run_id'), current_run_id),
    updated_by = COALESCE(sqlc.narg('updated_by'), updated_by),
    updated_at = now()
WHERE id = $1
RETURNING *;

-- name: SetWorkflowCaseOnlineVersion :one
UPDATE workflow_case SET
    online_version_id = $2,
    updated_by = sqlc.narg('updated_by'),
    updated_at = now()
WHERE id = $1
RETURNING *;

-- name: CountActiveWorkflowRunsByCase :one
SELECT count(*) FROM workflow_run
WHERE case_id = $1
  AND status IN ('pending', 'planning', 'running', 'finalizing', 'cancelling');

-- name: ClearWorkflowNodeCarrierIssueBindings :exec
UPDATE issue SET
    origin_type = NULL,
    origin_id = NULL,
    updated_at = now()
WHERE workspace_id = $1
  AND origin_type = 'workflow_node'
  AND origin_id = ANY(sqlc.arg('run_ids')::uuid[]);

-- name: DeleteWorkflowCase :one
DELETE FROM workflow_case
WHERE id = $1 AND workspace_id = $2
RETURNING id;
