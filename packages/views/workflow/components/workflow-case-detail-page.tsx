"use client";

import { useEffect, useMemo, useState } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { toast } from "sonner";
import { ArrowLeft, CheckCircle2, CircleAlert, FolderGit2, Play, Rocket, Trash2 } from "lucide-react";
import { api } from "@multica/core/api";
import { useWorkspaceId } from "@multica/core/hooks";
import { useWorkspacePaths } from "@multica/core/paths";
import {
  workflowCaseDefinitionOptions,
  workflowCaseDefinitionVersionsOptions,
  workflowCaseDetailOptions,
  workflowCaseRunsOptions,
  workflowRunKeys,
} from "@multica/core/workflow/queries";
import type {
  WorkflowDefinitionVersion,
  WorkflowRun,
  WorkflowRunKind,
  WorkflowValidationReport,
} from "@multica/core/workflow/types";
import { Button } from "@multica/ui/components/ui/button";
import { Input } from "@multica/ui/components/ui/input";
import { Skeleton } from "@multica/ui/components/ui/skeleton";
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from "@multica/ui/components/ui/alert-dialog";
import { cn } from "@multica/ui/lib/utils";
import { PageHeader } from "../../layout/page-header";
import { AppLink, useNavigation } from "../../navigation";
import { WorkflowCanvas } from "./workflow-canvas";
import { WorkflowCaseDraftEditor } from "./workflow-case-draft-editor";
import { WorkflowCaseRunList } from "./workflow-case-run-list";
import { WorkflowRunNodeDetailPanel } from "./workflow-run-node-detail-panel";

const ACTIVE_RUN_STATUSES = new Set<WorkflowRun["status"]>(["pending", "planning", "running", "finalizing", "cancelling"]);
const RUN_KINDS: WorkflowRunKind[] = ["primary", "experiment", "shadow", "replay", "debug"];

