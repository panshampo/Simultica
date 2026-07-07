import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { fireEvent, render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import type {
  WorkflowCase,
  WorkflowDefinition,
  WorkflowRun,
} from "@multica/core/workflow/types";

const mocks = vi.hoisted(() => ({
  getWorkflowCase: vi.fn(),
  listWorkflowCaseRuns: vi.fn(),
}));

vi.mock("@multica/core/api", () => ({
  api: {
    getWorkflowCase: mocks.getWorkflowCase,
    listWorkflowCaseRuns: mocks.listWorkflowCaseRuns,
  },
}));

vi.mock("@multica/core/hooks", () => ({
  useWorkspaceId: () => "ws-1",
}));

vi.mock("@multica/core/paths", () => ({
  useWorkspacePaths: () => ({
    workflowCaseDetail: (id: string) => `/workflow-cases/${id}`,
    issueDetail: (id: string) => `/issues/${id}`,
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

import { WorkflowRunDetailPage } from "./workflow-run-detail-page";

describe("WorkflowRunDetailPage", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mocks.getWorkflowCase.mockResolvedValue(makeCase());
    mocks.listWorkflowCaseRuns.mockResolvedValue([makeRun()]);
  });

  it("renders the run header, node table, and links back to the case", async () => {
    renderPage("run-1");

    expect(await screen.findByText("Approach A")).toBeInTheDocument();
    // "experiment" appears both as the header badge and the Kind property.
    expect(screen.getAllByText("experiment").length).toBeGreaterThanOrEqual(1);
    // Node table row.
    expect(screen.getByText("implement")).toBeInTheDocument();
    // Back link to the owning case.
    expect(screen.getByRole("link", { name: "Back to workflow case" })).toHaveAttribute(
      "href",
      "/workflow-cases/case-1",
    );
  });

  it("preselects the deep-linked node and opens its detail panel", async () => {
    renderPage("run-1", "implement");

    expect(await screen.findByText("Node detail")).toBeInTheDocument();
    const canvas = await screen.findByTestId("workflow-canvas");
    expect(canvas.getAttribute("data-selected")).toBe("implement");
    // Carrier issue is reachable from the node panel.
    expect(screen.getByRole("link", { name: "Open sub-issue" })).toHaveAttribute(
      "href",
      "/issues/e114f904-79c2-4db6-a3a9-c7d23f22c9aa",
    );
  });

  it("shows a not-found message for an unknown node but still renders the run", async () => {
    renderPage("run-1", "ghost-node");

    expect(await screen.findByText("Approach A")).toBeInTheDocument();
    expect(screen.getByText("Node not found in this run.")).toBeInTheDocument();
  });

  it("selects a node when clicking it in the graph", async () => {
    renderPage("run-1");

    const canvas = await screen.findByTestId("workflow-canvas");
    fireEvent.click(canvas);
    expect(await screen.findByText("Node detail")).toBeInTheDocument();
  });

  it("shows not-found when the run is missing from the case", async () => {
    renderPage("does-not-exist");

    expect(await screen.findByText("Workflow run not found in this case.")).toBeInTheDocument();
  });
});

function renderPage(runId: string, nodeId?: string) {
  const qc = new QueryClient({
    defaultOptions: {
      queries: { retry: false },
      mutations: { retry: false },
    },
  });
  return render(
    <QueryClientProvider client={qc}>
      <WorkflowRunDetailPage caseId="case-1" runId={runId} initialNodeId={nodeId ?? null} />
    </QueryClientProvider>,
  );
}

const definition: WorkflowDefinition = {
  meta: { name: "Run workflow", version: "1" },
  state: { fields: [] },
  nodes: [{ id: "implement", type: "agent", dispatch: "subissue" }],
  routing: [{ from: "START", to: "implement" }],
};

function makeCase(): WorkflowCase {
  return {
    id: "case-1",
    workspace_id: "ws-1",
    title: "Owning workflow case",
    description: "",
    entry_issue_id: "issue-1",
    source_issue_id: "issue-1",
    owner_agent_id: null,
    status: "draft",
    online_version_id: "version-1",
    current_run_id: null,
    created_at: "2026-07-06T00:00:00Z",
    updated_at: "2026-07-06T00:00:00Z",
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
    run_kind: "experiment",
    label: "Approach A",
    current_node: "implement",
    nodes_state: {
      implement: {
        status: "running",
        sub_issue_id: "sub-1",
        started_at: "2026-07-06T00:00:00Z",
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
      input_snapshot: { issue: "issue-1" },
      output_snapshot: null,
      error: null,
      logs: [],
      carrier_ref: { type: "subissue", issue_id: "e114f904-79c2-4db6-a3a9-c7d23f22c9aa", sub_issue_id: "e114f904-79c2-4db6-a3a9-c7d23f22c9aa" },
      started_at: "2026-07-06T00:00:00Z",
      completed_at: null,
      updated_at: "2026-07-06T00:00:30Z",
    }],
    definition_snapshot: definition,
    error: null,
    started_at: "2026-07-06T00:00:00Z",
    completed_at: null,
    created_at: "2026-07-06T00:00:00Z",
    updated_at: "2026-07-06T00:00:30Z",
  };
}
