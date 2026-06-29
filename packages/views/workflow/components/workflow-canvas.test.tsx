import { fireEvent, render, screen, within } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import type { WorkflowDefinition } from "@multica/core/workflow/types";

vi.mock("@xyflow/react", () => ({
  Background: () => <div data-testid="background" />,
  Controls: () => <div data-testid="controls" />,
  Handle: () => <div data-testid="handle" />,
  MarkerType: { ArrowClosed: "arrowclosed" },
  Position: { Left: "left", Right: "right" },
  ReactFlow: ({ nodes, children }: { nodes: Array<{ id: string; data: { carrierLabel?: string } }>; children: React.ReactNode }) => (
    <div data-testid="react-flow">
      {nodes.map((node) => (
        <div key={node.id}>{node.id} {node.data.carrierLabel}</div>
      ))}
      {children}
    </div>
  ),
  applyNodeChanges: (_changes: unknown, nodes: unknown) => nodes,
}));

vi.mock("@xyflow/react/dist/style.css", () => ({}));

import { WorkflowCanvas } from "./workflow-canvas";

describe("WorkflowCanvas", () => {
  it("opens and closes fullscreen overlay", () => {
    render(<WorkflowCanvas definition={definition} fullscreenTitle="Runtime workflow" />);

    fireEvent.click(screen.getByRole("button", { name: "Expand workflow canvas" }));

    const dialog = screen.getByRole("dialog", { name: "Runtime workflow" });
    expect(dialog).toBeInTheDocument();
    expect(within(dialog).getByText("Runtime workflow")).toBeInTheDocument();
    expect(within(dialog).getByText("Fullscreen")).toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: "Close fullscreen workflow canvas" }));
    expect(screen.queryByRole("dialog", { name: "Runtime workflow" })).not.toBeInTheDocument();
  });
});

const definition: WorkflowDefinition = {
  meta: { name: "test", version: "1" },
  state: { fields: [] },
  nodes: [{ id: "node_a", type: "agent", dispatch: "subissue" }],
  routing: [{ from: "START", to: "node_a" }],
};