export function WorkflowCaseDetailPage({
  caseId,
  initialRunId,
  initialNodeId,
}: {
  caseId: string;
  initialRunId?: string | null;
  initialNodeId?: string | null;
}) {
  const wsId = useWorkspaceId();
  const paths = useWorkspacePaths();
  const navigation = useNavigation();
  const qc = useQueryClient();
  const [validation, setValidation] = useState<WorkflowValidationReport | null>(null);
  const [validating, setValidating] = useState(false);
  const [publishing, setPublishing] = useState(false);
  const [starting, setStarting] = useState(false);
  const [cancellingRunId, setCancellingRunId] = useState<string | null>(null);
  const [editingDraft, setEditingDraft] = useState(false);
  const [deleteOpen, setDeleteOpen] = useState(false);
  const [deleting, setDeleting] = useState(false);
  const [runKind, setRunKind] = useState<WorkflowRunKind>("primary");
  const [runLabel, setRunLabel] = useState("");
  const [selectedRunId, setSelectedRunId] = useState<string | null>(initialRunId ?? null);
  const [selectedNodeId, setSelectedNodeId] = useState<string | null>(initialNodeId ?? null);
  const { data: workflowCase, isLoading: caseLoading } = useQuery(workflowCaseDetailOptions(wsId, caseId));
  const { data: definition, isLoading: definitionLoading } = useQuery(workflowCaseDefinitionOptions(wsId, caseId));
  const { data: definitionVersions = [] } = useQuery(workflowCaseDefinitionVersionsOptions(wsId, caseId));
  const { data: runs = [], isLoading: runsLoading } = useQuery(workflowCaseRunsOptions(wsId, caseId));

  const onlineVersionId = workflowCase?.online_version_id ?? null;
  const hasOnlineVersion = Boolean(onlineVersionId);
  const selectedRun = useMemo(() => {
    if (selectedRunId) return runs.find((run) => run.id === selectedRunId) ?? null;
    return runs[0] ?? null;
  }, [runs, selectedRunId]);
  const selectedRunNode = selectedRun?.nodes?.find((node) => node.node_id === selectedNodeId) ?? null;
  const canPublish = Boolean(definition && validation?.valid && !editingDraft);
  const canStart = hasOnlineVersion && !editingDraft;
  const entryIssueId = workflowCase?.entry_issue_id ?? workflowCase?.source_issue_id ?? null;
  const activeRunCount = runs.filter((run) => ACTIVE_RUN_STATUSES.has(run.status)).length;

  useEffect(() => {
    if (!selectedRunId && runs[0]) setSelectedRunId(runs[0].id);
  }, [runs, selectedRunId]);

  useEffect(() => {
    // Preserve a deep-linked node on the initially targeted run; otherwise
    // clear node selection whenever the viewed run changes.
    if (selectedRun?.id === initialRunId) return;
    setSelectedNodeId(null);
  }, [selectedRun?.id, initialRunId]);

  async function validateDefinition() {
    setValidating(true);
    try {
      const report = await api.validateWorkflowCaseDefinition(caseId);
      setValidation(report);
      toast.success(report.valid ? "Workflow definition is valid" : "Workflow definition has validation issues");
    } catch (err) {
      toast.error(err instanceof Error ? err.message : "Failed to validate workflow definition");
    } finally {
      setValidating(false);
    }
  }

  async function publishOnlineVersion() {
    if (!canPublish) return;
    setPublishing(true);
    try {
      await api.publishWorkflowCaseDefinition(caseId);
      await Promise.all([
        qc.invalidateQueries({ queryKey: workflowRunKeys.caseDetail(wsId, caseId) }),
        qc.invalidateQueries({ queryKey: workflowRunKeys.caseDefinitionVersions(wsId, caseId) }),
      ]);
      toast.success("Published online version");
    } catch (err) {
      toast.error(err instanceof Error ? err.message : "Failed to publish online version");
    } finally {
      setPublishing(false);
    }
  }

  async function discardDraft() {
    if (!definition) return;
    setEditingDraft(true);
  }

  async function startRun() {
    if (!canStart) return;
    setStarting(true);
    try {
      const run = await api.startWorkflowCaseRun(caseId, {
        run_kind: runKind,
        label: runLabel.trim() || undefined,
        initial_state: {},
      });
      qc.setQueryData(workflowRunKeys.caseRuns(wsId, caseId), (current: WorkflowRun[] | undefined) => {
        const existing = current ?? [];
        if (existing.some((item) => item.id === run.id)) return existing;
        return [run, ...existing];
      });
      setSelectedRunId(run.id);
      setRunLabel("");
      await qc.invalidateQueries({ queryKey: workflowRunKeys.caseRuns(wsId, caseId) });
      toast.success("Workflow run started");
    } catch (err) {
      toast.error(err instanceof Error ? err.message : "Failed to start workflow run");
    } finally {
      setStarting(false);
    }
  }

  async function cancelRun(runId: string) {
    setCancellingRunId(runId);
    try {
      const cancelled = await api.cancelWorkflowCaseRun(caseId, runId);
      qc.setQueryData(workflowRunKeys.caseRuns(wsId, caseId), (current: WorkflowRun[] | undefined) => {
        const existing = current ?? [];
        if (existing.some((item) => item.id === cancelled.id)) {
          return existing.map((item) => (item.id === cancelled.id ? cancelled : item));
        }
        return [cancelled, ...existing];
      });
      await qc.invalidateQueries({ queryKey: workflowRunKeys.caseRuns(wsId, caseId) });
      toast.success("Workflow run cancelled");
    } catch (err) {
      toast.error(err instanceof Error ? err.message : "Failed to cancel workflow run");
    } finally {
      setCancellingRunId(null);
    }
  }

  async function deleteCase() {
    setDeleting(true);
    try {
      await api.deleteWorkflowCase(caseId);
      await qc.invalidateQueries({ queryKey: workflowRunKeys.caseList(wsId) });
      toast.success("Workflow case deleted");
      setDeleteOpen(false);
      navigation.push(paths.workflowCases());
    } catch (err) {
      toast.error(err instanceof Error ? err.message : "Failed to delete workflow case");
      setDeleting(false);
    }
  }

  if (caseLoading) {
    return <WorkflowCaseDetailSkeleton />;
  }

  if (!workflowCase) {
    return (
      <div className="flex h-full flex-col">
        <PageHeader className="px-5">
          <AppLink href={paths.workflowCases()} className="flex items-center gap-1 text-sm text-muted-foreground hover:text-foreground">
            <ArrowLeft className="size-4" />
            Workflow cases
          </AppLink>
        </PageHeader>
        <div className="flex flex-1 items-center justify-center text-sm text-muted-foreground">Workflow case not found.</div>
      </div>
    );
  }

  return (
    <div className="flex h-full min-h-0 flex-col">
      <PageHeader className="justify-between px-5">
        <div className="flex min-w-0 items-center gap-2">
          <AppLink
            href={paths.workflowCases()}
            className="rounded-md p-1 text-muted-foreground hover:bg-accent hover:text-foreground"
            aria-label="Back to workflow cases"
          >
            <ArrowLeft className="size-4" />
          </AppLink>
          <FolderGit2 className="size-4 shrink-0 text-muted-foreground" />
          <h1 className="truncate text-sm font-medium">{workflowCase.title}</h1>
          <span className="rounded bg-muted px-1.5 py-0.5 text-xs text-muted-foreground">{workflowCase.status}</span>
        </div>
        <Button
          type="button"
          size="sm"
          variant="outline"
          className="shrink-0 text-red-600 hover:text-red-700"
          onClick={() => setDeleteOpen(true)}
        >
          <Trash2 className="mr-1 size-3.5" />
          Delete case
        </Button>
      </PageHeader>

      <div className="grid min-h-0 flex-1 gap-4 overflow-y-auto p-5 xl:grid-cols-[minmax(0,1fr)_320px]">
        <main className="min-w-0 space-y-4">
          <section className="rounded-lg border bg-card p-4">
            <div className="grid gap-3 text-sm md:grid-cols-3 lg:grid-cols-6">
              <Property label="Status" value={workflowCase.status} />
              <Property
                label="Entry issue"
                value={entryIssueId ? undefined : "-"}
              >
                {entryIssueId && (
                  <AppLink href={paths.issueDetail(entryIssueId)} className="font-mono text-xs underline underline-offset-2">
                    {entryIssueId}
                  </AppLink>
                )}
              </Property>
              <Property label="Online version" value={onlineVersionId ? shortId(onlineVersionId) : "-"} />
              <Property label="Active runs" value={String(activeRunCount)} />
              <Property label="Versions" value={String(definitionVersions.length)} />
              <Property label="Updated" value={formatDateTime(workflowCase.updated_at)} />
            </div>
            {workflowCase.description && <p className="mt-3 text-sm text-muted-foreground">{workflowCase.description}</p>}
          </section>

          {/* Draft Workspace */}
          <section className="space-y-3">
            <div className="flex items-center justify-between gap-3">
              <h2 className="text-sm font-medium">Draft workspace</h2>
              <div className="flex items-center gap-2">
                <Button type="button" size="sm" variant="outline" onClick={() => setEditingDraft(true)}>
                  {definition ? "Edit draft" : "Create starter draft"}
                </Button>
                <Button type="button" size="sm" variant="outline" onClick={() => void validateDefinition()} disabled={!definition || validating || editingDraft}>
                  {validating ? "Validating..." : "Validate draft"}
                </Button>
                <Button type="button" size="sm" onClick={() => void publishOnlineVersion()} disabled={!canPublish || publishing}>
                  <Rocket className="mr-1 size-3.5" />
                  {publishing ? "Publishing..." : "Publish online version"}
                </Button>
                <Button type="button" size="sm" variant="ghost" onClick={() => void discardDraft()} disabled={!definition || editingDraft}>
                  <Trash2 className="mr-1 size-3.5" />
                  Discard draft
                </Button>
              </div>
            </div>
            {editingDraft ? (
              <WorkflowCaseDraftEditor
                caseId={caseId}
                definition={definition}
                onCancel={() => setEditingDraft(false)}
                onSaved={(saved) => {
                  qc.setQueryData(workflowRunKeys.caseDefinition(wsId, caseId), saved);
                  setValidation(null);
                  setEditingDraft(false);
                }}
              />
            ) : definitionLoading ? (
              <Skeleton className="h-[360px] rounded-lg" />
            ) : definition ? (
              <WorkflowCanvas definition={definition.draft_json} fullscreenTitle="Workflow definition draft" />
            ) : (
              <div className="rounded-lg border border-dashed bg-background/60 p-4">
                <p className="text-sm text-muted-foreground">No definition draft is available for this workflow case.</p>
                <Button type="button" size="sm" variant="outline" className="mt-3" onClick={() => setEditingDraft(true)}>
                  Create starter draft
                </Button>
              </div>
            )}
          </section>

          <ValidationReportPanel report={validation} />

          {/* Versions Workspace */}
          <VersionsWorkspace versions={definitionVersions} onlineVersionId={onlineVersionId} />

          {/* Runs Workspace: selected run canvas */}
          <section className="space-y-3">
            <h2 className="text-sm font-medium">Selected run canvas</h2>
            {runsLoading ? (
              <Skeleton className="h-[360px] rounded-lg" />
            ) : selectedRun ? (
              <WorkflowCanvas
                definition={selectedRun.definition_snapshot}
                runState={selectedRun.nodes_state}
                runStatus={selectedRun.status}
                selectedNodeId={selectedNodeId}
                onSelectNode={setSelectedNodeId}
                onPaneClick={() => setSelectedNodeId(null)}
                hideSelectionOverlay
                fullscreenTitle="Workflow run"
              />
            ) : (
              <div className="rounded-lg border border-dashed bg-background/60 p-4 text-sm text-muted-foreground">
                Start a run from the online version to see its canvas.
              </div>
            )}
          </section>
        </main>

        <aside className="space-y-4">
          {/* Runs Workspace: start form + run list */}
          <section className="space-y-3 rounded-lg border bg-card p-3">
            <h2 className="text-sm font-medium">Start run</h2>
            <div className="flex flex-wrap items-center gap-2">
              <select
                aria-label="Run kind"
                className="h-8 rounded-md border bg-background px-2 text-sm"
                value={runKind}
                onChange={(event) => setRunKind(event.target.value as WorkflowRunKind)}
              >
                {RUN_KINDS.map((kind) => (
                  <option key={kind} value={kind}>{kind}</option>
                ))}
              </select>
              <Input
                aria-label="Run label"
                className="h-8 flex-1"
                placeholder="Label (optional)"
                value={runLabel}
                onChange={(event) => setRunLabel(event.target.value)}
              />
            </div>
            <Button type="button" size="sm" className="w-full" onClick={() => void startRun()} disabled={!canStart || starting}>
              <Play className="mr-1 size-3.5" />
              {starting ? "Starting..." : "Start run"}
            </Button>
            {!hasOnlineVersion && (
              <p className="text-xs text-muted-foreground">Publish an online version before starting a run.</p>
            )}
          </section>
          <WorkflowCaseRunList
            runs={runs}
            selectedRunId={selectedRun?.id}
            onSelectRun={setSelectedRunId}
            onCancelRun={(runId) => void cancelRun(runId)}
            cancellingRunId={cancellingRunId}
            activeStatuses={ACTIVE_RUN_STATUSES}
            openRunHref={(runId) => paths.workflowCaseRunDetail(caseId, runId)}
          />
          <WorkflowRunNodeDetailPanel node={selectedRunNode} issueHref={(issueId) => paths.issueDetail(issueId)} />
        </aside>
      </div>

      <AlertDialog open={deleteOpen} onOpenChange={(open) => { if (!open && !deleting) setDeleteOpen(false); }}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>Delete workflow case</AlertDialogTitle>
            <AlertDialogDescription>
              This permanently deletes the case together with its draft, all versions, and all runs and their node history. Carrier sub-issues are kept but detached from this workflow.
              <span className="mt-2 block text-xs text-muted-foreground/80">This action cannot be undone.</span>
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel disabled={deleting}>Cancel</AlertDialogCancel>
            <AlertDialogAction
              onClick={(event) => { event.preventDefault(); void deleteCase(); }}
              disabled={deleting}
              className="bg-destructive text-white hover:bg-destructive/90"
            >
              {deleting ? "Deleting..." : "Delete case"}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </div>
  );
}

