"use client";

import { useState } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { toast } from "sonner";
import { FolderGit2, Plus, Search } from "lucide-react";
import { api } from "@multica/core/api";
import { useWorkspaceId } from "@multica/core/hooks";
import { useWorkspacePaths } from "@multica/core/paths";
import { workflowCaseListOptions, workflowRunKeys } from "@multica/core/workflow/queries";
import type { WorkflowCase } from "@multica/core/workflow/types";
import { Button } from "@multica/ui/components/ui/button";
import { Input } from "@multica/ui/components/ui/input";
import { Skeleton } from "@multica/ui/components/ui/skeleton";
import { PageHeader } from "../../layout/page-header";
import { AppLink, useNavigation } from "../../navigation";

export function WorkflowCaseListPage() {
  const wsId = useWorkspaceId();
  const paths = useWorkspacePaths();
  const navigation = useNavigation();
  const qc = useQueryClient();
  const { data: cases = [], isLoading } = useQuery(workflowCaseListOptions(wsId));
  const [search, setSearch] = useState("");
  const [creating, setCreating] = useState(false);
  const filtered = cases.filter((workflowCase) => {
    const q = search.trim().toLowerCase();
    if (!q) return true;
    return [workflowCase.title, workflowCase.description, workflowCase.status, workflowCase.source_issue_id ?? ""]
      .join(" ")
      .toLowerCase()
      .includes(q);
  });

  async function createCase() {
    setCreating(true);
    try {
      const workflowCase = await api.createWorkflowCase({
        title: "Untitled workflow case",
        description: "",
      });
      await qc.invalidateQueries({ queryKey: workflowRunKeys.caseList(wsId) });
      navigation.push(paths.workflowCaseDetail(workflowCase.id));
    } catch (err) {
      toast.error(err instanceof Error ? err.message : "Failed to create workflow case");
    } finally {
      setCreating(false);
    }
  }

  return (
    <div className="flex h-full min-h-0 flex-col">
      <PageHeader className="justify-between px-5">
        <div className="flex min-w-0 items-center gap-2">
          <FolderGit2 className="size-4 text-muted-foreground" />
          <h1 className="text-sm font-medium">Workflow cases</h1>
          {!isLoading && <span className="text-xs tabular-nums text-muted-foreground">{cases.length}</span>}
        </div>
        <Button type="button" size="sm" variant="outline" onClick={() => void createCase()} disabled={creating}>
          <Plus className="mr-1 size-3.5" />
          {creating ? "Creating..." : "Create case"}
        </Button>
      </PageHeader>

      <div className="flex h-12 shrink-0 items-center gap-3 border-b px-4">
        <div className="relative min-w-0 flex-1 sm:max-w-sm">
          <Search className="pointer-events-none absolute left-2.5 top-1/2 size-3.5 -translate-y-1/2 text-muted-foreground" />
          <Input
            value={search}
            onChange={(event) => setSearch(event.target.value)}
            placeholder="Search workflow cases"
            className="h-8 pl-8 text-sm"
          />
        </div>
        <span className="hidden font-mono text-xs tabular-nums text-muted-foreground/70 sm:inline">
          {filtered.length} / {cases.length}
        </span>
      </div>

      <div className="min-h-0 flex-1 overflow-y-auto">
        {isLoading ? (
          <WorkflowCaseListSkeleton />
        ) : cases.length === 0 ? (
          <div className="flex flex-col items-center justify-center gap-3 py-16 text-center">
            <FolderGit2 className="size-10 text-muted-foreground/50" />
            <p className="text-sm text-muted-foreground">No workflow cases yet.</p>
            <Button type="button" size="sm" variant="outline" onClick={() => void createCase()} disabled={creating}>
              <Plus className="mr-1 size-3.5" />
              Create case
            </Button>
          </div>
        ) : filtered.length === 0 ? (
          <div className="py-16 text-center text-sm text-muted-foreground">No workflow cases match this search.</div>
        ) : (
          <div className="divide-y">
            <div className="sticky top-0 z-[1] hidden h-8 items-center gap-3 border-b bg-muted/30 px-5 text-[11px] font-medium uppercase tracking-wide text-muted-foreground sm:grid sm:grid-cols-[minmax(220px,1fr)_120px_160px_120px_120px]">
              <span>Case</span>
              <span>Status</span>
              <span>Source issue</span>
              <span>Current run</span>
              <span>Updated</span>
            </div>
            {filtered.map((workflowCase) => (
              <WorkflowCaseRow key={workflowCase.id} workflowCase={workflowCase} />
            ))}
          </div>
        )}
      </div>
    </div>
  );
}

function WorkflowCaseRow({ workflowCase }: { workflowCase: WorkflowCase }) {
  const paths = useWorkspacePaths();
  return (
    <AppLink
      href={paths.workflowCaseDetail(workflowCase.id)}
      className="grid gap-2 px-4 py-3 text-sm transition-colors hover:bg-accent/40 sm:h-14 sm:grid-cols-[minmax(220px,1fr)_120px_160px_120px_120px] sm:items-center sm:gap-3 sm:px-5 sm:py-0"
    >
      <div className="min-w-0">
        <div className="truncate font-medium">{workflowCase.title}</div>
        {workflowCase.description && (
          <div className="line-clamp-1 text-xs text-muted-foreground">{workflowCase.description}</div>
        )}
      </div>
      <div><span className={caseStatusClass(workflowCase.status)}>{workflowCase.status}</span></div>
      <div className="truncate font-mono text-xs text-muted-foreground">{workflowCase.source_issue_id ?? "-"}</div>
      <div className="truncate font-mono text-xs text-muted-foreground">{workflowCase.current_run_id ? shortId(workflowCase.current_run_id) : "-"}</div>
      <div className="text-xs tabular-nums text-muted-foreground">{formatDateTime(workflowCase.updated_at)}</div>
    </AppLink>
  );
}

function WorkflowCaseListSkeleton() {
  return (
    <div className="divide-y">
      {Array.from({ length: 6 }).map((_, index) => (
        <div key={index} className="grid gap-3 px-5 py-3 sm:h-14 sm:grid-cols-[minmax(220px,1fr)_120px_160px_120px_120px] sm:items-center sm:py-0">
          <div className="space-y-1.5">
            <Skeleton className="h-4 w-44" />
            <Skeleton className="h-3 w-64 max-w-full" />
          </div>
          <Skeleton className="h-4 w-16" />
          <Skeleton className="h-4 w-32" />
          <Skeleton className="h-4 w-16" />
          <Skeleton className="h-4 w-20" />
        </div>
      ))}
    </div>
  );
}

function caseStatusClass(status: WorkflowCase["status"]): string {
  const base = "rounded px-1.5 py-0.5 text-xs";
  switch (status) {
    case "running":
    case "planned":
      return `${base} bg-blue-100 text-blue-800`;
    case "succeeded":
      return `${base} bg-green-100 text-green-800`;
    case "failed":
      return `${base} bg-red-100 text-red-800`;
    case "paused":
      return `${base} bg-amber-100 text-amber-800`;
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
