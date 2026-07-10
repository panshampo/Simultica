"use client";

import { memo, useCallback, useEffect, useMemo, useState, type CSSProperties, type MouseEvent, type PointerEvent, type ReactNode } from "react";
import { createPortal } from "react-dom";
import {
  BaseEdge,
  Background,
  Controls,
  EdgeText,
  Handle,
  MarkerType,
  Position,
  ReactFlow,
  ReactFlowProvider,
  useReactFlow,
  type Connection,
  type Edge,
  type EdgeProps,
  type EdgeTypes,
  type Node,
  type NodeProps,
  type NodeTypes,
} from "@xyflow/react";
import { Maximize2, X } from "lucide-react";
import "@xyflow/react/dist/style.css";
import type { WorkflowDefinition, WorkflowNodeRunState, WorkflowRunStatus } from "@multica/core/workflow/types";
import { Button } from "@multica/ui/components/ui/button";
import { cn } from "@multica/ui/lib/utils";
import { WORKFLOW_NODE_HEIGHT, WORKFLOW_NODE_WIDTH, workflowToReactFlow, type WorkflowCanvasEdgeData, type WorkflowCanvasNodeData } from "../lib/to-react-flow";

export function WorkflowCanvas({
  definition,
  runState,
  onOpenSubIssue,
  editable = false,
  selectedNodeId,
  selectedEdgeId,
  onSelectNode,
  onSelectEdge,
  onPaneClick,
  onConnect,
  onAddNextNode,
  className,
  runStatus,
  fullscreenTitle,
  hideSelectionOverlay = false,
  fullscreenExtra,
}: {
  definition: WorkflowDefinition;
  runState?: Record<string, WorkflowNodeRunState>;
  runStatus?: WorkflowRunStatus;
  onOpenSubIssue?: (issueId: string) => void;
  editable?: boolean;
  selectedNodeId?: string | null;
  selectedEdgeId?: string | null;
  onSelectNode?: (nodeId: string) => void;
  onSelectEdge?: (edgeId: string) => void;
  onPaneClick?: () => void;
  onConnect?: (connection: Connection) => void;
  onAddNextNode?: (nodeId: string) => void;
  className?: string;
  fullscreenTitle?: string;
  hideSelectionOverlay?: boolean;
  fullscreenExtra?: ReactNode;
}) {
  const [fullscreen, setFullscreen] = useState(false);
  const [hoveredEdgeId, setHoveredEdgeId] = useState<string | null>(null);
  const [hoveredNodeId, setHoveredNodeId] = useState<string | null>(null);
  const [localSelectedNodeId, setLocalSelectedNodeId] = useState<string | null>(null);
  const [localSelectedEdgeId, setLocalSelectedEdgeId] = useState<string | null>(null);
  const activeSelectedNodeId = selectedNodeId ?? localSelectedNodeId;
  const activeSelectedEdgeId = selectedEdgeId ?? localSelectedEdgeId;
  const { nodes: baseNodes, edges: baseEdges } = useMemo(
    () => workflowToReactFlow(definition, runState ?? {}, { draggable: true, selectableEdges: true }),
    [definition, runState],
  );
  const selectedNode = useMemo(() => baseNodes.find((node) => node.id === activeSelectedNodeId), [activeSelectedNodeId, baseNodes]);
  const selectedEdge = useMemo(() => baseEdges.find((edge) => edge.id === activeSelectedEdgeId), [activeSelectedEdgeId, baseEdges]);
  const hoveredEdge = useMemo(() => baseEdges.find((e) => e.id === hoveredEdgeId), [baseEdges, hoveredEdgeId]);
  const hoveredNodeIds = useMemo(() => {
    if (!hoveredEdge) return EMPTY_NODE_ID_SET;
    return new Set([hoveredEdge.source, hoveredEdge.target]);
  }, [hoveredEdge]);
  const hoveredEdgeIds = useMemo(() => {
    if (!hoveredNodeId) return EMPTY_EDGE_ID_SET;
    return new Set(baseEdges.filter((edge) => edge.source === hoveredNodeId || edge.target === hoveredNodeId).map((edge) => edge.id));
  }, [baseEdges, hoveredNodeId]);
  const clearSelection = useCallback(() => {
    setLocalSelectedNodeId(null);
    setLocalSelectedEdgeId(null);
    onPaneClick?.();
  }, [onPaneClick]);

  const nodeBasePatch = useMemo(() => {
    return {
      onOpenSubIssue,
      onSelectNode: (id: string) => {
        setLocalSelectedNodeId(id);
        setLocalSelectedEdgeId(null);
        onSelectNode?.(id);
      },
      onAddNextNode,
      editable,
      runStatus,
    };
  }, [editable, onAddNextNode, onOpenSubIssue, onSelectNode, runStatus]);

  const selectionPatch = useMemo(() => ({
    selectedNodeId: activeSelectedNodeId,
    selectedEdgeId: activeSelectedEdgeId,
  }), [activeSelectedEdgeId, activeSelectedNodeId]);

  const hoverPatch = useMemo(() => ({
    hoveredEdgeId,
    hoveredNodeId,
    hoveredNodeIds,
    hoveredEdgeIds,
  }), [hoveredEdgeId, hoveredEdgeIds, hoveredNodeId, hoveredNodeIds]);

  useEffect(() => {
    if (!fullscreen) return;
    function handleKeyDown(event: KeyboardEvent) {
      if (event.key !== "Escape") return;
      if (activeSelectedNodeId || activeSelectedEdgeId) {
        clearSelection();
        event.preventDefault();
        event.stopPropagation();
      } else {
        setFullscreen(false);
      }
    }
    window.addEventListener("keydown", handleKeyDown);
    return () => window.removeEventListener("keydown", handleKeyDown);
  }, [fullscreen, activeSelectedNodeId, activeSelectedEdgeId, clearSelection]);

  function closeFullscreen(event?: MouseEvent | PointerEvent) {
    event?.preventDefault();
    event?.stopPropagation();
    setFullscreen(false);
  }

  const canvas = (extraClassName?: string, key?: string) => (
    <WorkflowCanvasInner
      key={key}
      baseNodes={baseNodes}
      baseEdges={baseEdges}
      nodeBasePatch={nodeBasePatch}
      selectionPatch={selectionPatch}
      hoverPatch={hoverPatch}
      structureEditable={editable}
      onConnect={onConnect}
      onSelectNode={(id) => {
        setLocalSelectedNodeId(id);
        setLocalSelectedEdgeId(null);
        onSelectNode?.(id);
      }}
      onSelectEdge={(id) => {
        setLocalSelectedEdgeId(id);
        setLocalSelectedNodeId(null);
        onSelectEdge?.(id);
      }}
      onHoverEdge={(id) => {
        setHoveredEdgeId(id);
        if (id) setHoveredNodeId(null);
      }}
      onHoverNode={(id) => {
        setHoveredNodeId(id);
        if (id) setHoveredEdgeId(null);
      }}
      onPaneClick={clearSelection}
      extraClassName={extraClassName}
    />
  );

  if (baseNodes.length === 0) {
    return <div className="rounded-md border bg-muted/20 p-4 text-sm text-muted-foreground">No workflow nodes.</div>;
  }

  return (
    <>
      <div
        className={cn(
          "relative h-[360px] min-h-[320px] overflow-hidden rounded-lg border border-border/80 bg-background shadow-[0_1px_1px_rgba(15,23,42,0.04),0_14px_36px_-28px_rgba(15,23,42,0.45)]",
          className,
        )}
      >
        <div
          aria-hidden="true"
          className="pointer-events-none absolute inset-0 z-0 opacity-80"
          style={{
            backgroundImage:
              "linear-gradient(180deg, color-mix(in srgb, var(--muted) 42%, transparent), color-mix(in srgb, var(--background) 10%, transparent) 38%, color-mix(in srgb, var(--muted) 22%, transparent)), radial-gradient(circle at 18px 18px, color-mix(in srgb, var(--muted-foreground) 16%, transparent) 1px, transparent 1px)",
            backgroundSize: "100% 100%, 18px 18px",
          }}
        />
        <div aria-hidden="true" className="pointer-events-none absolute inset-x-0 top-0 z-10 h-px bg-foreground/10" />
        {fullscreenTitle && (
          <div className="absolute right-2 top-2 z-20">
            <Button
              type="button"
              size="icon-xs"
              variant="secondary"
              aria-label="Expand workflow canvas"
              title="Expand workflow canvas"
              onClick={() => setFullscreen(true)}
            >
              <Maximize2 className="size-3.5" aria-hidden="true" />
            </Button>
          </div>
        )}
        {!fullscreen && canvas(undefined, "inline")}
        {!hideSelectionOverlay && (selectedNode || selectedEdge) && (
          <WorkflowSelectionOverlay
            node={selectedNode}
            edge={selectedEdge}
            onClose={clearSelection}
            onOpenSubIssue={onOpenSubIssue}
          />
        )}
      </div>
      {fullscreen && fullscreenTitle && createPortal(
        <div
          role="dialog"
          aria-label={fullscreenTitle}
          className="fixed inset-0 z-[9999] bg-background"
        >
          <div
            className="absolute right-3 top-16 z-30"
            style={{ WebkitAppRegion: "no-drag" } as CSSProperties}
          >
            <Button
              type="button"
              size="icon-sm"
              variant="outline"
              aria-label="Close fullscreen workflow canvas"
              title="Close fullscreen workflow canvas"
              onPointerDown={closeFullscreen}
              onClick={closeFullscreen}
            >
              <X className="size-4" aria-hidden="true" />
            </Button>
          </div>
          <div
            className="relative z-0 h-full overflow-hidden"
            style={{
              backgroundImage:
                "linear-gradient(180deg, color-mix(in srgb, var(--muted) 34%, transparent), var(--background) 48%, color-mix(in srgb, var(--muted) 18%, transparent)), radial-gradient(circle at 18px 18px, color-mix(in srgb, var(--muted-foreground) 14%, transparent) 1px, transparent 1px)",
              backgroundSize: "100% 100%, 18px 18px",
            }}
          >
            {canvas("h-full", "fullscreen")}
          </div>
          {!hideSelectionOverlay && (selectedNode || selectedEdge) && (
            <WorkflowSelectionOverlay
              node={selectedNode}
              edge={selectedEdge}
              onClose={clearSelection}
              onOpenSubIssue={onOpenSubIssue}
            />
          )}
          {fullscreenExtra}
        </div>,
        document.body,
      )}
    </>
  );
}

