-- Issue templates and dual-mode automation v2.
--
-- This schema intentionally coexists with the legacy autopilot tables during
-- handler migration. Do not remove autopilot columns or constraints here.

ALTER TABLE project
    ADD CONSTRAINT project_id_workspace_id_key UNIQUE (id, workspace_id);

CREATE TABLE IF NOT EXISTS issue_template (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id UUID NOT NULL REFERENCES workspace(id) ON DELETE CASCADE,
    project_id UUID,
    title TEXT NOT NULL,
    issue_title_template TEXT NOT NULL,
    issue_body_template TEXT,
    assignee_type TEXT NOT NULL CHECK (assignee_type IN ('agent', 'squad')),
    assignee_id UUID NOT NULL,
    priority TEXT NOT NULL DEFAULT 'medium'
        CHECK (priority IN ('urgent', 'high', 'medium', 'low', 'none')),
    labels JSONB,
    default_metadata JSONB,
    execution_spec JSONB,
    created_by_type TEXT NOT NULL CHECK (created_by_type IN ('member', 'agent')),
    created_by_id UUID NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (id, workspace_id),
    CONSTRAINT issue_template_project_workspace_fkey
        FOREIGN KEY (project_id, workspace_id)
        REFERENCES project(id, workspace_id)
        ON DELETE SET NULL (project_id)
);

CREATE INDEX IF NOT EXISTS idx_issue_template_workspace
    ON issue_template (workspace_id, updated_at DESC);
CREATE INDEX IF NOT EXISTS idx_issue_template_project
    ON issue_template (workspace_id, project_id, updated_at DESC)
    WHERE project_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_issue_template_assignee
    ON issue_template (assignee_type, assignee_id);

CREATE TABLE IF NOT EXISTS automation (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id UUID NOT NULL REFERENCES workspace(id) ON DELETE CASCADE,
    title TEXT NOT NULL,
    source_mode TEXT NOT NULL CHECK (source_mode IN ('inline', 'template')),
    template_id UUID,
    inline_issue_config JSONB,
    status TEXT NOT NULL DEFAULT 'active'
        CHECK (status IN ('active', 'paused', 'archived')),
    concurrency_policy TEXT NOT NULL DEFAULT 'skip'
        CHECK (concurrency_policy IN ('skip', 'queue', 'replace')),
    created_by_type TEXT NOT NULL CHECK (created_by_type IN ('member', 'agent')),
    created_by_id UUID NOT NULL,
    last_run_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (id, workspace_id),
    CONSTRAINT automation_template_workspace_fkey
        FOREIGN KEY (template_id, workspace_id)
        REFERENCES issue_template(id, workspace_id)
        ON DELETE RESTRICT,
    CONSTRAINT automation_source_fields_check CHECK (
        (
            source_mode = 'inline'
            AND inline_issue_config IS NOT NULL
            AND template_id IS NULL
        )
        OR
        (
            source_mode = 'template'
            AND template_id IS NOT NULL
            AND inline_issue_config IS NULL
        )
    )
);

