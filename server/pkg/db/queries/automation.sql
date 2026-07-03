-- =====================
-- Automation CRUD
-- =====================

-- name: ListAutomations :many
SELECT * FROM automation
WHERE workspace_id = $1
  AND (sqlc.narg('status')::text IS NULL OR status = sqlc.narg('status'))
  AND (sqlc.narg('source_mode')::text IS NULL OR source_mode = sqlc.narg('source_mode'))
ORDER BY created_at DESC;

-- name: GetAutomation :one
SELECT * FROM automation
WHERE id = $1;

-- name: GetAutomationInWorkspace :one
SELECT * FROM automation
WHERE id = $1 AND workspace_id = $2;

-- name: CreateAutomation :one
INSERT INTO automation (
    workspace_id, title, source_mode, template_id, inline_issue_config,
    status, concurrency_policy, created_by_type, created_by_id
) VALUES (
    $1, $2, $3, sqlc.narg('template_id'), sqlc.narg('inline_issue_config'),
    $4, $5, $6, $7
) RETURNING *;

-- name: UpdateAutomation :one
UPDATE automation SET
    title = COALESCE(sqlc.narg('title'), title),
    status = COALESCE(sqlc.narg('status'), status),
    concurrency_policy = COALESCE(sqlc.narg('concurrency_policy'), concurrency_policy),
    updated_at = now()
WHERE id = $1
RETURNING *;

-- name: ReplaceAutomationInlineSource :one
UPDATE automation SET
    source_mode = 'inline',
    template_id = NULL,
    inline_issue_config = $2,
    updated_at = now()
WHERE id = $1
RETURNING *;

-- name: ReplaceAutomationTemplateSource :one
UPDATE automation SET
    source_mode = 'template',
    template_id = $2,
    inline_issue_config = NULL,
    updated_at = now()
WHERE id = $1
RETURNING *;

-- name: DeleteAutomation :exec
DELETE FROM automation WHERE id = $1;

-- name: UpdateAutomationLastRunAt :exec
UPDATE automation SET last_run_at = now(), updated_at = now()
WHERE id = $1;

-- =====================
-- Automation Trigger CRUD
-- =====================

-- name: ListAutomationTriggers :many
SELECT * FROM automation_trigger
WHERE automation_id = $1
ORDER BY created_at ASC;

-- name: GetAutomationTrigger :one
SELECT * FROM automation_trigger
WHERE id = $1;

-- name: CreateAutomationTrigger :one
INSERT INTO automation_trigger (
    automation_id, kind, enabled, cron_expression, timezone,
    next_run_at, webhook_token, label, provider, signing_secret, event_filters
) VALUES (
    $1, $2, $3, sqlc.narg('cron_expression'), sqlc.narg('timezone'),
    sqlc.narg('next_run_at'), sqlc.narg('webhook_token'), sqlc.narg('label'),
    COALESCE(sqlc.narg('provider')::text, 'generic'),
    sqlc.narg('signing_secret'),
    sqlc.narg('event_filters')
) RETURNING *;

-- name: UpdateAutomationTrigger :one
UPDATE automation_trigger SET
    enabled = COALESCE(sqlc.narg('enabled')::boolean, enabled),
    cron_expression = COALESCE(sqlc.narg('cron_expression'), cron_expression),
    timezone = COALESCE(sqlc.narg('timezone'), timezone),
    next_run_at = sqlc.narg('next_run_at'),
    label = COALESCE(sqlc.narg('label'), label),
    provider = COALESCE(sqlc.narg('provider'), provider),
    event_filters = COALESCE(sqlc.narg('event_filters'), event_filters),
    updated_at = now()
WHERE id = $1
RETURNING *;

-- name: DeleteAutomationTrigger :exec
DELETE FROM automation_trigger WHERE id = $1;

-- name: AdvanceAutomationTriggerNextRun :exec
UPDATE automation_trigger
SET next_run_at = sqlc.narg('next_run_at'),
    last_fired_at = now(),
    updated_at = now()
WHERE id = $1;

-- name: GetAutomationWebhookTriggerByToken :one
SELECT t.*, a.workspace_id AS automation_workspace_id
FROM automation_trigger t
JOIN automation a ON a.id = t.automation_id
WHERE t.kind = 'webhook'
  AND t.webhook_token = $1;

-- name: TouchAutomationTriggerFiredAt :exec
UPDATE automation_trigger
SET last_fired_at = now(),
    updated_at = now()
