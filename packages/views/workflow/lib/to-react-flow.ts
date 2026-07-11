import type { Edge, Node } from "@xyflow/react";
import type { WorkflowDefinition, WorkflowEdge, WorkflowNode, WorkflowNodeRunState } from "@multica/core/workflow/types";

const COLUMN_GAP = 360;
const ROW_GAP = 180;
const EDGE_LANE_GAP = 56;
const EDGE_NODE_PADDING = 32;
export const WORKFLOW_NODE_WIDTH = 208;
export const WORKFLOW_NODE_HEIGHT = 104;
const MAX_CONDITION_LABEL_LENGTH = 44;

export type WorkflowCanvasNodeData = {
  id: string;
  label: string;
  type: string;
  dispatch: string;
  agentRoute: string | null;
  inputs: string[];
  outputs: string[];
  systemPrompt: string | null;
  carrierLabel: string;
  status: WorkflowNodeRunState["status"] | "pending";
  subIssueId: string | null;
  traexSessionId: string | null;
  traexLogUrl: string | null;
  mainIssueTaskId: string | null;
  error: string | null;
  selected?: boolean;
  hoveredRelated?: boolean;
  mutedByHover?: boolean;
};

type LayoutPoint = { layer: number; row: number };
type CanvasPoint = { x: number; y: number };
type WorkflowEdgeKind = "forward" | "condition" | "else" | "back";
type EdgeRouteSide = "top" | "bottom";
type EdgePlan = { routeKind: WorkflowEdgeKind; lane: number };

export type WorkflowCanvasEdgeData = {
  routeKind: WorkflowEdgeKind;
  lane: number;
  fullLabel: string | null;
  selectedByRouteDecision: boolean;
  mutedByRouteDecision: boolean;
  completedPath: boolean;
  routePoints: CanvasPoint[];
  routeSide?: EdgeRouteSide;
  hovered?: boolean;
  mutedByHover?: boolean;
};

export function workflowToReactFlow(
  definition: WorkflowDefinition,
  runState: Record<string, WorkflowNodeRunState> = {},
  opts: { draggable?: boolean; selectableEdges?: boolean; selectedNodeId?: string | null; selectedEdgeId?: string | null } = {},
): { nodes: Node<WorkflowCanvasNodeData>[]; edges: Edge[] } {
  const layout = computeLayout(definition);
  const edgePlans = computeEdgePlans(definition.routing, layout);
  const nodeById = new Map(definition.nodes.map((node) => [node.id, node]));
  const nodes: Node<WorkflowCanvasNodeData>[] = [];

  for (const [nodeId, point] of layout.entries()) {
    const node = nodeById.get(nodeId);
    if (!node) continue;
    const state = runState[nodeId];
    const dispatch = node.dispatch ?? inferDispatch(node.type);
    const carrierKind = carrierKindFromNode(node);
    nodes.push({
      id: nodeId,
      type: "workflow",
      position: { x: point.layer * COLUMN_GAP, y: point.row * ROW_GAP },
      draggable: opts.draggable ?? true,
      selectable: true,
      data: {
        id: nodeId,
        label: nodeId,
        type: node.type,
        dispatch,
        agentRoute: node.config?.agent ?? node.agent ?? null,
        inputs: node.inputs ?? [],
        outputs: node.outputs ?? [],
        systemPrompt: node.config?.system ?? null,
        carrierLabel: carrierLabel(node.type, carrierKind),
        status: state?.status ?? "pending",
        subIssueId: state?.sub_issue_id ?? null,
        traexSessionId: state?.traex_session_id ?? null,
        traexLogUrl: state?.traex_log_url ?? null,
        mainIssueTaskId: state?.main_issue_task_id ?? null,
        error: state?.error ?? null,
        selected: opts.selectedNodeId === nodeId,
      },
    });
  }

  const edges: Edge[] = definition.routing
    .flatMap((edge, index) => {
      const result: Edge[] = [];
      if (edge.from !== "START" && edge.to !== "END") {
	      const id = `edge-${index}-to`;
	      result.push(buildEdge({
	        id,
	        plan: edgePlans.get(id),
	        route: edge,
	        source: edge.from,
	        target: edge.to,
	        label: edge.condition,
	        layout,
	        kindHint: edge.condition ? "condition" : "forward",
	        runState,
	        opts,
	      }));
	    }
	    if (edge.else && edge.from !== "START" && edge.else !== "END") {
	      const id = `edge-${index}-else`;
	      result.push(buildEdge({
	        id,
	        plan: edgePlans.get(id),
	        route: edge,
	        source: edge.from,
	        target: edge.else,
	        label: "else",
	        layout,
	        kindHint: "else",
	        runState,
	        opts,
	      }));
	    }
      return result;
    });

  return { nodes, edges };
}