CREATE INDEX IF NOT EXISTS idx_automation_workspace_status
    ON automation (workspace_id, status, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_automation_template
    ON automation (template_id)
    WHERE template_id IS NOT NULL;

CREATE TABLE IF NOT EXISTS automation_trigger (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    automation_id UUID NOT NULL REFERENCES automation(id) ON DELETE CASCADE,
    kind TEXT NOT NULL CHECK (kind IN ('schedule', 'webhook', 'api')),
    enabled BOOLEAN NOT NULL DEFAULT true,
    cron_expression TEXT,
    timezone TEXT DEFAULT 'UTC',
    next_run_at TIMESTAMPTZ,
    webhook_token TEXT,
    label TEXT,
    provider TEXT NOT NULL DEFAULT 'generic',
    signing_secret TEXT,
    event_filters JSONB,
    last_fired_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (id, automation_id)
);

CREATE INDEX IF NOT EXISTS idx_automation_trigger_automation
    ON automation_trigger (automation_id, created_at ASC);
CREATE INDEX IF NOT EXISTS idx_automation_trigger_next_run
    ON automation_trigger (next_run_at)
    WHERE enabled = true AND kind = 'schedule';
CREATE UNIQUE INDEX IF NOT EXISTS idx_automation_trigger_webhook_token
    ON automation_trigger (webhook_token)
    WHERE kind = 'webhook' AND webhook_token IS NOT NULL;

CREATE TABLE IF NOT EXISTS automation_run (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    automation_id UUID NOT NULL REFERENCES automation(id) ON DELETE CASCADE,
    trigger_id UUID,
    source TEXT NOT NULL CHECK (source IN ('schedule', 'manual', 'webhook', 'api')),
    source_mode_snapshot TEXT NOT NULL CHECK (source_mode_snapshot IN ('inline', 'template')),
    status TEXT NOT NULL DEFAULT 'pending'
        CHECK (status IN ('pending', 'issue_created', 'running', 'skipped', 'completed', 'failed', 'cancelled')),
    issue_id UUID REFERENCES issue(id) ON DELETE SET NULL,
    task_id UUID REFERENCES agent_task_queue(id) ON DELETE SET NULL,
    triggered_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    completed_at TIMESTAMPTZ,
    failure_reason TEXT,
    trigger_payload JSONB,
    resolved_issue_payload JSONB,
    template_snapshot JSONB,
    result JSONB,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT automation_run_trigger_automation_fkey
        FOREIGN KEY (trigger_id, automation_id)
        REFERENCES automation_trigger(id, automation_id)
        ON DELETE SET NULL (trigger_id),
    CONSTRAINT automation_run_template_snapshot_check CHECK (
        source_mode_snapshot <> 'template'
        OR template_snapshot IS NOT NULL
    )
);

CREATE INDEX IF NOT EXISTS idx_automation_run_automation
    ON automation_run (automation_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_automation_run_status
    ON automation_run (automation_id, status)
    WHERE status IN ('pending', 'issue_created', 'running');
CREATE INDEX IF NOT EXISTS idx_automation_run_issue
    ON automation_run (issue_id)
    WHERE issue_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_automation_run_task
    ON automation_run (task_id)
    WHERE task_id IS NOT NULL;

ALTER TABLE issue
    ADD COLUMN IF NOT EXISTS origin_type TEXT,
    ADD COLUMN IF NOT EXISTS origin_id UUID,
    ADD COLUMN IF NOT EXISTS automation_run_id UUID REFERENCES automation_run(id) ON DELETE SET NULL,
    ADD COLUMN IF NOT EXISTS issue_template_id UUID,
    ADD COLUMN IF NOT EXISTS issue_template_snapshot JSONB;

ALTER TABLE issue DROP CONSTRAINT IF EXISTS issue_origin_type_check;
ALTER TABLE issue ADD CONSTRAINT issue_origin_type_check
    CHECK (origin_type IN ('autopilot', 'quick_create', 'lark_chat', 'workflow_node', 'automation'));

ALTER TABLE issue DROP CONSTRAINT IF EXISTS issue_template_snapshot_check;
ALTER TABLE issue ADD CONSTRAINT issue_template_snapshot_check
    CHECK (issue_template_id IS NULL OR issue_template_snapshot IS NOT NULL);

ALTER TABLE issue DROP CONSTRAINT IF EXISTS issue_template_workspace_fkey;
ALTER TABLE issue ADD CONSTRAINT issue_template_workspace_fkey
    FOREIGN KEY (issue_template_id, workspace_id)
    REFERENCES issue_template(id, workspace_id)
    ON DELETE SET NULL (issue_template_id);

CREATE INDEX IF NOT EXISTS idx_issue_automation_run
    ON issue (automation_run_id)
    WHERE automation_run_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_issue_template_reference
    ON issue (issue_template_id)
    WHERE issue_template_id IS NOT NULL;

ALTER TABLE agent_task_queue
    ADD COLUMN IF NOT EXISTS automation_run_id UUID REFERENCES automation_run(id) ON DELETE SET NULL;

CREATE INDEX IF NOT EXISTS idx_agent_task_queue_automation_run
    ON agent_task_queue (automation_run_id)
    WHERE automation_run_id IS NOT NULL;

ALTER TABLE webhook_delivery
    ALTER COLUMN autopilot_id DROP NOT NULL,
    ALTER COLUMN trigger_id DROP NOT NULL,
    ADD COLUMN IF NOT EXISTS automation_id UUID REFERENCES automation(id) ON DELETE CASCADE,
    ADD COLUMN IF NOT EXISTS automation_trigger_id UUID REFERENCES automation_trigger(id) ON DELETE CASCADE,
    ADD COLUMN IF NOT EXISTS automation_run_id UUID REFERENCES automation_run(id) ON DELETE SET NULL;

ALTER TABLE webhook_delivery DROP CONSTRAINT IF EXISTS webhook_delivery_parent_family_check;
ALTER TABLE webhook_delivery ADD CONSTRAINT webhook_delivery_parent_family_check CHECK (
    (
        autopilot_id IS NOT NULL
        AND trigger_id IS NOT NULL
        AND automation_id IS NULL
        AND automation_trigger_id IS NULL
    )
    OR
    (
        autopilot_id IS NULL
        AND trigger_id IS NULL
        AND automation_id IS NOT NULL
        AND automation_trigger_id IS NOT NULL
    )
);

CREATE INDEX IF NOT EXISTS idx_webhook_delivery_automation
    ON webhook_delivery(automation_id, created_at DESC)
    WHERE automation_id IS NOT NULL;

CREATE UNIQUE INDEX IF NOT EXISTS idx_webhook_delivery_automation_dedupe
    ON webhook_delivery(automation_trigger_id, dedupe_key)
    WHERE dedupe_key IS NOT NULL
      AND automation_trigger_id IS NOT NULL
      AND status NOT IN ('rejected', 'failed');

CREATE INDEX IF NOT EXISTS idx_webhook_delivery_automation_run
    ON webhook_delivery(automation_run_id)
    WHERE automation_run_id IS NOT NULL;
