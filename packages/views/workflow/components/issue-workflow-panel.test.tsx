import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import type {
  IssueWorkflowContext,
  WorkflowCase,
  WorkflowDefinition,
  WorkflowDefinitionVersion,
  WorkflowRun,
} from "@multica/core/workflow/types";

const mocks = vi.hoisted(() => ({
  getIssueWorkflowContext: vi.fn(),
  getWorkflowCase: vi.fn(),
  getWorkflowCaseCurrentRun: vi.fn(),
  listWorkflowCaseDefinitionVersions: vi.fn(),
  listWorkflowCaseRuns: vi.fn(),
  reviewWorkflowRunStep: vi.fn(),
  useWSEvent: vi.fn(),
}));

vi.mock("@multica/core/api", () => ({
  api: {
    getIssueWorkflowContext: mocks.getIssueWorkflowContext,
    getWorkflowCase: mocks.getWorkflowCase,
    getWorkflowCaseCurrentRun: mocks.getWorkflowCaseCurrentRun,
    listWorkflowCaseDefinitionVersions: mocks.listWorkflowCaseDefinitionVersions,
    listWorkflowCaseRuns: mocks.listWorkflowCaseRuns,
    reviewWorkflowRunStep: mocks.reviewWorkflowRunStep,
  },
}));

vi.mock("@multica/core/hooks", () => ({
  useWorkspaceId: () => "ws-1",
}));

vi.mock("@multica/core/realtime", () => ({
  useWSEvent: mocks.useWSEvent,
}));

vi.mock("@multica/core/paths", () => ({
  useWorkspacePaths: () => ({
    issueDetail: (id: string) => `/issues/${id}`,
    workflowCaseDetail: (id: string) => `/workflow-cases/${id}`,
    workflowCaseVersionDetail: (caseId: string, versionId: string) => `/workflow-cases/${caseId}/versions/${versionId}`,
    workflowCaseRunDetail: (caseId: string, runId: string, nodeId?: string) => {
      const base = `/workflow-cases/${caseId}/runs/${runId}`;
      return nodeId ? `${base}?node_id=${nodeId}` : base;
    },
  }),
}));

vi.mock("../../navigation", () => ({
  AppLink: ({ href, children, className, ...props }: { href: string; children: React.ReactNode; className?: string }) => (
    <a className={className} href={href} {...props}>{children}</a>
  ),
}));

vi.mock("./workflow-canvas", () => ({
  WorkflowCanvas: ({ definition, runStatus, selectedNodeId, onSelectNode }: {
    definition: WorkflowDefinition;
    runStatus?: string;
    selectedNodeId?: string | null;
    onSelectNode?: (id: string) => void;
  }) => (
    <button
      type="button"
      data-testid="workflow-canvas"
      data-status={runStatus ?? ""}
      data-selected={selectedNodeId ?? ""}
      onClick={() => onSelectNode?.("implement")}
    >
      {definition.meta.name}
    </button>
  ),
}));

import { IssueWorkflowPanel } from "./issue-workflow-panel";

