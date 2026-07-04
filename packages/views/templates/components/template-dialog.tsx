"use client";

import { useEffect, useMemo, useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { toast } from "sonner";
import { Bot, Users } from "lucide-react";
import type {
  CreateIssueTemplateRequest,
  IssuePriority,
  IssueTemplate,
  IssueTemplateAssigneeType,
} from "@multica/core/types";
import { useWorkspaceId } from "@multica/core/hooks";
import {
  agentListOptions,
  squadListOptions,
} from "@multica/core/workspace/queries";
import {
  useCreateIssueTemplate,
  useUpdateIssueTemplate,
} from "@multica/core/issue-templates";
import { Button } from "@multica/ui/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@multica/ui/components/ui/dialog";
import { Input } from "@multica/ui/components/ui/input";
import { Label } from "@multica/ui/components/ui/label";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@multica/ui/components/ui/select";
import { Textarea } from "@multica/ui/components/ui/textarea";
import { ProjectPicker } from "../../projects/components/project-picker";
import { PillButton } from "../../common/pill-button";
import { ActorAvatar } from "../../common/actor-avatar";
import { managementActionButtonClass } from "../../common/management-action-button";
import { useT } from "../../i18n";

type AssigneeValue = `${IssueTemplateAssigneeType}:${string}`;

const PRIORITIES: IssuePriority[] = ["none", "low", "medium", "high", "urgent"];

function encodeAssignee(type: IssueTemplateAssigneeType, id: string): AssigneeValue {
  return `${type}:${id}`;
}

function decodeAssignee(value: string): {
  type: IssueTemplateAssigneeType;
  id: string;
} | null {
  const [type, ...rest] = value.split(":");
  const id = rest.join(":");
  if ((type === "agent" || type === "squad") && id) {
    return { type, id };
  }
  return null;
}

export interface TemplateDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  template?: IssueTemplate | null;
}

