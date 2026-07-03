import { describe, expect, it } from "vitest";
import type { WorkflowDefinition } from "@multica/core/workflow/types";
import { WORKFLOW_NODE_HEIGHT, WORKFLOW_NODE_WIDTH, workflowToReactFlow } from "./to-react-flow";

const definition: WorkflowDefinition = {
  meta: { name: "canvas" },
  state: { fields: [] },
  nodes: [
    { id: "plan", type: "llm" },
    { id: "impl", type: "llm" },
    { id: "review", type: "router" },
  ],
  routing: [
    { from: "START", to: "plan" },
    { from: "plan", to: "impl" },
    { from: "impl", to: "review", condition: "ok == true" },
    { from: "review", to: "END" },
  ],
};

describe("workflowToReactFlow", () => {
  it("creates positioned nodes and edges", () => {
    const { nodes, edges } = workflowToReactFlow(definition);
    expect(nodes.map((n) => n.id)).toEqual(["plan", "impl", "review"]);
    expect(nodes.find((n) => n.id === "impl")?.position.x).toBeGreaterThan(nodes.find((n) => n.id === "plan")!.position.x);
    expect(edges).toHaveLength(2);
    expect(edges[1]?.label).toBe("ok == true");
    expect(edges[1]?.animated).toBe(false);
    expect(edges[1]?.data).toEqual(expect.objectContaining({
      routeKind: "condition",
      fullLabel: "ok == true",
    }));
  });

  it("shortens long condition labels while preserving the full expression", () => {
    const longCondition = "state.review.feedback.needs_another_iteration == true && state.review.severity >= 2";
    const { edges } = workflowToReactFlow({
      ...definition,
      routing: [
        { from: "START", to: "plan" },
        { from: "plan", to: "impl", condition: longCondition },
      ],
    });

    expect(edges[0]?.label).toBe("state.review.feedback.needs_another_iteration...");
    expect(edges[0]?.data).toEqual(expect.objectContaining({
      routeKind: "condition",
      fullLabel: longCondition,
    }));
  });

  it("uses route decisions to highlight selected runtime branches and mute unselected branches", () => {
    const branchDefinition: WorkflowDefinition = {
      meta: { name: "branch" },
      state: { fields: [] },
      nodes: [
        { id: "gate", type: "router" },
        { id: "fix", type: "agent" },
        { id: "finish", type: "final_response" },
      ],
      routing: [
        { from: "START", to: "gate" },
        { from: "gate", condition: "needs_fix == true", to: "fix", else: "finish" },
      ],
    };

    const { edges } = workflowToReactFlow(branchDefinition, {
      gate: {
        status: "done",
        sub_issue_id: null,
        started_at: null,
        ended_at: null,
        error: null,
        route_decision: {
          condition: "needs_fix == true",
          condition_result: false,
          selected_route: "finish",
          else_route: "finish",
        },
      },
    });
    const conditionEdge = edges.find((edge) => edge.id === "edge-1-to")!;
    const elseEdge = edges.find((edge) => edge.id === "edge-1-else")!;

    expect(conditionEdge.animated).toBe(false);
    expect(conditionEdge.data).toEqual(expect.objectContaining({
      mutedByRouteDecision: true,
      selectedByRouteDecision: false,
    }));
    expect(elseEdge.animated).toBe(true);
    expect(elseEdge.data).toEqual(expect.objectContaining({
      mutedByRouteDecision: false,
      selectedByRouteDecision: true,
    }));
  });

  it("maps runtime state onto node data", () => {
    const { nodes } = workflowToReactFlow(definition, {
      impl: { status: "running", sub_issue_id: "sub-1", started_at: null, ended_at: null, error: null },
    });
    const impl = nodes.find((n) => n.id === "impl")!;
    expect(impl.data.status).toBe("running");
    expect(impl.data.subIssueId).toBe("sub-1");
  });

  it("uses explicit handles saved on manually drawn routes", () => {
    const { edges } = workflowToReactFlow({
      ...definition,
      routing: [
        { from: "START", to: "plan" },
        { from: "plan", to: "impl", sourceHandle: "source-bottom", targetHandle: "target-top" },
      ],
    });

    expect(edges[0]).toEqual(expect.objectContaining({
      sourceHandle: "source-bottom",
      targetHandle: "target-top",
    }));
  });

  it("handles loop-back routes without unbounded layer growth", () => {
    const loopDefinition: WorkflowDefinition = {
      meta: { name: "loop" },
      state: { fields: [] },
      nodes: [
        { id: "author", type: "agent" },
        { id: "review", type: "agent" },
        { id: "gate", type: "transform" },
        { id: "final", type: "final_response" },
      ],
      routing: [
        { from: "START", to: "author" },
        { from: "author", to: "review" },
        { from: "review", to: "gate" },
        { from: "gate", condition: "needs_fix == true", to: "author", else: "final" },
        { from: "final", to: "END" },
      ],
    };

    const { nodes, edges } = workflowToReactFlow(loopDefinition);
    const nodesById = new Map(nodes.map((node) => [node.id, node]));
    const backEdge = edges.find((edge) => edge.id === "edge-3-to")!;
    const elseEdge = edges.find((edge) => edge.id === "edge-3-else")!;

    expect(nodes.map((node) => node.id).sort()).toEqual(["author", "final", "gate", "review"]);
    expect(Math.max(...nodes.map((node) => node.position.x))).toBeLessThan(2000);
    expect(nodesById.get("author")!.position.x).toBeLessThan(nodesById.get("review")!.position.x);
    expect(nodesById.get("review")!.position.x).toBeLessThan(nodesById.get("gate")!.position.x);
    expect(nodesById.get("final")!.position.x).toBeGreaterThan(nodesById.get("gate")!.position.x);
    expect(nodesById.get("final")!.position.y).not.toBe(nodesById.get("gate")!.position.y);
    expect(nodesDoNotOverlap(nodes)).toBe(true);
    expect(edges).toHaveLength(4);
    expect(edges).toEqual(
      expect.arrayContaining([
        expect.objectContaining({ id: "edge-3-to", source: "gate", target: "author", label: "needs_fix == true" }),
        expect.objectContaining({ id: "edge-3-else", source: "gate", target: "final", label: "else" }),
      ]),
    );
    expect(backEdge).toEqual(expect.objectContaining({
      type: "smoothstep",
      sourceHandle: "source-top",
      targetHandle: "target-top",
      data: expect.objectContaining({ routeKind: "back" }),
    }));
    expect(elseEdge).toEqual(expect.objectContaining({
      type: "smoothstep",
      sourceHandle: "source-bottom",
      targetHandle: "target-left",
      data: expect.objectContaining({ routeKind: "else" }),
    }));
  });
});

function nodesDoNotOverlap(nodes: ReturnType<typeof workflowToReactFlow>["nodes"]): boolean {
  for (let i = 0; i < nodes.length; i += 1) {
    for (let j = i + 1; j < nodes.length; j += 1) {
      const a = nodes[i]!;
      const b = nodes[j]!;
      const separateX = a.position.x + WORKFLOW_NODE_WIDTH <= b.position.x || b.position.x + WORKFLOW_NODE_WIDTH <= a.position.x;
      const separateY = a.position.y + WORKFLOW_NODE_HEIGHT <= b.position.y || b.position.y + WORKFLOW_NODE_HEIGHT <= a.position.y;
      if (!separateX && !separateY) return false;
    }
  }
  return true;
}
