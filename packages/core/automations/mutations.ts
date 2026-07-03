import { useMutation, useQueryClient } from "@tanstack/react-query";
import { api } from "../api";
import { useWorkspaceId } from "../hooks";
import type {
  Automation,
  CreateAutomationRequest,
  CreateAutomationTriggerRequest,
  GetAutomationResponse,
  ListAutomationsResponse,
  UpdateAutomationRequest,
} from "../types";
import { automationKeys } from "./queries";

export function useCreateAutomation() {
  const qc = useQueryClient();
  const wsId = useWorkspaceId();
  return useMutation({
    mutationFn: (data: CreateAutomationRequest) => api.createAutomation(data),
    onSuccess: (newAutomation) => {
      qc.setQueriesData<ListAutomationsResponse>(
        { queryKey: [...automationKeys.all(wsId), "list"] },
        (old) =>
          old && !old.automations.some((a) => a.id === newAutomation.id)
            ? { ...old, automations: [...old.automations, newAutomation], total: old.total + 1 }
            : old,
      );
    },
    onSettled: () => {
      qc.invalidateQueries({ queryKey: automationKeys.all(wsId) });
    },
  });
}

export function useUpdateAutomation() {
  const qc = useQueryClient();
  const wsId = useWorkspaceId();
  return useMutation({
    mutationFn: ({ id, ...data }: { id: string } & UpdateAutomationRequest) =>
      api.updateAutomation(id, data),
    onMutate: async ({ id, ...data }) => {
      await qc.cancelQueries({ queryKey: automationKeys.all(wsId) });
      const prevDetail = qc.getQueryData<GetAutomationResponse>(automationKeys.detail(wsId, id));
      qc.setQueryData<GetAutomationResponse>(automationKeys.detail(wsId, id), (old) =>
        old ? { ...old, automation: { ...old.automation, ...data } as Automation } : old,
      );
      return { prevDetail, id };
    },
    onError: (_err, _vars, ctx) => {
      if (ctx?.prevDetail) qc.setQueryData(automationKeys.detail(wsId, ctx.id), ctx.prevDetail);
    },
    onSettled: (_data, _err, vars) => {
      qc.invalidateQueries({ queryKey: automationKeys.detail(wsId, vars.id) });
      qc.invalidateQueries({ queryKey: automationKeys.all(wsId) });
    },
  });
}

export function useDeleteAutomation() {
  const qc = useQueryClient();
  const wsId = useWorkspaceId();
  return useMutation({
    mutationFn: (id: string) => api.deleteAutomation(id),
    onMutate: async (id) => {
      await qc.cancelQueries({ queryKey: automationKeys.all(wsId) });
      qc.removeQueries({ queryKey: automationKeys.detail(wsId, id) });
      return { id };
    },
    onSettled: () => {
      qc.invalidateQueries({ queryKey: automationKeys.all(wsId) });
    },
  });
}

export function useTriggerAutomation() {
  const qc = useQueryClient();
  const wsId = useWorkspaceId();
  return useMutation({
    mutationFn: (id: string) => api.triggerAutomation(id),
    onSettled: (_data, _err, id) => {
      qc.invalidateQueries({ queryKey: automationKeys.runs(wsId, id) });
      qc.invalidateQueries({ queryKey: automationKeys.detail(wsId, id) });
    },
  });
}

export function useCreateAutomationTrigger() {
  const qc = useQueryClient();
  const wsId = useWorkspaceId();
  return useMutation({
    mutationFn: ({ automationId, ...data }: { automationId: string } & CreateAutomationTriggerRequest) =>
      api.createAutomationTrigger(automationId, data),
    onSettled: (_data, _err, vars) => {
      qc.invalidateQueries({ queryKey: automationKeys.detail(wsId, vars.automationId) });
    },
  });
}

export function useReplayAutomationDelivery() {
  const qc = useQueryClient();
  const wsId = useWorkspaceId();
  return useMutation({
    mutationFn: ({ automationId, deliveryId }: { automationId: string; deliveryId: string }) =>
      api.replayAutomationDelivery(automationId, deliveryId),
    onSettled: (_data, _err, vars) => {
      qc.invalidateQueries({ queryKey: automationKeys.deliveries(wsId, vars.automationId) });
      qc.invalidateQueries({ queryKey: automationKeys.runs(wsId, vars.automationId) });
    },
  });
}
