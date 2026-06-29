ALTER TABLE workflow_run
  ADD COLUMN IF NOT EXISTS planner_task_id UUID REFERENCES agent_task_queue(id) ON DELETE SET NULL,
  ADD COLUMN IF NOT EXISTS source_skills JSONB NOT NULL DEFAULT '[]',
  ADD COLUMN IF NOT EXISTS cancel_reason TEXT,
  ADD COLUMN IF NOT EXISTS cancelled_at TIMESTAMPTZ,
  ADD COLUMN IF NOT EXISTS started_at TIMESTAMPTZ,
  ADD COLUMN IF NOT EXISTS completed_at TIMESTAMPTZ;

ALTER TABLE workflow_run
  DROP CONSTRAINT IF EXISTS workflow_run_status_check;

ALTER TABLE workflow_run
  ADD CONSTRAINT workflow_run_status_check
  CHECK (status IN ('pending','planning','running','finalizing','done','failed','cancelling','cancelled'));

CREATE INDEX IF NOT EXISTS idx_workflow_run_planner_task
  ON workflow_run(planner_task_id)
  WHERE planner_task_id IS NOT NULL;
