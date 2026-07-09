"use client";

import type { WorkflowRun } from "@multica/core/workflow/types";
import { Button } from "@multica/ui/components/ui/button";
import { cn } from "@multica/ui/lib/utils";
import { AppLink } from "../../navigation";

export function WorkflowCaseRunList({
  runs,
  selectedRunId,
  onSelectRun,
  onCancelRun,
  cancellingRunId,
  activeStatuses,
  openRunHref,
  versionHref,
  versionLabelById = {},
}: {
  runs: WorkflowRun[];
  selectedRunId?: string | null;
  onSelectRun: (runId: string) => void;
  onCancelRun?: (runId: string) => void;
  cancellingRunId?: string | null;
  activeStatuses?: Set<WorkflowRun["status"]>;
  openRunHref?: (runId: string) => string;
  versionHref?: (versionId: string) => string;
  versionLabelById?: Record<string, string>;
}) {
  if (runs.length === 0) {
    return (
      <div className="rounded-lg border border-dashed bg-background/60 p-4 text-sm text-muted-foreground">
        No workflow runs yet.
      </div>
    );
  }

  return (
    <div className="overflow-hidden rounded-lg border bg-card">
      <div className="border-b px-3 py-2">
        <h2 className="text-sm font-medium">Runs</h2>
      </div>
      <ul className="divide-y">
        {runs.map((run) => {
          const selected = run.id === selectedRunId;
          const cancellable = Boolean(onCancelRun) && (activeStatuses ? activeStatuses.has(run.status) : false);
          const versionLabel = run.definition_version_id ? (versionLabelById[run.definition_version_id] ?? shortId(run.definition_version_id)) : null;
          return (
            <li key={run.id} className={cn("flex items-center gap-1", selected && "bg-accent/60")}>
              <Button
                type="button"
                variant="ghost"
                className={cn(
                  "h-auto min-w-0 flex-1 justify-start rounded-none px-3 py-2 text-left",
                  selected && "text-accent-foreground",
                )}
                onClick={() => onSelectRun(run.id)}
              >
                <span className="min-w-0 flex-1">
                  <span className="flex items-center gap-2">
                    <span className={statusClass(run.status)}>{run.status}</span>
                    <span className="truncate font-mono text-xs text-muted-foreground">{shortId(run.id)}</span>
                  </span>
                  <span className="mt-1 block truncate text-xs text-muted-foreground">
                    {run.label ? `${run.label} · ` : ""}
                    {formatDateTime(run.started_at ?? run.created_at)}
                    {run.current_node ? ` · ${run.current_node}` : ""}
                  </span>
                </span>
              </Button>
              {openRunHref && (
                <AppLink
                  href={openRunHref(run.id)}
                  className="mr-1 inline-flex h-7 items-center rounded-md px-2 text-xs text-muted-foreground hover:bg-accent hover:text-foreground"
                >
                  Open
                </AppLink>
              )}
              {run.definition_version_id && versionHref && (
                <AppLink
                  href={versionHref(run.definition_version_id)}
                  className="mr-1 inline-flex h-7 items-center rounded-md px-2 text-xs text-muted-foreground hover:bg-accent hover:text-foreground"
                >
                  {versionLabel ?? "Version"}
                </AppLink>
              )}
              {cancellable && (
                <Button
                  type="button"
                  size="sm"
                  variant="ghost"
                  className="mr-1 h-7 px-2 text-xs text-red-600 hover:text-red-700"
                  onClick={() => onCancelRun?.(run.id)}
                  disabled={cancellingRunId === run.id}
                >
                  {cancellingRunId === run.id ? "Cancelling..." : "Cancel"}
                </Button>
              )}
            </li>
          );
        })}
      </ul>
    </div>
  );
}

function statusClass(status: WorkflowRun["status"]): string {
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
