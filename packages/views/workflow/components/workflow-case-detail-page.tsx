"use client";

import { useEffect, useMemo, useState } from "react";
import { useQuery, useQueryClient, type QueryClient } from "@tanstack/react-query";
import { toast } from "sonner";
import { ArrowLeft, CheckCircle2, CircleAlert, FolderGit2, Play, Rocket } from "lucide-react";
import { api } from "@multica/core/api";
import { useWorkspaceId } from "@multica/core/hooks";
import { useWorkspacePaths } from "@multica/core/paths";
import {
  workflowCaseDefinitionOptions,
  workflowCaseDefinitionVersionsOptions,
  workflowCaseCurrentRunOptions,
  workflowCaseDetailOptions,
  workflowCaseRunsOptions,
  workflowRunKeys,
} from "@multica/core/workflow/queries";
import type {
  WorkflowCase,
  WorkflowDefinitionDraft,
  WorkflowDefinitionVersion,
  WorkflowRun,
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

const ACTIVE_RUN_STATUSES = new Set<WorkflowRun["status"]>(["pending", "planning", "running", "finalizing", "cancelling"]);

type WorkflowCaseTab = "overview" | "definition" | "runs";

const WORKFLOW_CASE_TABS: Array<{ id: WorkflowCaseTab; label: string }> = [
  { id: "overview", label: "Overview" },
  { id: "definition", label: "Definition" },
  { id: "runs", label: "Runs" },
];

export function WorkflowCaseDetailPage({
  caseId,
  initialRunId,
}: {
  caseId: string;
  initialRunId?: string | null;
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
  const [runLabel, setRunLabel] = useState("");
  const [selectedRunId, setSelectedRunId] = useState<string | null>(initialRunId ?? null);
  const [activeTab, setActiveTab] = useState<WorkflowCaseTab>("overview");
  const [caseTitle, setCaseTitle] = useState("");
  const [caseDescription, setCaseDescription] = useState("");
  const [caseOwnerAgentId, setCaseOwnerAgentId] = useState("");
  const [savingCase, setSavingCase] = useState(false);
  const [archiving, setArchiving] = useState(false);
  const { data: workflowCase, isLoading: caseLoading } = useQuery(workflowCaseDetailOptions(wsId, caseId));
  const { data: definition, isLoading: definitionLoading } = useQuery(workflowCaseDefinitionOptions(wsId, caseId));
  const { data: definitionVersions = [] } = useQuery(workflowCaseDefinitionVersionsOptions(wsId, caseId));
  const { data: runs = [] } = useQuery(workflowCaseRunsOptions(wsId, caseId));
  const { data: currentRunFromApi = null } = useQuery(workflowCaseCurrentRunOptions(wsId, caseId));

  const onlineVersionId = workflowCase?.online_version_id ?? null;
  const hasOnlineVersion = Boolean(onlineVersionId);
  const canPublish = Boolean(definition && validation?.valid && !editingDraft);
  const canStart = hasOnlineVersion && !editingDraft;
  const entryIssueId = workflowCase?.entry_issue_id ?? workflowCase?.source_issue_id ?? null;
  const runsWithCurrent = useMemo(
    () => mergeCurrentRun(runs, currentRunFromApi),
    [runs, currentRunFromApi],
  );
  const activeRunCount = runsWithCurrent.filter((run) => ACTIVE_RUN_STATUSES.has(run.status)).length;
  const currentRun = (workflowCase?.current_run_id
    ? runsWithCurrent.find((run) => run.id === workflowCase.current_run_id)
    : null) ?? currentRunFromApi ?? runsWithCurrent[0] ?? null;

  useEffect(() => {
    if (!selectedRunId && runsWithCurrent[0]) setSelectedRunId(runsWithCurrent[0].id);
  }, [runsWithCurrent, selectedRunId]);

  useEffect(() => {
    if (initialRunId) setActiveTab("runs");
  }, [initialRunId]);

  useEffect(() => {
    if (!workflowCase) return;
    setCaseTitle(workflowCase.title);
    setCaseDescription(workflowCase.description ?? "");
    setCaseOwnerAgentId(workflowCase.owner_agent_id ?? "");
  }, [workflowCase]);

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
        qc.invalidateQueries({ queryKey: workflowRunKeys.caseCurrentRun(wsId, caseId) }),
        qc.invalidateQueries({ queryKey: workflowRunKeys.caseDefinitionVersions(wsId, caseId) }),
      ]);
      toast.success("Published online version");
    } catch (err) {
      toast.error(err instanceof Error ? err.message : "Failed to publish online version");
    } finally {
      setPublishing(false);
    }
  }

  async function startRun() {
    if (!canStart) return;
    setStarting(true);
    try {
      const run = await api.startWorkflowCaseRun(caseId, {
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
      await qc.invalidateQueries({ queryKey: workflowRunKeys.caseCurrentRun(wsId, caseId) });
      toast.success("Workflow run created");
    } catch (err) {
      toast.error(err instanceof Error ? err.message : "Failed to create workflow run");
    } finally {
      setStarting(false);
    }
  }

  async function saveCaseSettings() {
    const title = caseTitle.trim();
    if (!title) {
      toast.error("Case title is required");
      return;
    }
    setSavingCase(true);
    try {
      const updated = await api.updateWorkflowCase(caseId, {
        title,
        description: caseDescription,
        owner_agent_id: caseOwnerAgentId.trim() || null,
      });
      qc.setQueryData(workflowRunKeys.caseDetail(wsId, caseId), updated);
      await qc.invalidateQueries({ queryKey: workflowRunKeys.caseList(wsId) });
      toast.success("Workflow case updated");
    } catch (err) {
      toast.error(err instanceof Error ? err.message : "Failed to update workflow case");
    } finally {
      setSavingCase(false);
    }
  }

  async function archiveCase() {
    setArchiving(true);
    try {
      const updated = await api.updateWorkflowCase(caseId, { status: "archived" });
      qc.setQueryData(workflowRunKeys.caseDetail(wsId, caseId), updated);
      await qc.invalidateQueries({ queryKey: workflowRunKeys.caseList(wsId) });
      toast.success("Workflow case archived");
    } catch (err) {
      toast.error(err instanceof Error ? err.message : "Failed to archive workflow case");
    } finally {
      setArchiving(false);
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
      </PageHeader>

      <WorkflowCaseTabBar activeTab={activeTab} onTabChange={setActiveTab} />

      <div className="min-h-0 flex-1 overflow-y-auto p-5">
        {activeTab === "overview" && (
          <WorkflowCaseOverview
            workflowCase={workflowCase}
            entryIssueId={entryIssueId}
            issueHref={(issueId) => paths.issueDetail(issueId)}
            onlineVersionId={onlineVersionId}
            currentRun={currentRun}
            currentRunHref={currentRun ? paths.workflowCaseRunDetail(caseId, currentRun.id) : null}
            activeRunCount={activeRunCount}
            versionCount={definitionVersions.length}
            caseTitle={caseTitle}
            setCaseTitle={setCaseTitle}
            caseDescription={caseDescription}
            setCaseDescription={setCaseDescription}
            caseOwnerAgentId={caseOwnerAgentId}
            setCaseOwnerAgentId={setCaseOwnerAgentId}
            savingCase={savingCase}
            saveCaseSettings={saveCaseSettings}
            archiving={archiving}
            archiveCase={archiveCase}
            versions={definitionVersions}
            versionHref={(versionId) => paths.workflowCaseVersionDetail(caseId, versionId)}
            setDeleteOpen={setDeleteOpen}
          />
        )}
        {activeTab === "definition" && (
          <WorkflowCaseDefinitionPanel
            caseId={caseId}
            wsId={wsId}
            qc={qc}
            definition={definition}
            definitionLoading={definitionLoading}
            editingDraft={editingDraft}
            setEditingDraft={setEditingDraft}
            validation={validation}
            setValidation={setValidation}
            validating={validating}
            validateDefinition={validateDefinition}
            publishing={publishing}
            publishOnlineVersion={publishOnlineVersion}
            canPublish={canPublish}
          />
        )}
        {activeTab === "runs" && (
          <WorkflowCaseRunsPanel
            runs={runs}
            currentRun={currentRun}
            versions={definitionVersions}
            selectedRunId={selectedRunId}
            runLabel={runLabel}
            setRunLabel={setRunLabel}
            canStart={canStart}
            starting={starting}
            startRun={startRun}
            hasOnlineVersion={hasOnlineVersion}
            setSelectedRunId={setSelectedRunId}
            cancelRun={cancelRun}
            cancellingRunId={cancellingRunId}
            runHref={(runId) => paths.workflowCaseRunDetail(caseId, runId)}
            versionHref={(versionId) => paths.workflowCaseVersionDetail(caseId, versionId)}
          />
        )}
      </div>

      <AlertDialog open={deleteOpen} onOpenChange={(open) => { if (!open && !deleting) setDeleteOpen(false); }}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>Delete permanently</AlertDialogTitle>
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
              {deleting ? "Deleting..." : "Delete permanently"}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </div>
  );
}

function WorkflowCaseTabBar({
  activeTab,
  onTabChange,
}: {
  activeTab: WorkflowCaseTab;
  onTabChange: (tab: WorkflowCaseTab) => void;
}) {
  return (
    <div role="tablist" aria-label="Workflow case sections" className="flex h-10 shrink-0 items-center gap-1 border-b px-5">
      {WORKFLOW_CASE_TABS.map((tab) => (
        <button
          key={tab.id}
          type="button"
          role="tab"
          aria-selected={activeTab === tab.id}
          className={cn(
            "h-8 rounded-md px-3 text-sm text-muted-foreground hover:bg-accent hover:text-foreground",
            activeTab === tab.id && "bg-accent text-foreground",
          )}
          onClick={() => onTabChange(tab.id)}
        >
          {tab.label}
        </button>
      ))}
    </div>
  );
}

function WorkflowCaseOverview({
  workflowCase,
  entryIssueId,
  issueHref,
  onlineVersionId,
  currentRun,
  currentRunHref,
  activeRunCount,
  versionCount,
  caseTitle,
  setCaseTitle,
  caseDescription,
  setCaseDescription,
  caseOwnerAgentId,
  setCaseOwnerAgentId,
  savingCase,
  saveCaseSettings,
  archiving,
  archiveCase,
  versions,
  versionHref,
  setDeleteOpen,
}: {
  workflowCase: WorkflowCase;
  entryIssueId: string | null;
  issueHref: (issueId: string) => string;
  onlineVersionId: string | null;
  currentRun: WorkflowRun | null;
  currentRunHref: string | null;
  activeRunCount: number;
  versionCount: number;
  caseTitle: string;
  setCaseTitle: (title: string) => void;
  caseDescription: string;
  setCaseDescription: (description: string) => void;
  caseOwnerAgentId: string;
  setCaseOwnerAgentId: (ownerAgentId: string) => void;
  savingCase: boolean;
  saveCaseSettings: () => Promise<void>;
  archiving: boolean;
  archiveCase: () => Promise<void>;
  versions: WorkflowDefinitionVersion[];
  versionHref: (versionId: string) => string;
  setDeleteOpen: (open: boolean) => void;
}) {
  return (
    <main className="max-w-5xl space-y-4">
      <section className="rounded-lg border bg-card p-4">
        <div className="grid gap-3 text-sm md:grid-cols-3 lg:grid-cols-6">
          <Property label="Status" value={workflowCase.status} />
          <Property label="Entry issue" value={entryIssueId ? undefined : "-"}>
            {entryIssueId && (
              <AppLink href={issueHref(entryIssueId)} className="font-mono text-xs underline underline-offset-2">
                {entryIssueId}
              </AppLink>
            )}
          </Property>
          <Property label="Online version" value={onlineVersionId ? shortId(onlineVersionId) : "-"} />
          <Property label="Active runs" value={String(activeRunCount)} />
          <Property label="Versions" value={String(versionCount)} />
          <Property label="Updated" value={formatDateTime(workflowCase.updated_at)} />
        </div>
        {workflowCase.description && <p className="mt-3 text-sm text-muted-foreground">{workflowCase.description}</p>}
      </section>

      <section className="rounded-lg border bg-card p-4">
        <div className="flex flex-wrap items-center justify-between gap-3">
          <div className="min-w-0">
            <h2 className="text-sm font-medium">Current run</h2>
            {currentRun ? (
              <div className="mt-2 flex flex-wrap items-center gap-2 text-xs text-muted-foreground">
                <span className={runStatusClass(currentRun.status)}>{currentRun.status}</span>
                <span className="font-mono">{shortId(currentRun.id)}</span>
                {currentRun.label && <span className="truncate">{currentRun.label}</span>}
                {currentRun.current_node && <span className="rounded bg-muted px-1.5 py-0.5 font-mono">{currentRun.current_node}</span>}
              </div>
            ) : (
              <p className="mt-1 text-xs text-muted-foreground">No run has been created yet.</p>
            )}
          </div>
          {currentRun && currentRunHref && (
            <AppLink
              href={currentRunHref}
              className="inline-flex h-8 shrink-0 items-center justify-center rounded-md border border-border bg-background px-3 text-xs font-medium hover:bg-muted hover:text-foreground"
            >
              Open current run
            </AppLink>
          )}
        </div>
      </section>

      <WorkflowCaseSettingsPanel
        caseTitle={caseTitle}
        setCaseTitle={setCaseTitle}
        caseDescription={caseDescription}
        setCaseDescription={setCaseDescription}
        caseOwnerAgentId={caseOwnerAgentId}
        setCaseOwnerAgentId={setCaseOwnerAgentId}
        savingCase={savingCase}
        saveCaseSettings={saveCaseSettings}
        setDeleteOpen={setDeleteOpen}
      />

      <WorkflowCaseLifecyclePanel
        versions={versions}
        onlineVersionId={onlineVersionId}
        workflowCaseStatus={workflowCase.status}
        archiving={archiving}
        archiveCase={archiveCase}
        versionHref={versionHref}
      />
    </main>
  );
}

function WorkflowCaseDefinitionPanel({
  caseId,
  wsId,
  qc,
  definition,
  definitionLoading,
  editingDraft,
  setEditingDraft,
  validation,
  setValidation,
  validating,
  validateDefinition,
  publishing,
  publishOnlineVersion,
  canPublish,
}: {
  caseId: string;
  wsId: string;
  qc: QueryClient;
  definition: WorkflowDefinitionDraft | null | undefined;
  definitionLoading: boolean;
  editingDraft: boolean;
  setEditingDraft: (editing: boolean) => void;
  validation: WorkflowValidationReport | null;
  setValidation: (report: WorkflowValidationReport | null) => void;
  validating: boolean;
  validateDefinition: () => Promise<void>;
  publishing: boolean;
  publishOnlineVersion: () => Promise<void>;
  canPublish: boolean;
}) {
  return (
    <main className="max-w-6xl space-y-4">
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
    </main>
  );
}

function WorkflowCaseRunsPanel({
  runs,
  currentRun,
  versions,
  selectedRunId,
  runLabel,
  setRunLabel,
  canStart,
  starting,
  startRun,
  hasOnlineVersion,
  setSelectedRunId,
  cancelRun,
  cancellingRunId,
  runHref,
  versionHref,
}: {
  runs: WorkflowRun[];
  currentRun: WorkflowRun | null;
  versions: WorkflowDefinitionVersion[];
  selectedRunId: string | null;
  runLabel: string;
  setRunLabel: (label: string) => void;
  canStart: boolean;
  starting: boolean;
  startRun: () => Promise<void>;
  hasOnlineVersion: boolean;
  setSelectedRunId: (runId: string) => void;
  cancelRun: (runId: string) => Promise<void>;
  cancellingRunId: string | null;
  runHref: (runId: string) => string;
  versionHref: (versionId: string) => string;
}) {
  const versionLabelById = Object.fromEntries(
    versions.map((version) => [version.id, `v${version.version}`]),
  );
  return (
    <div className="grid max-w-6xl gap-4 xl:grid-cols-[320px_minmax(0,1fr)]">
      <section className="h-fit space-y-3 rounded-lg border bg-card p-3">
        <h2 className="text-sm font-medium">Create run</h2>
        <Input
          aria-label="Run label"
          className="h-8"
          placeholder="Label (optional)"
          value={runLabel}
          onChange={(event) => setRunLabel(event.target.value)}
        />
        <Button type="button" size="sm" className="w-full" onClick={() => void startRun()} disabled={!canStart || starting}>
          <Play className="mr-1 size-3.5" />
          {starting ? "Creating..." : "Create run"}
        </Button>
        {!hasOnlineVersion && (
          <p className="text-xs text-muted-foreground">Publish an online version before creating a run.</p>
        )}
      </section>
      <WorkflowCaseRunList
        runs={mergeCurrentRun(runs, currentRun)}
        selectedRunId={selectedRunId}
        onSelectRun={setSelectedRunId}
        onCancelRun={(runId) => void cancelRun(runId)}
        cancellingRunId={cancellingRunId}
        activeStatuses={ACTIVE_RUN_STATUSES}
        openRunHref={(runId) => runHref(runId)}
        versionHref={versionHref}
        versionLabelById={versionLabelById}
      />
    </div>
  );
}

function mergeCurrentRun(runs: WorkflowRun[], currentRun: WorkflowRun | null | undefined): WorkflowRun[] {
  if (!currentRun) return runs;
  const rest = runs.filter((run) => run.id !== currentRun.id);
  return [currentRun, ...rest];
}

function WorkflowCaseSettingsPanel({
  caseTitle,
  setCaseTitle,
  caseDescription,
  setCaseDescription,
  caseOwnerAgentId,
  setCaseOwnerAgentId,
  savingCase,
  saveCaseSettings,
  setDeleteOpen,
}: {
  caseTitle: string;
  setCaseTitle: (title: string) => void;
  caseDescription: string;
  setCaseDescription: (description: string) => void;
  caseOwnerAgentId: string;
  setCaseOwnerAgentId: (ownerAgentId: string) => void;
  savingCase: boolean;
  saveCaseSettings: () => Promise<void>;
  setDeleteOpen: (open: boolean) => void;
}) {
  return (
    <div className="space-y-4">
      <section className="space-y-3 rounded-lg border bg-card p-4">
        <div>
          <h2 className="text-sm font-medium">Case metadata</h2>
          <p className="mt-1 text-xs text-muted-foreground">Update the human-facing name, description, and owning agent for this workflow case.</p>
        </div>
        <label className="block space-y-1">
          <span className="text-xs font-medium text-muted-foreground">Case title</span>
          <Input aria-label="Case title" value={caseTitle} onChange={(event) => setCaseTitle(event.target.value)} />
        </label>
        <label className="block space-y-1">
          <span className="text-xs font-medium text-muted-foreground">Owner agent ID</span>
          <Input aria-label="Owner agent ID" value={caseOwnerAgentId} onChange={(event) => setCaseOwnerAgentId(event.target.value)} />
        </label>
        <label className="block space-y-1">
          <span className="text-xs font-medium text-muted-foreground">Description</span>
          <textarea
            aria-label="Description"
            className="min-h-24 w-full rounded-lg border border-input bg-transparent px-2.5 py-2 text-sm outline-none placeholder:text-muted-foreground focus-visible:border-ring focus-visible:ring-3 focus-visible:ring-ring/50"
            value={caseDescription}
            onChange={(event) => setCaseDescription(event.target.value)}
          />
        </label>
        <Button type="button" size="sm" onClick={() => void saveCaseSettings()} disabled={savingCase}>
          {savingCase ? "Saving..." : "Save changes"}
        </Button>
      </section>

      <section className="rounded-lg border border-red-200 bg-card p-4">
        <h2 className="text-sm font-medium text-red-700">Danger zone</h2>
        <p className="mt-1 text-xs text-muted-foreground">
          Permanent deletion removes the case, draft, versions, runs, node history, and event history. Active runs must be cancelled first.
        </p>
        <Button type="button" size="sm" variant="outline" className="mt-3 text-red-600 hover:text-red-700" onClick={() => setDeleteOpen(true)}>
          Delete permanently
        </Button>
      </section>
    </div>
  );
}

function WorkflowCaseLifecyclePanel({
  versions,
  onlineVersionId,
  workflowCaseStatus,
  archiving,
  archiveCase,
  versionHref,
}: {
  versions: WorkflowDefinitionVersion[];
  onlineVersionId: string | null;
  workflowCaseStatus: WorkflowCase["status"];
  archiving: boolean;
  archiveCase: () => Promise<void>;
  versionHref: (versionId: string) => string;
}) {
  if (versions.length === 0) {
    return (
      <section className="space-y-3 rounded-lg border bg-card p-4">
        <div>
          <h2 className="text-sm font-medium">Lifecycle</h2>
          <p className="mt-1 text-xs text-muted-foreground">
            No published versions yet. Publish the draft to create the first online version.
          </p>
        </div>
        <Button type="button" size="sm" variant="outline" onClick={() => void archiveCase()} disabled={archiving || workflowCaseStatus === "archived"}>
          {archiving ? "Archiving..." : "Archive case"}
        </Button>
      </section>
    );
  }
  const sorted = [...versions].sort((a, b) => b.version - a.version);
  return (
    <section className="overflow-hidden rounded-lg border bg-card">
      <div className="flex items-start justify-between gap-3 border-b px-3 py-2">
        <div>
          <h2 className="text-sm font-medium">Lifecycle</h2>
          <p className="mt-1 text-xs text-muted-foreground">Published versions and archive controls.</p>
        </div>
        <Button type="button" size="sm" variant="outline" onClick={() => void archiveCase()} disabled={archiving || workflowCaseStatus === "archived"}>
          {archiving ? "Archiving..." : "Archive case"}
        </Button>
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
              <AppLink
                href={versionHref(version.id)}
                className="shrink-0 rounded-md px-2 py-1 text-xs text-muted-foreground hover:bg-accent hover:text-foreground"
              >
                Open v{version.version}
              </AppLink>
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