const EMPTY_NODE_ID_SET = new Set<string>();
const EMPTY_EDGE_ID_SET = new Set<string>();
const EMPTY_HOVER_PATCH: HoverPatch = {
  hoveredEdgeId: null,
  hoveredNodeId: null,
  hoveredNodeIds: EMPTY_NODE_ID_SET,
  hoveredEdgeIds: EMPTY_EDGE_ID_SET,
};

type NodeBasePatch = {
  onOpenSubIssue?: (issueId: string) => void;
  onSelectNode?: (nodeId: string) => void;
  onAddNextNode?: (nodeId: string) => void;
  editable?: boolean;
  runStatus?: WorkflowRunStatus;
};

type SelectionPatch = {
  selectedNodeId: string | null | undefined;
  selectedEdgeId: string | null | undefined;
};

type HoverPatch = {
  hoveredEdgeId: string | null;
  hoveredNodeId: string | null;
  hoveredNodeIds: Set<string>;
  hoveredEdgeIds: Set<string>;
};

function WorkflowCanvasInner(props: {
  baseNodes: Node<WorkflowCanvasNodeData>[];
  baseEdges: Edge[];
  nodeBasePatch: NodeBasePatch;
  selectionPatch: SelectionPatch;
  hoverPatch: HoverPatch;
  structureEditable: boolean;
  onConnect?: (connection: Connection) => void;
  onSelectNode: (id: string) => void;
  onSelectEdge: (id: string) => void;
  onHoverEdge: (id: string | null) => void;
  onHoverNode: (id: string | null) => void;
  onPaneClick: () => void;
  extraClassName?: string;
}) {
  return (
    <ReactFlowProvider>
      <WorkflowCanvasInnerInner {...props} />
    </ReactFlowProvider>
  );
}