export function TemplateDialog({
  open,
  onOpenChange,
  template,
}: TemplateDialogProps) {
  const { t } = useT("templates");
  const wsId = useWorkspaceId();
  const createTemplate = useCreateIssueTemplate();
  const updateTemplate = useUpdateIssueTemplate();
  const { data: agents = [] } = useQuery(agentListOptions(wsId));
  const { data: squads = [] } = useQuery(squadListOptions(wsId));

  const activeAgents = useMemo(
    () => agents.filter((agent) => !agent.archived_at),
    [agents],
  );
  const activeSquads = useMemo(
    () => squads.filter((squad) => !squad.archived_at),
    [squads],
  );
  const firstAssignee = activeAgents[0]
    ? encodeAssignee("agent", activeAgents[0].id)
    : activeSquads[0]
      ? encodeAssignee("squad", activeSquads[0].id)
      : "";

  const [title, setTitle] = useState("");
  const [issueTitle, setIssueTitle] = useState("");
  const [issueBody, setIssueBody] = useState("");
  const [projectId, setProjectId] = useState<string | null>(null);
  const [assignee, setAssignee] = useState("");
  const [priority, setPriority] = useState<IssuePriority>("none");
  const priorityLabels = {
    none: t(($) => $.priority.none),
    low: t(($) => $.priority.low),
    medium: t(($) => $.priority.medium),
    high: t(($) => $.priority.high),
    urgent: t(($) => $.priority.urgent),
  };

  useEffect(() => {
    if (!open) return;
    setTitle(template?.title ?? "");
    setIssueTitle(template?.issue_title_template ?? "");
    setIssueBody(template?.issue_body_template ?? "");
    setProjectId(template?.project_id ?? null);
    setAssignee(
      template
        ? encodeAssignee(template.assignee_type, template.assignee_id)
        : "",
    );
    setPriority((template?.priority as IssuePriority | undefined) ?? "none");
  }, [open, template?.id]);

  useEffect(() => {
    if (!open || template || assignee || !firstAssignee) return;
    setAssignee(firstAssignee);
  }, [assignee, firstAssignee, open, template]);

  const decodedAssignee = decodeAssignee(assignee);
  const selectedAssignee =
    decodedAssignee?.type === "agent"
      ? activeAgents.find((agent) => agent.id === decodedAssignee.id)
      : decodedAssignee?.type === "squad"
        ? activeSquads.find((squad) => squad.id === decodedAssignee.id)
        : null;
  const canSubmit =
    title.trim().length > 0 &&
    issueTitle.trim().length > 0 &&
    decodedAssignee !== null;
  const submitting = createTemplate.isPending || updateTemplate.isPending;

  async function handleSubmit() {
    if (!canSubmit || !decodedAssignee) return;

    const data: CreateIssueTemplateRequest = {
      title: title.trim(),
      issue_title_template: issueTitle.trim(),
      issue_body_template: issueBody.trim() || null,
      project_id: projectId,
      assignee_type: decodedAssignee.type,
      assignee_id: decodedAssignee.id,
      priority,
    };

    try {
      if (template) {
        await updateTemplate.mutateAsync({ id: template.id, ...data });
        toast.success(t(($) => $.toast.updated));
      } else {
        await createTemplate.mutateAsync(data);
        toast.success(t(($) => $.toast.created));
      }
      onOpenChange(false);
    } catch (err) {
      toast.error(
        err instanceof Error && err.message
          ? err.message
          : t(($) => $.toast.save_failed),
      );
    }
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="sm:max-w-2xl">
        <DialogHeader>
          <DialogTitle>
            {template ? t(($) => $.dialog.edit_title) : t(($) => $.dialog.create_title)}
          </DialogTitle>
          <DialogDescription>{t(($) => $.dialog.description)}</DialogDescription>
        </DialogHeader>

        <div className="grid gap-3">
          <div className="grid gap-1.5">
            <Label htmlFor="template-title">{t(($) => $.fields.name)}</Label>
            <Input
              id="template-title"
              value={title}
              onChange={(event) => setTitle(event.target.value)}
              placeholder={t(($) => $.fields.name_placeholder)}
            />
          </div>

          <div className="grid gap-1.5">
            <Label htmlFor="template-issue-title">
              {t(($) => $.fields.issue_title)}
            </Label>
            <Input
              id="template-issue-title"
              value={issueTitle}
              onChange={(event) => setIssueTitle(event.target.value)}
              placeholder={t(($) => $.fields.issue_title_placeholder)}
            />
          </div>

          <div className="grid gap-1.5">
            <Label htmlFor="template-issue-body">
              {t(($) => $.fields.issue_body)}
            </Label>
            <Textarea
              id="template-issue-body"
              value={issueBody}
              onChange={(event) => setIssueBody(event.target.value)}
              placeholder={t(($) => $.fields.issue_body_placeholder)}
              className="min-h-32 font-mono text-xs"
            />
          </div>

          <div className="grid gap-3 sm:grid-cols-3">
            <div className="grid gap-1.5">
              <Label>{t(($) => $.fields.project)}</Label>
              <ProjectPicker
                projectId={projectId}
                onUpdate={(updates) => setProjectId(updates.project_id ?? null)}
                triggerRender={<PillButton className="w-full justify-start" />}
                align="start"
              />
            </div>

            <div className="grid gap-1.5">
              <Label>{t(($) => $.fields.assignee)}</Label>
              <Select
                value={assignee}
                onValueChange={(value) => setAssignee(value ?? "")}
              >
                <SelectTrigger className="w-full">
                  <SelectValue placeholder={t(($) => $.fields.assignee_placeholder)}>
                    {selectedAssignee ? (
                      <>
                        {decodedAssignee?.type === "squad" ? (
                          <Users className="size-3.5 text-muted-foreground" />
                        ) : (
                          <Bot className="size-3.5 text-muted-foreground" />
                        )}
                        <span className="truncate">{selectedAssignee.name}</span>
                      </>
                    ) : null}
                  </SelectValue>
                </SelectTrigger>
                <SelectContent align="start" className="max-h-72">
                  {activeAgents.map((agent) => (
                    <SelectItem
                      key={`agent:${agent.id}`}
                      value={encodeAssignee("agent", agent.id)}
                    >
                      <Bot className="size-3.5 text-muted-foreground" />
                      <span className="truncate">{agent.name}</span>
                    </SelectItem>
                  ))}
                  {activeSquads.map((squad) => (
                    <SelectItem
                      key={`squad:${squad.id}`}
                      value={encodeAssignee("squad", squad.id)}
                    >
                      <Users className="size-3.5 text-muted-foreground" />
                      <span className="truncate">{squad.name}</span>
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
              {!firstAssignee && (
                <p className="text-xs text-destructive">
                  {t(($) => $.fields.assignee_empty)}
                </p>
              )}
            </div>

            <div className="grid gap-1.5">
              <Label>{t(($) => $.fields.priority)}</Label>
              <Select
                value={priority}
                onValueChange={(value) => setPriority(value as IssuePriority)}
              >
                <SelectTrigger className="w-full">
                  <SelectValue>{priorityLabels[priority]}</SelectValue>
                </SelectTrigger>
                <SelectContent align="start">
                  {PRIORITIES.map((value) => (
                    <SelectItem key={value} value={value}>
                      {priorityLabels[value]}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>
          </div>
        </div>

        {decodedAssignee && (
          <div className="flex items-center gap-2 rounded-md bg-muted/40 px-3 py-2 text-xs text-muted-foreground">
            <ActorAvatar
              actorType={decodedAssignee.type}
              actorId={decodedAssignee.id}
              size={18}
              enableHoverCard={decodedAssignee.type === "agent"}
              showStatusDot={decodedAssignee.type === "agent"}
            />
            <span>{t(($) => $.dialog.assignee_note)}</span>
          </div>
        )}

        <DialogFooter>
          <Button
            type="button"
            variant="outline"
            onClick={() => onOpenChange(false)}
          >
            {t(($) => $.actions.cancel)}
          </Button>
          <Button
            type="button"
            className={managementActionButtonClass("save")}
            onClick={handleSubmit}
            disabled={!canSubmit || submitting}
          >
            {submitting
              ? t(($) => $.actions.saving)
              : template
                ? t(($) => $.actions.save)
                : t(($) => $.actions.create)}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
