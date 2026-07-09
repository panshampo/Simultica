import { render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import type { WorkflowNode, WorkflowRunNode } from "@multica/core/workflow/types";

vi.mock("../../navigation", () => ({
  AppLink: ({ href, children }: { href: string; children: React.ReactNode }) => <a href={href}>{children}</a>,
}));

import { WorkflowRunNodeDetailPanel } from "./workflow-run-node-detail-panel";

describe("WorkflowRunNodeDetailPanel", () => {
  it("labels the definition block with product-aligned Step copy", () => {
    render(<WorkflowRunNodeDetailPanel node={makeRunNode()} definitionNode={makeDefinitionNode()} />);

    // Step-oriented copy, never "Definition".
    expect(screen.getByText("Step configuration")).toBeInTheDocument();
    expect(screen.getByText("Step type")).toBeInTheDocument();
    expect(screen.getByText("Step dispatch")).toBeInTheDocument();
    expect(screen.queryByText("Definition")).not.toBeInTheDocument();
    expect(screen.queryByText("Definition type")).not.toBeInTheDocument();
    expect(screen.queryByText("Definition dispatch")).not.toBeInTheDocument();
  });
});

function makeRunNode(overrides: Partial<WorkflowRunNode> = {}): WorkflowRunNode {
  return {
    id: "run-node-1",
    run_id: "run-1",
    node_id: "plan",
    node_type: "agent",
    dispatch: "subissue",
    carrier_kind: "issue",
    status: "succeeded",
    attempt: 1,
    input_snapshot: {},
    output_snapshot: {},
    error: null,
    logs: [],
    carrier_ref: {},
    started_at: "2026-07-01T00:00:00Z",
    completed_at: "2026-07-01T00:01:00Z",
    updated_at: "2026-07-01T00:01:00Z",
    ...overrides,
  };
}

function makeDefinitionNode(overrides: Partial<WorkflowNode> = {}): WorkflowNode {
  return {
    id: "plan",
    type: "agent",
    dispatch: "subissue",
    ...overrides,
  };
}
