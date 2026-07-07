DROP INDEX IF EXISTS idx_workflow_run_node_event_node;
DROP INDEX IF EXISTS idx_workflow_run_node_event_run;
DROP TABLE IF EXISTS workflow_run_node_event;

DROP INDEX IF EXISTS idx_workflow_run_node_status;
DROP INDEX IF EXISTS idx_workflow_run_node_case;
DROP INDEX IF EXISTS idx_workflow_run_node_run;
DROP TABLE IF EXISTS workflow_run_node;

DROP INDEX IF EXISTS idx_workflow_run_case;
ALTER TABLE workflow_run
    DROP COLUMN IF EXISTS definition_version_id,
    DROP COLUMN IF EXISTS case_id;

DROP INDEX IF EXISTS idx_workflow_definition_version_case;
DROP TABLE IF EXISTS workflow_definition_version;

DROP INDEX IF EXISTS idx_workflow_definition_case;
DROP TABLE IF EXISTS workflow_definition;

DROP INDEX IF EXISTS idx_workflow_case_status;
DROP INDEX IF EXISTS idx_workflow_case_source_issue;
DROP INDEX IF EXISTS idx_workflow_case_workspace;
DROP TABLE IF EXISTS workflow_case;

-- Do not restore root_issue_id NOT NULL in down migration because existing
-- standalone case runs may have written nulls before rollback.