function buildEdge({
  id,
  plan,
  route,
  source,
  target,
  label,
  layout,
  kindHint,
  runState,
  opts,
}: {
  id: string;
  plan?: EdgePlan;
  route: WorkflowEdge;
  source: string;
  target: string;
  label?: string;
  layout: Map<string, LayoutPoint>;
  kindHint: Exclude<WorkflowEdgeKind, "back">;
  runState: Record<string, WorkflowNodeRunState>;
  opts: { selectableEdges?: boolean; selectedEdgeId?: string | null };
}): Edge {
  const sourcePoint = layout.get(source);
  const targetPoint = layout.get(target);
  const routeKind: WorkflowEdgeKind =
    plan?.routeKind ?? (sourcePoint && targetPoint && targetPoint.layer <= sourcePoint.layer ? "back" : kindHint);
  const lane = plan?.lane ?? 0;
  const routeSide = routeKind === "back" ? backRouteSide(lane, sourcePoint) : undefined;
  const handles = edgeHandles(routeKind, sourcePoint, targetPoint, lane);
  const routePoints = buildRoutePoints({ routeKind, lane, routeSide, sourcePoint, targetPoint });
  const routeDecision = runState[source]?.route_decision;
  const selectedByRouteDecision = Boolean(routeDecision && routeDecision.selected_route === target);
  const mutedByRouteDecision = Boolean(routeDecision && !selectedByRouteDecision);
  const completedPath = runState[source]?.status === "done" && ["done", "running"].includes(runState[target]?.status ?? "");
  const fullLabel = label ?? null;
  return {
    id,
    source,
    target,
    label: displayEdgeLabel(label),
    type: "workflow",
    sourceHandle: route.sourceHandle || handles.sourceHandle,
    targetHandle: route.targetHandle || handles.targetHandle,
    animated: selectedByRouteDecision,
    selectable: opts.selectableEdges ?? false,
    interactionWidth: 28,
    data: {
      routeKind,
      lane,
      fullLabel,
      selectedByRouteDecision,
      mutedByRouteDecision,
      completedPath,
      routePoints,
      ...(routeSide ? { routeSide } : {}),
    } satisfies WorkflowCanvasEdgeData,
    style: edgeStyle({
      routeKind,
      lane,
      selected: opts.selectedEdgeId === id,
      selectedByRouteDecision,
      mutedByRouteDecision,
      completedPath,
    }),
    labelStyle: {
      fill: "#475569",
      fontSize: 11,
      fontWeight: 500,
      transform: lane ? `translateY(${lane * 8}px)` : undefined,
    },
    labelBgStyle: {
      fill: "rgba(255,255,255,0.92)",
    },
    labelBgPadding: [6, 3],
    labelBgBorderRadius: 4,
  };
}

