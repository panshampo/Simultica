import { useMutation, useQueryClient } from "@tanstack/react-query";
import { api } from "../api";
import { useWorkspaceId } from "../hooks";
import { issueKeys } from "../issues/queries";
import type {
  CreateIssueTemplateRequest,
  IssueTemplate,
  ListIssueTemplatesResponse,
  UpdateIssueTemplateRequest,
} from "../types";
import { issueTemplateKeys } from "./queries";

export function useCreateIssueTemplate() {
  const qc = useQueryClient();
  const wsId = useWorkspaceId();
  return useMutation({
    mutationFn: (data: CreateIssueTemplateRequest) => api.createIssueTemplate(data),
    onSuccess: (newTemplate) => {
      qc.setQueriesData<ListIssueTemplatesResponse>(
        { queryKey: [...issueTemplateKeys.all(wsId), "list"] },
        (old) => old && !old.some((t) => t.id === newTemplate.id) ? [...old, newTemplate] : old,
      );
    },
    onSettled: () => {
      qc.invalidateQueries({ queryKey: issueTemplateKeys.all(wsId) });
    },
  });
}

export function useUpdateIssueTemplate() {
  const qc = useQueryClient();
  const wsId = useWorkspaceId();
  return useMutation({
    mutationFn: ({ id, ...data }: { id: string } & UpdateIssueTemplateRequest) =>
      api.updateIssueTemplate(id, data),
    onMutate: async ({ id, ...data }) => {
      await qc.cancelQueries({ queryKey: issueTemplateKeys.all(wsId) });
      const prevDetail = qc.getQueryData<IssueTemplate>(issueTemplateKeys.detail(wsId, id));
      qc.setQueryData<IssueTemplate>(issueTemplateKeys.detail(wsId, id), (old) =>
        old ? { ...old, ...data } : old,
      );
      return { prevDetail, id };
    },
    onError: (_err, _vars, ctx) => {
      if (ctx?.prevDetail) {
        qc.setQueryData(issueTemplateKeys.detail(wsId, ctx.id), ctx.prevDetail);
      }
    },
    onSettled: (_data, _err, vars) => {
      qc.invalidateQueries({ queryKey: issueTemplateKeys.detail(wsId, vars.id) });
      qc.invalidateQueries({ queryKey: issueTemplateKeys.all(wsId) });
    },
  });
}

export function useDeleteIssueTemplate() {
  const qc = useQueryClient();
  const wsId = useWorkspaceId();
  return useMutation({
    mutationFn: (id: string) => api.deleteIssueTemplate(id),
    onMutate: async (id) => {
      await qc.cancelQueries({ queryKey: issueTemplateKeys.all(wsId) });
      qc.removeQueries({ queryKey: issueTemplateKeys.detail(wsId, id) });
      return { id };
    },
    onSettled: () => {
      qc.invalidateQueries({ queryKey: issueTemplateKeys.all(wsId) });
    },
  });
}

export function useInstantiateIssueTemplate() {
  const qc = useQueryClient();
  const wsId = useWorkspaceId();
  return useMutation({
    mutationFn: (id: string) => api.instantiateIssueTemplate(id),
    onSuccess: (issue) => {
      qc.setQueryData(issueKeys.detail(wsId, issue.id), issue);
    },
    onSettled: () => {
      qc.invalidateQueries({ queryKey: issueKeys.all(wsId) });
      qc.invalidateQueries({ queryKey: issueTemplateKeys.all(wsId) });
    },
  });
}