function VersionsWorkspace({
  versions,
  onlineVersionId,
}: {
  versions: WorkflowDefinitionVersion[];
  onlineVersionId: string | null;
}) {
  if (versions.length === 0) {
    return (
      <section className="rounded-lg border border-dashed bg-background/60 p-4 text-sm text-muted-foreground">
        No published versions yet. Publish the draft to create the first online version.
      </section>
    );
  }
  const sorted = [...versions].sort((a, b) => b.version - a.version);
  return (
    <section className="overflow-hidden rounded-lg border bg-card">
      <div className="border-b px-3 py-2">
        <h2 className="text-sm font-medium">Versions</h2>
      </div>
      <ul className="divide-y">
        {sorted.map((version) => {
          const online = version.id === onlineVersionId;
          return (
            <li key={version.id} className="flex items-center justify-between gap-3 px-3 py-2">
              <div className="min-w-0">
                <div className="flex items-center gap-2 text-sm">
                  <span className="font-medium">v{version.version}</span>
                  <span
                    className={cn(
                      "rounded px-1.5 py-0.5 text-xs",
                      online ? "bg-green-100 text-green-800" : "bg-muted text-muted-foreground",
                    )}
                  >
                    {online ? "Online" : "Historical"}
                  </span>
                </div>
                <div className="mt-1 text-xs text-muted-foreground">
                  {formatDateTime(version.created_at)}
                  {version.validation_report?.valid === false ? " · validation issues" : ""}
                </div>
              </div>
            </li>
          );
        })}
      </ul>
    </section>
  );
}

