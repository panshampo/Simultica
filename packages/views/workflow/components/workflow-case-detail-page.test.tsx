import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";
import type {
  WorkflowCase,
  WorkflowDefinition,
  WorkflowDefinitionDraft,
  WorkflowDefinitionVersion,
  WorkflowRun,
  WorkflowValidationReport,
} from "@multica/core/workflow/types";

const mocks = vi.hoisted(() => ({
  getWorkflowCase: vi.fn(),
  getWorkflowCaseDefinition: vi.fn(),
  listWorkflowCaseDefinitionVersions: vi.fn(),
  listWorkflowCaseRuns: vi.fn(),
  validateWorkflowCaseDefinition: vi.fn(),
  publishWorkflowCaseDefinition: vi.fn(),
  startWorkflowCaseRun: vi.fn(),
  updateWorkflowCase: vi.fn(),
  cancelWorkflowCaseRun: vi.fn(),
  deleteWorkflowCase: vi.fn(),
  upsertWorkflowCaseDefinitionDraft: vi.fn(),
  navigationPush: vi.fn(),
}));

vi.mock("@multica/core/api", () => ({
  api: {
    getWorkflowCase: mocks.getWorkflowCase,
    getWorkflowCaseDefinition: mocks.getWorkflowCaseDefinition,
    listWorkflowCaseDefinitionVersions: mocks.listWorkflowCaseDefinitionVersions,
    listWorkflowCaseRuns: mocks.listWorkflowCaseRuns,
    validateWorkflowCaseDefinition: mocks.validateWorkflowCaseDefinition,
    publishWorkflowCaseDefinition: mocks.publishWorkflowCaseDefinition,
    startWorkflowCaseRun: mocks.startWorkflowCaseRun,
    updateWorkflowCase: mocks.updateWorkflowCase,
    cancelWorkflowCaseRun: mocks.cancelWorkflowCaseRun,
    deleteWorkflowCase: mocks.deleteWorkflowCase,
    upsertWorkflowCaseDefinitionDraft: mocks.upsertWorkflowCaseDefinitionDraft,
  },
}));

vi.mock("@multica/core/hooks", () => ({
  useWorkspaceId: () => "ws-1",
}));

vi.mock("@multica/core/paths", () => ({
  useWorkspacePaths: () => ({
    workflowCases: () => "/workflow-cases",
    workflowCaseDetail: (id: string) => `/workflow-cases/${id}`,
    workflowCaseVersionDetail: (caseId: string, versionId: string) => `/workflow-cases/${caseId}/versions/${versionId}`,
    workflowCaseRunDetail: (caseId: string, runId: string, nodeId?: string) => {
      const base = `/workflow-cases/${caseId}/runs/${runId}`;
      return nodeId ? `${base}?node_id=${nodeId}` : base;
    },
    issueDetail: (id: string) => `/issues/${id}`,
  }),
}));

vi.mock("../../navigation", () => ({
  AppLink: ({ href, children, className, ...props }: { href: string; children: React.ReactNode; className?: string }) => (
    <a href={href} className={className} {...props}>{children}</a>
  ),
  useNavigation: () => ({ push: mocks.navigationPush }),
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
      data-name={definition.meta.name}
      data-status={runStatus ?? ""}
      data-selected={selectedNodeId ?? ""}
      onClick={() => onSelectNode?.("plan")}
    >
      {definition.meta.name}
    </button>
  ),
}));

vi.mock("./workflow-editor", () => ({
  WorkflowEditor: ({ title, saveLabel, onSave, onSaved }: {
    title?: string;
    saveLabel?: string;
    onSave: (yaml: string) => Promise<void>;
    onSaved?: (yaml: string) => void;
  }) => (
    <div data-testid="workflow-editor">
      <span>{title}</span>
      <button
        type="button"
        onClick={async () => {
          const yaml = "meta:\n  name: workflow-case\nstate:\n  fields: []\nnodes: []\nrouting: []\n";
          await onSave(yaml);
          onSaved?.(yaml);
        }}
      >
        {saveLabel ?? "Save workflow"}
      </button>
    </div>
  ),
}));

