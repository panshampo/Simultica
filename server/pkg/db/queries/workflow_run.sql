-- name: CreateWorkflowRun :one
INSERT INTO workflow_run (
    workspace_id, root_issue_id, skill_id, planner_task_id, status,
    current_node, nodes_state, definition_snapshot, source_skills,
    case_id, definition_version_id, label, started_at
) VALUES (
    $1, $2, $3, sqlc.narg('planner_task_id'), $4,
    $5, $6, $7, sqlc.arg('source_skills'),
    sqlc.narg('case_id'), sqlc.narg('definition_version_id'),
    COALESCE(sqlc.narg('label'), ''),
    now()
)
RETURNING *;

-- name: GetWorkflowRun :one
SELECT * FROM workflow_run WHERE id = $1;

-- name: GetWorkflowRunByRootIssue :one
SELECT * FROM workflow_run
WHERE root_issue_id = $1
ORDER BY created_at DESC
LIMIT 1;

-- name: UpdateWorkflowRunProgress :one
UPDATE workflow_run SET
    status       = COALESCE(sqlc.narg('status'), status),
    current_node = COALESCE(sqlc.narg('current_node'), current_node),
    nodes_state  = COALESCE(sqlc.narg('nodes_state'), nodes_state),
    error        = COALESCE(sqlc.narg('error'), error),
    completed_at = CASE
        WHEN COALESCE(sqlc.narg('status'), status) IN ('done', 'failed', 'cancelled') THEN COALESCE(completed_at, now())
        ELSE completed_at
    END,
    updated_at   = now()
WHERE id = $1
RETURNING *;

-- name: ListWorkflowRunsByCase :many
SELECT * FROM workflow_run
WHERE case_id = $1
ORDER BY created_at DESC;

-- name: GetLatestWorkflowRunByCase :one
SELECT * FROM workflow_run
WHERE case_id = $1
ORDER BY created_at DESC
LIMIT 1;

-- name: CancelWorkflowRun :one
UPDATE workflow_run SET
    status = 'cancelled',
    cancel_reason = sqlc.arg('cancel_reason'),
    error = sqlc.arg('cancel_reason'),
    cancelled_at = now(),
    completed_at = COALESCE(completed_at, now()),
    updated_at = now()
WHERE id = $1
RETURNING *;
