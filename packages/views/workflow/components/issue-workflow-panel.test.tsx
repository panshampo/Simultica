import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import type { IssueWorkflowContext } from "@multica/core/workflow/types";

const mocks = vi.hoisted(() => ({
  getIssueWorkflowContext: vi.fn(),
  useWSEvent: vi.fn(),
}));

vi.mock("@multica/core/api", () => ({
  api: {
    getIssueWorkflowContext: mocks.getIssueWorkflowContext,
  },
}));

vi.mock("@multica/core/realtime", () => ({
  useWSEvent: mocks.useWSEvent,
}));

vi.mock("@multica/core/paths", () => ({
  useWorkspacePaths: () => ({
    issueDetail: (id: string) => `/issues/${id}`,
    workflowCaseDetail: (id: string) => `/workflow-cases/${id}`,
    workflowCaseRunDetail: (caseId: string, runId: string, nodeId?: string) => {
      const base = `/workflow-cases/${caseId}/runs/${runId}`;
      return nodeId ? `${base}?node_id=${nodeId}` : base;
    },
  }),
}));

vi.mock("../../navigation", () => ({
  AppLink: ({ href, children, className }: { href: string; children: React.ReactNode; className?: string }) => (
    <a className={className} href={href}>{children}</a>
  ),
}));

import { IssueWorkflowPanel } from "./issue-workflow-panel";

describe("IssueWorkflowPanel", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mocks.getIssueWorkflowContext.mockResolvedValue(makeContext({ role: "none" }));
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
    const openRun = screen.getByRole("link", { name: "Open Run at Node" });
    expect(openRun).toHaveAttribute(
      "href",
      "/workflow-cases/case-1/runs/run-1?node_id=implement-node-123456",
    );
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
