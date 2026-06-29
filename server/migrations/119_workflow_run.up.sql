-- workflow_run stores the user-facing progress state for one workflow run on a
-- root issue. LangGraph checkpoints are stored separately by the sidecar.
CREATE TABLE workflow_run (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id        UUID NOT NULL REFERENCES workspace(id) ON DELETE CASCADE,
    root_issue_id       UUID NOT NULL REFERENCES issue(id) ON DELETE CASCADE,
    skill_id            UUID REFERENCES skill(id) ON DELETE SET NULL,
    planner_task_id     UUID REFERENCES agent_task_queue(id) ON DELETE SET NULL,
    status              TEXT NOT NULL DEFAULT 'pending'
        CHECK (status IN ('pending','planning','running','finalizing','done','failed','cancelling','cancelled')),
    current_node        TEXT NOT NULL DEFAULT '',
    nodes_state         JSONB NOT NULL DEFAULT '{}',
    definition_snapshot JSONB NOT NULL DEFAULT '{}',
    source_skills       JSONB NOT NULL DEFAULT '[]',
    error               TEXT,
    cancel_reason       TEXT,
    cancelled_at        TIMESTAMPTZ,
    started_at          TIMESTAMPTZ,
    completed_at        TIMESTAMPTZ,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_workflow_run_root_issue ON workflow_run(root_issue_id);
CREATE INDEX idx_workflow_run_status ON workflow_run(status);
CREATE INDEX idx_workflow_run_workspace ON workflow_run(workspace_id);
CREATE INDEX idx_workflow_run_planner_task ON workflow_run(planner_task_id) WHERE planner_task_id IS NOT NULL;