describe("IssueWorkflowPanel", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mocks.getIssueWorkflowContext.mockResolvedValue(makeContext({ role: "none" }));
    mocks.getWorkflowCase.mockResolvedValue(makeCase());
    mocks.getWorkflowCaseCurrentRun.mockResolvedValue(null);
    mocks.listWorkflowCaseDefinitionVersions.mockResolvedValue([makeVersion()]);
    mocks.listWorkflowCaseRuns.mockResolvedValue([]);
    mocks.reviewWorkflowRunStep.mockResolvedValue({
      run_id: "run-1",
      step_id: "review",
      decision: "approved",
      reviewed_by: "user-1",
      reviewed_at: "2026-07-11T00:00:00Z",
    });
  });

  it("shows read-only native issue mode when an issue is not managed by a WorkflowCase", async () => {
    mocks.getIssueWorkflowContext.mockResolvedValue(makeContext({ role: "none" }));

    renderPanel();

    expect(await screen.findByText("Runtime workflow")).toBeInTheDocument();
    expect(screen.getByText("Native issue mode")).toBeInTheDocument();
    expect(await screen.findByText("WorkflowCase not created")).toBeInTheDocument();
    expect(screen.getByText("Create or attach a WorkflowCase from the WorkflowCase control plane.")).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Create WorkflowCase from this issue" })).not.toBeInTheDocument();
    expect(screen.queryByText("Managed by WorkflowCase")).not.toBeInTheDocument();
    expect(screen.queryByText("Run workflow")).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Stop workflow" })).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Continue workflow" })).not.toBeInTheDocument();
  });

  it("shows entry issue workflow context and links to the case", async () => {
    mocks.getIssueWorkflowContext.mockResolvedValue(makeContext({
      role: "entry_issue",
      workflow_case_id: "case-1",
      workflow_run_id: "run-1",
      carrier_kind: "issue",
    }));

    renderPanel();

    expect(await screen.findByText("Managed by WorkflowCase")).toBeInTheDocument();
    expect(screen.getByText("Role: Entry issue")).toBeInTheDocument();
    expect(screen.getByText("Carrier: issue")).toBeInTheDocument();
    expect(screen.getByText("Run run-1")).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "Open case" })).toHaveAttribute("href", "/workflow-cases/case-1");
    expect(screen.queryByRole("button", { name: "Stop workflow" })).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Continue workflow" })).not.toBeInTheDocument();
  });

  it("shows node issue workflow context with carrier and node id", async () => {
    mocks.getIssueWorkflowContext.mockResolvedValue(makeContext({
      role: "node_issue",
      workflow_case_id: "case-1",
      workflow_run_id: "run-1",
      workflow_node_id: "implement-node-123456",
      carrier_kind: "issue_task",
    }));

    renderPanel();

    expect(await screen.findByText("Managed by WorkflowCase")).toBeInTheDocument();
    expect(screen.getByText("Role: Node issue")).toBeInTheDocument();
    expect(screen.getByText("Carrier: issue_task")).toBeInTheDocument();
    expect(screen.getByText("Node implemen")).toBeInTheDocument();
    expect(screen.queryByRole("link", { name: "Open Run at Node" })).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Stop workflow" })).not.toBeInTheDocument();
  });

  it("does not show Open Run at Node for an entry issue", async () => {
    mocks.getIssueWorkflowContext.mockResolvedValue(makeContext({
      role: "entry_issue",
      workflow_case_id: "case-1",
      workflow_run_id: "run-1",
      carrier_kind: "issue",
    }));

    renderPanel();

    expect(await screen.findByText("Managed by WorkflowCase")).toBeInTheDocument();
    expect(screen.queryByRole("link", { name: "Open Run at Node" })).not.toBeInTheDocument();
  });

  it.each(["issue", "issue_task", "agent_runtime", "inline"] as const)(
    "shows canonical carrier name %s",
    async (carrierKind) => {
      mocks.getIssueWorkflowContext.mockResolvedValue(makeContext({
        role: "node_issue",
        workflow_case_id: "case-1",
        carrier_kind: carrierKind,
      }));

      renderPanel();

      expect(await screen.findByText(`Carrier: ${carrierKind}`)).toBeInTheDocument();
    },
  );

  it("embeds the current run detail in the workflow tab by default", async () => {
    mocks.getIssueWorkflowContext.mockResolvedValue(makeContext({
      role: "entry_issue",
      workflow_case_id: "case-1",
      workflow_run_id: "run-1",
      carrier_kind: "issue",
    }));
    mocks.getWorkflowCaseCurrentRun.mockResolvedValue(makeRun());
    mocks.listWorkflowCaseRuns.mockResolvedValue([makeRun()]);

    renderPanel();

    expect(await screen.findByText("Current investigation run")).toBeInTheDocument();
    expect(screen.getByTestId("workflow-canvas")).toHaveAttribute("data-status", "running");
    expect(screen.getByText("Run graph")).toBeInTheDocument();
    expect(screen.getByText("Nodes")).toBeInTheDocument();
    expect(screen.queryByRole("link", { name: "Open Run at Node" })).not.toBeInTheDocument();
  });

  it("preselects the workflow node when embedding a current run for a node issue", async () => {
    mocks.getIssueWorkflowContext.mockResolvedValue(makeContext({
      role: "node_issue",
      workflow_case_id: "case-1",
      workflow_run_id: "run-1",
      workflow_node_id: "implement",
      carrier_kind: "issue",
    }));
    mocks.getWorkflowCaseCurrentRun.mockResolvedValue(makeRun());
    mocks.listWorkflowCaseRuns.mockResolvedValue([makeRun()]);

    renderPanel();

    const canvas = await screen.findByTestId("workflow-canvas");
    expect(canvas).toHaveAttribute("data-selected", "implement");
    expect(screen.getByText("Node detail")).toBeInTheDocument();
  });
});

