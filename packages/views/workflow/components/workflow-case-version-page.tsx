"use client";

import { useMemo } from "react";
import { useQuery } from "@tanstack/react-query";
import { ArrowLeft, FolderGit2 } from "lucide-react";
import { useWorkspaceId } from "@multica/core/hooks";
import { useWorkspacePaths } from "@multica/core/paths";
import {
  workflowCaseDefinitionVersionOptions,
  workflowCaseDetailOptions,
} from "@multica/core/workflow/queries";
import type { WorkflowDefinition } from "@multica/core/workflow/types";
import { Skeleton } from "@multica/ui/components/ui/skeleton";
import { PageHeader } from "../../layout/page-header";
import { AppLink } from "../../navigation";
import { WorkflowCanvas } from "./workflow-canvas";
import { serializeWorkflow } from "../lib/serialize";

export function WorkflowCaseVersionPage({
  caseId,
  versionId,
}: {
  caseId: string;
  versionId: string;
}) {
  const wsId = useWorkspaceId();
  const paths = useWorkspacePaths();
  const { data: workflowCase, isLoading: caseLoading } = useQuery(workflowCaseDetailOptions(wsId, caseId));
  const { data: version, isLoading: versionLoading } = useQuery(workflowCaseDefinitionVersionOptions(wsId, caseId, versionId));
  const yaml = useMemo(() => version ? serializeWorkflow(version.snapshot_json as WorkflowDefinition) : "", [version]);
  const isOnline = Boolean(workflowCase?.online_version_id && workflowCase.online_version_id === versionId);

  if (caseLoading || versionLoading) {
    return <WorkflowCaseVersionSkeleton />;
  }

  if (!workflowCase || !version) {
    return (
      <div className="flex h-full flex-col">
        <PageHeader className="px-5">
          <AppLink href={paths.workflowCaseDetail(caseId)} className="flex items-center gap-1 text-sm text-muted-foreground hover:text-foreground">
            <ArrowLeft className="size-4" />
            Workflow case
          </AppLink>
        </PageHeader>
        <div className="flex flex-1 items-center justify-center text-sm text-muted-foreground">Workflow case version not found.</div>
      </div>
    );
  }

  return (
    <div className="flex h-full min-h-0 flex-col">
      <PageHeader className="justify-between px-5">
        <div className="flex min-w-0 items-center gap-2">
          <AppLink
            href={paths.workflowCaseDetail(caseId)}
            className="rounded-md p-1 text-muted-foreground hover:bg-accent hover:text-foreground"
            aria-label="Back to workflow case"
          >
            <ArrowLeft className="size-4" />
          </AppLink>
          <FolderGit2 className="size-4 shrink-0 text-muted-foreground" />
          <h1 className="truncate text-sm font-medium">{workflowCase.title}</h1>
          <span className="rounded bg-muted px-1.5 py-0.5 text-xs text-muted-foreground">v{version.version}</span>
          <span className={isOnline ? "rounded bg-green-100 px-1.5 py-0.5 text-xs text-green-800" : "rounded bg-muted px-1.5 py-0.5 text-xs text-muted-foreground"}>
            {isOnline ? "Online" : "Historical"}
          </span>
        </div>
      </PageHeader>

      <div className="min-h-0 flex-1 overflow-y-auto p-5">
        <main className="mx-auto flex max-w-6xl flex-col gap-4">
          <section className="rounded-lg border bg-card p-4">
            <div>
              <h2 className="text-sm font-medium">Published version snapshot</h2>
              <p className="mt-1 text-xs text-muted-foreground">
                This is an immutable read-only snapshot. New runs use only the current online version.
              </p>
            </div>
            <div className="mt-4 grid gap-3 text-sm md:grid-cols-4">
              <Property label="Version" value={`v${version.version}`} />
              <Property label="State" value={isOnline ? "Online" : "Historical"} />
              <Property label="Validation" value={version.validation_report?.valid ? "Valid" : "Has issues"} />
              <Property label="Published" value={formatDateTime(version.created_at)} />
            </div>
          </section>

          <section className="space-y-3 rounded-lg border bg-card p-4">
            <div>
              <h2 className="text-sm font-medium">Version canvas</h2>
              <p className="mt-1 text-xs text-muted-foreground">Read-only graph from the published snapshot.</p>
            </div>
            <WorkflowCanvas
              key={version.id}
              definition={version.snapshot_json as WorkflowDefinition}
              className="min-h-[520px]"
              fullscreenTitle={`Workflow case v${version.version}`}
              hideSelectionOverlay
            />
          </section>

          <section className="space-y-3 rounded-lg border bg-card p-4">
            <div>
              <h2 className="text-sm font-medium">YAML snapshot</h2>
              <p className="mt-1 text-xs text-muted-foreground">Immutable serialized definition for this version.</p>
            </div>
            <pre
              aria-label="Workflow version YAML"
              className="max-h-[520px] overflow-auto rounded-md border bg-muted/30 p-3 font-mono text-xs text-muted-foreground"
            >
              {yaml}
            </pre>
          </section>
        </main>
      </div>
    </div>
  );
}

function WorkflowCaseVersionSkeleton() {
  return (
    <div className="flex h-full min-h-0 flex-col">
      <PageHeader className="px-5">
        <Skeleton className="h-5 w-48" />
      </PageHeader>
      <div className="space-y-4 p-5">
        <Skeleton className="h-28 rounded-lg" />
        <Skeleton className="h-[640px] rounded-lg" />
      </div>
    </div>
  );
}

function Property({
  label,
  value,
}: {
  label: string;
  value: string;
}) {
  return (
    <div className="min-w-0">
      <div className="text-[11px] uppercase text-muted-foreground">{label}</div>
      <div className="mt-1 truncate text-sm">{value}</div>
    </div>
  );
}

function formatDateTime(value: string): string {
  return new Intl.DateTimeFormat(undefined, {
    month: "short",
    day: "numeric",
    hour: "2-digit",
    minute: "2-digit",
  }).format(new Date(value));
}