function buildRoutePoints({
  routeKind,
  lane,
  routeSide,
  sourcePoint,
  targetPoint,
}: {
  routeKind: WorkflowEdgeKind;
  lane: number;
  routeSide?: EdgeRouteSide;
  sourcePoint: LayoutPoint | undefined;
  targetPoint: LayoutPoint | undefined;
}): CanvasPoint[] {
  if (!sourcePoint || !targetPoint) return [];
  const sourceBox = nodeBox(sourcePoint);
  const targetBox = nodeBox(targetPoint);

  if (routeKind === "back") {
    const useBottom = routeSide === "bottom";
    const sourceAnchor = useBottom
      ? { x: sourceBox.centerX, y: sourceBox.bottom }
      : { x: sourceBox.centerX, y: sourceBox.top };
    const targetAnchor = useBottom
      ? { x: targetBox.centerX, y: targetBox.bottom }
      : { x: targetBox.centerX, y: targetBox.top };
    const laneY = useBottom
      ? Math.max(sourceBox.bottom, targetBox.bottom) + EDGE_NODE_PADDING + lane * EDGE_LANE_GAP
      : Math.min(sourceBox.top, targetBox.top) - EDGE_NODE_PADDING - lane * EDGE_LANE_GAP;
    return [
      sourceAnchor,
      { x: sourceAnchor.x, y: laneY },
      { x: targetAnchor.x, y: laneY },
      targetAnchor,
    ];
  }

  const sourceAnchor = sourcePoint.row < targetPoint.row
    ? { x: sourceBox.centerX, y: sourceBox.bottom }
    : sourcePoint.row > targetPoint.row
      ? { x: sourceBox.centerX, y: sourceBox.top }
      : { x: sourceBox.right, y: sourceBox.centerY };
  const targetAnchor = { x: targetBox.left, y: targetBox.centerY };
  const corridorX = edgeCorridorX(sourceBox.right, targetBox.left, lane);
  return [
    sourceAnchor,
    { x: corridorX, y: sourceAnchor.y },
    { x: corridorX, y: targetAnchor.y },
    targetAnchor,
  ];
}

function backRouteSide(lane: number, sourcePoint: LayoutPoint | undefined): EdgeRouteSide {
  if ((sourcePoint?.row ?? 0) > 0) return "bottom";
  return lane % 2 === 0 ? "bottom" : "top";
}

function edgeCorridorX(sourceBoundaryX: number, targetBoundaryX: number, lane: number): number {
  const minX = Math.min(sourceBoundaryX, targetBoundaryX) + EDGE_NODE_PADDING;
  const maxX = Math.max(sourceBoundaryX, targetBoundaryX) - EDGE_NODE_PADDING;
  if (maxX <= minX) return (sourceBoundaryX + targetBoundaryX) / 2;

  const midX = (minX + maxX) / 2;
  const maxOffset = Math.max(0, (maxX - minX) / 2);
  const step = Math.min(EDGE_LANE_GAP / 2, Math.max(8, maxOffset / 2));
  const offset = centeredLaneOffset(lane, step, maxOffset);
  return midX + offset;
}

function centeredLaneOffset(lane: number, step: number, maxOffset: number): number {
  if (!lane) return 0;
  const magnitude = Math.ceil(lane / 2) * step;
  const direction = lane % 2 === 1 ? 1 : -1;
  return direction * Math.min(magnitude, maxOffset);
}

function nodeBox(point: LayoutPoint) {
  const left = point.layer * COLUMN_GAP;
  const top = point.row * ROW_GAP;
  return {
    left,
    right: left + WORKFLOW_NODE_WIDTH,
    top,
    bottom: top + WORKFLOW_NODE_HEIGHT,
    centerX: left + WORKFLOW_NODE_WIDTH / 2,
    centerY: top + WORKFLOW_NODE_HEIGHT / 2,
  };
}