function renderPanel() {
  const qc = new QueryClient({
    defaultOptions: {
      queries: { retry: false },
      mutations: { retry: false },
    },
  });
  return render(
    <QueryClientProvider client={qc}>
      <IssueWorkflowPanel wsId="ws-1" issueId="issue-1" />
    </QueryClientProvider>,
  );
}

function makeContext(overrides: Partial<IssueWorkflowContext> = {}): IssueWorkflowContext {
  return {
    role: "none",
    workflow_case_id: null,
    workflow_run_id: null,
    workflow_node_id: null,
    carrier_kind: null,
    ...overrides,
  };
}

const definition: WorkflowDefinition = {
  meta: { name: "Frontend Investigation", version: "1" },
  state: { fields: [] },
  nodes: [{
    id: "implement",
    type: "agent",
    dispatch: "subissue",
    agent: "frontend-bug-investigation",
    inputs: ["task"],
    outputs: ["investigation_report"],
    config: {
      system: "Investigate and verify the frontend bug.",
    },
  }],
  routing: [{ from: "START", to: "implement" }],
};

function makeCase(): WorkflowCase {
  return {
    id: "case-1",
    workspace_id: "ws-1",
    title: "Frontend bug workflow",
    description: "",
    entry_issue_id: "issue-1",
    source_issue_id: "issue-1",
    owner_agent_id: null,
    status: "running",
    online_version_id: "version-1",
    current_run_id: "run-1",
    created_at: "2026-07-11T00:00:00Z",
    updated_at: "2026-07-11T00:00:00Z",
  };
}

function makeVersion(): WorkflowDefinitionVersion {
  return {
    id: "version-1",
    workspace_id: "ws-1",
    case_id: "case-1",
    definition_id: "definition-1",
    version: 1,
    snapshot_json: definition,
    source_skills: [],
    validation_report: { valid: true, errors: [], warnings: [] },
    confirmed_by: "user-1",
    confirmed_at: "2026-07-11T00:00:00Z",
    created_at: "2026-07-11T00:00:00Z",
  };
}

function makeRun(): WorkflowRun {
  return {
    id: "run-1",
    root_issue_id: "issue-1",
    case_id: "case-1",
    definition_version_id: "version-1",
    skill_id: null,
    status: "running",
    label: "Current investigation run",
    current_node: "implement",
    nodes_state: {
      implement: {
        status: "running",
        sub_issue_id: "sub-1",
        started_at: "2026-07-11T00:00:00Z",
        ended_at: null,
        error: null,
      },
    },
    nodes: [{
      id: "run-node-1",
      run_id: "run-1",
      node_id: "implement",
      node_type: "agent",
      dispatch: "subissue",
      carrier_kind: "issue",
      status: "running",
      attempt: 1,
      input_snapshot: { task: "Find the bug" },
      output_snapshot: null,
      error: null,
      logs: [],
      carrier_ref: { issue_id: "sub-1" },
      started_at: "2026-07-11T00:00:00Z",
      completed_at: null,
      updated_at: "2026-07-11T00:01:00Z",
    }],
    definition_snapshot: definition,
    error: null,
    started_at: "2026-07-11T00:00:00Z",
    completed_at: null,
    created_at: "2026-07-11T00:00:00Z",
    updated_at: "2026-07-11T00:01:00Z",
  };
}
