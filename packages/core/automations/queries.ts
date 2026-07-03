import { queryOptions } from "@tanstack/react-query";
import { api } from "../api";
import type { AutomationSourceMode } from "../types";

export const automationKeys = {
  all: (wsId: string) => ["automations", wsId] as const,
  list: (wsId: string, params?: { status?: string; source_mode?: AutomationSourceMode }) =>
    [...automationKeys.all(wsId), "list", params ?? {}] as const,
  detail: (wsId: string, id: string) =>
    [...automationKeys.all(wsId), "detail", id] as const,
  runs: (wsId: string, id: string) =>
    [...automationKeys.all(wsId), "runs", id] as const,
  run: (wsId: string, automationId: string, runId: string) =>
    [...automationKeys.all(wsId), "runs", automationId, runId] as const,
  deliveries: (wsId: string, id: string) =>
    [...automationKeys.all(wsId), "deliveries", id] as const,
  delivery: (wsId: string, automationId: string, deliveryId: string) =>
    [...automationKeys.all(wsId), "deliveries", automationId, deliveryId] as const,
};

export function automationListOptions(
  wsId: string,
  params?: { status?: string; source_mode?: AutomationSourceMode },
) {
  return queryOptions({
    queryKey: automationKeys.list(wsId, params),
    queryFn: () => api.listAutomations(params),
    select: (data) => data.automations,
  });
}

export function automationDetailOptions(wsId: string, id: string) {
  return queryOptions({
    queryKey: automationKeys.detail(wsId, id),
    queryFn: () => api.getAutomation(id),
  });
}

export function automationRunsOptions(wsId: string, id: string) {
  return queryOptions({
    queryKey: automationKeys.runs(wsId, id),
    queryFn: () => api.listAutomationRuns(id),
    select: (data) => data.runs,
  });
}

export function automationRunOptions(
  wsId: string,
  automationId: string,
  runId: string,
  options?: { enabled?: boolean },
) {
  return queryOptions({
    queryKey: automationKeys.run(wsId, automationId, runId),
    queryFn: () => api.getAutomationRun(automationId, runId),
    enabled: options?.enabled ?? true,
  });
}

export function automationDeliveriesOptions(
  wsId: string,
  automationId: string,
  options?: { enabled?: boolean },
) {
  return queryOptions({
    queryKey: automationKeys.deliveries(wsId, automationId),
    queryFn: () => api.listAutomationDeliveries(automationId),
    select: (data) => data.deliveries,
    enabled: options?.enabled ?? true,
  });
}

export function automationDeliveryOptions(
  wsId: string,
  automationId: string,
  deliveryId: string,
  options?: { enabled?: boolean },
) {
  return queryOptions({
    queryKey: automationKeys.delivery(wsId, automationId, deliveryId),
    queryFn: () => api.getAutomationDelivery(automationId, deliveryId),
    enabled: options?.enabled ?? true,
  });
}