vi.mock("@multica/core/workspace/queries", () => ({
  agentListOptions: (wsId: string) => ({ queryKey: ["agents", wsId], queryFn: async () => [] }),
}));

import { WorkflowCaseDetailPage } from "./workflow-case-detail-page";

describe("WorkflowCaseDetailPage", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mocks.getWorkflowCase.mockResolvedValue(makeCase());
    mocks.getWorkflowCaseDefinition.mockResolvedValue(makeDraft());
    mocks.listWorkflowCaseDefinitionVersions.mockResolvedValue([makeVersion()]);
    mocks.listWorkflowCaseRuns.mockResolvedValue([makeRun({ id: "run-1", status: "done" })]);
    mocks.validateWorkflowCaseDefinition.mockResolvedValue(makeValidation({ valid: true }));
    mocks.upsertWorkflowCaseDefinitionDraft.mockResolvedValue(makeDraft());
    mocks.publishWorkflowCaseDefinition.mockResolvedValue({ version: makeVersion() });
    mocks.startWorkflowCaseRun.mockResolvedValue(makeRun({ id: "run-2", status: "running" }));
    mocks.updateWorkflowCase.mockResolvedValue(makeCase());
    mocks.cancelWorkflowCaseRun.mockResolvedValue(makeRun({ id: "run-active", status: "cancelled" }));
    mocks.deleteWorkflowCase.mockResolvedValue(undefined);
  });

  it("renders Overview as the default metadata tab without definition or run controls", async () => {
    renderPage();

    expect(await screen.findByText("Workflow case title")).toBeInTheDocument();
    expect(screen.getByRole("tab", { name: "Overview" })).toHaveAttribute("aria-selected", "true");
    expect(screen.queryByRole("tab", { name: "Settings" })).not.toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Save changes" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Archive case" })).toBeInTheDocument();
    expect(screen.queryByText("Lifecycle")).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Create run" })).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Validate draft" })).not.toBeInTheDocument();
    expect(screen.queryByLabelText("Workflow YAML")).not.toBeInTheDocument();
  });

  it("uses product-aligned copy: Workflow tab, Active Workflow, Set active", async () => {
    renderPage();

    expect(await screen.findByText("Workflow case title")).toBeInTheDocument();

    // Tab reads Workflow, never Definition.
    expect(screen.getByRole("tab", { name: "Workflow" })).toBeInTheDocument();
    expect(screen.queryByRole("tab", { name: "Definition" })).not.toBeInTheDocument();

    // Overview surfaces "Active Workflow", not "Online version".
    expect(screen.getByText("Active Workflow")).toBeInTheDocument();
    expect(screen.queryByText("Online version")).not.toBeInTheDocument();

    // The Workflow tab exposes "Set active", never "Publish online version".
    await userEvent.click(screen.getByRole("tab", { name: "Workflow" }));
    expect(screen.getByRole("button", { name: "Set active" })).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Publish online version" })).not.toBeInTheDocument();
  });

  it("creates a run from the Runs tab without run_kind", async () => {
    renderPage();

    await userEvent.click(await screen.findByRole("tab", { name: "Runs" }));
    await userEvent.click(screen.getByRole("button", { name: "Create run" }));

    await waitFor(() => {
      expect(mocks.startWorkflowCaseRun).toHaveBeenCalledWith("case-1", {
        label: undefined,
        initial_state: {},
      });
    });
    expect(mocks.startWorkflowCaseRun.mock.calls[0]?.[1]).not.toHaveProperty("run_kind");
  });

  it("updates case metadata from Overview", async () => {
    mocks.updateWorkflowCase.mockResolvedValue(makeCase({ title: "Renamed workflow case", description: "Updated" }));
    renderPage();

    await screen.findByRole("tab", { name: "Overview" });
    await userEvent.clear(screen.getByLabelText("Case title"));
    await userEvent.type(screen.getByLabelText("Case title"), "Renamed workflow case");
    await userEvent.clear(screen.getByLabelText("Description"));
    await userEvent.type(screen.getByLabelText("Description"), "Updated");
    await userEvent.click(screen.getByRole("button", { name: "Save changes" }));

    await waitFor(() => {
      expect(mocks.updateWorkflowCase).toHaveBeenCalledWith("case-1", expect.objectContaining({
        title: "Renamed workflow case",
        description: "Updated",
      }));
    });
  });

  it("gates publish on validation, then starts an online-version run without a version id", async () => {
    renderPage();

    expect(await screen.findByText("Workflow case title")).toBeInTheDocument();
    await userEvent.click(screen.getByRole("tab", { name: "Workflow" }));
    expect(screen.getByRole("button", { name: "Set active" })).toBeDisabled();

    fireEvent.click(screen.getByRole("button", { name: "Validate draft" }));
    await waitFor(() => expect(mocks.validateWorkflowCaseDefinition).toHaveBeenCalledWith("case-1"));
    expect(await screen.findByText("Validation passed")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Set active" })).toBeEnabled();

    fireEvent.click(screen.getByRole("button", { name: "Set active" }));
    await waitFor(() => expect(mocks.publishWorkflowCaseDefinition).toHaveBeenCalledWith("case-1"));

    await userEvent.click(screen.getByRole("tab", { name: "Runs" }));
    fireEvent.click(screen.getByRole("button", { name: "Create run" }));
    await waitFor(() => {
      expect(mocks.startWorkflowCaseRun).toHaveBeenCalledWith("case-1", {
        label: undefined,
        initial_state: {},
      });
    });
    expect(mocks.startWorkflowCaseRun.mock.calls[0]?.[1]).not.toHaveProperty("run_kind");
    // Create run must never carry a definition_version_id.
    expect(mocks.startWorkflowCaseRun.mock.calls[0]?.[1]).not.toHaveProperty("definition_version_id");
  });

  it("disables Create run and publishing when there is no online version", async () => {
    mocks.getWorkflowCase.mockResolvedValue(makeCase({ online_version_id: null }));

    renderPage();

    expect(await screen.findByText("Workflow case title")).toBeInTheDocument();
    await userEvent.click(screen.getByRole("tab", { name: "Runs" }));
    expect(screen.getByRole("button", { name: "Create run" })).toBeDisabled();
    expect(screen.getByText("Set a workflow active before creating a run.")).toBeInTheDocument();
  });

  it("shows Active Workflow and historical snapshots on the Workflow tab, not Overview", async () => {
    mocks.getWorkflowCase.mockResolvedValue(makeCase({ online_version_id: "version-2" }));
    mocks.listWorkflowCaseDefinitionVersions.mockResolvedValue([
      makeVersion({ id: "version-1", version: 1 }),
      makeVersion({ id: "version-2", version: 2 }),
    ]);

    renderPage();

    // Overview no longer carries the version list.
    await screen.findByRole("tab", { name: "Overview" });
    expect(screen.queryByText("Historical")).not.toBeInTheDocument();
    expect(screen.queryByRole("link", { name: "Open v1" })).not.toBeInTheDocument();

    // Workflow tab shows the full timeline: Active Workflow + Snapshots.
    await userEvent.click(screen.getByRole("tab", { name: "Workflow" }));
    expect(await screen.findByText("Active Workflow")).toBeInTheDocument();
    expect(screen.getByText("Historical")).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "Open v1" })).toHaveAttribute(
      "href",
      "/workflow-cases/case-1/versions/version-1",
    );
    expect(screen.getByRole("link", { name: "Open v2" })).toHaveAttribute(
      "href",
      "/workflow-cases/case-1/versions/version-2",
    );
    expect(screen.queryByRole("button", { name: /Create run from/i })).not.toBeInTheDocument();
  });

  it("renders multiple parallel active runs and cancels a single run", async () => {
    mocks.getWorkflowCase.mockResolvedValue(makeCase({ online_version_id: "version-1" }));
    mocks.listWorkflowCaseRuns.mockResolvedValue([
      makeRun({ id: "run-a", status: "running" }),
      makeRun({ id: "run-b", status: "running" }),
    ]);

    renderPage();

    await userEvent.click(await screen.findByRole("tab", { name: "Runs" }));
    expect((await screen.findAllByText("run-a")).length).toBeGreaterThan(0);
    expect(screen.getByText("run-b")).toBeInTheDocument();
    expect(screen.getAllByRole("link", { name: "v1" }).length).toBeGreaterThanOrEqual(2);
    expect(screen.queryAllByText("v1").length).toBeGreaterThanOrEqual(2);
    const cancelButtons = screen.getAllByRole("button", { name: "Cancel" });
    expect(cancelButtons.length).toBe(2);
    fireEvent.click(cancelButtons[0]!);
    await waitFor(() => expect(mocks.cancelWorkflowCaseRun).toHaveBeenCalledWith("case-1", "run-a"));
  });

  it("embeds the current run detail directly in the Runs tab", async () => {
    mocks.getWorkflowCase.mockResolvedValue(makeCase({ current_run_id: "run-active" }));
    mocks.listWorkflowCaseRuns.mockResolvedValue([
      makeRun({ id: "run-active", label: "Active investigation", status: "running" }),
      makeRun({ id: "run-history", label: "Older run", status: "done" }),
    ]);

    renderPage();

    await userEvent.click(await screen.findByRole("tab", { name: "Runs" }));

    expect(await screen.findByText("Active investigation")).toBeInTheDocument();
    expect(screen.getByText("Run graph")).toBeInTheDocument();
    expect(screen.getByText("Nodes")).toBeInTheDocument();
    expect(screen.getByTestId("workflow-canvas")).toHaveAttribute("data-status", "running");
    expect(screen.queryByRole("link", { name: "Open current run" })).not.toBeInTheDocument();
    expect(screen.getByText(/Older run/)).toBeInTheDocument();
  });

  it("has no confirm-and-run affordance in the UI", async () => {
    renderPage();
    await screen.findByText("Workflow case title");
    expect(screen.queryByText(/Confirm and run/i)).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: /Confirm version/i })).not.toBeInTheDocument();
  });

  it("deletes the case after confirmation and navigates back to the list", async () => {
    const user = userEvent.setup();
    renderPage();

    await screen.findByRole("tab", { name: "Overview" });
    await user.click(screen.getByRole("button", { name: "Delete permanently" }));
    // The dialog's confirm action carries the alert-dialog-action data-slot.
    const dialogConfirm = await waitFor(() => {
      const match = screen
        .getAllByRole("button", { name: "Delete permanently" })
        .find((b) => b.getAttribute("data-slot") === "alert-dialog-action");
      if (!match) throw new Error("dialog confirm not mounted yet");
      return match;
    });
    await user.click(dialogConfirm);

    await waitFor(() => expect(mocks.deleteWorkflowCase).toHaveBeenCalledWith("case-1"));
    await waitFor(() => expect(mocks.navigationPush).toHaveBeenCalledWith("/workflow-cases"));
  });

  it("names the section Draft and enters the canvas+YAML editor from Edit draft", async () => {
    renderPage();

    await userEvent.click(await screen.findByRole("tab", { name: "Workflow" }));
    // Section is titled "Draft", not "Draft workspace".
    expect(screen.getByRole("heading", { name: "Draft" })).toBeInTheDocument();
    expect(screen.queryByText("Draft workspace")).not.toBeInTheDocument();
    // Non-edit state shows a read-only canvas and an Edit draft button.
    expect(screen.getByTestId("workflow-canvas")).toBeInTheDocument();
    expect(screen.queryByTestId("workflow-editor")).not.toBeInTheDocument();

    await userEvent.click(screen.getByRole("button", { name: "Edit draft" }));
    // Editing state renders the shared canvas+YAML editor.
    expect(await screen.findByTestId("workflow-editor")).toBeInTheDocument();
  });

  it("creates a starter draft from an empty case via the editor", async () => {
    mocks.getWorkflowCaseDefinition.mockResolvedValue(null);
    mocks.upsertWorkflowCaseDefinitionDraft.mockResolvedValue(makeDraft());

    renderPage();

    await userEvent.click(await screen.findByRole("tab", { name: "Workflow" }));
    expect(await screen.findByText("No definition draft is available for this workflow case.")).toBeInTheDocument();
    fireEvent.click(screen.getAllByRole("button", { name: "Create starter draft" }).at(-1)!);
    expect(await screen.findByTestId("workflow-editor")).toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: "Save draft" }));

    await waitFor(() => {
      expect(mocks.upsertWorkflowCaseDefinitionDraft).toHaveBeenCalledWith(
        "case-1",
        expect.objectContaining({
          source_templates: [],
          draft_json: expect.objectContaining({
            meta: expect.objectContaining({ name: "workflow-case" }),
          }),
        }),
      );
    });
  });
});

