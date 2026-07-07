ALTER TABLE workflow_run_node
  DROP CONSTRAINT IF EXISTS workflow_run_node_carrier_kind_check,
  DROP COLUMN IF EXISTS carrier_kind;

DROP INDEX IF EXISTS idx_workflow_case_active_entry_issue_unique;
