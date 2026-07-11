"use client";

import { useQuery, useQueryClient } from "@tanstack/react-query";
import { FolderGit2 } from "lucide-react";
import {
  issueWorkflowContextOptions,
  workflowCaseCurrentRunOptions,
  workflowRunKeys,
} from "@multica/core/workflow/queries";
import type { IssueWorkflowContext, WorkflowRun } from "@multica/core/workflow/types";
import { useWSEvent } from "@multica/core/realtime";
import { useWorkspacePaths } from "@multica/core/paths";
import { AppLink } from "../../navigation";
import { WorkflowRunDetailView } from "./workflow-run-detail-page";

const LOADING_CURRENT_RUN_COPY = "Loading current workflow run...";
const NO_CURRENT_RUN_COPY = "No current workflow run is available for this issue yet.";

export function IssueWorkflowPanel({ wsId, issueId }: { wsId: string; issueId: string }) {
  const qc = useQueryClient();
  const paths = useWorkspacePaths();
  const { data: context, isLoading } = useQuery(issueWorkflowContextOptions(wsId, issueId));
  const relatedCaseId = context?.workflow_case_id ?? null;
  const { data: currentRun, isLoading: isCurrentRunLoading } = useQuery(
    workflowCaseCurrentRunOptions(wsId, relatedCaseId ?? ""),
  );

  useWSEvent("workflow_run:updated", (payload: unknown) => {
    const incoming = (payload as { workflow_run?: WorkflowRun } | null)?.workflow_run;
    if (incoming?.root_issue_id === issueId || incoming?.case_id === relatedCaseId) {
      void qc.invalidateQueries({ queryKey: workflowRunKeys.issueContext(wsId, issueId) });
      if (incoming.case_id) void qc.invalidateQueries({ queryKey: workflowRunKeys.caseCurrentRun(wsId, incoming.case_id) });
    }
  });

  return (
    <section className="mt-8 rounded-lg border border-border/80 bg-card/45 p-3 shadow-[0_1px_1px_rgba(15,23,42,0.03)]">
      <div className="mb-3 flex items-start justify-between gap-3 px-0.5">
        <div className="min-w-0">
          <h2 className="text-sm font-semibold">Runtime workflow</h2>
          {(!context || context.role === "none") && (
            <div className="mt-1 flex flex-wrap items-center gap-2 text-xs text-muted-foreground">
              <span className="rounded bg-muted px-1.5 py-0.5">Native issue mode</span>
              <span>Open WorkflowCases to create managed workflow planning for this issue.</span>
            </div>
          )}
        </div>
      </div>
      <div className="mb-3 rounded-lg border bg-background/70 p-3">
        {isLoading ? (
          <div className="text-sm text-muted-foreground">Loading workflow context...</div>
        ) : relatedCaseId && context ? (
          <div className="flex flex-wrap items-center justify-between gap-3">
            <div className="min-w-0">
              <div className="flex items-center gap-2 text-sm font-medium">
                <FolderGit2 className="size-4 text-muted-foreground" />
                Managed by WorkflowCase
              </div>
              <div className="mt-2 flex flex-wrap items-center gap-2 text-xs text-muted-foreground">
                <span className="rounded bg-muted px-1.5 py-0.5">Role: {roleLabel(context.role)}</span>
                <span className="rounded bg-muted px-1.5 py-0.5">Carrier: {carrierKindLabel(context)}</span>
                {context.workflow_node_id && (
                  <span className="rounded bg-muted px-1.5 py-0.5 font-mono">Node {shortId(context.workflow_node_id)}</span>
                )}
                {context.workflow_run_id && (
                  <span className="rounded bg-muted px-1.5 py-0.5 font-mono">Run {shortId(context.workflow_run_id)}</span>
                )}
              </div>
              <div className="mt-1 truncate font-mono text-xs text-muted-foreground">{relatedCaseId}</div>
            </div>
            <div className="flex shrink-0 items-center gap-2">
              <AppLink
                href={paths.workflowCaseDetail(relatedCaseId)}
                className="inline-flex h-7 shrink-0 items-center justify-center rounded-[min(var(--radius-md),12px)] border border-border bg-background px-2.5 text-[0.8rem] font-medium hover:bg-muted hover:text-foreground"
              >
                Open case
              </AppLink>
            </div>
          </div>
        ) : (
          <div className="flex flex-wrap items-center justify-between gap-3">
            <div className="min-w-0">
              <div className="flex items-center gap-2 text-sm font-medium">
                <FolderGit2 className="size-4 text-muted-foreground" />
                WorkflowCase not created
              </div>
              <div className="mt-1 text-xs text-muted-foreground">
                Create or attach a WorkflowCase from the WorkflowCase control plane.
              </div>
            </div>
          </div>
        )}
      </div>
      {relatedCaseId && context && (
        <div>
          {isCurrentRunLoading ? (
            <div className="rounded-lg border border-dashed bg-background/60 p-4 text-sm text-muted-foreground">
              {LOADING_CURRENT_RUN_COPY}
            </div>
          ) : currentRun ? (
            <WorkflowRunDetailView
              caseId={relatedCaseId}
              runId={currentRun.id}
              initialNodeId={context.role === "node_issue" ? context.workflow_node_id ?? null : null}
            />
          ) : (
            <div className="rounded-lg border border-dashed bg-background/60 p-4 text-sm text-muted-foreground">
              {NO_CURRENT_RUN_COPY}
            </div>
          )}
        </div>
      )}
    </section>
  );
}

function shortId(id: string): string {
  return id.length > 8 ? id.slice(0, 8) : id;
}

function roleLabel(role: IssueWorkflowContext["role"]): string {
  switch (role) {
    case "entry_issue":
      return "Entry issue";
    case "node_issue":
      return "Node issue";
    case "none":
      return "None";
  }
}

function carrierKindLabel(context: IssueWorkflowContext): string {
  return context.carrier_kind || "issue";
}