function WorkflowCanvasInnerInner({
  baseNodes,
  baseEdges,
  nodeBasePatch,
  selectionPatch,
  hoverPatch,
  structureEditable,
  onConnect,
  onSelectNode,
  onSelectEdge,
  onHoverEdge,
  onHoverNode,
  onPaneClick,
  extraClassName,
}: {
  baseNodes: Node<WorkflowCanvasNodeData>[];
  baseEdges: Edge[];
  nodeBasePatch: NodeBasePatch;
  selectionPatch: SelectionPatch;
  hoverPatch: HoverPatch;
  structureEditable: boolean;
  onConnect?: (connection: Connection) => void;
  onSelectNode: (id: string) => void;
  onSelectEdge: (id: string) => void;
  onHoverEdge: (id: string | null) => void;
  onHoverNode: (id: string | null) => void;
  onPaneClick: () => void;
  extraClassName?: string;
}) {
  const { setNodes, setEdges } = useReactFlow();
  const nodeIdKey = baseNodes.map((n) => n.id).sort().join("|");
  const edgeIdKey = baseEdges.map((e) => e.id).sort().join("|");

  const defaultNodes = useMemo(
    () => buildNodesWithPatch(baseNodes, nodeBasePatch, selectionPatch, hoverPatch),
    // eslint-disable-next-line react-hooks/exhaustive-deps
    [],
  );
  const defaultEdges = useMemo(
    () => buildEdgesWithPatch(baseEdges, hoverPatch),
    // eslint-disable-next-line react-hooks/exhaustive-deps
    [],
  );

  useEffect(() => {
    setNodes((existing) => {
      const existingById = new Map(existing.map((n) => [n.id, n]));
      return baseNodes.map((baseNode) => {
        const prev = existingById.get(baseNode.id);
        const position = prev?.position ?? baseNode.position;
        return {
          ...baseNode,
          position,
          data: {
            ...baseNode.data,
            ...getNodeBaseDataOverrides(nodeBasePatch),
          },
        } as Node<WorkflowCanvasNodeData>;
      });
    });
  }, [baseNodes, nodeBasePatch, nodeIdKey, setNodes]);

  useEffect(() => {
    setEdges(buildEdgesWithPatch(baseEdges, EMPTY_HOVER_PATCH));
  }, [baseEdges, edgeIdKey, setEdges]);

  useEffect(() => {
    setNodes((existing) =>
      existing.map((node) => {
        const selected = selectionPatch.selectedNodeId === node.id;
        const data = node.data as WorkflowCanvasNodeData;
        if (data.selected === selected) return node;
        return {
          ...node,
          data: {
            ...node.data,
            selected,
          },
        };
      }),
    );
  }, [selectionPatch.selectedNodeId, setNodes]);

  useEffect(() => {
    setNodes((existing) =>
      existing.map((node) => {
        const hoveredRelated = hoverPatch.hoveredNodeIds.has(node.id);
        const mutedByHover = Boolean(hoverPatch.hoveredEdgeId && !hoveredRelated);
        const data = node.data as WorkflowCanvasNodeData;
        if (data.hoveredRelated === hoveredRelated && data.mutedByHover === mutedByHover) return node;
        return {
          ...node,
          data: {
            ...node.data,
            hoveredRelated,
            mutedByHover,
          },
        };
      }),
    );
  }, [hoverPatch.hoveredEdgeId, hoverPatch.hoveredNodeIds, setNodes]);

  useEffect(() => {
    setEdges((existing) =>
      existing.map((edge) => {
        const edgeData = edge.data as WorkflowCanvasEdgeData | undefined;
        const hovered = hoverPatch.hoveredEdgeId === edge.id || hoverPatch.hoveredEdgeIds.has(edge.id);
        const mutedByHover = Boolean((hoverPatch.hoveredEdgeId || hoverPatch.hoveredNodeId) && !hovered);
        if (
          edgeData?.hovered === hovered &&
          edgeData?.mutedByHover === mutedByHover
        ) {
          return edge;
        }
        return {
          ...edge,
          data: {
            ...edgeData,
            hovered,
            mutedByHover,
          },
          style: {
            ...edge.style,
            opacity: mutedByHover ? 0.18 : undefined,
            strokeWidth: hovered ? 3 : undefined,
          },
          labelStyle: {
            ...edge.labelStyle,
            opacity: mutedByHover ? 0.35 : 1,
          },
        };
      }),
    );
  }, [hoverPatch.hoveredEdgeId, hoverPatch.hoveredEdgeIds, hoverPatch.hoveredNodeId, setEdges]);

  return (
    <ReactFlow
      defaultNodes={defaultNodes}
      defaultEdges={defaultEdges}
      nodeTypes={nodeTypes}
      edgeTypes={edgeTypes}
      fitView
      fitViewOptions={{ padding: 0.25 }}
      nodesDraggable
      nodesConnectable={structureEditable}
      elementsSelectable={structureEditable}
      elevateEdgesOnSelect
      onConnect={onConnect}
      onNodeClick={(_, node: Node<WorkflowCanvasNodeData>) => {
        onSelectNode(node.id);
      }}
      onEdgeClick={(_, edge: Edge) => {
        onSelectEdge(edge.id);
      }}
      onEdgeMouseEnter={(_, edge: Edge) => onHoverEdge(edge.id)}
      onEdgeMouseLeave={() => onHoverEdge(null)}
      onNodeMouseEnter={(_, node: Node<WorkflowCanvasNodeData>) => onHoverNode(node.id)}
      onNodeMouseLeave={() => onHoverNode(null)}
      onPaneClick={() => {
        onPaneClick();
      }}
      proOptions={{ hideAttribution: true }}
      className={cn("workflow-canvas-flow relative z-10", extraClassName)}
    >
      <Background gap={20} size={1} color="color-mix(in srgb, var(--muted-foreground) 22%, transparent)" />
      <Controls showInteractive={false} />
    </ReactFlow>
  );
}

