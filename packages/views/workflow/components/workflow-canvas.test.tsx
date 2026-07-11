import { fireEvent, render, screen, within } from "@testing-library/react";
import { useState } from "react";
import { describe, expect, it, vi } from "vitest";
import type { WorkflowDefinition } from "@multica/core/workflow/types";

let mockSetNodes: ((updater: unknown) => void) | null = null;
let mockSetEdges: ((updater: unknown) => void) | null = null;

type MockFlowNode = {
  id: string;
  draggable?: boolean;
  data: {
    carrierLabel?: string;
    hoveredRelated?: boolean;
    mutedByHover?: boolean;
    runningNow?: boolean;
  };
};

vi.mock("@xyflow/react", () => ({
  BaseEdge: ({ path }: { path: string }) => <path data-testid="workflow-edge-path" data-path={path} />,
  Background: () => <div data-testid="background" />,
  Controls: () => <div data-testid="controls" />,
  EdgeText: ({ label }: { label: string }) => <text>{label}</text>,
  Handle: ({ id, type, position }: { id?: string; type: string; position: string }) => (
    <div data-testid="handle" data-handle-id={id} data-type={type} data-position={position} />
  ),
  MarkerType: { ArrowClosed: "arrowclosed" },
  Position: { Bottom: "bottom", Left: "left", Right: "right", Top: "top" },
  ReactFlowProvider: ({ children }: { children: React.ReactNode }) => <>{children}</>,
  ReactFlow: ({
    nodes,
    edges,
    defaultNodes,
    defaultEdges,
    nodesDraggable,
    nodesConnectable,
    nodeTypes,
    edgeTypes,
    children,
    onNodeClick,
    onNodeMouseEnter,
    onNodeMouseLeave,
    onEdgeMouseEnter,
    onEdgeMouseLeave,
    onEdgeClick,
  }: {
    nodes?: MockFlowNode[];
    edges?: Array<{ id: string; label?: string; type?: string; data?: { fullLabel?: string; hovered?: boolean; mutedByHover?: boolean; routePoints?: Array<{ x: number; y: number }> } }>;
    defaultNodes?: MockFlowNode[];
    defaultEdges?: Array<{ id: string; label?: string; type?: string; data?: { fullLabel?: string; hovered?: boolean; mutedByHover?: boolean; routePoints?: Array<{ x: number; y: number }> } }>;
    nodesDraggable?: boolean;
    nodesConnectable?: boolean;
    nodeTypes: Record<string, React.ComponentType<{ data: unknown }>>;
    edgeTypes?: Record<string, React.ComponentType<Record<string, unknown>>>;
    children: React.ReactNode;
    onNodeClick?: (event: unknown, node: { id: string }) => void;
    onNodeMouseEnter?: (event: unknown, node: { id: string }) => void;
    onNodeMouseLeave?: () => void;
    onEdgeMouseEnter?: (event: unknown, edge: { id: string }) => void;
    onEdgeMouseLeave?: () => void;
    onEdgeClick?: (event: unknown, edge: { id: string }) => void;
  }) => {
    const [internalNodes, setInternalNodes] = useState(nodes ?? defaultNodes ?? []);
    const [internalEdges, setInternalEdges] = useState(edges ?? defaultEdges ?? []);
    mockSetNodes = (updater: unknown) => {
      setInternalNodes((current) => (typeof updater === "function" ? updater(current) : updater) as typeof current);
    };
    mockSetEdges = (updater: unknown) => {
      setInternalEdges((current) => (typeof updater === "function" ? updater(current) : updater) as typeof current);
    };
    const renderedNodes = nodes ?? internalNodes;
    const renderedEdges = edges ?? internalEdges;
    const WorkflowEdgeComponent = edgeTypes?.workflow;

    return (
    <div
      data-testid="react-flow"
      data-nodes-draggable={String(Boolean(nodesDraggable))}
      data-nodes-connectable={String(Boolean(nodesConnectable))}
      data-edge-types={Object.keys(edgeTypes ?? {}).join(",")}
    >
      {renderedNodes.map((node) => {
        const NodeComponent = nodeTypes.workflow;
        return (
          <div
            key={node.id}
            data-testid={`node-wrapper-${node.id}`}
            data-draggable={String(Boolean(node.draggable))}
            onClick={() => onNodeClick?.({}, node)}
            onMouseEnter={() => onNodeMouseEnter?.({}, node)}
            onMouseLeave={() => onNodeMouseLeave?.()}
          >
            {NodeComponent ? <NodeComponent data={node.data} /> : <div>{node.id} {node.data.carrierLabel}</div>}
          </div>
        );
      })}
      {renderedEdges.map((edge) => (
        <div key={edge.id}>
          <button
            type="button"
            data-testid={`edge-${edge.id}`}
            data-full-label={edge.data?.fullLabel}
            data-hovered={String(Boolean(edge.data?.hovered))}
            data-muted={String(Boolean(edge.data?.mutedByHover))}
            data-edge-type={edge.type}
            data-route-points={String(edge.data?.routePoints?.length ?? 0)}
            onMouseEnter={() => onEdgeMouseEnter?.({}, edge)}
            onMouseLeave={() => onEdgeMouseLeave?.()}
            onClick={() => onEdgeClick?.({}, edge)}
          >
            {edge.label}
          </button>
          {WorkflowEdgeComponent ? (
            <svg>
              <WorkflowEdgeComponent
                id={edge.id}
                source={edge.id}
                target={edge.id}
                sourceX={10}
                sourceY={20}
                targetX={210}
                targetY={120}
                data={edge.data}
                label={edge.label}
                style={{}}
              />
            </svg>
          ) : null}
        </div>
      ))}
      {children}
    </div>
  );
  },
  applyNodeChanges: (_changes: unknown, nodes: unknown) => nodes,
  useReactFlow: () => ({
    setNodes: (updater: unknown) => {
      mockSetNodes?.(updater);
    },
    setEdges: (updater: unknown) => {
      mockSetEdges?.(updater);
    },
  }),
}));

