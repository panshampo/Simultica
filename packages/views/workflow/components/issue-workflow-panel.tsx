"use client";

import { useQuery, useQueryClient } from "@tanstack/react-query";
import { useState } from "react";
import { ChevronRight } from "lucide-react";
import { issueWorkflowRunOptions, workflowRunKeys } from "@multica/core/workflow/queries";
import type { WorkflowRun } from "@multica/core/workflow/types";
import { api } from "@multica/core/api";
import { Button } from "@multica/ui/components/ui/button";
import { useWSEvent } from "@multica/core/realtime";
import { useWorkspacePaths } from "@multica/core/paths";
import { AppLink } from "../../navigation";
import { useNavigation } from "../../navigation";
import { WorkflowCanvas } from "./workflow-canvas";

const ACTIVE_RUN_STATUSES = new Set<WorkflowRun["status"]>(["planning", "running", "finalizing"]);

export function IssueWorkflowPanel({ wsId, issueId }: { wsId: string; issueId: string }) {
  const qc = useQueryClient();
  const paths = useWorkspacePaths();
  const navigation = useNavigation();
  const [stopping, setStopping] = useState(false);
  const [continuing, setContinuing] = useState(false);
  const [showNodeStatus, setShowNodeStatus] = useState(false);
  const { data: run, isLoading } = useQuery(issueWorkflowRunOptions(wsId, issueId));
  const canStop = Boolean(run && ACTIVE_RUN_STATUSES.has(run.status));
  const canContinue = Boolean(run && workflowNeedsContinuation(run));

  useWSEvent("workflow_run:updated", (payload: unknown) => {
    const incoming = (payload as { workflow_run?: WorkflowRun } | null)?.workflow_run;
    if (incoming?.root_issue_id === issueId) {
      qc.setQueryData(workflowRunKeys.byIssue(wsId, issueId), incoming);
    }
  });

  const openSubIssue = (subIssueId: string) => {
    navigation.push(paths.issueDetail(subIssueId));
  };

  async function stopWorkflow() {
    if (!run || !canStop) return;
    setStopping(true);
    try {
      const cancelled = await api.cancelWorkflowRun(run.id);
      qc.setQueryData(workflowRunKeys.byIssue(wsId, issueId), cancelled);
      await qc.invalidateQueries({ queryKey: workflowRunKeys.byIssue(wsId, issueId) });
    } finally {
      setStopping(false);
    }
  }

  async function continueWorkflow() {
    if (!run || !canContinue) return;
    setContinuing(true);
    try {
      const continued = await api.continueWorkflowRun(run.id, "continue workflow after human approval");
      qc.setQueryData(workflowRunKeys.byIssue(wsId, issueId), continued);
      await qc.invalidateQueries({ queryKey: workflowRunKeys.byIssue(wsId, issueId) });
    } finally {
      setContinuing(false);
    }
  }

  return (
    <section className="mt-8 rounded-lg border border-border/80 bg-card/45 p-3 shadow-[0_1px_1px_rgba(15,23,42,0.03)]">
      <div className="mb-3 flex items-start justify-between gap-3 px-0.5">
        <div className="min-w-0">
          <h2 className="text-sm font-semibold">Runtime workflow</h2>
          {!run && (
            <div className="mt-1 flex flex-wrap items-center gap-2 text-xs text-muted-foreground">
              <span className="rounded bg-muted px-1.5 py-0.5">Native issue mode</span>
              <span>Waiting for the main agent to plan this issue.</span>
            </div>
          )}
        </div>
        <div className="flex shrink-0 items-center gap-2">
          {canStop && (
            <Button type="button" size="sm" variant="destructive" onClick={() => void stopWorkflow()} disabled={stopping}>
              {stopping ? "Stopping..." : "Stop workflow"}
            </Button>
          )}
          {canContinue && (
            <Button type="button" size="sm" variant="secondary" onClick={() => void continueWorkflow()} disabled={continuing}>
              {continuing ? "Continuing..." : "Continue workflow"}
            </Button>
          )}
        </div>
      </div>
      {isLoading ? (
        <div className="rounded-md border border-dashed bg-background/60 p-4 text-sm text-muted-foreground">Loading workflow...</div>
      ) : run ? (
        <>
          {run.status === "cancelled" && (
            <div className="mb-3 rounded-lg border border-slate-300/80 bg-slate-50/90 px-3 py-2.5 text-sm text-slate-700">
              <div className="font-medium text-slate-900">Workflow stopped</div>
              <p className="mt-1 text-xs">
                Active child tasks were cancelled. Child issues remain as records and are not actionable unless a new workflow is planned.
              </p>
            </div>
          )}
          <WorkflowCanvas
            definition={run.definition_snapshot}
            runState={run.nodes_state}
            runStatus={run.status}
            onOpenSubIssue={openSubIssue}
            fullscreenTitle="Runtime workflow"
          />
          <div className="mt-3 overflow-hidden rounded-lg border border-border/80 bg-background/70">
            <button
              type="button"
              className="flex w-full items-center gap-2 px-3 py-2.5 text-left text-xs font-medium text-muted-foreground transition-colors hover:bg-muted/40"
              onClick={() => setShowNodeStatus((value) => !value)}
            >
              <ChevronRight className={`size-3 transition-transform ${showNodeStatus ? "rotate-90" : ""}`} />
              <span>Node and child issue status</span>
              <span className="ml-auto rounded bg-muted px-1.5 py-0.5 font-mono tabular-nums">{run.definition_snapshot.nodes.length}</span>
            </button>
            {showNodeStatus && (
              <ul className="divide-y divide-border/60 border-t text-sm">
                {run.definition_snapshot.nodes.map((node) => {
                  const state = run.nodes_state[node.id];
                  const explanations = nodeExecutionExplanation(run.status, state);
                  return (
                    <li key={node.id} className="grid gap-2 px-3 py-2.5 md:grid-cols-[minmax(9rem,0.9fr)_minmax(0,2fr)]">
                      <div className="min-w-0">
                        <div className="truncate font-mono text-xs font-medium">{node.id}</div>
                        <div className="mt-1 flex flex-wrap items-center gap-1.5">
                          <span className={statusClass(state?.status ?? "pending")}>{state?.status ?? "pending"}</span>
                          <span className="rounded bg-muted px-1.5 py-0.5 text-xs text-muted-foreground">
                            {nodeCarrierLabel(node)}
                          </span>
                        </div>
                      </div>
                      <div className="flex min-w-0 flex-wrap items-center gap-2">
                        {(state?.source_skill_name ?? node.source_skill_name) && (
                          <span className="truncate text-xs text-muted-foreground">
                            {state?.source_skill_name ?? node.source_skill_name}
                            {(state?.source_node_id ?? node.source_node_id) ? `/${state?.source_node_id ?? node.source_node_id}` : ""}
                          </span>
                        )}
                        {state?.sub_issue_id && (
                          <AppLink className="text-xs underline underline-offset-2" href={paths.issueDetail(state.sub_issue_id)}>
                            sub-issue
                          </AppLink>
                        )}
                        {state?.traex_session_id && <span className="text-xs text-muted-foreground">TraeX session {shortId(state.traex_session_id)}</span>}
                        {state?.main_issue_task_id && <span className="text-xs text-muted-foreground">Main task {shortId(state.main_issue_task_id)}</span>}
                        {state?.route_decision && (
                          <>
                            <span className="text-xs text-muted-foreground">Route {state.route_decision.selected_route}</span>
                            <span className={state.route_decision.condition_result ? "text-xs text-green-700" : "text-xs text-slate-600"}>
                              Condition {String(state.route_decision.condition_result)}
                            </span>
                            <code className="max-w-full truncate rounded bg-muted px-1 py-0.5 text-[11px] text-muted-foreground">
                              {state.route_decision.condition}
                            </code>
                          </>
                        )}
                        {explanations.map((explanation) => (
                          <span key={explanation} className="text-xs text-muted-foreground">{explanation}</span>
                        ))}
                        {state?.error && <span className="text-xs text-destructive">{state.error}</span>}
                      </div>
                    </li>
                  );
                })}
              </ul>
            )}
          </div>
        </>
      ) : (
        <div className="rounded-lg border border-dashed bg-background/50 p-4 text-sm text-muted-foreground">
          Runtime workflow will appear here after the main agent submits a plan for this issue.
        </div>
      )}
    </section>
  );
}