function renderPage() {
  const qc = new QueryClient({
    defaultOptions: {
      queries: { retry: false },
      mutations: { retry: false },
    },
  });
  return render(
    <QueryClientProvider client={qc}>
      <WorkflowCaseDetailPage caseId="case-1" />
    </QueryClientProvider>,
  );
}

const definition: WorkflowDefinition = {
  meta: { name: "Case workflow", version: "1" },
  state: { fields: [] },
  nodes: [{ id: "plan", type: "agent", dispatch: "subissue" }],
  routing: [{ from: "START", to: "plan" }],
};

function makeCase(overrides: Partial<WorkflowCase> = {}): WorkflowCase {
  return {
    id: "case-1",
    workspace_id: "ws-1",
    title: "Workflow case title",
    description: "Plan this issue",
    entry_issue_id: "issue-1",
    source_issue_id: "issue-1",
    owner_agent_id: null,
    status: "draft",
    online_version_id: "version-1",
    current_run_id: null,
    created_at: "2026-07-01T00:00:00Z",
    updated_at: "2026-07-01T00:00:00Z",
    ...overrides,
  };
}

function makeDraft(): WorkflowDefinitionDraft {
  return {
    id: "definition-1",
    case_id: "case-1",
    draft_json: definition,
    source_templates: [],
    status: "draft",
    created_at: "2026-07-01T00:00:00Z",
    updated_at: "2026-07-01T00:00:00Z",
  };
}