vi.mock("@xyflow/react/dist/style.css", () => ({}));

import { WorkflowCanvas } from "./workflow-canvas";

describe("WorkflowCanvas", () => {
  it("opens and closes fullscreen overlay", () => {
    render(<WorkflowCanvas definition={definition} fullscreenTitle="Runtime workflow" />);

    fireEvent.click(screen.getByRole("button", { name: "Expand workflow canvas" }));

    const dialog = screen.getByRole("dialog", { name: "Runtime workflow" });
    expect(dialog).toBeInTheDocument();
    expect(within(dialog).getByRole("button", { name: "Close fullscreen workflow canvas" }).closest(".absolute")).toHaveClass("z-30");

    fireEvent.click(screen.getByRole("button", { name: "Close fullscreen workflow canvas" }));
    expect(screen.queryByRole("dialog", { name: "Runtime workflow" })).not.toBeInTheDocument();
  });

  it("renders directional handles used by routed workflow edges", () => {
    render(<WorkflowCanvas definition={definition} />);

    expect(screen.getByTestId("react-flow")).toBeInTheDocument();
    expect(screen.getByTestId("react-flow")).toHaveAttribute("data-edge-types", "workflow");
    expect(screen.getAllByTestId("handle").map((handle) => handle.getAttribute("data-handle-id")).sort()).toEqual([
      "source-bottom",
      "source-right",
      "source-top",
      "target-bottom",
      "target-left",
      "target-top",
    ]);
  });

  it("allows temporary node dragging in read-only canvases without enabling connections", () => {
    render(<WorkflowCanvas definition={definition} />);

    expect(screen.getByTestId("react-flow")).toHaveAttribute("data-nodes-draggable", "true");
    expect(screen.getByTestId("react-flow")).toHaveAttribute("data-nodes-connectable", "false");
    expect(screen.getByTestId("node-wrapper-node_a")).toHaveAttribute("data-draggable", "true");
  });

  it("preserves full edge labels and marks related graph elements while hovering an edge", () => {
    render(<WorkflowCanvas definition={branchDefinition} />);

    const edge = screen.getByTestId("edge-edge-1-to");
    expect(edge).toHaveAttribute("data-full-label", "a_very_long_condition_expression == true && second_check == true");
    expect(edge).toHaveAttribute("data-edge-type", "workflow");
    expect(Number(edge.getAttribute("data-route-points"))).toBeGreaterThanOrEqual(2);
    expect(screen.getAllByTestId("workflow-edge-path")[0]).toHaveAttribute("data-path", expect.stringContaining("M 10 20"));

    fireEvent.mouseEnter(edge);

    expect(edge).toHaveAttribute("data-hovered", "true");
    expect(screen.getByText("start").closest(".relative")).toHaveClass("ring-1");
    expect(screen.getByText("selected").closest(".relative")).toHaveClass("ring-1");
    expect(screen.getByText("fallback").closest(".relative")).toHaveClass("opacity-45");
    expect(screen.getByTestId("edge-edge-1-else")).toHaveAttribute("data-muted", "true");
  });

  it("marks related edges while hovering a node", () => {
    render(<WorkflowCanvas definition={branchDefinition} />);

    fireEvent.mouseEnter(screen.getByTestId("node-wrapper-start"));

    expect(screen.getByTestId("edge-edge-1-to")).toHaveAttribute("data-hovered", "true");
    expect(screen.getByTestId("edge-edge-1-else")).toHaveAttribute("data-hovered", "true");
  });

  it("adds a runtime ring to the running node", () => {
    render(
      <WorkflowCanvas
        definition={definition}
        runState={{
          node_a: { status: "running", sub_issue_id: null, started_at: null, ended_at: null, error: null },
        }}
      />,
    );

    expect(screen.getByText("node_a").closest(".relative")).toHaveClass("animate-pulse");
  });

  it("updates inline node status when run state changes", () => {
    const { rerender } = render(<WorkflowCanvas definition={definition} runState={{}} />);

    expect(screen.getByText("node_a").closest(".relative")).not.toHaveClass("animate-pulse");

    rerender(
      <WorkflowCanvas
        definition={definition}
        runState={{
          node_a: { status: "running", sub_issue_id: null, started_at: null, ended_at: null, error: null },
        }}
      />,
    );

    expect(screen.getByText("node_a").closest(".relative")).toHaveClass("animate-pulse");
  });

  it("can rerender from an empty workflow to a populated workflow", () => {
    const { rerender } = render(<WorkflowCanvas definition={emptyDefinition} />);

    expect(screen.getByText("No workflow nodes.")).toBeInTheDocument();

    expect(() => rerender(<WorkflowCanvas definition={definition} />)).not.toThrow();
    expect(screen.getByTestId("react-flow")).toBeInTheDocument();
  });

  it("selects a node on click and opens sub-issues only through the explicit action", () => {
    const onSelectNode = vi.fn();
    const onOpenSubIssue = vi.fn();
    render(
      <WorkflowCanvas
        definition={definition}
        runState={{
          node_a: { status: "done", sub_issue_id: "sub-1", started_at: null, ended_at: null, error: null },
        }}
        onSelectNode={onSelectNode}
        onOpenSubIssue={onOpenSubIssue}
      />,
    );

    fireEvent.click(screen.getByText("node_a"));
    expect(onSelectNode).toHaveBeenCalledWith("node_a");
    expect(onOpenSubIssue).not.toHaveBeenCalled();

    fireEvent.click(screen.getByRole("button", { name: "Open sub-issue for node_a" }));
    expect(onOpenSubIssue).toHaveBeenCalledWith("sub-1");
  });

  it("shows a closeable details overlay for selected nodes and edges", () => {
    render(
      <WorkflowCanvas
        definition={branchDefinition}
        runState={{
          selected: { status: "done", sub_issue_id: "sub-2", started_at: null, ended_at: null, error: null },
        }}
      />,
    );

    fireEvent.click(screen.getByText("selected"));
    expect(screen.getByText("Node details")).toBeInTheDocument();
    expect(screen.getByText("subissue")).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Close workflow details" }));
    expect(screen.queryByText("Node details")).not.toBeInTheDocument();

    fireEvent.click(screen.getByTestId("edge-edge-1-to"));
    expect(screen.getByText("Edge details")).toBeInTheDocument();
    expect(screen.getByText("a_very_long_condition_expression == true && second_check == true")).toBeInTheDocument();
  });

  it("shows node definition route and prompt in the details overlay", () => {
    render(<WorkflowCanvas definition={definitionWithPrompt} />);

    fireEvent.click(screen.getByText("investigate"));

    expect(screen.getByText("Node details")).toBeInTheDocument();
    expect(screen.getByText("Agent route")).toBeInTheDocument();
    expect(screen.getByText("frontend-bug-investigation")).toBeInTheDocument();
    expect(screen.getByText("Inputs")).toBeInTheDocument();
    expect(screen.getByText("task, scope_result")).toBeInTheDocument();
    expect(screen.getByText("Outputs")).toBeInTheDocument();
    expect(screen.getByText("investigation_report")).toBeInTheDocument();
    expect(screen.getByText("System prompt")).toBeInTheDocument();
    expect(screen.getByText("Investigate the target platform and capture browser evidence.")).toBeInTheDocument();
  });

  it("keeps the details overlay inside the active canvas container in fullscreen", () => {
    render(<WorkflowCanvas definition={branchDefinition} fullscreenTitle="Runtime workflow" />);

    fireEvent.click(screen.getByRole("button", { name: "Expand workflow canvas" }));

    const dialog = screen.getByRole("dialog", { name: "Runtime workflow" });
    fireEvent.click(within(dialog).getByText("selected"));

    const overlay = within(dialog).getByText("Node details").closest("aside");
    expect(overlay).toBeInTheDocument();
    expect(overlay).toHaveClass("absolute", "right-3", "top-3");

    fireEvent.click(within(dialog).getByRole("button", { name: "Close workflow details" }));
    expect(within(dialog).queryByText("Node details")).not.toBeInTheDocument();
  });
});