function buildNodesWithPatch(
  baseNodes: Node<WorkflowCanvasNodeData>[],
  basePatch: NodeBasePatch,
  selectionPatch: SelectionPatch,
  hoverPatch: HoverPatch,
): Node<WorkflowCanvasNodeData>[] {
  return baseNodes.map((node) => ({
    ...node,
    data: {
      ...node.data,
      ...getNodeBaseDataOverrides(basePatch),
      ...getNodeSelectionDataOverrides(node.id, selectionPatch),
      ...getNodeHoverDataOverrides(node.id, hoverPatch),
    },
  }));
}

function getNodeBaseDataOverrides(
  patch: NodeBasePatch,
): Partial<WorkflowCanvasNodeData> & Record<string, unknown> {
  return {
    onOpenSubIssue: patch.onOpenSubIssue,
    onSelectNode: patch.onSelectNode,
    onAddNextNode: patch.onAddNextNode,
    editable: patch.editable,
    runStatus: patch.runStatus,
  };
}

function getNodeSelectionDataOverrides(
  nodeId: string,
  patch: SelectionPatch,
): Partial<WorkflowCanvasNodeData> {
  return {
    selected: patch.selectedNodeId === nodeId,
  };
}

function getNodeHoverDataOverrides(
  nodeId: string,
  patch: HoverPatch,
): Partial<WorkflowCanvasNodeData> {
  return {
    hoveredRelated: patch.hoveredNodeIds.has(nodeId),
    mutedByHover: Boolean(patch.hoveredEdgeId && !patch.hoveredNodeIds.has(nodeId)),
  };
}

