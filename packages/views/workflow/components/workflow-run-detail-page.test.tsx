import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import type {
  WorkflowCase,
  WorkflowDefinition,
  WorkflowDefinitionVersion,
  WorkflowRun,
} from "@multica/core/workflow/types";

const mocks = vi.hoisted(() => ({
  getWorkflowCase: vi.fn(),
  listWorkflowCaseDefinitionVersions: vi.fn(),
  listWorkflowCaseRuns: vi.fn(),
  reviewWorkflowRunStep: vi.fn(),
}));

vi.mock("@multica/core/api", () => ({
  api: {
    getWorkflowCase: mocks.getWorkflowCase,
    listWorkflowCaseDefinitionVersions: mocks.listWorkflowCaseDefinitionVersions,
    listWorkflowCaseRuns: mocks.listWorkflowCaseRuns,
    reviewWorkflowRunStep: mocks.reviewWorkflowRunStep,
  },
}));

vi.mock("@multica/core/hooks", () => ({
  useWorkspaceId: () => "ws-1",
}));

vi.mock("@multica/core/paths", () => ({
  useWorkspacePaths: () => ({
    workflowCaseDetail: (id: string) => `/workflow-cases/${id}`,
    workflowCaseVersionDetail: (caseId: string, versionId: string) => `/workflow-cases/${caseId}/versions/${versionId}`,
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
    mocks.listWorkflowCaseDefinitionVersions.mockResolvedValue([makeVersion()]);
    mocks.listWorkflowCaseRuns.mockResolvedValue([makeRun()]);
    mocks.reviewWorkflowRunStep.mockResolvedValue({
      run_id: "run-1",
      step_id: "review_before_mutation",
      decision: "approved",
      reviewed_by: "user-1",
      reviewed_at: "2026-07-09T12:00:00Z",
    });
  });

  it("renders the run header, node table, and links back to the case", async () => {
    renderPage("run-1");

    expect(await screen.findByText("Approach A")).toBeInTheDocument();
    expect(screen.queryByText("Kind")).not.toBeInTheDocument();
    expect(screen.queryByText("experiment")).not.toBeInTheDocument();
    // Node table row.
    expect(screen.getByText("implement")).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "Open issue" })).toHaveAttribute(
      "href",
      "/issues/e114f904-79c2-4db6-a3a9-c7d23f22c9aa",
    );
    // Back link to the owning case.
    expect(screen.getByRole("link", { name: "Back to workflow case" })).toHaveAttribute(
      "href",
      "/workflow-cases/case-1",
    );
    expect(screen.getByRole("link", { name: "v1" })).toHaveAttribute(
      "href",
      "/workflow-cases/case-1/versions/version-1",
    );
  });

  it("does not render an empty node detail panel before a node is selected", async () => {
    renderPage("run-1");

    expect(await screen.findByText("Approach A")).toBeInTheDocument();
    expect(screen.queryByText("Node detail")).not.toBeInTheDocument();
    expect(screen.queryByText("Select a run node to inspect its projection.")).not.toBeInTheDocument();
  });

  it("renders graph before run basic info and keeps info panels below the graph", async () => {
    renderPage("run-1");

    expect(await screen.findByText("Approach A")).toBeInTheDocument();
    const layout = screen.getByTestId("workflow-run-detail-layout");
    const graph = screen.getByTestId("workflow-run-graph-section");
    const metadata = screen.getByTestId("workflow-run-basic-info");
    const lowerPanels = screen.getByTestId("workflow-run-lower-panels");

    expect(layout.compareDocumentPosition(graph) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();
    expect(graph.compareDocumentPosition(lowerPanels) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();
    expect(graph.compareDocumentPosition(metadata) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();
  });

  it("preselects the deep-linked node and opens its detail panel", async () => {
    renderPage("run-1", "implement");

    expect(await screen.findByText("Node detail")).toBeInTheDocument();
    const canvas = await screen.findByTestId("workflow-canvas");
    expect(canvas.getAttribute("data-selected")).toBe("implement");
    expect(screen.getByText("Agent route")).toBeInTheDocument();
    expect(screen.getByText("frontend-bug-investigation")).toBeInTheDocument();
    expect(screen.getByText("System prompt")).toBeInTheDocument();
    expect(screen.getByText("Investigate the browser failure and capture Playwright evidence.")).toBeInTheDocument();
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

  it("shows a Review required banner with Approve/Reject when the current step is pending_review", async () => {
    mocks.listWorkflowCaseRuns.mockResolvedValue([makeReviewRun()]);
    renderPage("run-1");

    expect(await screen.findByText(/Review required/i)).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /Approve/i })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /Reject/i })).toBeInTheDocument();
  });

  it("does not show the Review banner when no step is pending_review", async () => {
    renderPage("run-1");

    expect(await screen.findByText("Approach A")).toBeInTheDocument();
    expect(screen.queryByText(/Review required/i)).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: /Approve/i })).not.toBeInTheDocument();
  });

  it("calls the review API with approved when Approve is clicked", async () => {
    mocks.listWorkflowCaseRuns.mockResolvedValue([makeReviewRun()]);
    renderPage("run-1");

    fireEvent.click(await screen.findByRole("button", { name: /Approve/i }));

    await waitFor(() =>
      expect(mocks.reviewWorkflowRunStep).toHaveBeenCalledWith(
        "case-1",
        "run-1",
        "review_before_mutation",
        expect.objectContaining({ decision: "approved" }),
      ),
    );
  });

  it("calls the review API with rejected when Reject is clicked", async () => {
    mocks.listWorkflowCaseRuns.mockResolvedValue([makeReviewRun()]);
    renderPage("run-1");

    fireEvent.click(await screen.findByRole("button", { name: /Reject/i }));

    await waitFor(() =>
      expect(mocks.reviewWorkflowRunStep).toHaveBeenCalledWith(
        "case-1",
        "run-1",
        "review_before_mutation",
        expect.objectContaining({ decision: "rejected" }),
      ),
    );
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
  nodes: [{
    id: "implement",
    type: "agent",
    dispatch: "subissue",
    agent: "frontend-bug-investigation",
    inputs: ["task", "scope_result"],
    outputs: ["investigation_report"],
    config: {
      system: "Investigate the browser failure and capture Playwright evidence.",
    },
  }],
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
    confirmed_at: "2026-07-06T00:00:00Z",
    created_at: "2026-07-06T00:00:00Z",
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

const reviewDefinition: WorkflowDefinition = {
  meta: { name: "Run workflow", version: "1" },
  state: { fields: [] },
  nodes: [{
    id: "review_before_mutation",
    type: "human_review",
    dispatch: "human_gate",
    inputs: ["proposed_fix"],
    outputs: ["review_decision", "review_comment"],
    config: { title: "Review before code changes", question: "Approve the agent to modify files?" },
  }],
  routing: [{ from: "START", to: "review_before_mutation" }],
};

function makeReviewRun(): WorkflowRun {
  return {
    id: "run-1",
    root_issue_id: "issue-1",
    case_id: "case-1",
    definition_version_id: "version-1",
    skill_id: null,
    status: "waiting_for_review",
    label: "Review run",
    current_node: "review_before_mutation",
    nodes_state: {
      review_before_mutation: {
        status: "pending_review",
        sub_issue_id: null,
        started_at: "2026-07-09T00:00:00Z",
        ended_at: null,
        error: null,
        input: { proposed_fix: "Change request builder", risk_assessment: "Touches submit path" },
      } as never,
    },
    nodes: [{
      id: "run-node-review",
      run_id: "run-1",
      node_id: "review_before_mutation",
      node_type: "human_review",
      dispatch: "human_gate",
      carrier_kind: "inline",
      status: "pending_review" as never,
      attempt: 1,
      input_snapshot: { proposed_fix: "Change request builder" },
      output_snapshot: null,
      error: null,
      logs: [],
      carrier_ref: null,
      started_at: "2026-07-09T00:00:00Z",
      completed_at: null,
      updated_at: "2026-07-09T00:00:30Z",
    }],
    definition_snapshot: reviewDefinition,
    error: null,
    started_at: "2026-07-09T00:00:00Z",
    completed_at: null,
    created_at: "2026-07-09T00:00:00Z",
    updated_at: "2026-07-09T00:00:30Z",
  };
}
