import { describe, expect, it } from "vitest";
import type { WorkflowDefinition } from "@multica/core/workflow/types";
import { WORKFLOW_NODE_HEIGHT, WORKFLOW_NODE_WIDTH, type WorkflowCanvasEdgeData, workflowToReactFlow } from "./to-react-flow";

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

  it("prefers carrier_kind and maps legacy dispatch values to carrier labels", () => {
    const { nodes } = workflowToReactFlow({
      meta: { name: "carriers" },
      state: { fields: [] },
      nodes: [
        { id: "entry", type: "agent", dispatch: "subissue" },
        { id: "main", type: "main_agent", dispatch: "main_issue_task" },
        { id: "runtime", type: "agent", dispatch: "direct_subagent" },
        { id: "explicit", type: "agent", dispatch: "subissue", carrier_kind: "inline" },
      ],
      routing: [
        { from: "START", to: "entry" },
        { from: "entry", to: "main" },
        { from: "main", to: "runtime" },
        { from: "runtime", to: "explicit" },
      ],
    });

    const labels = new Map(nodes.map((node) => [node.id, node.data.carrierLabel]));
    expect(labels.get("entry")).toBe("agent · issue");
    expect(labels.get("main")).toBe("agent · issue task");
    expect(labels.get("runtime")).toBe("agent · runtime");
    expect(labels.get("explicit")).toBe("agent · inline");
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
    expect(nodesById.get("final")!.position.y).toBe(nodesById.get("gate")!.position.y);
    expect(nodesDoNotOverlap(nodes)).toBe(true);
    expect(edges).toHaveLength(4);
    expect(edges).toEqual(
      expect.arrayContaining([
        expect.objectContaining({ id: "edge-3-to", source: "gate", target: "author", label: "needs_fix == true" }),
        expect.objectContaining({ id: "edge-3-else", source: "gate", target: "final", label: "else" }),
      ]),
    );
    expect(backEdge).toEqual(expect.objectContaining({
      type: "workflow",
      sourceHandle: "source-top",
      targetHandle: "target-top",
      data: expect.objectContaining({ routeKind: "back", routePoints: expect.any(Array) }),
    }));
    expect(elseEdge).toEqual(expect.objectContaining({
      type: "workflow",
      sourceHandle: "source-right",
      targetHandle: "target-left",
      data: expect.objectContaining({ routeKind: "else", routePoints: expect.any(Array) }),
    }));
  });

  it("routes frontend bug investigation loop edges through separate lanes", () => {
    const { nodes, edges } = workflowToReactFlow(frontendBugInvestigationDefinition);
    const nodesById = new Map(nodes.map((node) => [node.id, node]));
    const loopGate = nodesById.get("loop_gate")!;
    const finalRouteGate = nodesById.get("final_route_gate")!;
    const investigate = nodesById.get("investigate_and_verify")!;
    const classify = nodesById.get("classify_and_record")!;
    const loopBackEdge = edges.find((edge) => edge.id === "edge-5-to")!;
    const classifyBackEdge = edges.find((edge) => edge.id === "edge-6-to")!;
    const finalEdge = edges.find((edge) => edge.id === "edge-6-else")!;

    expect(loopGate.position.y).toBe(nodesById.get("review_cleanup")!.position.y);
    expect(finalRouteGate.position.y).toBe(nodesById.get("review_cleanup")!.position.y);
    expect(nodesById.get("scope_and_baseline")!.position.y).toBe(loopGate.position.y);
    expect(nodesById.get("final_report")!.position.y).toBe(loopGate.position.y);
    expect(investigate.position.x).toBeLessThan(loopGate.position.x);
    expect(classify.position.x).toBeLessThan(finalRouteGate.position.x);
    expect(loopBackEdge.data).toEqual(expect.objectContaining({ routeKind: "back", lane: 1 }));
    expect(classifyBackEdge.data).toEqual(expect.objectContaining({ routeKind: "back", lane: 2 }));
    expect(loopBackEdge.style).toEqual(expect.objectContaining({ strokeDashoffset: 56 }));
    expect(classifyBackEdge.style).toEqual(expect.objectContaining({ strokeDashoffset: 112 }));
    expect(loopBackEdge.style).not.toEqual(classifyBackEdge.style);
    expect(finalEdge.data).toEqual(expect.objectContaining({ routeKind: "else", lane: 2 }));
  });

  it("plans edge routes that avoid node boxes without overlapping each other", () => {
    const { nodes, edges } = workflowToReactFlow(frontendBugInvestigationDefinition);
    const nodeBoxes = new Map(nodes.map((node) => [
      node.id,
      {
        left: node.position.x,
        right: node.position.x + WORKFLOW_NODE_WIDTH,
        top: node.position.y,
        bottom: node.position.y + WORKFLOW_NODE_HEIGHT,
      },
    ]));
    const occupiedSegments = new Set<string>();

    for (const edge of edges) {
      const data = edge.data as WorkflowCanvasEdgeData;
      expect(data.routePoints.length).toBeGreaterThanOrEqual(2);

      for (let i = 0; i < data.routePoints.length - 1; i += 1) {
        const from = data.routePoints[i]!;
        const to = data.routePoints[i + 1]!;
        expect(segmentIntersectsAnyNonEndpointNode(from, to, edge.source, edge.target, nodeBoxes)).toBe(false);

        const key = segmentKey(from, to);
        expect(occupiedSegments.has(key)).toBe(false);
        occupiedSegments.add(key);
      }
    }
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

function segmentIntersectsAnyNonEndpointNode(
  from: { x: number; y: number },
  to: { x: number; y: number },
  source: string,
  target: string,
  nodeBoxes: Map<string, { left: number; right: number; top: number; bottom: number }>,
): boolean {
  for (const [nodeId, box] of nodeBoxes) {
    if (nodeId === source || nodeId === target) continue;
    if (from.y === to.y) {
      const minX = Math.min(from.x, to.x);
      const maxX = Math.max(from.x, to.x);
      if (from.y > box.top && from.y < box.bottom && minX < box.right && maxX > box.left) return true;
    }
    if (from.x === to.x) {
      const minY = Math.min(from.y, to.y);
      const maxY = Math.max(from.y, to.y);
      if (from.x > box.left && from.x < box.right && minY < box.bottom && maxY > box.top) return true;
    }
  }
  return false;
}

function segmentKey(from: { x: number; y: number }, to: { x: number; y: number }): string {
  const a = `${from.x},${from.y}`;
  const b = `${to.x},${to.y}`;
  return a < b ? `${a}->${b}` : `${b}->${a}`;
}

const frontendBugInvestigationDefinition: WorkflowDefinition = {
  meta: { name: "Frontend Bug Investigation Workflow" },
  state: { fields: [] },
  nodes: [
    { id: "scope_and_baseline", type: "agent", dispatch: "subissue" },
    { id: "investigate_and_verify", type: "agent", dispatch: "subissue" },
    { id: "classify_and_record", type: "agent", dispatch: "subissue" },
    { id: "review_cleanup", type: "agent", dispatch: "subissue" },
    { id: "loop_gate", type: "transform", dispatch: "inline" },
    { id: "final_route_gate", type: "transform", dispatch: "inline" },
    { id: "final_report", type: "final_response", dispatch: "main_issue_task" },
  ],
  routing: [
    { from: "START", to: "scope_and_baseline" },
    { from: "scope_and_baseline", to: "investigate_and_verify" },
    { from: "investigate_and_verify", to: "classify_and_record" },
    { from: "classify_and_record", to: "review_cleanup" },
    { from: "review_cleanup", to: "loop_gate" },
    {
      from: "loop_gate",
      condition: 'workflow_status == "fixable_auto" && next_target == "investigate_and_verify" && revisionCount < 2',
      to: "investigate_and_verify",
      else: "final_route_gate",
    },
    {
      from: "final_route_gate",
      condition: 'workflow_status == "fixable_auto" && next_target == "classify_and_record" && revisionCount < 2',
      to: "classify_and_record",
      else: "final_report",
    },
    { from: "final_report", to: "END" },
  ],
};
