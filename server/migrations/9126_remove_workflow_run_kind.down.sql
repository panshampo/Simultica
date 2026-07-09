DROP INDEX IF EXISTS idx_workflow_run_case_status_created;

ALTER TABLE workflow_run
  ADD COLUMN IF NOT EXISTS run_kind TEXT NOT NULL DEFAULT 'primary'
    CHECK (run_kind IN ('primary','experiment','shadow','replay','debug'));

CREATE INDEX IF NOT EXISTS idx_workflow_run_case_status_kind
  ON workflow_run(case_id, status, run_kind, created_at DESC)
  WHERE case_id IS NOT NULL;
