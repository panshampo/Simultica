import { render, screen, within } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import type { WorkflowRun } from "@multica/core/workflow/types";
import { WorkflowCaseRunList } from "./workflow-case-run-list";

vi.mock("../../navigation", () => ({
  AppLink: ({ href, children, className }: { href: string; children: React.ReactNode; className?: string }) => (
    <a href={href} className={className}>{children}</a>
  ),
}));

describe("WorkflowCaseRunList", () => {
  it("orders runs by most recently created first", () => {
    render(
      <WorkflowCaseRunList
        runs={[
          makeRun({ id: "older-run", label: "Older", created_at: "2026-07-01T00:00:00Z", started_at: "2026-07-03T00:00:00Z" }),
          makeRun({ id: "newest-run", label: "Newest", created_at: "2026-07-03T00:00:00Z", started_at: "2026-07-01T00:00:00Z" }),
          makeRun({ id: "middle-run", label: "Middle", created_at: "2026-07-02T00:00:00Z", started_at: "2026-07-02T00:00:00Z" }),
        ]}
        selectedRunId="older-run"
        onSelectRun={() => {}}
      />,
    );

    const rows = screen.getAllByRole("listitem");
    expect(within(rows[0]!).getByText("newest-r")).toBeInTheDocument();
    expect(within(rows[1]!).getByText("middle-r")).toBeInTheDocument();
    expect(within(rows[2]!).getByText("older-ru")).toBeInTheDocument();
  });
});

function makeRun(overrides: Partial<WorkflowRun> = {}): WorkflowRun {
  return {
    id: "run-1",
    root_issue_id: "issue-1",
    case_id: "case-1",
    definition_version_id: "version-1",
    skill_id: null,
    status: "done",
    label: "Run",
    current_node: "END",
    nodes_state: {},
    nodes: [],
    definition_snapshot: {
      meta: { name: "workflow", version: "1" },
      state: { fields: [] },
      nodes: [],
      routing: [],
    },
    error: null,
    started_at: "2026-07-01T00:00:00Z",
    completed_at: "2026-07-01T00:01:00Z",
    created_at: "2026-07-01T00:00:00Z",
    updated_at: "2026-07-01T00:01:00Z",
    ...overrides,
  };
}
