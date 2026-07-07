CREATE UNIQUE INDEX IF NOT EXISTS idx_workflow_case_active_entry_issue_unique
ON workflow_case(workspace_id, source_issue_id)
WHERE source_issue_id IS NOT NULL
  AND status <> 'archived';

ALTER TABLE workflow_run_node
  ADD COLUMN IF NOT EXISTS carrier_kind TEXT;

UPDATE workflow_run_node
SET carrier_kind = CASE dispatch
  WHEN 'subissue' THEN 'issue'
  WHEN 'main_issue_task' THEN 'issue_task'
  WHEN 'direct_subagent' THEN 'agent_runtime'
  ELSE 'inline'
END
WHERE carrier_kind IS NULL;

ALTER TABLE workflow_run_node
  ALTER COLUMN carrier_kind SET NOT NULL,
  ADD CONSTRAINT workflow_run_node_carrier_kind_check
  CHECK (carrier_kind IN ('issue','issue_task','agent_runtime','inline'));
