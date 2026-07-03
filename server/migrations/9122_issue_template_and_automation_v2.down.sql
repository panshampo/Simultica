DROP INDEX IF EXISTS idx_agent_task_queue_automation_run;
ALTER TABLE agent_task_queue
    DROP COLUMN IF EXISTS automation_run_id;

DROP INDEX IF EXISTS idx_webhook_delivery_automation_run;
DROP INDEX IF EXISTS idx_webhook_delivery_automation_dedupe;
DROP INDEX IF EXISTS idx_webhook_delivery_automation;
DELETE FROM webhook_delivery WHERE automation_id IS NOT NULL;
ALTER TABLE webhook_delivery DROP CONSTRAINT IF EXISTS webhook_delivery_parent_family_check;
ALTER TABLE webhook_delivery
    DROP COLUMN IF EXISTS automation_run_id,
    DROP COLUMN IF EXISTS automation_trigger_id,
    DROP COLUMN IF EXISTS automation_id;
ALTER TABLE webhook_delivery
    ALTER COLUMN autopilot_id SET NOT NULL,
    ALTER COLUMN trigger_id SET NOT NULL;

DROP INDEX IF EXISTS idx_issue_template_reference;
DROP INDEX IF EXISTS idx_issue_automation_run;

ALTER TABLE issue
    DROP COLUMN IF EXISTS issue_template_snapshot,
    DROP COLUMN IF EXISTS issue_template_id,
    DROP COLUMN IF EXISTS automation_run_id;

UPDATE issue
SET origin_type = NULL,
    origin_id = NULL
WHERE origin_type = 'automation';

ALTER TABLE issue DROP CONSTRAINT IF EXISTS issue_origin_type_check;
ALTER TABLE issue ADD CONSTRAINT issue_origin_type_check
    CHECK (origin_type IN ('autopilot', 'quick_create', 'lark_chat', 'workflow_node'));

DROP INDEX IF EXISTS idx_automation_run_task;
DROP INDEX IF EXISTS idx_automation_run_issue;
DROP INDEX IF EXISTS idx_automation_run_status;
DROP INDEX IF EXISTS idx_automation_run_automation;
DROP TABLE IF EXISTS automation_run;

DROP INDEX IF EXISTS idx_automation_trigger_webhook_token;
DROP INDEX IF EXISTS idx_automation_trigger_next_run;
DROP INDEX IF EXISTS idx_automation_trigger_automation;
DROP TABLE IF EXISTS automation_trigger;

DROP INDEX IF EXISTS idx_automation_template;
DROP INDEX IF EXISTS idx_automation_workspace_status;
DROP TABLE IF EXISTS automation;

DROP INDEX IF EXISTS idx_issue_template_assignee;
DROP INDEX IF EXISTS idx_issue_template_project;
DROP INDEX IF EXISTS idx_issue_template_workspace;
DROP TABLE IF EXISTS issue_template;

ALTER TABLE project
    DROP CONSTRAINT IF EXISTS project_id_workspace_id_key;
