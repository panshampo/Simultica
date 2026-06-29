import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import type { WorkflowRun } from "@multica/core/workflow/types";

const mocks = vi.hoisted(() => ({
  getIssueWorkflowRun: vi.fn(),
  listSkills: vi.fn(),
  cancelWorkflowRun: vi.fn(),
  continueWorkflowRun: vi.fn(),
  startIssueWorkflowRun: vi.fn(),
  useWSEvent: vi.fn(),
  push: vi.fn(),
}));

vi.mock("@multica/core/api", () => ({
  api: {
    getIssueWorkflowRun: mocks.getIssueWorkflowRun,
    listSkills: mocks.listSkills,
    cancelWorkflowRun: mocks.cancelWorkflowRun,
    continueWorkflowRun: mocks.continueWorkflowRun,
    startIssueWorkflowRun: mocks.startIssueWorkflowRun,
  },
}));

vi.mock("@multica/core/realtime", () => ({
  useWSEvent: mocks.useWSEvent,
}));

vi.mock("@multica/core/paths", () => ({
  useWorkspacePaths: () => ({
    issueDetail: (id: string) => `/issues/${id}`,
  }),
}));

vi.mock("../../navigation", () => ({
  AppLink: ({ href, children, className }: { href: string; children: React.ReactNode; className?: string }) => (
    <a className={className} href={href}>{children}</a>
  ),
  useNavigation: () => ({
    push: mocks.push,
  }),
}));

vi.mock("./workflow-canvas", () => ({
  WorkflowCanvas: () => <div data-testid="workflow-canvas" />,
}));

import { IssueWorkflowPanel } from "./issue-workflow-panel";

