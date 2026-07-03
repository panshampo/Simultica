DROP INDEX IF EXISTS idx_workflow_run_planner_task;

ALTER TABLE workflow_run
  DROP CONSTRAINT IF EXISTS workflow_run_status_check;

ALTER TABLE workflow_run
  ADD CONSTRAINT workflow_run_status_check
  CHECK (status IN ('pending','running','done','failed','cancelled'));

ALTER TABLE workflow_run
  DROP COLUMN IF EXISTS completed_at,
  DROP COLUMN IF EXISTS started_at,
  DROP COLUMN IF EXISTS cancelled_at,
  DROP COLUMN IF EXISTS cancel_reason,
  DROP COLUMN IF EXISTS source_skills,
  DROP COLUMN IF EXISTS planner_task_id;