function edgeHandles(
  routeKind: WorkflowEdgeKind,
  sourcePoint: LayoutPoint | undefined,
  targetPoint: LayoutPoint | undefined,
  lane = 0,
): { sourceHandle: string; targetHandle: string } {
  if (routeKind === "back") {
    return backRouteSide(lane, sourcePoint) === "bottom"
      ? { sourceHandle: "source-bottom", targetHandle: "target-bottom" }
      : { sourceHandle: "source-top", targetHandle: "target-top" };
  }
  if (sourcePoint && targetPoint && targetPoint.row > sourcePoint.row) {
    return { sourceHandle: "source-bottom", targetHandle: "target-left" };
  }
  if (sourcePoint && targetPoint && targetPoint.row < sourcePoint.row) {
    return { sourceHandle: "source-top", targetHandle: "target-left" };
  }
  return { sourceHandle: "source-right", targetHandle: "target-left" };
}

function displayEdgeLabel(label?: string): string | undefined {
  if (!label) return undefined;
  if (label === "else") return label;
  if (label.length <= MAX_CONDITION_LABEL_LENGTH) return label;

  const operatorIndex = label.search(/\s(?:==|!=|>=|<=|>|<|&&|\|\|)\s/);
  const shortened = operatorIndex > 0 ? label.slice(0, operatorIndex).trim() : label.slice(0, MAX_CONDITION_LABEL_LENGTH - 3).trim();
  return `${shortened}...`;
}

function edgeStyle({
  routeKind,
  lane,
  selected,
  selectedByRouteDecision,
  mutedByRouteDecision,
  completedPath,
}: {
  routeKind: WorkflowEdgeKind;
  lane: number;
  selected: boolean;
  selectedByRouteDecision: boolean;
  mutedByRouteDecision: boolean;
  completedPath: boolean;
}): Edge["style"] {
  const stroke = selectedByRouteDecision
    ? "color-mix(in srgb, var(--primary) 86%, #2563eb)"
    : completedPath
      ? "#16a34a"
      : routeKind === "back"
        ? "#a16207"
        : routeKind === "else"
          ? "#94a3b8"
          : "#64748b";
  return {
    stroke,
    strokeLinecap: "round",
    strokeLinejoin: "round",
    strokeWidth: selected || selectedByRouteDecision ? 2.75 : completedPath ? 2 : 1.5,
    opacity: mutedByRouteDecision ? 0.28 : routeKind === "else" ? 0.78 : 1,
    strokeDashoffset: lane ? lane * EDGE_LANE_GAP : undefined,
    ...(routeKind === "else" ? { strokeDasharray: "3 5" } : {}),
    ...(routeKind === "back" ? { strokeDasharray: "8 5" } : {}),
  };
}

function inferDispatch(type: string): string {
  switch (type) {
    case "agent":
    case "subissue":
    case "llm":
      return "subissue";
    case "main_agent":
    case "final_response":
      return "main_issue_task";
    default:
      return "inline";
  }
}

function carrierKindFromNode(node: Pick<WorkflowNode, "carrier_kind" | "dispatch" | "type">): string {
  if (node.carrier_kind) return node.carrier_kind;
  switch (node.dispatch) {
    case "subissue":
      return "issue";
    case "main_issue_task":
      return "issue_task";
    case "direct_subagent":
      return "agent_runtime";
    case "inline":
      return "inline";
    default:
      return inferDispatch(node.type) === "subissue" ? "issue" : "inline";
  }
}

function carrierLabel(type: string, carrierKind: string): string {
  switch (carrierKind) {
    case "issue":
      return "agent · issue";
    case "issue_task":
      return "agent · issue task";
    case "agent_runtime":
      return "agent · runtime";
    case "inline":
    default:
      return `${type} · inline`;
  }
}

function computeEdgePlans(routing: WorkflowEdge[], layout: Map<string, LayoutPoint>): Map<string, EdgePlan> {
  const plans = new Map<string, EdgePlan>();
  let backLane = 0;
  let branchLane = 0;

  routing.forEach((route, index) => {
    if (route.from !== "START" && route.to !== "END") {
      const id = `edge-${index}-to`;
      const routeKind = inferRouteKind(route.from, route.to, route.condition ? "condition" : "forward", layout);
      const lane = routeKind === "back" ? ++backLane : routeKind === "condition" ? ++branchLane : 0;
      plans.set(id, { routeKind, lane });
    }
    if (route.else && route.from !== "START" && route.else !== "END") {
      const id = `edge-${index}-else`;
      const routeKind = inferRouteKind(route.from, route.else, "else", layout);
      const lane = routeKind === "back" ? ++backLane : ++branchLane;
      plans.set(id, { routeKind, lane });
    }
  });

  return plans;
}

