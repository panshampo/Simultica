-- name: CreateWorkflowRunNode :one
INSERT INTO workflow_run_node (
    workspace_id, case_id, run_id, node_id, node_type, dispatch, carrier_kind, status
) VALUES (
    $1, $2, $3, $4, $5, $6, sqlc.arg('carrier_kind'), $7
)
ON CONFLICT (run_id, node_id) DO UPDATE SET
    node_type = EXCLUDED.node_type,
    dispatch = EXCLUDED.dispatch,
    carrier_kind = EXCLUDED.carrier_kind,
    updated_at = now()
RETURNING *;

-- name: ListWorkflowRunNodes :many
SELECT * FROM workflow_run_node
WHERE run_id = $1
ORDER BY created_at ASC;

-- name: GetWorkflowRunNodeByIssueCarrier :one
SELECT * FROM workflow_run_node
WHERE workspace_id = $1
  AND run_id = $2
  AND (
    carrier_ref @> jsonb_build_object('issue_id', sqlc.arg('issue_id')::text)
    OR carrier_ref @> jsonb_build_object('sub_issue_id', sqlc.arg('issue_id')::text)
    OR carrier_ref @> jsonb_build_object('subIssueId', sqlc.arg('issue_id')::text)
  )
ORDER BY updated_at DESC
LIMIT 1;

-- name: CreateWorkflowRunNodeEvent :one
INSERT INTO workflow_run_node_event (
    workspace_id, case_id, run_id, node_id, event_type, attempt,
    input_snapshot, output_snapshot, error, logs, carrier_ref, sequence, occurred_at
) VALUES (
    $1, $2, $3, $4, $5, $6,
    sqlc.narg('input_snapshot')::jsonb, sqlc.narg('output_snapshot')::jsonb, sqlc.narg('error')::jsonb,
    COALESCE(sqlc.narg('logs')::jsonb, '[]'::jsonb),
    sqlc.narg('carrier_ref')::jsonb, sqlc.narg('sequence'), COALESCE(sqlc.narg('occurred_at')::timestamptz, now())
)
RETURNING *;

-- name: ListWorkflowRunNodeEvents :many
SELECT * FROM workflow_run_node_event
WHERE run_id = $1 AND node_id = $2
ORDER BY created_at ASC;

-- name: ProjectWorkflowRunNodeEvent :one
UPDATE workflow_run_node SET
    status = CASE sqlc.arg('event_type')::text
        WHEN 'node_started' THEN 'running'
        WHEN 'node_review_requested' THEN 'pending_review'
        WHEN 'node_blocked' THEN 'blocked'
        WHEN 'node_succeeded' THEN 'succeeded'
        WHEN 'node_failed' THEN 'failed'
        WHEN 'node_cancelled' THEN 'cancelled'
        WHEN 'node_skipped' THEN 'skipped'
        ELSE status
    END,
    attempt = GREATEST(attempt, sqlc.arg('attempt')::int),
    input_snapshot = COALESCE(sqlc.narg('input_snapshot')::jsonb, input_snapshot),
    output_snapshot = COALESCE(sqlc.narg('output_snapshot')::jsonb, output_snapshot),
    error = COALESCE(sqlc.narg('error')::jsonb, error),
    logs = CASE
        WHEN sqlc.narg('logs')::jsonb IS NULL THEN logs
        ELSE COALESCE(logs, '[]'::jsonb) || sqlc.narg('logs')::jsonb
    END,
    carrier_ref = COALESCE(sqlc.narg('carrier_ref')::jsonb, carrier_ref),
    started_at = CASE
        WHEN sqlc.arg('event_type')::text = 'node_started' THEN COALESCE(started_at, now())
        ELSE started_at
    END,
    completed_at = CASE
        WHEN sqlc.arg('event_type')::text IN ('node_succeeded','node_failed','node_cancelled','node_skipped') THEN COALESCE(completed_at, now())
        ELSE completed_at
    END,
    updated_at = now()
WHERE run_id = $1 AND node_id = $2
RETURNING *;

-- name: GetWorkflowRunNode :one
SELECT * FROM workflow_run_node
WHERE run_id = $1 AND node_id = $2;

-- name: MarkWorkflowRunNodeReviewed :one
UPDATE workflow_run_node
SET status = 'succeeded',
    output_snapshot = sqlc.arg('output_snapshot')::jsonb,
    completed_at = now(),
    updated_at = now()
WHERE run_id = $1
  AND node_id = $2
  AND status = 'pending_review'
RETURNING *;