function ValidationReportPanel({ report }: { report: WorkflowValidationReport | null }) {
  if (!report) {
    return (
      <section className="rounded-lg border border-dashed bg-background/60 p-4 text-sm text-muted-foreground">
        Validation has not been run for this draft.
      </section>
    );
  }

  const issues = [...report.errors, ...report.warnings];
  return (
    <section className="rounded-lg border bg-card p-4">
      <div className="flex items-center gap-2">
        {report.valid ? <CheckCircle2 className="size-4 text-green-600" /> : <CircleAlert className="size-4 text-red-600" />}
        <h2 className="text-sm font-medium">{report.valid ? "Validation passed" : "Validation failed"}</h2>
      </div>
      {issues.length > 0 && (
        <ul className="mt-3 space-y-2">
          {issues.map((issue, index) => (
            <li key={`${issue.code}-${issue.path}-${index}`} className="rounded-md bg-muted/45 p-2 text-xs">
              <div className="font-medium">{issue.code}</div>
              <div className="mt-1 text-muted-foreground">{issue.message}</div>
              <div className="mt-1 font-mono text-[11px] text-muted-foreground">{issue.path}</div>
            </li>
          ))}
        </ul>
      )}
    </section>
  );
}

function Property({ label, value, children }: { label: string; value?: string; children?: React.ReactNode }) {
  return (
    <div className="min-w-0">
      <div className="text-[11px] uppercase text-muted-foreground">{label}</div>
      <div className="mt-1 truncate text-sm">{children ?? value}</div>
    </div>
  );
}

function WorkflowCaseDetailSkeleton() {
  return (
    <div className="flex h-full flex-col">
      <PageHeader className="justify-between px-5">
        <Skeleton className="h-5 w-56" />
        <Skeleton className="h-8 w-32" />
      </PageHeader>
      <div className="grid flex-1 gap-4 p-5 xl:grid-cols-[minmax(0,1fr)_320px]">
        <div className="space-y-4">
          <Skeleton className="h-24 rounded-lg" />
          <Skeleton className="h-[360px] rounded-lg" />
          <Skeleton className="h-24 rounded-lg" />
        </div>
        <div className="space-y-4">
          <Skeleton className="h-48 rounded-lg" />
          <Skeleton className="h-80 rounded-lg" />
        </div>
      </div>
    </div>
  );
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
