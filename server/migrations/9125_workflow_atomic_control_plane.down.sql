DROP INDEX IF EXISTS idx_workflow_run_case_status_kind;
DROP INDEX IF EXISTS idx_workflow_case_online_version;

ALTER TABLE workflow_run
  DROP COLUMN IF EXISTS label,
  DROP COLUMN IF EXISTS run_kind;

ALTER TABLE workflow_case
  DROP COLUMN IF EXISTS online_version_id;