function buildEdgesWithPatch(
  baseEdges: Edge[],
  hoverPatch: HoverPatch,
): Edge[] {
  return baseEdges.map((edge) => {
    const edgeData = edge.data as WorkflowCanvasEdgeData | undefined;
    const hovered = hoverPatch.hoveredEdgeId === edge.id || hoverPatch.hoveredEdgeIds.has(edge.id);
    const mutedByHover = Boolean((hoverPatch.hoveredEdgeId || hoverPatch.hoveredNodeId) && !hovered);
    return {
      ...edge,
      data: {
        ...edgeData,
        hovered,
        mutedByHover,
      },
      title: edgeData?.fullLabel ?? undefined,
      markerEnd: { type: MarkerType.ArrowClosed },
      style: {
        ...edge.style,
        opacity: mutedByHover ? 0.18 : edge.style?.opacity,
        strokeWidth: hovered ? 3 : edge.style?.strokeWidth,
      },
      labelStyle: {
        ...edge.labelStyle,
        opacity: mutedByHover ? 0.35 : 1,
      },
    };
  });
}

function WorkflowSelectionOverlay({
  node,
  edge,
  onClose,
  onOpenSubIssue,
}: {
  node?: Node<WorkflowCanvasNodeData>;
  edge?: Edge;
  onClose: () => void;
  onOpenSubIssue?: (issueId: string) => void;
}) {
  const edgeData = edge?.data as WorkflowCanvasEdgeData | undefined;
  return (
    <aside className="absolute bottom-3 right-3 top-3 z-30 flex w-[20rem] max-w-[calc(100%-1.5rem)] flex-col overflow-hidden rounded-lg border border-border/80 bg-background/95 text-sm shadow-[0_22px_60px_-34px_rgba(15,23,42,0.75)] backdrop-blur-md">
      <div className="flex items-start gap-2 border-b bg-muted/30 px-3 py-2.5">
        <div className="min-w-0 flex-1">
          <div className="truncate text-[10px] font-semibold uppercase tracking-wider text-muted-foreground">{node ? "Node details" : "Edge details"}</div>
          <div className="mt-0.5 truncate font-mono text-sm font-semibold text-foreground">{node?.id ?? edge?.id}</div>
        </div>
        <Button type="button" size="icon-xs" variant="ghost" className="-mr-1 -mt-0.5 text-muted-foreground" aria-label="Close workflow details" onClick={onClose}>
          <X className="size-3.5" aria-hidden="true" />
        </Button>
      </div>
      <div className="min-h-0 flex-1 overflow-y-auto p-3">
        {node ? (
          <div className="space-y-3 text-xs">
            <DetailSection title="Execution">
              <DetailRow label="Status" value={node.data.status} valueClassName={statusTextClass(node.data.status)} />
              <DetailRow label="Carrier" value={node.data.carrierLabel} />
            </DetailSection>
            <DetailSection title="Definition">
              <DetailRow label="Type" value={node.data.type} />
              <DetailRow label="Dispatch" value={node.data.dispatch} />
              {node.data.agentRoute && <DetailRow label="Agent route" value={node.data.agentRoute} />}
              {node.data.inputs.length > 0 && <DetailRow label="Inputs" value={node.data.inputs.join(", ")} />}
              {node.data.outputs.length > 0 && <DetailRow label="Outputs" value={node.data.outputs.join(", ")} />}
              {node.data.systemPrompt && <DetailBlock label="System prompt" value={node.data.systemPrompt} />}
            </DetailSection>
            {(node.data.traexSessionId || node.data.mainIssueTaskId || node.data.error) && (
              <DetailSection title="Runtime">
                {node.data.traexSessionId && <DetailRow label="TraeX session" value={node.data.traexSessionId} />}
                {node.data.mainIssueTaskId && <DetailRow label="Main task" value={node.data.mainIssueTaskId} />}
                {node.data.error && <DetailRow label="Error" value={node.data.error} tone="danger" />}
              </DetailSection>
            )}
            {node.data.subIssueId && (
              <div className="border-t pt-3">
                <Button type="button" size="xs" variant="secondary" onClick={() => onOpenSubIssue?.(node.data.subIssueId!)}>
                  Open sub-issue
                </Button>
              </div>
            )}
          </div>
        ) : edge ? (
          <div className="space-y-3 text-xs">
            <DetailSection title="Route">
              <DetailRow label="Source" value={edge.source} />
              <DetailRow label="Target" value={edge.target} />
              <DetailRow label="Kind" value={String(edgeData?.routeKind ?? "edge")} />
            </DetailSection>
            {edgeData?.fullLabel && (
              <DetailSection title="Condition">
                <DetailRow label="Expression" value={edgeData.fullLabel} />
              </DetailSection>
            )}
            <DetailSection title="Runtime">
              <DetailRow label="Selected branch" value={edgeData?.selectedByRouteDecision ? "yes" : "no"} />
              <DetailRow label="Completed path" value={edgeData?.completedPath ? "yes" : "no"} />
            </DetailSection>
          </div>
        ) : null}
      </div>
    </aside>
  );
}

