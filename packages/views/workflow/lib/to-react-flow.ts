import type { Edge, Node } from "@xyflow/react";
import type { WorkflowDefinition, WorkflowNodeRunState } from "@multica/core/workflow/types";

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
};

export function workflowToReactFlow(
  definition: WorkflowDefinition,
  runState: Record<string, WorkflowNodeRunState> = {},
  opts: { draggable?: boolean; selectableEdges?: boolean; selectedNodeId?: string | null; selectedEdgeId?: string | null } = {},
): { nodes: Node<WorkflowCanvasNodeData>[]; edges: Edge[] } {
  const layers = computeLayers(definition);
  const nodeById = new Map(definition.nodes.map((node) => [node.id, node]));
  const nodes: Node<WorkflowCanvasNodeData>[] = [];

  for (const [nodeId, layer] of layers.entries()) {
    const node = nodeById.get(nodeId);
    if (!node) continue;
    const siblings = [...layers.entries()]
      .filter(([, value]) => value === layer)
      .map(([id]) => id)
      .sort();
    const row = siblings.indexOf(nodeId);
    const state = runState[nodeId];
    const dispatch = node.dispatch ?? inferDispatch(node.type);
    nodes.push({
      id: nodeId,
      type: "workflow",
      position: { x: layer * 260, y: row * 120 },
      draggable: opts.draggable ?? false,
      selectable: true,
      data: {
        id: nodeId,
        label: nodeId,
        type: node.type,
        dispatch,
        carrierLabel: carrierLabel(node.type, dispatch),
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
    .filter((edge) => edge.from !== "START" && edge.to !== "END")
    .map((edge, index) => ({
      id: `edge-${index}`,
      source: edge.from,
      target: edge.to,
      label: edge.condition,
      animated: false,
      selectable: opts.selectableEdges ?? false,
      style: opts.selectedEdgeId === `edge-${index}` ? { strokeWidth: 2.5 } : undefined,
    }));

  return { nodes, edges };
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

function carrierLabel(type: string, dispatch: string): string {
  switch (dispatch) {
    case "subissue":
      return "agent · subissue";
    case "direct_subagent":
      return "agent · direct";
    case "main_issue_task":
      return "main agent · main issue";
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
