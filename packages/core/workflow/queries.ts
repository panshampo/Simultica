import { queryOptions } from "@tanstack/react-query";
import { api } from "../api";
import type { WorkflowRun } from "./types";

export const workflowRunKeys = {
  all: ["workflow-run"] as const,
  byIssue: (wsId: string, issueId: string) => ["workflow-run", wsId, issueId] as const,
};

export function issueWorkflowRunOptions(wsId: string, issueId: string) {
  return queryOptions({
    queryKey: workflowRunKeys.byIssue(wsId, issueId),
    queryFn: (): Promise<WorkflowRun | null> => api.getIssueWorkflowRun(issueId),
    enabled: !!wsId && !!issueId,
  });
}