function DetailSection({ title, children }: { title: string; children: ReactNode }) {
  return (
    <div className="rounded-md border border-border/70 bg-card/70">
      <div className="border-b px-2.5 py-1.5 text-[10px] font-semibold uppercase tracking-wider text-muted-foreground">{title}</div>
      <div className="divide-y divide-border/60">{children}</div>
    </div>
  );
}

function DetailRow({ label, value, tone, valueClassName }: { label: string; value: string; tone?: "danger"; valueClassName?: string }) {
  return (
    <div className="grid grid-cols-[6.75rem_minmax(0,1fr)] gap-2 px-2.5 py-1.5">
      <div className="text-[11px] text-muted-foreground">{label}</div>
      <div className={cn("min-w-0 break-words font-mono text-[11px] text-foreground", tone === "danger" && "text-destructive", valueClassName)}>{value}</div>
    </div>
  );
}

function DetailBlock({ label, value }: { label: string; value: string }) {
  return (
    <div className="space-y-1 px-2.5 py-1.5">
      <div className="text-[11px] text-muted-foreground">{label}</div>
      <pre className="max-h-40 overflow-auto whitespace-pre-wrap break-words rounded bg-muted/45 p-2 font-mono text-[11px] leading-4 text-foreground">
        {value}
      </pre>
    </div>
  );
}

