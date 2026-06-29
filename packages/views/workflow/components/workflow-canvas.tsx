"use client";

import { memo, useEffect, useMemo, useState } from "react";
import {
  applyNodeChanges,
  Background,
  Controls,
  Handle,
  MarkerType,
  Position,
  ReactFlow,
  type Connection,
  type Edge,
  type Node,
  type NodeChange,
  type NodeProps,
  type NodeTypes,
} from "@xyflow/react";
import "@xyflow/react/dist/style.css";
import type { WorkflowDefinition, WorkflowNodeRunState, WorkflowRunStatus } from "@multica/core/workflow/types";
import { Button } from "@multica/ui/components/ui/button";
import { cn } from "@multica/ui/lib/utils";
import { workflowToReactFlow, type WorkflowCanvasNodeData } from "../lib/to-react-flow";

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
}) {
  const [fullscreen, setFullscreen] = useState(false);
  const { nodes, edges } = useMemo(
    () => workflowToReactFlow(definition, runState ?? {}, { draggable: editable, selectableEdges: editable, selectedNodeId, selectedEdgeId }),
    [definition, editable, runState, selectedEdgeId, selectedNodeId],
  );
  const clickableNodes = useMemo(
    () => nodes.map((node) => ({ ...node, data: { ...node.data, onOpenSubIssue, onSelectNode, onAddNextNode, editable, runStatus } })),
    [editable, nodes, onAddNextNode, onOpenSubIssue, onSelectNode, runStatus],
  );
  const [flowNodes, setFlowNodes] = useState(clickableNodes);
  useEffect(() => {
    setFlowNodes((current) => {
      const positions = new Map(current.map((node) => [node.id, node.position]));
      return clickableNodes.map((node) => ({
        ...node,
        position: positions.get(node.id) ?? node.position,
      })) as typeof clickableNodes;
    });
  }, [clickableNodes]);
  const visibleEdges = useMemo(
    () => edges.map((edge) => ({ ...edge, markerEnd: { type: MarkerType.ArrowClosed } })),
    [edges],
  );

  if (nodes.length === 0) {
    return <div className="rounded-md border bg-muted/20 p-4 text-sm text-muted-foreground">No workflow nodes.</div>;
  }

  const canvas = (extraClassName?: string) => (
    <ReactFlow
      nodes={flowNodes}
      edges={visibleEdges}
      nodeTypes={nodeTypes}
      fitView
      fitViewOptions={{ padding: 0.25 }}
      nodesDraggable={editable}
      nodesConnectable={editable}
      elementsSelectable={editable}
      onNodesChange={(changes: NodeChange[]) => setFlowNodes((current) => applyNodeChanges(changes, current) as typeof clickableNodes)}
      onConnect={onConnect}
      onNodeClick={(_, node: Node<WorkflowCanvasNodeData>) => {
        if (editable) onSelectNode?.(node.id);
      }}
      onEdgeClick={(_, edge: Edge) => {
        if (editable) onSelectEdge?.(edge.id);
      }}
      onPaneClick={onPaneClick}
      proOptions={{ hideAttribution: true }}
      className={extraClassName}
    >
      <Background gap={18} size={1} />
      <Controls showInteractive={false} />
    </ReactFlow>
  );

  return (
    <>
      <div className={cn("relative h-[360px] overflow-hidden rounded-md border bg-background", className)}>
        {fullscreenTitle && (
          <div className="absolute right-2 top-2 z-20">
            <Button
              type="button"
              size="xs"
              variant="secondary"
              aria-label="Expand workflow canvas"
              onClick={() => setFullscreen(true)}
            >
              Fullscreen
            </Button>
          </div>
        )}
        {canvas()}
      </div>
      {fullscreen && fullscreenTitle && (
        <div
          role="dialog"
          aria-label={fullscreenTitle}
          className="fixed inset-0 z-50 flex flex-col bg-background"
          onKeyDown={(event) => {
            if (event.key === "Escape") setFullscreen(false);
          }}
        >
          <div className="flex h-12 shrink-0 items-center gap-3 border-b px-4">
            <div className="min-w-0">
              <h2 className="truncate text-sm font-semibold">{fullscreenTitle}</h2>
              <p className="text-xs text-muted-foreground">Fullscreen</p>
            </div>
            <Button
              type="button"
              size="sm"
              variant="outline"
              className="ml-auto"
              aria-label="Close fullscreen workflow canvas"
              onClick={() => setFullscreen(false)}
            >
              Close
            </Button>
          </div>
          <div className="min-h-0 flex-1">
            {canvas("h-full")}
          </div>
        </div>
      )}
    </>
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
      role={canOpen || node.editable ? "button" : undefined}
      tabIndex={canOpen || node.editable ? 0 : undefined}
      onClick={() => {
        if (node.editable) node.onSelectNode?.(node.id);
        else if (node.subIssueId) node.onOpenSubIssue?.(node.subIssueId);
      }}
      onKeyDown={(event) => {
        if ((event.key === "Enter" || event.key === " ") && (canOpen || node.editable)) {
          event.preventDefault();
          if (node.editable) node.onSelectNode?.(node.id);
          else if (node.subIssueId) node.onOpenSubIssue?.(node.subIssueId);
        }
      }}
      className={cn(
        "relative min-w-44 rounded-md border bg-card px-3 py-2 text-left shadow-sm transition-colors",
        statusClass(node.status),
        node.selected && "ring-2 ring-primary",
        canOpen || node.editable ? "cursor-pointer hover:bg-accent" : "cursor-default",
      )}
    >
      <Handle type="target" position={Position.Left} className="!bg-muted-foreground" />
      <div className="flex items-center justify-between gap-3">
        <span className="truncate font-mono text-xs font-semibold">{node.label}</span>
        <span className="rounded bg-background/70 px-1.5 py-0.5 text-[10px] uppercase text-muted-foreground">
          {canvasStatusLabel(node.runStatus, node.status, Boolean(node.subIssueId))}
        </span>
      </div>
      <div className="mt-1 text-xs text-muted-foreground">{node.type}</div>
      <div className="mt-1 text-xs font-medium text-muted-foreground">{node.carrierLabel}</div>
      {node.traexSessionId && (
        <div className="mt-1 text-xs text-muted-foreground">TraeX session {node.traexSessionId.slice(0, 8)}</div>
      )}
      {node.mainIssueTaskId && (
        <div className="mt-1 text-xs text-muted-foreground">Main task {node.mainIssueTaskId.slice(0, 8)}</div>
      )}
      {canvasStatusHint(node.runStatus, node.status, Boolean(node.subIssueId)) && (
        <div className="mt-1 text-xs text-muted-foreground">
          {canvasStatusHint(node.runStatus, node.status, Boolean(node.subIssueId))}
        </div>
      )}
      {node.error && <div className="mt-1 line-clamp-2 text-xs text-destructive">{node.error}</div>}
      {node.editable && (
        <span
          role="button"
          tabIndex={0}
          aria-label={`Add node after ${node.id}`}
          onClick={(event) => {
            event.stopPropagation();
            node.onAddNextNode?.(node.id);
          }}
          onKeyDown={(event) => {
            if (event.key === "Enter" || event.key === " ") {
              event.preventDefault();
              event.stopPropagation();
              node.onAddNextNode?.(node.id);
            }
          }}
          className="absolute -right-3 top-1/2 z-10 flex h-6 w-6 -translate-y-1/2 items-center justify-center rounded-full border bg-background text-xs shadow-sm hover:bg-accent"
        >
          +
        </span>
      )}
      <Handle type="source" position={Position.Right} className="!bg-muted-foreground" />
    </div>
  );
});

const nodeTypes: NodeTypes = { workflow: WorkflowNode };

function statusClass(status: string): string {
  switch (status) {
    case "running":
      return "border-amber-500/70 bg-amber-50";
    case "done":
      return "border-green-500/70 bg-green-50";
    case "failed":
      return "border-red-500/70 bg-red-50";
    case "cancelled":
      return "border-slate-400/80 bg-slate-50";
    default:
      return "border-border";
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
