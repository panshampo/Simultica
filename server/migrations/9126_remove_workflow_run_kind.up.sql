DROP INDEX IF EXISTS idx_workflow_run_case_status_kind;

ALTER TABLE workflow_run
  DROP COLUMN IF EXISTS run_kind;

CREATE INDEX IF NOT EXISTS idx_workflow_run_case_status_created
  ON workflow_run(case_id, status, created_at DESC)
  WHERE case_id IS NOT NULL;