function makeValidation(overrides: Partial<WorkflowValidationReport> = {}): WorkflowValidationReport {
  return {
    valid: true,
    errors: [],
    warnings: [],
    ...overrides,
  };
}

function makeVersion(overrides: Partial<WorkflowDefinitionVersion> = {}): WorkflowDefinitionVersion {
  return {
    id: "version-1",
    workspace_id: "ws-1",
    case_id: "case-1",
    definition_id: "definition-1",
    version: 1,
    snapshot_json: definition,
    source_skills: [],
    validation_report: makeValidation({ valid: true }),
    confirmed_by: "user-1",
    confirmed_at: "2026-07-01T00:00:00Z",
    created_at: "2026-07-01T00:00:00Z",
    ...overrides,
  };
}

function makeRun(overrides: Partial<WorkflowRun> = {}): WorkflowRun {
  return {
    id: "run-1",
    root_issue_id: "issue-1",
    case_id: "case-1",
    definition_version_id: "version-1",
    skill_id: null,
    status: "done",
    current_node: "plan",
    nodes_state: {
      plan: {
        status: "done",
        sub_issue_id: "sub-1",
        started_at: "2026-07-01T00:00:00Z",
        ended_at: "2026-07-01T00:01:00Z",
        error: null,
      },
    },
    nodes: [{
      id: "run-node-1",
      run_id: "run-1",
      node_id: "plan",
      node_type: "agent",
      dispatch: "subissue",
      carrier_kind: "issue",
      status: "succeeded",
      attempt: 1,
      input_snapshot: { issue: "issue-1" },
      output_snapshot: { result: "ok" },
      error: null,
      logs: ["done"],
      carrier_ref: { sub_issue_id: "sub-1" },
      started_at: "2026-07-01T00:00:00Z",
      completed_at: "2026-07-01T00:01:00Z",
      updated_at: "2026-07-01T00:01:00Z",
    }],
    definition_snapshot: definition,
    error: null,
    started_at: "2026-07-01T00:00:00Z",
    completed_at: "2026-07-01T00:01:00Z",
    created_at: "2026-07-01T00:00:00Z",
    updated_at: "2026-07-01T00:01:00Z",
    ...overrides,
  };
}
