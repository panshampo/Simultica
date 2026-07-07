CREATE TABLE workflow_case (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id UUID NOT NULL REFERENCES workspace(id) ON DELETE CASCADE,
    title TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    source_issue_id UUID REFERENCES issue(id) ON DELETE SET NULL,
    owner_agent_id UUID REFERENCES agent(id) ON DELETE SET NULL,
    status TEXT NOT NULL DEFAULT 'draft'
        CHECK (status IN ('draft','planned','running','paused','succeeded','failed','cancelling','cancelled','archived')),
    current_run_id UUID,
    created_by TEXT,
    updated_by TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_workflow_case_workspace ON workflow_case(workspace_id, updated_at DESC);
CREATE INDEX idx_workflow_case_source_issue ON workflow_case(source_issue_id) WHERE source_issue_id IS NOT NULL;
CREATE INDEX idx_workflow_case_status ON workflow_case(workspace_id, status);

CREATE TABLE workflow_definition (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id UUID NOT NULL REFERENCES workspace(id) ON DELETE CASCADE,
    case_id UUID NOT NULL REFERENCES workflow_case(id) ON DELETE CASCADE,
    draft_json JSONB NOT NULL DEFAULT '{}',
    source_templates JSONB NOT NULL DEFAULT '[]',
    status TEXT NOT NULL DEFAULT 'draft'
        CHECK (status IN ('draft','archived')),
    created_by TEXT,
    updated_by TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (case_id)
);

CREATE INDEX idx_workflow_definition_case ON workflow_definition(case_id);

CREATE TABLE workflow_definition_version (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id UUID NOT NULL REFERENCES workspace(id) ON DELETE CASCADE,
    case_id UUID NOT NULL REFERENCES workflow_case(id) ON DELETE CASCADE,
    definition_id UUID NOT NULL REFERENCES workflow_definition(id) ON DELETE CASCADE,
    version INTEGER NOT NULL,
    snapshot_json JSONB NOT NULL,
    source_skills JSONB NOT NULL DEFAULT '[]',
    validation_report JSONB NOT NULL DEFAULT '{}',
    confirmed_by TEXT,
    confirmed_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (definition_id, version)
);

CREATE INDEX idx_workflow_definition_version_case ON workflow_definition_version(case_id, version DESC);

ALTER TABLE workflow_run
    ALTER COLUMN root_issue_id DROP NOT NULL;

ALTER TABLE workflow_run
    ADD COLUMN IF NOT EXISTS case_id UUID REFERENCES workflow_case(id) ON DELETE CASCADE,
    ADD COLUMN IF NOT EXISTS definition_version_id UUID REFERENCES workflow_definition_version(id) ON DELETE SET NULL;

CREATE INDEX IF NOT EXISTS idx_workflow_run_case ON workflow_run(case_id, created_at DESC);

CREATE TABLE workflow_run_node (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id UUID NOT NULL REFERENCES workspace(id) ON DELETE CASCADE,
    case_id UUID NOT NULL REFERENCES workflow_case(id) ON DELETE CASCADE,
    run_id UUID NOT NULL REFERENCES workflow_run(id) ON DELETE CASCADE,
    node_id TEXT NOT NULL,
    node_type TEXT NOT NULL,
    dispatch TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'pending'
        CHECK (status IN ('pending','running','blocked','succeeded','failed','cancelling','cancelled','skipped')),
    attempt INTEGER NOT NULL DEFAULT 0,
    input_snapshot JSONB,
    output_snapshot JSONB,
    error JSONB,
    logs JSONB NOT NULL DEFAULT '[]',
    carrier_ref JSONB,
    started_at TIMESTAMPTZ,
    completed_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (run_id, node_id)
);

CREATE INDEX idx_workflow_run_node_run ON workflow_run_node(run_id);
CREATE INDEX idx_workflow_run_node_case ON workflow_run_node(case_id);
CREATE INDEX idx_workflow_run_node_status ON workflow_run_node(workspace_id, status);

CREATE TABLE workflow_run_node_event (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id UUID NOT NULL REFERENCES workspace(id) ON DELETE CASCADE,
    case_id UUID NOT NULL REFERENCES workflow_case(id) ON DELETE CASCADE,
    run_id UUID NOT NULL REFERENCES workflow_run(id) ON DELETE CASCADE,
    node_id TEXT NOT NULL,
    event_type TEXT NOT NULL
        CHECK (event_type IN (
            'node_started',
            'node_blocked',
            'node_succeeded',
            'node_failed',
            'node_cancelled',
            'node_skipped',
            'node_carrier_attached',
            'node_log_appended'
        )),
    attempt INTEGER NOT NULL DEFAULT 0,
    input_snapshot JSONB,
    output_snapshot JSONB,
    error JSONB,
    logs JSONB NOT NULL DEFAULT '[]',
    carrier_ref JSONB,
    sequence INTEGER,
    occurred_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_workflow_run_node_event_run ON workflow_run_node_event(run_id, created_at ASC);
CREATE INDEX idx_workflow_run_node_event_node ON workflow_run_node_event(run_id, node_id, created_at ASC);
