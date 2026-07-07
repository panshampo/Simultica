import type { Edge, Node } from "@xyflow/react";
import type { WorkflowDefinition, WorkflowEdge, WorkflowNode, WorkflowNodeRunState } from "@multica/core/workflow/types";

const COLUMN_GAP = 360;
const ROW_GAP = 180;
export const WORKFLOW_NODE_WIDTH = 208;
export const WORKFLOW_NODE_HEIGHT = 104;
const MAX_CONDITION_LABEL_LENGTH = 44;

export type WorkflowCanvasNodeData = {
  id: string;
  label: string;
  type: string;
  dispatch: string;
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
type WorkflowEdgeKind = "forward" | "condition" | "else" | "back";

export type WorkflowCanvasEdgeData = {
  routeKind: WorkflowEdgeKind;
  fullLabel: string | null;
  selectedByRouteDecision: boolean;
  mutedByRouteDecision: boolean;
  completedPath: boolean;
  hovered?: boolean;
  mutedByHover?: boolean;
};

export function workflowToReactFlow(
  definition: WorkflowDefinition,
  runState: Record<string, WorkflowNodeRunState> = {},
  opts: { draggable?: boolean; selectableEdges?: boolean; selectedNodeId?: string | null; selectedEdgeId?: string | null } = {},
): { nodes: Node<WorkflowCanvasNodeData>[]; edges: Edge[] } {
  const layout = computeLayout(definition);
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
    sourcePoint && targetPoint && targetPoint.layer <= sourcePoint.layer ? "back" : kindHint;
  const handles = edgeHandles(routeKind, sourcePoint, targetPoint);
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
    type: "smoothstep",
    sourceHandle: route.sourceHandle || handles.sourceHandle,
    targetHandle: route.targetHandle || handles.targetHandle,
    animated: selectedByRouteDecision,
    selectable: opts.selectableEdges ?? false,
    interactionWidth: 28,
    data: {
      routeKind,
      fullLabel,
      selectedByRouteDecision,
      mutedByRouteDecision,
      completedPath,
    } satisfies WorkflowCanvasEdgeData,
    style: edgeStyle({
      routeKind,
      selected: opts.selectedEdgeId === id,
      selectedByRouteDecision,
      mutedByRouteDecision,
      completedPath,
    }),
    labelStyle: {
      fill: "#475569",
      fontSize: 11,
      fontWeight: 500,
    },
    labelBgStyle: {
      fill: "rgba(255,255,255,0.92)",
    },
    labelBgPadding: [6, 3],
    labelBgBorderRadius: 4,
  };
}

function edgeHandles(
  routeKind: WorkflowEdgeKind,
  sourcePoint: LayoutPoint | undefined,
  targetPoint: LayoutPoint | undefined,
): { sourceHandle: string; targetHandle: string } {
  if (routeKind === "back") {
    return { sourceHandle: "source-top", targetHandle: "target-top" };
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
  selected,
  selectedByRouteDecision,
  mutedByRouteDecision,
  completedPath,
}: {
  routeKind: WorkflowEdgeKind;
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
        const elseLayer = layers.get(route.else);
        if (elseLayer === undefined || elseLayer <= layer) continue;
        preferredRows.set(route.else, Math.max(preferredRows.get(route.else) ?? 0, row + 1));
      }
    }
  }

  return layout;
}