const WorkflowNode = memo(function WorkflowNode({ data }: NodeProps) {
  const node = data as WorkflowCanvasNodeData & {
    editable?: boolean;
    onOpenSubIssue?: (issueId: string) => void;
    onSelectNode?: (nodeId: string) => void;
    onAddNextNode?: (nodeId: string) => void;
    runStatus?: WorkflowRunStatus;
  };
  const canOpen = Boolean(node.subIssueId);

  return (
    <div
      role={node.onSelectNode ? "button" : undefined}
      tabIndex={node.onSelectNode ? 0 : undefined}
      onClick={() => {
        node.onSelectNode?.(node.id);
      }}
      onKeyDown={(event) => {
        if ((event.key === "Enter" || event.key === " ") && node.onSelectNode) {
          event.preventDefault();
          node.onSelectNode?.(node.id);
        }
      }}
      className={cn(
        "relative overflow-hidden rounded-lg border bg-card/95 px-3 py-2.5 text-left shadow-[0_1px_1px_rgba(15,23,42,0.04),0_8px_18px_-16px_rgba(15,23,42,0.5)] transition-[border-color,background-color,box-shadow,opacity]",
        statusClass(node.status),
        node.selected && "ring-2 ring-primary/55 shadow-[0_0_0_1px_color-mix(in_srgb,var(--primary)_18%,transparent),0_14px_28px_-20px_rgba(15,23,42,0.55)]",
        node.status === "running" && "animate-pulse ring-2 ring-amber-400/35",
        node.hoveredRelated && !node.selected && "ring-1 ring-primary/45",
        node.mutedByHover && "opacity-45",
        node.onSelectNode ? "cursor-grab hover:border-foreground/20 hover:bg-accent/60 active:cursor-grabbing" : "cursor-default",
      )}
      style={{ width: WORKFLOW_NODE_WIDTH, minHeight: WORKFLOW_NODE_HEIGHT }}
    >
      <Handle id="target-left" type="target" position={Position.Left} className="!bg-muted-foreground" />
      <Handle id="target-top" type="target" position={Position.Top} className="!bg-muted-foreground" />
      <Handle id="target-bottom" type="target" position={Position.Bottom} className="!bg-muted-foreground" />
      <div className={cn("absolute inset-y-0 left-0 w-1", statusRailClass(node.status))} aria-hidden="true" />
      <div className="flex items-start justify-between gap-3 pl-1">
        <span className="min-w-0 truncate font-mono text-xs font-semibold leading-5 text-foreground">{node.label}</span>
        <span className={cn("shrink-0 rounded px-1.5 py-0.5 text-[10px] font-medium uppercase leading-4", statusBadgeClass(node.status))}>
          {canvasStatusLabel(node.runStatus, node.status, Boolean(node.subIssueId))}
        </span>
      </div>
      <div className="mt-2 flex min-w-0 items-center gap-1.5 pl-1">
        <span className={cn("size-1.5 shrink-0 rounded-full", statusDotClass(node.status))} aria-hidden="true" />
        <span className="truncate text-[11px] font-medium text-muted-foreground">{node.type}</span>
      </div>
      <div className="mt-1 truncate pl-1 text-[11px] text-muted-foreground">{node.carrierLabel}</div>
      {node.traexSessionId && (
        <div className="mt-1 truncate pl-1 text-[11px] text-muted-foreground">TraeX session {node.traexSessionId.slice(0, 8)}</div>
      )}
      {node.mainIssueTaskId && (
        <div className="mt-1 truncate pl-1 text-[11px] text-muted-foreground">Main task {node.mainIssueTaskId.slice(0, 8)}</div>
      )}
      {canvasStatusHint(node.runStatus, node.status, Boolean(node.subIssueId)) && (
        <div className="mt-1 truncate pl-1 text-[11px] text-muted-foreground">
          {canvasStatusHint(node.runStatus, node.status, Boolean(node.subIssueId))}
        </div>
      )}
      {node.error && <div className="mt-1 line-clamp-2 pl-1 text-[11px] text-destructive">{node.error}</div>}
      {canOpen && (
        <Button
          type="button"
          size="xs"
          variant="ghost"
          className="mt-2 h-6 px-1.5 text-[11px] text-muted-foreground hover:text-foreground"
          aria-label={`Open sub-issue for ${node.id}`}
          onClick={(event) => {
            event.stopPropagation();
            node.onOpenSubIssue?.(node.subIssueId!);
          }}
          onPointerDown={(event) => event.stopPropagation()}
        >
          Open sub-issue
        </Button>
      )}
      <Handle id="source-right" type="source" position={Position.Right} className="!bg-muted-foreground" />
      <Handle id="source-top" type="source" position={Position.Top} className="!bg-muted-foreground" />
      <Handle id="source-bottom" type="source" position={Position.Bottom} className="!bg-muted-foreground" />
    </div>
  );
});

const nodeTypes: NodeTypes = { workflow: WorkflowNode };