describe("IssueWorkflowPanel", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mocks.listSkills.mockResolvedValue([]);
    mocks.startIssueWorkflowRun.mockResolvedValue({ status: "started" });
    mocks.continueWorkflowRun.mockResolvedValue(makeRun({ id: "run-2", status: "running" }));
  });

  it("shows lifecycle details and stops active workflow runs", async () => {
    const activeRun = makeRun({ status: "running", current_node: "implement" });
    const cancelledRun = makeRun({
      status: "cancelled",
      cancel_reason: "workflow stopped by user",
      cancelled_at: "2026-06-27T02:00:00Z",
    });
    mocks.getIssueWorkflowRun.mockResolvedValue(activeRun);
    mocks.cancelWorkflowRun.mockResolvedValue(cancelledRun);

    renderPanel();

    expect(await screen.findByText("Runtime workflow")).toBeInTheDocument();
    expect(await screen.findByText("Running")).toBeInTheDocument();
    expect(screen.getByText("Workflow managed")).toBeInTheDocument();
    expect(screen.getByText("Node implement")).toBeInTheDocument();
    expect(screen.getByText("Planner task-123")).toBeInTheDocument();
    expect(screen.getByText("Source implementation")).toBeInTheDocument();
    expect(screen.getByText("agent · subissue")).toBeInTheDocument();
    expect(screen.queryByText("Run workflow")).not.toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: "Stop workflow" }));

    await waitFor(() => {
      expect(mocks.cancelWorkflowRun).toHaveBeenCalledWith("run-1");
    });
  });

  it.each(["planning", "running", "finalizing"] as const)(
    "shows Stop workflow for %s runs",
    async (status) => {
      mocks.getIssueWorkflowRun.mockResolvedValue(makeRun({ status }));

      renderPanel();

      expect(await screen.findByRole("button", { name: "Stop workflow" })).toBeInTheDocument();
    },
  );

  it.each(["pending", "done", "failed", "cancelling", "cancelled"] as const)(
    "does not show Stop workflow for %s runs",
    async (status) => {
      mocks.getIssueWorkflowRun.mockResolvedValue(makeRun({ status }));

      renderPanel();

      expect(await screen.findByText(statusLabel(status))).toBeInTheDocument();
      expect(screen.queryByRole("button", { name: "Stop workflow" })).not.toBeInTheDocument();
    },
  );

  it("keeps manual skill execution behind the debug disclosure", async () => {
    mocks.getIssueWorkflowRun.mockResolvedValue(null);
    mocks.listSkills.mockResolvedValue([
      { id: "skill-1", name: "Debug skill", config: { has_workflow: true } },
    ]);

    renderPanel();

    expect(await screen.findByText("Waiting for the main agent to plan this issue.")).toBeInTheDocument();
    expect(screen.getByText("Native issue mode")).toBeInTheDocument();
    expect(screen.getByText("Debug run skill workflow")).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Run workflow" })).not.toBeInTheDocument();
  });

  it("explains stopped workflows and distinguishes child task from child issue status", async () => {
    mocks.getIssueWorkflowRun.mockResolvedValue(makeRun({
      status: "cancelled",
      cancel_reason: "real e2e active cancellation",
      cancelled_at: "2026-06-27T02:00:00Z",
      nodes_state: {
        implement: {
          status: "cancelled",
          sub_issue_id: "sub-1",
          task_id: "task-1",
          source_skill_name: "implementation",
          source_node_id: "implement-template",
          started_at: "2026-06-27T01:00:00Z",
          ended_at: "2026-06-27T02:00:00Z",
          error: null,
        },
        review: {
          status: "pending",
          sub_issue_id: null,
          started_at: null,
          ended_at: null,
          error: null,
        },
      },
      definition_snapshot: {
        meta: { name: "runtime", version: "1" },
        state: { fields: [] },
        nodes: [
          { id: "implement", type: "subissue", source_skill_name: "implementation", source_node_id: "implement-template" },
          { id: "review", type: "final_response" },
        ],
        routing: [{ from: "START", to: "implement" }, { from: "implement", to: "review" }],
      },
    }));

    renderPanel();

    expect(await screen.findByText("Workflow stopped")).toBeInTheDocument();
    expect(screen.getByText(/Active child tasks were cancelled/)).toBeInTheDocument();
    expect(screen.getByText("Child task cancelled")).toBeInTheDocument();
    expect(screen.getByText("Child issue kept as record")).toBeInTheDocument();
    expect(screen.getByText("Skipped after stop")).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Stop workflow" })).not.toBeInTheDocument();
  });

  it("shows direct and main issue task carriers", async () => {
    mocks.getIssueWorkflowRun.mockResolvedValue(makeRun({
      nodes_state: {
        quick_review: {
          status: "done",
          sub_issue_id: null,
          traex_session_id: "traex-session-123456",
          started_at: "2026-06-27T01:00:00Z",
          ended_at: "2026-06-27T01:01:00Z",
          error: null,
        },
        final_summary: {
          status: "running",
          sub_issue_id: null,
          main_issue_task_id: "main-task-123456",
          started_at: "2026-06-27T01:01:00Z",
          ended_at: null,
          error: null,
        },
      },
      definition_snapshot: {
        meta: { name: "runtime", version: "1" },
        state: { fields: [] },
        nodes: [
          { id: "quick_review", type: "agent", dispatch: "direct_subagent", agent: "architect" },
          { id: "final_summary", type: "main_agent", dispatch: "main_issue_task" },
        ],
        routing: [{ from: "START", to: "quick_review" }, { from: "quick_review", to: "final_summary" }],
      },
    }));

    renderPanel();

    expect(await screen.findByText("agent · direct")).toBeInTheDocument();
    expect(screen.getByText("main agent · main issue")).toBeInTheDocument();
    expect(screen.getByText("TraeX session traex-se")).toBeInTheDocument();
    expect(screen.getByText("Main task main-tas")).toBeInTheDocument();
  });

  it("shows conditional route decisions for looped workflow nodes", async () => {
    mocks.getIssueWorkflowRun.mockResolvedValue(makeRun({
      nodes_state: {
        review_result: {
          status: "done",
          sub_issue_id: "sub-review",
          started_at: "2026-06-27T01:00:00Z",
          ended_at: "2026-06-27T01:01:00Z",
          route_decision: {
            condition: 'workflow_status == "fixable_auto" && revisionCount < 3',
            condition_result: true,
            selected_route: "stabilize_run",
            else_route: "final_report",
            decided_at: "2026-06-27T01:01:00Z",
          },
          error: null,
        },
      },
      definition_snapshot: {
        meta: { name: "runtime", version: "1" },
        state: { fields: [] },
        nodes: [
          { id: "review_result", type: "agent", dispatch: "subissue", agent: "reviewer" },
        ],
        routing: [
          {
            from: "review_result",
            condition: 'workflow_status == "fixable_auto" && revisionCount < 3',
            to: "stabilize_run",
            else: "final_report",
          },
        ],
      },
    }));

    renderPanel();

    expect(await screen.findByText("Route stabilize_run")).toBeInTheDocument();
    expect(screen.getByText("Condition true")).toBeInTheDocument();
    expect(screen.getByText('workflow_status == "fixable_auto" && revisionCount < 3')).toBeInTheDocument();
  });

  it("offers continuation for budget exhausted workflow runs", async () => {
    mocks.getIssueWorkflowRun.mockResolvedValue(makeRun({
      status: "done",
      nodes_state: {
        review_result: {
          status: "done",
          sub_issue_id: "sub-review",
          output: { workflow_status: "budget_exhausted", needs_user_decision: true },
          started_at: "2026-06-27T01:00:00Z",
          ended_at: "2026-06-27T01:01:00Z",
          error: null,
        },
      },
      definition_snapshot: {
        meta: { name: "runtime", version: "1" },
        state: { fields: [] },
        nodes: [{ id: "review_result", type: "agent", dispatch: "subissue", agent: "reviewer" }],
        routing: [{ from: "START", to: "review_result" }],
      },
    }));

    renderPanel();

    const button = await screen.findByRole("button", { name: "Continue workflow" });
    fireEvent.click(button);

    await waitFor(() => {
      expect(mocks.continueWorkflowRun).toHaveBeenCalledWith("run-1", "continue workflow after human approval");
    });
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

function makeRun(overrides: Partial<WorkflowRun> = {}): WorkflowRun {
  return {
    id: "run-1",
    root_issue_id: "issue-1",
    skill_id: null,
    planner_task_id: "task-123456",
    source_skills: [{ id: "skill-1", name: "implementation" }],
    status: "running",
    current_node: "implement",
    nodes_state: {
      implement: {
        status: "running",
        sub_issue_id: "sub-1",
        source_skill_name: "implementation",
        source_node_id: "implement-template",
        started_at: "2026-06-27T01:00:00Z",
        ended_at: null,
        error: null,
      },
    },
    definition_snapshot: {
      meta: { name: "runtime", version: "1" },
      source_skills: [{ id: "skill-1", name: "implementation" }],
      state: { fields: [] },
      nodes: [
        {
          id: "implement",
          type: "agent",
          dispatch: "subissue",
          source_skill_name: "implementation",
          source_node_id: "implement-template",
        },
      ],
      routing: [{ from: "START", to: "implement" }],
    },
    error: null,
    started_at: "2026-06-27T01:00:00Z",
    completed_at: null,
    created_at: "2026-06-27T01:00:00Z",
    updated_at: "2026-06-27T01:00:00Z",
    ...overrides,
  };
}

function statusLabel(status: WorkflowRun["status"]): string {
  switch (status) {
    case "planning":
      return "Planning";
    case "running":
      return "Running";
    case "finalizing":
      return "Finalizing";
    case "done":
      return "Done";
    case "failed":
      return "Failed";
    case "cancelling":
      return "Cancelling";
    case "cancelled":
      return "Cancelled";
    case "pending":
      return "Pending";
  }
}
