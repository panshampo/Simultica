"use client";

import { useQuery, useQueryClient } from "@tanstack/react-query";
import { useState } from "react";
import { issueWorkflowRunOptions, workflowRunKeys } from "@multica/core/workflow/queries";
import type { WorkflowRun } from "@multica/core/workflow/types";
import { skillListOptions } from "@multica/core/workspace/queries";
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
  const [skillId, setSkillId] = useState("");
  const [starting, setStarting] = useState(false);
  const [stopping, setStopping] = useState(false);
  const [continuing, setContinuing] = useState(false);
  const [showCanvas, setShowCanvas] = useState(false);
  const { data: run, isLoading } = useQuery(issueWorkflowRunOptions(wsId, issueId));
  const { data: skills = [] } = useQuery(skillListOptions(wsId));
  const workflowSkills = skills.filter((skill) => skill.config?.has_workflow === true);
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

  async function startWorkflow() {
    if (!skillId) return;
    setStarting(true);
    try {
      await api.startIssueWorkflowRun(issueId, { skill_id: skillId });
      await qc.invalidateQueries({ queryKey: workflowRunKeys.byIssue(wsId, issueId) });
    } finally {
      setStarting(false);
    }
  }

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
    <section className="mt-8 rounded-md border bg-card/30 p-3">
      <div className="mb-3 flex items-center justify-between gap-3">
        <div className="min-w-0">
          <h2 className="text-sm font-semibold">Runtime workflow</h2>
          {run ? (
            <div className="mt-1 flex flex-wrap items-center gap-x-3 gap-y-1 text-xs text-muted-foreground">
              <span className="rounded bg-blue-50 px-1.5 py-0.5 text-blue-700">Workflow managed</span>
              <span className={statusClass(run.status)}>{statusLabel(run.status)}</span>
              {run.current_node && <span>Node {run.current_node}</span>}
              {run.planner_task_id && <span>Planner {shortId(run.planner_task_id)}</span>}
              {run.started_at && <span>Started {formatTimestamp(run.started_at)}</span>}
              {run.completed_at && <span>Completed {formatTimestamp(run.completed_at)}</span>}
              {run.cancelled_at && <span>Stopped {formatTimestamp(run.cancelled_at)}</span>}
            </div>
          ) : (
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
        <div className="text-sm text-muted-foreground">Loading workflow...</div>
      ) : run ? (
        <>
          {run.status === "cancelled" && (
            <div className="mb-3 rounded-md border border-slate-300 bg-slate-50 px-3 py-2 text-sm text-slate-700">
              <div className="font-medium text-slate-900">Workflow stopped</div>
              <p className="mt-1 text-xs">
                Active child tasks were cancelled. Child issues remain as records and are not actionable unless a new workflow is planned.
              </p>
            </div>
          )}
          {(run.source_skills?.length || run.definition_snapshot.source_skills?.length || run.error || run.cancel_reason) && (
            <div className="mb-3 flex flex-wrap items-center gap-x-3 gap-y-1 text-xs text-muted-foreground">
              {sourceSkills(run).map((skill) => (
                <span key={`${skill.id ?? "skill"}-${skill.name ?? "unknown"}`}>
                  Source {skill.name ?? skill.id ?? "unknown skill"}
                </span>
              ))}
              {run.error && <span className="text-destructive">{run.error}</span>}
              {run.cancel_reason && <span>Reason {run.cancel_reason}</span>}
            </div>
          )}
          <div className="mb-3 rounded-md border bg-background/70 p-2">
            <div className="flex items-center justify-between gap-3">
              <div className="min-w-0">
                <div className="text-xs font-medium text-muted-foreground">Workflow graph</div>
                <div className="text-xs text-muted-foreground">
                  {run.definition_snapshot.nodes.length} nodes · {run.definition_snapshot.routing.length} routes
                </div>
              </div>
              <Button type="button" size="xs" variant="secondary" onClick={() => setShowCanvas((value) => !value)}>
                {showCanvas ? "Hide graph" : "Show graph"}
              </Button>
            </div>
          </div>
          {showCanvas && (
            <WorkflowCanvas
              definition={run.definition_snapshot}
              runState={run.nodes_state}
              runStatus={run.status}
              onOpenSubIssue={openSubIssue}
              fullscreenTitle="Runtime workflow"
            />
          )}
          <ul className="mt-3 space-y-1 text-sm">
            {run.definition_snapshot.nodes.map((node) => {
              const state = run.nodes_state[node.id];
              const explanations = nodeExecutionExplanation(run.status, state);
              return (
                <li key={node.id} className="flex flex-wrap items-center gap-2">
                  <span className="font-mono text-xs">{node.id}</span>
                  <span className="rounded bg-muted px-1.5 py-0.5 text-xs text-muted-foreground">
                    {nodeCarrierLabel(node)}
                  </span>
                  <span className={statusClass(state?.status ?? "pending")}>{state?.status ?? "pending"}</span>
                  {(state?.source_skill_name ?? node.source_skill_name) && (
                    <span className="text-xs text-muted-foreground">
                      {state?.source_skill_name ?? node.source_skill_name}
                      {(state?.source_node_id ?? node.source_node_id) ? `/${state?.source_node_id ?? node.source_node_id}` : ""}
                    </span>
                  )}
                  {state?.sub_issue_id && (
                    <AppLink className="text-xs underline" href={paths.issueDetail(state.sub_issue_id)}>
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
                      <code className="rounded bg-muted px-1 py-0.5 text-[11px] text-muted-foreground">
                        {state.route_decision.condition}
                      </code>
                    </>
                  )}
                  {explanations.map((explanation) => (
                    <span key={explanation} className="text-xs text-muted-foreground">{explanation}</span>
                  ))}
                  {state?.error && <span className="text-xs text-destructive">{state.error}</span>}
                </li>
              );
            })}
          </ul>
        </>
      ) : (
        <div className="rounded-md border border-dashed p-4 text-sm text-muted-foreground">
          Runtime workflow will appear here after the main agent submits a plan for this issue.
        </div>
      )}
      <details className="mt-3 border-t pt-3">
        <summary className="cursor-pointer text-xs font-medium text-muted-foreground">Debug run skill workflow</summary>
        <div className="mt-2 flex flex-wrap items-center gap-2">
          <select
            className="h-8 rounded-md border bg-background px-2 text-xs"
            value={skillId}
            onChange={(e) => setSkillId(e.target.value)}
          >
            <option value="">Select workflow skill</option>
            {workflowSkills.map((skill) => (
              <option key={skill.id} value={skill.id}>{skill.name}</option>
            ))}
          </select>
          <Button type="button" size="xs" variant="secondary" onClick={startWorkflow} disabled={!skillId || starting}>
            {starting ? "Starting..." : "Debug run"}
          </Button>
        </div>
      </details>
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

function statusLabel(status: WorkflowRun["status"]): string {
  switch (status) {
    case "planning":
      return "Planning";
    case "running":
      return "Running";
    case "finalizing":
      return "Finalizing";
    case "done":
      return "Done";
    case "failed":
      return "Failed";
    case "cancelling":
      return "Cancelling";
    case "cancelled":
      return "Cancelled";
    case "pending":
      return "Pending";
  }
}

function formatTimestamp(value: string): string {
  const timestamp = new Date(value);
  if (Number.isNaN(timestamp.getTime())) return value;
  return timestamp.toLocaleString();
}

function shortId(id: string): string {
  return id.length > 8 ? id.slice(0, 8) : id;
}

function sourceSkills(run: WorkflowRun): Array<{ id?: string; name?: string }> {
  return run.source_skills?.length ? run.source_skills : run.definition_snapshot.source_skills ?? [];
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