const definition: WorkflowDefinition = {
  meta: { name: "test", version: "1" },
  state: { fields: [] },
  nodes: [{ id: "node_a", type: "agent", dispatch: "subissue" }],
  routing: [{ from: "START", to: "node_a" }],
};

const emptyDefinition: WorkflowDefinition = {
  meta: { name: "empty", version: "1" },
  state: { fields: [] },
  nodes: [],
  routing: [],
};

const branchDefinition: WorkflowDefinition = {
  meta: { name: "branch", version: "1" },
  state: { fields: [] },
  nodes: [
    { id: "start", type: "router" },
    { id: "selected", type: "agent", dispatch: "subissue" },
    { id: "fallback", type: "agent", dispatch: "subissue" },
  ],
  routing: [
    { from: "START", to: "start" },
    { from: "start", to: "selected", condition: "a_very_long_condition_expression == true && second_check == true", else: "fallback" },
  ],
};

const definitionWithPrompt: WorkflowDefinition = {
  meta: { name: "prompted", version: "1" },
  state: { fields: [] },
  nodes: [{
    id: "investigate",
    type: "agent",
    dispatch: "subissue",
    agent: "frontend-bug-investigation",
    inputs: ["task", "scope_result"],
    outputs: ["investigation_report"],
    config: {
      system: "Investigate the target platform and capture browser evidence.",
    },
  }],
  routing: [{ from: "START", to: "investigate" }],
};
