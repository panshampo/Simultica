"use client";

import { useEffect, useMemo, useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { ArrowLeft, FolderGit2 } from "lucide-react";
import { useWorkspaceId } from "@multica/core/hooks";
import { useWorkspacePaths } from "@multica/core/paths";
import {
  workflowCaseDefinitionVersionsOptions,
  workflowCaseDetailOptions,
  workflowCaseRunsOptions,
} from "@multica/core/workflow/queries";
import type { WorkflowRun, WorkflowRunNode } from "@multica/core/workflow/types";
import { Skeleton } from "@multica/ui/components/ui/skeleton";
import { cn } from "@multica/ui/lib/utils";
import { PageHeader } from "../../layout/page-header";
import { AppLink } from "../../navigation";
import { WorkflowCanvas } from "./workflow-canvas";
import { WorkflowRunNodeDetailPanel } from "./workflow-run-node-detail-panel";

export function WorkflowRunDetailPage({
  caseId,
  runId,
  initialNodeId,
}: {
  caseId: string;
  runId: string;
  initialNodeId?: string | null;
}) {
  const wsId = useWorkspaceId();
  const paths = useWorkspacePaths();
  const [selectedNodeId, setSelectedNodeId] = useState<string | null>(initialNodeId ?? null);
  const { data: workflowCase } = useQuery(workflowCaseDetailOptions(wsId, caseId));
  const { data: versions = [] } = useQuery(workflowCaseDefinitionVersionsOptions(wsId, caseId));
  const { data: runs = [], isLoading } = useQuery(workflowCaseRunsOptions(wsId, caseId));

  const run = useMemo(() => runs.find((item) => item.id === runId) ?? null, [runs, runId]);
  const runVersion = run?.definition_version_id
    ? versions.find((version) => version.id === run.definition_version_id) ?? null
    : null;
  const runVersionLabel = runVersion ? `v${runVersion.version}` : (run?.definition_version_id ? shortId(run.definition_version_id) : "-");
  const nodes = run?.nodes ?? [];
  const selectedNode = nodes.find((node) => node.node_id === selectedNodeId) ?? null;
  const selectedDefinitionNode = run?.definition_snapshot.nodes.find((node) => node.id === selectedNodeId) ?? null;
  const nodeMissing = Boolean(initialNodeId) && !isLoading && Boolean(run) && !nodes.some((n) => n.node_id === initialNodeId);

  useEffect(() => {
    if (initialNodeId) setSelectedNodeId(initialNodeId);
  }, [initialNodeId]);

  if (isLoading) {
    return <WorkflowRunDetailSkeleton />;
  }

  if (!run) {
    return (
      <div className="flex h-full flex-col">
        <PageHeader className="px-5">
          <AppLink href={paths.workflowCaseDetail(caseId)} className="flex items-center gap-1 text-sm text-muted-foreground hover:text-foreground">
            <ArrowLeft className="size-4" />
            Workflow case
          </AppLink>
        </PageHeader>
        <div className="flex flex-1 items-center justify-center text-sm text-muted-foreground">Workflow run not found in this case.</div>
      </div>
    );
  }

  return (
    <div className="flex h-full min-h-0 flex-col">
      <PageHeader className="justify-between px-5">
        <div className="flex min-w-0 items-center gap-2">
          <AppLink
            href={paths.workflowCaseDetail(caseId)}
            className="rounded-md p-1 text-muted-foreground hover:bg-accent hover:text-foreground"
            aria-label="Back to workflow case"
          >
            <ArrowLeft className="size-4" />
          </AppLink>
          <FolderGit2 className="size-4 shrink-0 text-muted-foreground" />
          <h1 className="truncate text-sm font-medium">
            {run.label || `Run ${shortId(run.id)}`}
          </h1>
          <span className={runStatusClass(run.status)}>{run.status}</span>
        </div>
        {workflowCase && (
          <AppLink
            href={paths.workflowCaseDetail(caseId)}
            className="shrink-0 truncate text-xs text-muted-foreground underline underline-offset-2 hover:text-foreground"
          >
            {workflowCase.title}
          </AppLink>
        )}
      </PageHeader>

      <div className="grid min-h-0 flex-1 gap-4 overflow-y-auto p-5 xl:grid-cols-[minmax(0,1fr)_320px]">
        <main className="min-w-0 space-y-4">
          <section className="rounded-lg border bg-card p-4">
            <div className="grid gap-3 text-sm md:grid-cols-3 lg:grid-cols-5">
              <Property label="Run" value={shortId(run.id)} mono />
              <Property label="Status" value={run.status} />
              <Property label="Version" value={run.definition_version_id ? undefined : "-"} mono>
                {run.definition_version_id && (
                  <AppLink href={paths.workflowCaseVersionDetail(caseId, run.definition_version_id)} className="underline underline-offset-2 hover:text-foreground">
                    {runVersionLabel}
                  </AppLink>
                )}
              </Property>
              <Property label="Started" value={run.started_at ? formatDateTime(run.started_at) : "-"} />
              <Property label="Completed" value={run.completed_at ? formatDateTime(run.completed_at) : "-"} />
            </div>
            {run.error && <p className="mt-3 rounded-md bg-red-50 p-2 text-xs text-red-700">{run.error}</p>}
          </section>

          <section className="space-y-3">
            <h2 className="text-sm font-medium">Run graph</h2>
            {nodeMissing && (
              <div className="rounded-md border border-dashed bg-background/60 p-2 text-xs text-muted-foreground">
                Node not found in this run.
              </div>
            )}
            <WorkflowCanvas
              definition={run.definition_snapshot}
              runState={run.nodes_state}
              runStatus={run.status}
              selectedNodeId={selectedNodeId}
              onSelectNode={setSelectedNodeId}
              onPaneClick={() => setSelectedNodeId(null)}
              hideSelectionOverlay
              fullscreenTitle={run.label || `Run ${shortId(run.id)}`}
            />
          </section>

          <NodeTable
            nodes={nodes}
            selectedNodeId={selectedNodeId}
            onSelectNode={setSelectedNodeId}
            issueHref={(issueId) => paths.issueDetail(issueId)}
          />
        </main>

        <aside className="space-y-4">
          <WorkflowRunNodeDetailPanel
            node={selectedNode}
            definitionNode={selectedDefinitionNode}
            issueHref={(issueId) => paths.issueDetail(issueId)}
          />
        </aside>
      </div>
    </div>
  );
}

function NodeTable({
  nodes,
  selectedNodeId,
  onSelectNode,
  issueHref,
}: {
  nodes: WorkflowRunNode[];
  selectedNodeId: string | null;
  onSelectNode: (nodeId: string) => void;
  issueHref: (issueId: string) => string;
}) {
  if (nodes.length === 0) {
    return (
      <section className="rounded-lg border border-dashed bg-background/60 p-4 text-sm text-muted-foreground">
        This run has no projected nodes yet.
      </section>
    );
  }
  return (
    <section className="overflow-hidden rounded-lg border bg-card">
      <div className="border-b px-3 py-2">
        <h2 className="text-sm font-medium">Nodes</h2>
      </div>
      <ul className="divide-y">
        {nodes.map((node) => {
          const selected = node.node_id === selectedNodeId;
          const carrierIssueId = node.carrier_kind === "issue" ? issueIdFromCarrierRef(node.carrier_ref) : null;
          return (
            <li key={node.id} className={cn("flex items-center gap-2", selected && "bg-accent/60")}>
              <button
                type="button"
                className={cn(
                  "flex min-w-0 flex-1 items-center gap-3 px-3 py-2 text-left text-sm hover:bg-accent/40",
                  selected && "text-accent-foreground",
                )}
                onClick={() => onSelectNode(node.node_id)}
              >
                <span className="min-w-0 flex-1 truncate font-mono text-xs">{node.node_id}</span>
                <span className="shrink-0 text-xs text-muted-foreground">{node.node_type}</span>
                <span className="shrink-0 rounded bg-muted px-1.5 py-0.5 text-xs text-muted-foreground">{node.carrier_kind ?? node.dispatch}</span>
                <span className={nodeStatusClass(node.status)}>{node.status}</span>
              </button>
              {carrierIssueId && (
                <AppLink
                  href={issueHref(carrierIssueId)}
                  className="mr-2 inline-flex h-7 shrink-0 items-center rounded-md px-2 text-xs text-muted-foreground hover:bg-accent hover:text-foreground"
                >
                  Open issue
                </AppLink>
              )}
            </li>
          );
        })}
      </ul>
    </section>
  );
}

function issueIdFromCarrierRef(ref: unknown): string | null {
  if (!ref || typeof ref !== "object") return null;
  const rec = ref as Record<string, unknown>;
  for (const key of ["issue_id", "sub_issue_id", "issueId", "subIssueId"]) {
    const value = rec[key];
    if (typeof value === "string" && value.trim() !== "") return value;
  }
  return null;
}

function Property({ label, value, mono = false, children }: { label: string; value?: string; mono?: boolean; children?: React.ReactNode }) {
  return (
    <div className="min-w-0">
      <div className="text-[11px] uppercase text-muted-foreground">{label}</div>
      <div className={mono ? "mt-1 truncate font-mono text-xs" : "mt-1 truncate text-sm"}>{children ?? value ?? "-"}</div>
    </div>
  );
}

function WorkflowRunDetailSkeleton() {
  return (
    <div className="flex h-full flex-col">
      <PageHeader className="justify-between px-5">
        <Skeleton className="h-5 w-48" />
      </PageHeader>
      <div className="grid flex-1 gap-4 p-5 xl:grid-cols-[minmax(0,1fr)_320px]">
        <div className="space-y-4">
          <Skeleton className="h-24 rounded-lg" />
          <Skeleton className="h-[360px] rounded-lg" />
        </div>
        <Skeleton className="h-80 rounded-lg" />
      </div>
    </div>
  );
}

function runStatusClass(status: WorkflowRun["status"]): string {
  const base = "rounded px-1.5 py-0.5 text-xs";
  switch (status) {
    case "planning":
    case "running":
    case "finalizing":
      return `${base} bg-blue-100 text-blue-800`;
    case "done":
      return `${base} bg-green-100 text-green-800`;
    case "failed":
      return `${base} bg-red-100 text-red-800`;
    case "cancelled":
    case "cancelling":
      return `${base} bg-slate-100 text-slate-700`;
    default:
      return `${base} bg-muted text-muted-foreground`;
  }
}

function nodeStatusClass(status: WorkflowRunNode["status"]): string {
  const base = "shrink-0 rounded px-1.5 py-0.5 text-xs";
  switch (status) {
    case "running":
      return `${base} bg-blue-100 text-blue-800`;
    case "succeeded":
      return `${base} bg-green-100 text-green-800`;
    case "failed":
      return `${base} bg-red-100 text-red-800`;
    case "blocked":
      return `${base} bg-amber-100 text-amber-800`;
    case "cancelled":
    case "cancelling":
    case "skipped":
      return `${base} bg-slate-100 text-slate-700`;
    default:
      return `${base} bg-muted text-muted-foreground`;
  }
}

function shortId(id: string): string {
  return id.length > 8 ? id.slice(0, 8) : id;
}

function formatDateTime(value: string): string {
  return new Intl.DateTimeFormat(undefined, {
    month: "short",
    day: "numeric",
    hour: "2-digit",
    minute: "2-digit",
  }).format(new Date(value));
}
