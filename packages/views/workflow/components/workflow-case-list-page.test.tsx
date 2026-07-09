import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";
import type { WorkflowCase } from "@multica/core/workflow/types";

const mocks = vi.hoisted(() => ({
  listWorkflowCases: vi.fn(),
  createWorkflowCase: vi.fn(),
  navigationPush: vi.fn(),
}));

vi.mock("@multica/core/api", () => ({
  api: {
    listWorkflowCases: mocks.listWorkflowCases,
    createWorkflowCase: mocks.createWorkflowCase,
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
    <a href={href} className={className} {...props}>
      {children}
    </a>
  ),
  useNavigation: () => ({ push: mocks.navigationPush }),
}));

import { WorkflowCaseListPage } from "./workflow-case-list-page";

describe("WorkflowCaseListPage", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mocks.listWorkflowCases.mockResolvedValue([makeCase()]);
    mocks.createWorkflowCase.mockResolvedValue(makeCase({ id: "case-new", title: "My workflow case" }));
  });

  it("opens a create dialog instead of creating an Untitled case immediately", async () => {
    renderPage();

    await userEvent.click(await screen.findByRole("button", { name: "Create case" }));

    expect(screen.getByRole("dialog")).toBeInTheDocument();
    expect(screen.getByLabelText("Case title")).toBeInTheDocument();
    expect(mocks.createWorkflowCase).not.toHaveBeenCalled();
  });

  it("requires a title before creating a workflow case", async () => {
    renderPage();

    await userEvent.click(await screen.findByRole("button", { name: "Create case" }));
    await userEvent.click(screen.getByRole("button", { name: "Create workflow case" }));

    expect(await screen.findByText("Case title is required")).toBeInTheDocument();
    expect(mocks.createWorkflowCase).not.toHaveBeenCalled();
  });

  it("creates a workflow case from dialog values and navigates to detail", async () => {
    mocks.createWorkflowCase.mockResolvedValue(makeCase({ id: "case-new", title: "My workflow case" }));
    renderPage();

    await userEvent.click(await screen.findByRole("button", { name: "Create case" }));
    await userEvent.type(screen.getByLabelText("Case title"), "My workflow case");
    await userEvent.type(screen.getByLabelText("Description"), "Plan this work");
    await userEvent.click(screen.getByRole("button", { name: "Create workflow case" }));

    await waitFor(() => {
      expect(mocks.createWorkflowCase).toHaveBeenCalledWith({
        title: "My workflow case",
        description: "Plan this work",
        source_issue_id: null,
        owner_agent_id: null,
      });
    });
    await waitFor(() => expect(mocks.navigationPush).toHaveBeenCalledWith("/workflow-cases/case-new"));
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
      <WorkflowCaseListPage />
    </QueryClientProvider>,
  );
}

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
