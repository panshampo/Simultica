"use client";

import { useMemo, useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { toast } from "sonner";
import { FileText, Play, Plus, Search } from "lucide-react";
import { issueTemplateListOptions, useInstantiateIssueTemplate } from "@multica/core/issue-templates";
import { useWorkspaceId } from "@multica/core/hooks";
import { useWorkspacePaths } from "@multica/core/paths";
import type { IssueTemplate } from "@multica/core/types";
import { useActorName } from "@multica/core/workspace/hooks";
import { Button } from "@multica/ui/components/ui/button";
import { Input } from "@multica/ui/components/ui/input";
import { Skeleton } from "@multica/ui/components/ui/skeleton";
import { AppLink, useNavigation } from "../../navigation";
import { ActorAvatar } from "../../common/actor-avatar";
import { PageHeader } from "../../layout/page-header";
import { useT } from "../../i18n";
import { matchesPinyin } from "../../editor/extensions/pinyin-match";
import { TemplateDialog } from "./template-dialog";
import { formatTemplateDate, templatePreview } from "./template-utils";

export function TemplatesPage() {
  const { t } = useT("templates");
  const wsId = useWorkspaceId();
  const p = useWorkspacePaths();
  const navigation = useNavigation();
  const { getActorName } = useActorName();
  const { data: templates = [], isLoading } = useQuery(
    issueTemplateListOptions(wsId),
  );
  const instantiate = useInstantiateIssueTemplate();
  const [search, setSearch] = useState("");
  const [createOpen, setCreateOpen] = useState(false);
  const priorityLabels = {
    none: t(($) => $.priority.none),
    low: t(($) => $.priority.low),
    medium: t(($) => $.priority.medium),
    high: t(($) => $.priority.high),
    urgent: t(($) => $.priority.urgent),
  };

  const filtered = useMemo(() => {
    const q = search.trim().toLowerCase();
    if (!q) return templates;
    return templates.filter((template) => {
      const haystack = [
        template.title,
        template.issue_title_template,
        template.issue_body_template ?? "",
      ].join(" ");
      return haystack.toLowerCase().includes(q) || matchesPinyin(haystack, q);
    });
  }, [search, templates]);

  async function handleInstantiate(template: IssueTemplate) {
    try {
      const issue = await instantiate.mutateAsync(template.id);
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

  return (
    <div className="flex h-full min-h-0 flex-col">
      <PageHeader className="justify-between px-5">
        <div className="flex items-center gap-2">
          <FileText className="h-4 w-4 text-muted-foreground" />
          <h1 className="text-sm font-medium">{t(($) => $.page.title)}</h1>
          {!isLoading && templates.length > 0 && (
            <span className="text-xs tabular-nums text-muted-foreground">
              {templates.length}
            </span>
          )}
        </div>
        <Button size="sm" variant="outline" onClick={() => setCreateOpen(true)}>
          <Plus className="mr-1 size-3.5" />
          {t(($) => $.page.new_template)}
        </Button>
      </PageHeader>

      <div className="flex h-12 shrink-0 items-center gap-3 border-b px-4">
        <div className="relative min-w-0 flex-1 sm:max-w-sm">
          <Search className="pointer-events-none absolute left-2.5 top-1/2 size-3.5 -translate-y-1/2 text-muted-foreground" />
          <Input
            value={search}
            onChange={(event) => setSearch(event.target.value)}
            placeholder={t(($) => $.page.search_placeholder)}
            className="h-8 pl-8 text-sm"
          />
        </div>
        <span className="hidden font-mono text-xs tabular-nums text-muted-foreground/70 sm:inline">
          {filtered.length} / {templates.length}
        </span>
      </div>

      <div className="min-h-0 flex-1 overflow-y-auto">
        {isLoading ? (
          <TemplatesSkeleton />
        ) : templates.length === 0 ? (
          <div className="flex flex-col items-center justify-center gap-3 py-16 text-center">
            <FileText className="size-10 text-muted-foreground/50" />
            <p className="text-sm text-muted-foreground">{t(($) => $.page.empty)}</p>
            <Button size="sm" variant="outline" onClick={() => setCreateOpen(true)}>
              <Plus className="mr-1 size-3.5" />
              {t(($) => $.page.new_template)}
            </Button>
          </div>
        ) : filtered.length === 0 ? (
          <div className="flex flex-col items-center justify-center gap-2 py-16 text-center">
            <p className="text-sm text-muted-foreground">
              {t(($) => $.page.empty_search)}
            </p>
          </div>
        ) : (
          <div className="divide-y">
            <div className="sticky top-0 z-[1] hidden h-8 items-center gap-3 border-b bg-muted/30 px-5 text-[11px] font-medium uppercase tracking-wide text-muted-foreground sm:grid sm:grid-cols-[minmax(220px,1fr)_180px_96px_76px_88px]">
              <span>{t(($) => $.table.template)}</span>
              <span>{t(($) => $.table.assignee)}</span>
              <span>{t(($) => $.table.priority)}</span>
              <span>{t(($) => $.table.updated)}</span>
              <span className="text-right">{t(($) => $.table.actions)}</span>
            </div>
            {filtered.map((template) => (
              <div
                key={template.id}
                className="group/row grid gap-2 px-4 py-3 text-sm transition-colors hover:bg-accent/40 sm:h-14 sm:grid-cols-[minmax(220px,1fr)_180px_96px_76px_88px] sm:items-center sm:gap-3 sm:px-5 sm:py-0"
              >
                <AppLink
                  href={p.templateDetail(template.id)}
                  className="min-w-0 space-y-0.5"
                >
                  <div className="truncate font-medium">{template.title}</div>
                  <div className="line-clamp-1 text-xs text-muted-foreground">
                    {templatePreview(template)}
                  </div>
                </AppLink>
                <div className="flex min-w-0 items-center gap-1.5 text-xs text-muted-foreground">
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
                </div>
                <div className="text-xs text-muted-foreground">
                  {priorityLabels[template.priority as keyof typeof priorityLabels] ?? template.priority}
                </div>
                <div className="text-xs tabular-nums text-muted-foreground">
                  {formatTemplateDate(template.updated_at)}
                </div>
                <div className="flex justify-start sm:justify-end">
                  <Button
                    size="sm"
                    variant="ghost"
                    onClick={() => handleInstantiate(template)}
                    disabled={instantiate.isPending}
                  >
                    <Play className="mr-1 size-3.5" />
                    {t(($) => $.actions.use)}
                  </Button>
                </div>
              </div>
            ))}
          </div>
        )}
      </div>

      <TemplateDialog open={createOpen} onOpenChange={setCreateOpen} />
    </div>
  );
}

function TemplatesSkeleton() {
  return (
    <div className="divide-y">
      {Array.from({ length: 6 }).map((_, index) => (
        <div
          key={index}
          className="grid gap-3 px-5 py-3 sm:h-14 sm:grid-cols-[minmax(220px,1fr)_180px_96px_76px_88px] sm:items-center sm:py-0"
        >
          <div className="space-y-1.5">
            <Skeleton className="h-4 w-40" />
            <Skeleton className="h-3 w-64 max-w-full" />
          </div>
          <Skeleton className="h-4 w-28" />
          <Skeleton className="h-4 w-16" />
          <Skeleton className="h-4 w-12" />
          <Skeleton className="h-7 w-16 sm:ml-auto" />
        </div>
      ))}
    </div>
  );
}
