ALTER TABLE workflow_case
  ADD COLUMN IF NOT EXISTS online_version_id UUID REFERENCES workflow_definition_version(id) ON DELETE SET NULL;

ALTER TABLE workflow_run
  ADD COLUMN IF NOT EXISTS run_kind TEXT NOT NULL DEFAULT 'primary'
    CHECK (run_kind IN ('primary','experiment','shadow','replay','debug')),
  ADD COLUMN IF NOT EXISTS label TEXT NOT NULL DEFAULT '';

CREATE INDEX IF NOT EXISTS idx_workflow_case_online_version
  ON workflow_case(online_version_id)
  WHERE online_version_id IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_workflow_run_case_status_kind
  ON workflow_run(case_id, status, run_kind, created_at DESC)
  WHERE case_id IS NOT NULL;
