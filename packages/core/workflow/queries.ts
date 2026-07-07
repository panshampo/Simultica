import { queryOptions } from "@tanstack/react-query";
import { api } from "../api";
export const workflowRunKeys = {
  all: ["workflow-run"] as const,
  caseList: (wsId: string) => ["workflow-cases", wsId] as const,
  caseDetail: (wsId: string, caseId: string) => ["workflow-case", wsId, caseId] as const,
  caseDefinition: (wsId: string, caseId: string) => ["workflow-case-definition", wsId, caseId] as const,
  caseDefinitionVersions: (wsId: string, caseId: string) => ["workflow-case-definition-versions", wsId, caseId] as const,
  caseRuns: (wsId: string, caseId: string) => ["workflow-case-runs", wsId, caseId] as const,
  issueContext: (wsId: string, issueId: string) => ["workflow-context", wsId, issueId] as const,
  caseCurrentRun: (wsId: string, caseId: string) => ["workflow-case-current-run", wsId, caseId] as const,
};

export function workflowCaseListOptions(wsId: string) {
  return queryOptions({
    queryKey: workflowRunKeys.caseList(wsId),
    queryFn: () => api.listWorkflowCases(),
    enabled: !!wsId,
  });
}

export function workflowCaseDetailOptions(wsId: string, caseId: string) {
  return queryOptions({
    queryKey: workflowRunKeys.caseDetail(wsId, caseId),
    queryFn: () => api.getWorkflowCase(caseId),
    enabled: !!wsId && !!caseId,
  });
}

export function workflowCaseDefinitionOptions(wsId: string, caseId: string) {
  return queryOptions({
    queryKey: workflowRunKeys.caseDefinition(wsId, caseId),
    queryFn: () => api.getWorkflowCaseDefinition(caseId),
    enabled: !!wsId && !!caseId,
  });
}

export function workflowCaseDefinitionVersionsOptions(wsId: string, caseId: string) {
  return queryOptions({
    queryKey: workflowRunKeys.caseDefinitionVersions(wsId, caseId),
    queryFn: () => api.listWorkflowCaseDefinitionVersions(caseId),
    enabled: !!wsId && !!caseId,
  });
}

export function workflowCaseRunsOptions(wsId: string, caseId: string) {
  return queryOptions({
    queryKey: workflowRunKeys.caseRuns(wsId, caseId),
    queryFn: () => api.listWorkflowCaseRuns(caseId),
    enabled: !!wsId && !!caseId,
  });
}

export function issueWorkflowContextOptions(wsId: string, issueId: string) {
  return queryOptions({
    queryKey: workflowRunKeys.issueContext(wsId, issueId),
    queryFn: () => api.getIssueWorkflowContext(issueId),
    enabled: !!wsId && !!issueId,
  });
}

export function workflowCaseCurrentRunOptions(wsId: string, caseId: string) {
  return queryOptions({
    queryKey: workflowRunKeys.caseCurrentRun(wsId, caseId),
    queryFn: () => api.getWorkflowCaseCurrentRun(caseId),
    enabled: !!wsId && !!caseId,
  });
}
