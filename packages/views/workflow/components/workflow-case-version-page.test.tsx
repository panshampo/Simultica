import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import type {
  WorkflowCase,
  WorkflowDefinition,
  WorkflowDefinitionVersion,
} from "@multica/core/workflow/types";

const mocks = vi.hoisted(() => ({
  getWorkflowCase: vi.fn(),
  getWorkflowCaseDefinitionVersion: vi.fn(),
}));

vi.mock("@multica/core/api", () => ({
  api: {
    getWorkflowCase: mocks.getWorkflowCase,
    getWorkflowCaseDefinitionVersion: mocks.getWorkflowCaseDefinitionVersion,
  },
}));

vi.mock("@multica/core/hooks", () => ({
  useWorkspaceId: () => "ws-1",
}));

vi.mock("@multica/core/paths", () => ({
  useWorkspacePaths: () => ({
    workflowCaseDetail: (id: string) => `/workflow-cases/${id}`,
  }),
}));

vi.mock("../../navigation", () => ({
  AppLink: ({ href, children, className, ...props }: { href: string; children: React.ReactNode; className?: string }) => (
    <a href={href} className={className} {...props}>{children}</a>
  ),
}));

vi.mock("./workflow-canvas", () => ({
  WorkflowCanvas: ({ definition }: { definition: WorkflowDefinition }) => (
    <section data-testid="workflow-canvas">
      {definition.meta.name}
    </section>
  ),
}));

import { WorkflowCaseVersionPage } from "./workflow-case-version-page";

describe("WorkflowCaseVersionPage", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mocks.getWorkflowCase.mockResolvedValue(makeCase());
    mocks.getWorkflowCaseDefinitionVersion.mockResolvedValue(makeVersion());
  });

  it("renders a historical WorkflowCase version as read-only", async () => {
    renderPage();

    expect(await screen.findByText("Workflow case title")).toBeInTheDocument();
    expect(screen.getAllByText("v1").length).toBeGreaterThan(0);
    expect(screen.getAllByText("Historical").length).toBeGreaterThan(0);
    expect(screen.getByText("Published version snapshot")).toBeInTheDocument();
    expect(screen.getByText("Version canvas")).toBeInTheDocument();
    expect(screen.getByTestId("workflow-canvas")).toHaveTextContent("Historical workflow");
    expect(screen.getByText("YAML snapshot")).toBeInTheDocument();
    expect(screen.getByLabelText("Workflow version YAML")).toHaveTextContent("Historical workflow");
    expect(screen.getByLabelText("Workflow version YAML")).toHaveTextContent("inspect");
    expect(screen.queryByRole("button", { name: "Create run" })).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: /Publish/i })).not.toBeInTheDocument();
    expect(screen.getByRole("link", { name: "Back to workflow case" })).toHaveAttribute("href", "/workflow-cases/case-1");
  });

  it("marks the online version when the case points at this version", async () => {
    mocks.getWorkflowCase.mockResolvedValue(makeCase({ online_version_id: "version-1" }));

    renderPage();

    expect((await screen.findAllByText("Online")).length).toBeGreaterThan(0);
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
      <WorkflowCaseVersionPage caseId="case-1" versionId="version-1" />
    </QueryClientProvider>,
  );
}

const definition: WorkflowDefinition = {
  meta: { name: "Historical workflow", version: "1" },
  state: { fields: [] },
  nodes: [{ id: "inspect", type: "agent", dispatch: "subissue" }],
  routing: [{ from: "START", to: "inspect" }],
};

function makeCase(overrides: Partial<WorkflowCase> = {}): WorkflowCase {
  return {
    id: "case-1",
    workspace_id: "ws-1",
    title: "Workflow case title",
    description: "Versioned workflow",
    entry_issue_id: "issue-1",
    source_issue_id: "issue-1",
    owner_agent_id: null,
    status: "draft",
    online_version_id: "version-2",
    current_run_id: null,
    created_at: "2026-07-08T00:00:00Z",
    updated_at: "2026-07-08T00:00:00Z",
    ...overrides,
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
    confirmed_at: "2026-07-08T00:00:00Z",
    created_at: "2026-07-08T00:00:00Z",
  };
}