const WorkflowEdge = memo(function WorkflowEdge({
  id,
  data,
  label,
  labelStyle,
  labelBgStyle,
  labelBgPadding,
  labelBgBorderRadius,
  style,
  markerEnd,
  interactionWidth,
}: EdgeProps<Edge<WorkflowCanvasEdgeData>>) {
  const points = data?.routePoints ?? [];
  if (points.length < 2) return null;
  const path = pointsToPath(points);
  const labelPoint = midpoint(points);
  return (
    <>
      <BaseEdge
        id={id}
        path={path}
        style={style}
        markerEnd={markerEnd}
        interactionWidth={interactionWidth}
      />
      {label && (
        <EdgeText
          x={labelPoint.x}
          y={labelPoint.y}
          label={String(label)}
          labelStyle={labelStyle}
          labelBgStyle={labelBgStyle}
          labelBgPadding={labelBgPadding}
          labelBgBorderRadius={labelBgBorderRadius}
        />
      )}
    </>
  );
});

const edgeTypes: EdgeTypes = { workflow: WorkflowEdge };

function pointsToPath(points: Array<{ x: number; y: number }>): string {
  const [first, ...rest] = points;
  if (!first) return "";
  return [`M ${first.x} ${first.y}`, ...rest.map((point) => `L ${point.x} ${point.y}`)].join(" ");
}

function midpoint(points: Array<{ x: number; y: number }>): { x: number; y: number } {
  if (points.length === 0) return { x: 0, y: 0 };
  if (points.length === 1) return points[0]!;
  const middleIndex = Math.floor((points.length - 1) / 2);
  const from = points[middleIndex]!;
  const to = points[middleIndex + 1] ?? from;
  return { x: (from.x + to.x) / 2, y: (from.y + to.y) / 2 };
}

function statusClass(status: string): string {
  switch (status) {
    case "running":
      return "border-amber-500/60 bg-amber-50/90";
    case "done":
      return "border-green-500/60 bg-green-50/90";
    case "blocked":
      return "border-orange-500/70 bg-orange-50/90";
    case "failed":
      return "border-red-500/60 bg-red-50/90";
    case "cancelled":
      return "border-slate-400/70 bg-slate-50/90";
    default:
      return "border-border/80";
  }
}

function statusRailClass(status: string): string {
  switch (status) {
    case "running":
      return "bg-amber-500/70";
    case "done":
      return "bg-green-500/70";
    case "blocked":
      return "bg-orange-500/80";
    case "failed":
      return "bg-red-500/70";
    case "cancelled":
      return "bg-slate-400/70";
    case "planning":
      return "bg-blue-500/70";
    case "finalizing":
      return "bg-indigo-500/70";
    default:
      return "bg-border";
  }
}

function statusDotClass(status: string): string {
  switch (status) {
    case "running":
      return "bg-amber-500";
    case "done":
      return "bg-green-500";
    case "blocked":
      return "bg-orange-500";
    case "failed":
      return "bg-red-500";
    case "cancelled":
      return "bg-slate-400";
    case "planning":
      return "bg-blue-500";
    case "finalizing":
      return "bg-indigo-500";
    default:
      return "bg-muted-foreground/45";
  }
}

function statusBadgeClass(status: string): string {
  switch (status) {
    case "running":
      return "bg-amber-100 text-amber-800";
    case "done":
      return "bg-green-100 text-green-800";
    case "blocked":
      return "bg-orange-100 text-orange-800";
    case "failed":
      return "bg-red-100 text-red-800";
    case "cancelled":
      return "bg-slate-100 text-slate-700";
    case "planning":
      return "bg-blue-100 text-blue-800";
    case "finalizing":
      return "bg-indigo-100 text-indigo-800";
    default:
      return "bg-muted text-muted-foreground";
  }
}

function statusTextClass(status: string): string {
  switch (status) {
    case "running":
      return "text-amber-700";
    case "done":
      return "text-green-700";
    case "blocked":
      return "text-orange-700";
    case "failed":
      return "text-destructive";
    case "cancelled":
      return "text-slate-600";
    default:
      return "";
  }
}

function canvasStatusLabel(runStatus: WorkflowRunStatus | undefined, nodeStatus: string, hasSubIssue: boolean): string {
  if (runStatus === "cancelled" && nodeStatus === "pending") return "skipped";
  if (runStatus === "cancelled" && nodeStatus === "cancelled" && hasSubIssue) return "stopped";
  return nodeStatus;
}

function canvasStatusHint(runStatus: WorkflowRunStatus | undefined, nodeStatus: string, hasSubIssue: boolean): string | null {
  if (runStatus !== "cancelled") return null;
  if (nodeStatus === "done") return "Completed before stop";
  if (nodeStatus === "cancelled" && hasSubIssue) return "Child task cancelled";
  if (nodeStatus === "pending") return "Skipped after stop";
  return null;
}