WHERE id = $1;

-- name: RotateAutomationTriggerWebhookToken :one
UPDATE automation_trigger
SET webhook_token = $2,
    updated_at = now()
WHERE id = $1
  AND kind = 'webhook'
RETURNING *;

-- name: SetAutomationTriggerWebhookToken :one
UPDATE automation_trigger
SET webhook_token = $2,
    updated_at = now()
WHERE id = $1
RETURNING *;

-- name: SetAutomationTriggerSigningSecret :one
UPDATE automation_trigger
SET signing_secret = sqlc.narg('signing_secret'),
    updated_at = now()
WHERE id = $1
  AND kind = 'webhook'
RETURNING *;

-- =====================
-- Automation Run Management
-- =====================

-- name: CreateAutomationRun :one
INSERT INTO automation_run (
    automation_id, trigger_id, source, source_mode_snapshot, status,
    trigger_payload, resolved_issue_payload, template_snapshot
) VALUES (
    $1, sqlc.narg('trigger_id'), $2, $3, $4,
    sqlc.narg('trigger_payload'), sqlc.narg('resolved_issue_payload'), sqlc.narg('template_snapshot')
) RETURNING *;

-- name: GetAutomationRun :one
SELECT * FROM automation_run
WHERE id = $1;

-- name: ListAutomationRuns :many
SELECT * FROM automation_run
WHERE automation_id = $1
ORDER BY created_at DESC
LIMIT $2 OFFSET $3;

-- name: UpdateAutomationRunIssueCreated :one
UPDATE automation_run
SET status = 'issue_created', issue_id = $2
WHERE id = $1
RETURNING *;

-- name: UpdateAutomationRunRunning :one
UPDATE automation_run
SET status = 'running', task_id = $2
WHERE id = $1
RETURNING *;

-- name: UpdateAutomationRunCompleted :one
UPDATE automation_run
SET status = 'completed', completed_at = now(), result = sqlc.narg('result')
WHERE id = $1
RETURNING *;

-- name: UpdateAutomationRunFailed :one
UPDATE automation_run
SET status = 'failed', completed_at = now(), failure_reason = $2
WHERE id = $1
RETURNING *;

-- name: UpdateAutomationRunCancelled :one
UPDATE automation_run
SET status = 'cancelled', completed_at = now(), failure_reason = sqlc.narg('failure_reason')
WHERE id = $1
RETURNING *;

-- name: UpdateAutomationRunSkipped :one
UPDATE automation_run
SET status = 'skipped', completed_at = now(), failure_reason = $2
WHERE id = $1
RETURNING *;

-- name: UpdateAutomationRunSkippedWithResult :one
UPDATE automation_run
SET status = 'skipped',
    completed_at = now(),
    failure_reason = $2,
    result = sqlc.narg('result')
WHERE id = $1
RETURNING *;

-- =====================
-- Scheduler Queries
-- =====================

-- name: ClaimDueAutomationScheduleTriggers :many
UPDATE automation_trigger t
SET next_run_at = NULL
FROM automation a
WHERE t.automation_id = a.id
  AND t.kind = 'schedule'
  AND t.enabled = true
  AND t.next_run_at IS NOT NULL
  AND t.next_run_at <= now()
  AND a.status = 'active'
RETURNING t.*, a.workspace_id AS automation_workspace_id;

-- name: RecoverLostAutomationTriggers :many
-- Finds schedule triggers that were claimed (next_run_at = NULL) but never
-- advanced, typically due to a scheduler crash. Returns them so the scheduler
-- can recompute next_run_at.
SELECT t.*, a.workspace_id AS automation_workspace_id
FROM automation_trigger t
JOIN automation a ON t.automation_id = a.id
WHERE t.kind = 'schedule'
  AND t.enabled = true
  AND t.next_run_at IS NULL
  AND t.cron_expression IS NOT NULL
  AND a.status = 'active';

-- =====================
-- Run lookup by linked entities
-- =====================

-- name: GetAutomationRunByIssue :one
SELECT * FROM automation_run
WHERE issue_id = $1 AND status IN ('issue_created', 'running')
LIMIT 1;

-- name: GetAutomationRunByTask :one
SELECT * FROM automation_run
WHERE task_id = $1 AND status IN ('running', 'completed', 'failed', 'cancelled', 'skipped')
LIMIT 1;
