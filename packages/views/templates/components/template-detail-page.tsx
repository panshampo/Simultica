"use client";

import { useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { toast } from "sonner";
import { ArrowLeft, FileText, Play, Trash2 } from "lucide-react";
import {
  issueTemplateDetailOptions,
  useDeleteIssueTemplate,
  useInstantiateIssueTemplate,
} from "@multica/core/issue-templates";
import { useWorkspaceId } from "@multica/core/hooks";
import { useWorkspacePaths } from "@multica/core/paths";
import type { IssueTemplate } from "@multica/core/types";
import { useActorName } from "@multica/core/workspace/hooks";
import { Button } from "@multica/ui/components/ui/button";
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
import { AppLink, useNavigation } from "../../navigation";
import { ActorAvatar } from "../../common/actor-avatar";
import { PageHeader } from "../../layout/page-header";
import { ProjectChip } from "../../projects/components/project-chip";
import { useT } from "../../i18n";
import { TemplateDialog } from "./template-dialog";
import { formatTemplateDate } from "./template-utils";

export function TemplateDetailPage({ templateId }: { templateId: string }) {
  const { t } = useT("templates");
  const wsId = useWorkspaceId();
  const p = useWorkspacePaths();
  const navigation = useNavigation();
  const { getActorName } = useActorName();
  const { data: template, isLoading } = useQuery(
    issueTemplateDetailOptions(wsId, templateId),
  );
  const instantiate = useInstantiateIssueTemplate();
  const deleteTemplate = useDeleteIssueTemplate();
  const [editOpen, setEditOpen] = useState(false);
  const [deleteOpen, setDeleteOpen] = useState(false);
  const priorityLabels = {
    none: t(($) => $.priority.none),
    low: t(($) => $.priority.low),
    medium: t(($) => $.priority.medium),
    high: t(($) => $.priority.high),
    urgent: t(($) => $.priority.urgent),
  };

  async function handleInstantiate(next: IssueTemplate) {
    try {
      const issue = await instantiate.mutateAsync(next.id);
      toast.success(t(($) => $.toast.instantiated));
      navigation.push(p.issueDetail(issue.id));
    } catch (err) {
      toast.error(
        err instanceof Error && err.message
          ? err.message
          : t(($) => $.toast.instantiate_failed),
      );
    }
  }

  async function handleDelete() {
    if (!template) return;
    try {
      await deleteTemplate.mutateAsync(template.id);
      toast.success(t(($) => $.toast.deleted));
      navigation.push(p.templates());
    } catch (err) {
      toast.error(
        err instanceof Error && err.message
          ? err.message
          : t(($) => $.toast.delete_failed),
      );
    }
  }

  if (isLoading) {
    return <TemplateDetailSkeleton />;
  }

  if (!template) {
    return (
      <div className="flex h-full flex-col">
        <PageHeader className="px-5">
          <AppLink
            href={p.templates()}
            className="flex items-center gap-1 text-sm text-muted-foreground hover:text-foreground"
          >
            <ArrowLeft className="size-4" />
            {t(($) => $.detail.back)}
          </AppLink>
        </PageHeader>
        <div className="flex flex-1 items-center justify-center text-sm text-muted-foreground">
          {t(($) => $.detail.not_found)}
        </div>
      </div>
    );
  }

  return (
    <div className="flex h-full min-h-0 flex-col">
      <PageHeader className="justify-between px-5">
        <div className="flex min-w-0 items-center gap-2">
          <AppLink
            href={p.templates()}
            className="rounded-md p-1 text-muted-foreground hover:bg-accent hover:text-foreground"
            aria-label={t(($) => $.detail.back)}
          >
            <ArrowLeft className="size-4" />
          </AppLink>
          <FileText className="size-4 shrink-0 text-muted-foreground" />
          <h1 className="truncate text-sm font-medium">{template.title}</h1>
        </div>
        <div className="flex items-center gap-2">
          <Button variant="outline" size="sm" onClick={() => setEditOpen(true)}>
            {t(($) => $.actions.edit)}
          </Button>
          <Button
            size="sm"
            onClick={() => handleInstantiate(template)}
            disabled={instantiate.isPending}
          >
            <Play className="mr-1 size-3.5" />
            {t(($) => $.actions.use)}
          </Button>
        </div>
      </PageHeader>

      <div className="grid min-h-0 flex-1 gap-4 overflow-y-auto p-5 lg:grid-cols-[minmax(0,1fr)_280px]">
        <main className="min-w-0 space-y-4">
          <section className="rounded-lg border bg-card">
            <div className="border-b px-4 py-3">
              <h2 className="text-sm font-medium">{t(($) => $.detail.issue_title)}</h2>
            </div>
            <div className="px-4 py-3 text-sm">{template.issue_title_template}</div>
          </section>

          <section className="rounded-lg border bg-card">
            <div className="border-b px-4 py-3">
              <h2 className="text-sm font-medium">{t(($) => $.detail.issue_body)}</h2>
            </div>
            <pre className="max-h-[520px] overflow-auto whitespace-pre-wrap px-4 py-3 font-mono text-xs leading-5 text-muted-foreground">
              {template.issue_body_template || t(($) => $.detail.empty_body)}
            </pre>
          </section>
        </main>

        <aside className="space-y-4">
          <section className="rounded-lg border bg-card p-3">
            <h2 className="mb-2 text-xs font-medium uppercase tracking-wide text-muted-foreground">
              {t(($) => $.detail.properties)}
            </h2>
            <div className="space-y-2">
              <PropRow label={t(($) => $.fields.assignee)}>
                <ActorAvatar
                  actorType={template.assignee_type}
                  actorId={template.assignee_id}
                  size={18}
                  enableHoverCard={template.assignee_type === "agent"}
                  showStatusDot={template.assignee_type === "agent"}
                />
                <span className="truncate">
                  {getActorName(template.assignee_type, template.assignee_id)}
                </span>
              </PropRow>
              <PropRow label={t(($) => $.fields.priority)}>
                {priorityLabels[template.priority as keyof typeof priorityLabels] ?? template.priority}
              </PropRow>
              <PropRow label={t(($) => $.fields.project)}>
                {template.project_id ? (
                  <ProjectChip projectId={template.project_id} />
                ) : (
                  <span className="text-muted-foreground">
                    {t(($) => $.fields.no_project)}
                  </span>
                )}
              </PropRow>
              <PropRow label={t(($) => $.fields.updated)}>
                {formatTemplateDate(template.updated_at)}
              </PropRow>
              <PropRow label={t(($) => $.fields.created)}>
                {formatTemplateDate(template.created_at)}
              </PropRow>
            </div>
          </section>

          <section className="rounded-lg border bg-card p-3">
            <h2 className="mb-2 text-xs font-medium uppercase tracking-wide text-muted-foreground">
              {t(($) => $.detail.references)}
            </h2>
            <div className="grid grid-cols-2 gap-2 text-xs">
              <Metric label={t(($) => $.detail.issue_refs)} value={template.issue_ref_count ?? 0} />
              <Metric
                label={t(($) => $.detail.automation_refs)}
                value={template.automation_ref_count ?? 0}
              />
            </div>
          </section>

          <Button
            variant="ghost"
            className="w-full justify-start text-destructive hover:text-destructive"
            onClick={() => setDeleteOpen(true)}
          >
            <Trash2 className="mr-2 size-4" />
            {t(($) => $.actions.delete)}
          </Button>
        </aside>
      </div>

      <TemplateDialog
        open={editOpen}
        onOpenChange={setEditOpen}
        template={template}
      />
      <AlertDialog open={deleteOpen} onOpenChange={setDeleteOpen}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>{t(($) => $.delete_dialog.title)}</AlertDialogTitle>
            <AlertDialogDescription>
              {t(($) => $.delete_dialog.description, { name: template.title })}
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>{t(($) => $.actions.cancel)}</AlertDialogCancel>
            <AlertDialogAction
              onClick={handleDelete}
              className="bg-destructive text-destructive-foreground hover:bg-destructive/90"
            >
              {deleteTemplate.isPending
                ? t(($) => $.actions.deleting)
                : t(($) => $.actions.delete)}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </div>
  );
}

function PropRow({
  label,
  children,
}: {
  label: string;
  children: React.ReactNode;
}) {
  return (
    <div className="flex min-h-8 items-center gap-2 rounded-md px-2 py-1 text-xs hover:bg-accent/50">
      <span className="w-20 shrink-0 text-muted-foreground">{label}</span>
      <div className="flex min-w-0 flex-1 items-center gap-1.5">{children}</div>
    </div>
  );
}

function Metric({ label, value }: { label: string; value: number }) {
  return (
    <div className="rounded-md bg-muted/50 px-2 py-2">
      <div className="font-mono text-sm tabular-nums">{value}</div>
      <div className="truncate text-muted-foreground">{label}</div>
    </div>
  );
}

function TemplateDetailSkeleton() {
  return (
    <div className="flex h-full flex-col">
      <PageHeader className="justify-between px-5">
        <Skeleton className="h-5 w-48" />
        <div className="flex gap-2">
          <Skeleton className="h-8 w-14" />
          <Skeleton className="h-8 w-16" />
        </div>
      </PageHeader>
      <div className="grid flex-1 gap-4 p-5 lg:grid-cols-[minmax(0,1fr)_280px]">
        <div className="space-y-4">
          <Skeleton className="h-28 w-full" />
          <Skeleton className="h-64 w-full" />
        </div>
        <Skeleton className="h-56 w-full" />
      </div>
    </div>
  );
}