function statusClass(status: string): string {
  const base = "rounded px-1.5 py-0.5 text-xs";
  switch (status) {
    case "running":
      return `${base} bg-amber-100 text-amber-800`;
    case "planning":
      return `${base} bg-blue-100 text-blue-800`;
    case "finalizing":
      return `${base} bg-indigo-100 text-indigo-800`;
    case "done":
      return `${base} bg-green-100 text-green-800`;
    case "blocked":
      return `${base} bg-orange-100 text-orange-800`;
    case "failed":
      return `${base} bg-red-100 text-red-800`;
    case "cancelling":
      return `${base} bg-orange-100 text-orange-800`;
    case "cancelled":
      return `${base} bg-slate-100 text-slate-700`;
    default:
      return `${base} bg-muted text-muted-foreground`;
  }
}

function shortId(id: string): string {
  return id.length > 8 ? id.slice(0, 8) : id;
}

function nodeCarrierLabel(node: WorkflowRun["definition_snapshot"]["nodes"][number]): string {
  const dispatch = node.dispatch ?? inferDispatch(node.type);
  switch (dispatch) {
    case "subissue":
      return "agent · subissue";
    case "direct_subagent":
      return "agent · direct";
    case "main_issue_task":
      return "main agent · main issue";
    case "inline":
      return `${node.type} · inline`;
  }
}

function inferDispatch(type: string): NonNullable<WorkflowRun["definition_snapshot"]["nodes"][number]["dispatch"]> {
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

function nodeExecutionExplanation(runStatus: WorkflowRun["status"], state: WorkflowRun["nodes_state"][string] | undefined): string[] {
  if (runStatus !== "cancelled") return [];
  if (state?.status === "done") return ["Completed before stop"];
  if (state?.status === "cancelled" && state.sub_issue_id) return ["Child task cancelled", "Child issue kept as record"];
  if (state?.status === "cancelled") return ["Cancelled"];
  if (!state || state.status === "pending") return ["Skipped after stop"];
  return [];
}

function workflowNeedsContinuation(run: WorkflowRun): boolean {
  const serialized = JSON.stringify(run.nodes_state ?? {});
  return serialized.includes("budget_exhausted") || serialized.includes("fixable_needs_decision");
}