function inferRouteKind(
  source: string,
  target: string,
  fallback: Exclude<WorkflowEdgeKind, "back">,
  layout: Map<string, LayoutPoint>,
): WorkflowEdgeKind {
  const sourcePoint = layout.get(source);
  const targetPoint = layout.get(target);
  if (sourcePoint && targetPoint && targetPoint.layer <= sourcePoint.layer) return "back";
  return fallback;
}

function computeLayers(definition: WorkflowDefinition): Map<string, number> {
  const nodeIds = new Set(definition.nodes.map((node) => node.id));
  const outgoing = new Map<string, string[]>();
  for (const route of definition.routing) {
    if (!route.to || route.to === "END") continue;
    const from = route.from;
    if (!outgoing.has(from)) outgoing.set(from, []);
    outgoing.get(from)!.push(route.to);
    if (route.else && route.else !== "END") outgoing.get(from)!.push(route.else);
  }

  const layers = new Map<string, number>();
  const queue: Array<{ id: string; layer: number }> = [];
  const enqueued = new Set<string>();
  for (const first of outgoing.get("START") ?? []) {
    queue.push({ id: first, layer: 0 });
    enqueued.add(first);
  }

  while (queue.length > 0) {
    const { id, layer } = queue.shift()!;
    if (!nodeIds.has(id)) continue;
    if (layers.has(id)) continue;
    layers.set(id, layer);
    for (const next of outgoing.get(id) ?? []) {
      if (enqueued.has(next)) continue;
      queue.push({ id: next, layer: layer + 1 });
      enqueued.add(next);
    }
  }

  const fallbackLayer = Math.max(0, ...layers.values()) + 1;
  for (const nodeId of nodeIds) {
    if (!layers.has(nodeId)) layers.set(nodeId, fallbackLayer);
  }
  return layers;
}

function computeLayout(definition: WorkflowDefinition): Map<string, LayoutPoint> {
  const layers = computeLayers(definition);
  const nodeIds = new Set(definition.nodes.map((node) => node.id));
  const idsByLayer = new Map<number, string[]>();
  for (const [id, layer] of layers.entries()) {
    if (!idsByLayer.has(layer)) idsByLayer.set(layer, []);
    idsByLayer.get(layer)!.push(id);
  }

  const preferredRows = new Map<string, number>();
  const layout = new Map<string, LayoutPoint>();
  const sortedLayers = [...idsByLayer.keys()].sort((a, b) => a - b);

  for (const layer of sortedLayers) {
    const occupiedRows = new Set<number>();
    const ids = idsByLayer.get(layer)!.sort((a, b) => {
      const rowDelta = (preferredRows.get(a) ?? 0) - (preferredRows.get(b) ?? 0);
      return rowDelta || a.localeCompare(b);
    });

    for (const id of ids) {
      let row = preferredRows.get(id) ?? 0;
      while (occupiedRows.has(row)) row += 1;
      occupiedRows.add(row);
      layout.set(id, { layer, row });

      for (const route of definition.routing) {
        if (route.from !== id || !route.else || route.else === "END" || !nodeIds.has(route.else)) continue;
        const toLayer = layers.get(route.to);
        const isLoopGate = toLayer !== undefined && toLayer <= layer;
        if (isLoopGate) continue;
        const elseLayer = layers.get(route.else);
        if (elseLayer === undefined || elseLayer <= layer) continue;
        preferredRows.set(route.else, Math.max(preferredRows.get(route.else) ?? 0, row + 1));
      }
    }
  }

  return layout;
}
