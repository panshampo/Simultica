-- Move already-applied Simultica-owned migration records out of the upstream
-- Multica numeric range. Fresh databases do not have these old records, so the
-- updates are no-ops and the 91xx migrations below will run normally.
UPDATE schema_migrations
SET version = '9117_task_message_attachment'
WHERE version = '117_task_message_attachment'
  AND NOT EXISTS (
    SELECT 1 FROM schema_migrations WHERE version = '9117_task_message_attachment'
  );

UPDATE schema_migrations
SET version = '9118_lark_thread_issue_binding'
WHERE version = '118_lark_thread_issue_binding'
  AND NOT EXISTS (
    SELECT 1 FROM schema_migrations WHERE version = '9118_lark_thread_issue_binding'
  );

UPDATE schema_migrations
SET version = '9119_workflow_run'
WHERE version = '119_workflow_run'
  AND NOT EXISTS (
    SELECT 1 FROM schema_migrations WHERE version = '9119_workflow_run'
  );

UPDATE schema_migrations
SET version = '9120_workflow_run_lifecycle_fields'
WHERE version = '120_workflow_run_lifecycle_fields'
  AND NOT EXISTS (
    SELECT 1 FROM schema_migrations WHERE version = '9120_workflow_run_lifecycle_fields'
  );

UPDATE schema_migrations
SET version = '9121_issue_origin_workflow_node'
WHERE version = '121_issue_origin_workflow_node'
  AND NOT EXISTS (
    SELECT 1 FROM schema_migrations WHERE version = '9121_issue_origin_workflow_node'
  );
