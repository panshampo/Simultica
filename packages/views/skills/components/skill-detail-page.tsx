"use client";

import { useEffect, useMemo, useRef, useState, type ReactNode } from "react";
import {
  AlertCircle,
  AlertTriangle,
  ArrowLeft,
  HardDrive,
  Loader2,
  Lock,
  Pencil,
  Plus,
  Save,
  Sparkles,
  Trash2,
} from "lucide-react";
import type {
  AgentRuntime,
  MemberWithUser,
  Skill,
  SkillFile,
  UpdateSkillRequest,
} from "@multica/core/types";
import { toast } from "sonner";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { api } from "@multica/core/api";
import { useTimeAgo } from "../../i18n";
import { useWorkspaceId } from "@multica/core/hooks";
import { useWorkspacePaths } from "@multica/core/paths";
import {
  agentListOptions,
  memberListOptions,
  selectSkillAssignments,
  skillDetailOptions,
  workspaceKeys,
} from "@multica/core/workspace/queries";
import { resolvePublicFileUrl } from "@multica/core/workspace/avatar-url";
import { runtimeListOptions } from "@multica/core/runtimes";
import { ActorAvatar } from "@multica/ui/components/common/actor-avatar";
import { Button, buttonVariants } from "@multica/ui/components/ui/button";
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
import { Skeleton } from "@multica/ui/components/ui/skeleton";
import { Textarea } from "@multica/ui/components/ui/textarea";
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from "@multica/ui/components/ui/tooltip";
import { AppLink, useNavigation } from "../../navigation";
import { BreadcrumbHeader } from "../../layout/breadcrumb-header";
import { useCanEditSkill } from "../hooks/use-can-edit-skill";
import { useSkillPermissions } from "@multica/core/permissions";
import { CapabilityBanner } from "@multica/ui/components/common/capability-banner";
import { readOrigin, totalFileCount } from "../lib/origin";
import { FileTree } from "./file-tree";
import { FileViewer } from "./file-viewer";
import { WorkflowEditor } from "../../workflow/components/workflow-editor";
import { useT } from "../../i18n";

const SKILL_MD = "SKILL.md";

type DraftFile = { id?: string; path: string; content: string };

// ---------------------------------------------------------------------------
// File path validation + inline add
// ---------------------------------------------------------------------------

function useValidateNewFilePath() {
  const { t } = useT("skills");
  return (path: string, existing: string[]): string => {
    const p = path.trim();
    if (!p) return t(($) => $.detail.add_file.errors.empty);
    if (p.startsWith("/")) return t(($) => $.detail.add_file.errors.absolute);
    if (p.split("/").includes("..")) return t(($) => $.detail.add_file.errors.double_dot);
    if (p === SKILL_MD) return t(($) => $.detail.add_file.errors.reserved);
    if (existing.includes(p)) return t(($) => $.detail.add_file.errors.exists);
    return "";
  };
}

function AddFileInline({
  existingPaths,
  onAdd,
  onCancel,
}: {
  existingPaths: string[];
  onAdd: (path: string) => void;
  onCancel: () => void;
}) {
  const { t } = useT("skills");
  const validate = useValidateNewFilePath();
  const [path, setPath] = useState("");
  const [error, setError] = useState("");

  const submit = () => {
    const err = validate(path, existingPaths);
    if (err) {
      setError(err);
      return;
    }
    onAdd(path.trim());
  };

  return (
    <div className="border-b bg-muted/30 px-2 py-2">
      <Input
        autoFocus
        value={path}
        onChange={(e) => {
          setPath(e.target.value);
          setError("");
        }}
        onKeyDown={(e) => {
          if (e.key === "Enter") submit();
          if (e.key === "Escape") onCancel();
        }}
        placeholder={t(($) => $.detail.add_file.placeholder)}
        className="h-7 font-mono text-xs"
      />
      {error && (
        <p role="alert" className="mt-1 text-xs text-destructive">
          {error}
        </p>
      )}
      <div className="mt-1.5 flex items-center gap-1.5">
        <Button type="button" size="xs" onClick={submit}>
          {t(($) => $.detail.add_file.add)}
        </Button>
        <Button type="button" size="xs" variant="ghost" onClick={onCancel}>
          {t(($) => $.detail.add_file.cancel)}
        </Button>
      </div>
    </div>
  );
}

// ---------------------------------------------------------------------------
// Compact metadata
// ---------------------------------------------------------------------------

export function MetadataChip({
  children,
  title,
}: {
  children: ReactNode;
  title?: string;
}) {
  return (
    <span
      title={title}
      className="inline-flex max-w-full items-center gap-1 rounded-md border border-border/70 bg-background/70 px-2 py-1 text-[11px] leading-none text-muted-foreground"
    >
      {children}
    </span>
  );
}

export function SkillHeaderCompact({
  name,
  description,
  canEdit,
  originLabel,
  originType,
  updatedLabel,
  createdLabel,
  creatorName,
  creatorAvatarUrl,
  fileCountLabel,
  idLabel,
  idTitle,
  onNameChange,
  onDescriptionChange,
}: {
  name: string;
  description: string;
  canEdit: boolean;
  originLabel: string | null;
  originType?: string;
  updatedLabel: string;
  createdLabel: string;
  creatorName?: string;
  creatorAvatarUrl?: string | null;
  fileCountLabel: string;
  idLabel: string;
  idTitle: string;
  onNameChange: (value: string) => void;
  onDescriptionChange: (value: string) => void;
}) {
  const { t } = useT("skills");
  return (
    <div className="shrink-0 space-y-3 border-b bg-card/35 px-4 py-3 sm:px-5">
      <div className="grid min-w-0 gap-3 lg:grid-cols-[minmax(14rem,0.8fr)_minmax(18rem,1.2fr)]">
        <Input
          value={name}
          readOnly={!canEdit}
          onChange={(e) => onNameChange(e.target.value)}
          placeholder={t(($) => $.detail.name_placeholder)}
          className="h-9 min-w-0 border-0 bg-transparent px-0 text-lg font-semibold shadow-none focus-visible:ring-0 read-only:cursor-default dark:bg-transparent"
          aria-label={t(($) => $.detail.name_aria)}
        />
        <div className="min-w-0 space-y-1">
          <Label
            htmlFor="skill-description"
            className="flex items-center gap-1 text-[11px] text-muted-foreground"
          >
            <Pencil className="h-3 w-3" />
            {t(($) => $.detail.description_label)}
          </Label>
          <Textarea
            id="skill-description"
            value={description}
            readOnly={!canEdit}
            onChange={(e) => onDescriptionChange(e.target.value)}
            placeholder={t(($) => $.detail.description_placeholder)}
            rows={2}
            className="max-h-16 min-h-0 resize-none overflow-y-auto break-words text-sm leading-relaxed read-only:cursor-default"
          />
        </div>
      </div>
      <div className="flex min-w-0 flex-wrap items-center gap-1.5">
        {originLabel && (
          <MetadataChip>
            {originType === "runtime_local" ? (
              <HardDrive className="h-3 w-3 shrink-0" />
            ) : (
              <Sparkles className="h-3 w-3 shrink-0" />
            )}
            <span className="min-w-0 truncate">{originLabel}</span>
          </MetadataChip>
        )}
        <MetadataChip>{updatedLabel}</MetadataChip>
        <MetadataChip>{createdLabel}</MetadataChip>
        {creatorName && (
          <MetadataChip>
            <ActorAvatar
              name={creatorName}
              initials={creatorName.slice(0, 2).toUpperCase()}
              avatarUrl={creatorAvatarUrl}
              size={14}
            />
            <span className="min-w-0 truncate">{creatorName}</span>
          </MetadataChip>
        )}
        <MetadataChip>{fileCountLabel}</MetadataChip>
        <MetadataChip title={idTitle}>{idLabel}</MetadataChip>
      </div>
    </div>
  );
}

// ---------------------------------------------------------------------------
// Main page
// ---------------------------------------------------------------------------

export function SkillDetailPage({ skillId }: { skillId: string }) {
  const { t } = useT("skills");
  const timeAgo = useTimeAgo();
  const wsId = useWorkspaceId();
  const qc = useQueryClient();
  const paths = useWorkspacePaths();
  const navigation = useNavigation();

  const {
    data: skill,
    isLoading,
    error,
  } = useQuery(skillDetailOptions(wsId, skillId));
  const { data: agents = [], error: agentsError } = useQuery(
    agentListOptions(wsId),
  );
  const { data: members = [], error: membersError } = useQuery(
    memberListOptions(wsId),
  );
  const { data: runtimes = [], error: runtimesError } = useQuery(
    runtimeListOptions(wsId),
  );

  const assignments = useMemo(
    () => selectSkillAssignments(agents),
    [agents],
  );

  const canEdit = useCanEditSkill(skill, wsId);
  const skillPermissions = useSkillPermissions(skill ?? null, wsId);

  const [name, setName] = useState("");
  const [description, setDescription] = useState("");
  const [content, setContent] = useState("");
  const [files, setFiles] = useState<DraftFile[]>([]);
  const [selectedPath, setSelectedPath] = useState(SKILL_MD);
  const [saving, setSaving] = useState(false);
  const [deleting, setDeleting] = useState(false);
  const [confirmDelete, setConfirmDelete] = useState(false);
  const [addingFile, setAddingFile] = useState(false);
  const [conflictPending, setConflictPending] = useState(false);
  const [activeTab, setActiveTab] = useState<"content" | "workflow">("content");
  const [debugStarting, setDebugStarting] = useState(false);

  const draftRef = useRef({ name, description, content, files });
  draftRef.current = { name, description, content, files };

  const seededKeyRef = useRef<string | null>(null);

  useEffect(() => {
    if (!skill) return;
    const key = `${wsId}:${skill.id}@${skill.updated_at}`;
    if (seededKeyRef.current === key) return;

    const sameSkill =
      seededKeyRef.current !== null &&
      seededKeyRef.current.startsWith(`${wsId}:${skill.id}@`);

    if (sameSkill) {
      const d = draftRef.current;
      const serverFilesJson = JSON.stringify(
        (skill.files ?? []).map((f) => ({ path: f.path, content: f.content })),
      );
      const draftFilesJson = JSON.stringify(
        d.files.map((f) => ({ path: f.path, content: f.content })),
      );
      const hasEdits =
        d.name.trim() !== skill.name ||
        d.description.trim() !== skill.description ||
        d.content !== skill.content ||
        draftFilesJson !== serverFilesJson;
      if (hasEdits) {
        setConflictPending(true);
        return;
      }
    }

    seededKeyRef.current = key;
    setConflictPending(false);
    setName(skill.name);
    setDescription(skill.description);
    setContent(skill.content);
    setFiles(
      (skill.files ?? []).map((f: SkillFile) => ({
        id: f.id,
        path: f.path,
        content: f.content,
      })),
    );
    if (!sameSkill) setSelectedPath(SKILL_MD);
  }, [skill, wsId]);

  const creator = useMemo<MemberWithUser | null>(
    () =>
      skill?.created_by
        ? members.find((m) => m.user_id === skill.created_by) ?? null
        : null,
    [members, skill?.created_by],
  );

  const origin = useMemo(
    () => (skill ? readOrigin(skill) : null),
    [skill],
  );
  const originRuntime = useMemo<AgentRuntime | null>(() => {
    if (!origin || origin.type !== "runtime_local" || !origin.runtime_id)
      return null;
    return runtimes.find((r) => r.id === origin.runtime_id) ?? null;
  }, [origin, runtimes]);

  const skillAgents = useMemo(
    () => assignments.get(skillId) ?? [],
    [assignments, skillId],
  );

  const fileMap = useMemo(() => {
    const map = new Map<string, string>();
    map.set(SKILL_MD, content);
    for (const f of files) if (f.path.trim()) map.set(f.path, f.content);
    return map;
  }, [content, files]);
  const filePaths = useMemo(() => Array.from(fileMap.keys()), [fileMap]);
  const selectedContent = fileMap.get(selectedPath) ?? "";

  useEffect(() => {
    if (selectedPath !== SKILL_MD && !fileMap.has(selectedPath)) {
      setSelectedPath(SKILL_MD);
    }
  }, [fileMap, selectedPath]);

  const isDirty = useMemo(() => {
    if (!skill) return false;
    const serverFiles = (skill.files ?? []).map((f: SkillFile) => ({
      path: f.path,
      content: f.content,
    }));
    const draftFiles = files.map((f) => ({ path: f.path, content: f.content }));
    return (
      name.trim() !== skill.name ||
      description.trim() !== skill.description ||
      content !== skill.content ||
      JSON.stringify(draftFiles) !== JSON.stringify(serverFiles)
    );
  }, [skill, name, description, content, files]);

  const seedFromSkill = (s: Skill) => {
    setName(s.name);
    setDescription(s.description);
    setContent(s.content);
    setFiles(
      (s.files ?? []).map((f: SkillFile) => ({
        id: f.id,
        path: f.path,
        content: f.content,
      })),
    );
  };

  const handleSave = async () => {
    if (!skill || !canEdit) return;
    const trimmedName = name.trim();
    const trimmedDesc = description.trim();
    setSaving(true);
    try {
      const payload: UpdateSkillRequest = {
        name: trimmedName,
        description: trimmedDesc,
        content,
        files: files.filter((f) => f.path.trim()),
      };
      const updated = await api.updateSkill(skill.id, payload);
      qc.setQueryData(
        skillDetailOptions(wsId, skill.id).queryKey,
        updated,
      );
      seedFromSkill(updated);
      seededKeyRef.current = `${wsId}:${updated.id}@${updated.updated_at}`;
      setConflictPending(false);
      qc.invalidateQueries({
        queryKey: workspaceKeys.skills(wsId),
        exact: true,
      });
      qc.invalidateQueries({ queryKey: workspaceKeys.agents(wsId) });
      toast.success(t(($) => $.detail.toast_saved));
    } catch (err) {
      toast.error(err instanceof Error ? err.message : t(($) => $.detail.toast_save_failed));
    } finally {
      setSaving(false);
    }
  };

  const handleDiscard = () => {
    if (!skill) return;
    seedFromSkill(skill);
    seededKeyRef.current = `${wsId}:${skill.id}@${skill.updated_at}`;
    setConflictPending(false);
  };

  const handleDelete = async () => {
    if (!skill) return;
    setDeleting(true);
    try {
      await api.deleteSkill(skill.id);
      navigation.replace(paths.skills());
      qc.removeQueries({
        queryKey: skillDetailOptions(wsId, skill.id).queryKey,
      });
      qc.invalidateQueries({ queryKey: workspaceKeys.skills(wsId) });
      qc.invalidateQueries({ queryKey: workspaceKeys.agents(wsId) });
      toast.success(t(($) => $.detail.toast_deleted));
    } catch (err) {
      toast.error(
        err instanceof Error ? err.message : t(($) => $.detail.toast_delete_failed),
      );
      setDeleting(false);
      setConfirmDelete(false);
    }
  };

  const handleAddFile = (path: string) => {
    setFiles((prev) => [...prev, { path, content: "" }]);
    setSelectedPath(path);
    setAddingFile(false);
  };

  const handleDeleteFile = () => {
    if (selectedPath === SKILL_MD) return;
    const deletedPath = selectedPath;
    setFiles((prev) => prev.filter((f) => f.path !== selectedPath));
    setSelectedPath(SKILL_MD);
    toast.success(t(($) => $.detail.toast_file_removed, { path: deletedPath }));
  };

  const handleWorkflowSaved = (yaml: string) => {
    setFiles((prev) => {
      const existing = prev.find((f) => f.path === "workflow.yaml");
      if (existing) {
        return prev.map((f) => (f.path === "workflow.yaml" ? { ...f, content: yaml } : f));
      }
      return [...prev, { path: "workflow.yaml", content: yaml }];
    });
    qc.invalidateQueries({ queryKey: skillDetailOptions(wsId, skillId).queryKey });
    qc.invalidateQueries({ queryKey: workspaceKeys.skills(wsId) });
  };

  const handleDebugWorkflow = async () => {
    if (!skill || !fileMap.has("workflow.yaml")) return;
    setDebugStarting(true);
    try {
      const issue = await api.createIssue({
        title: `Debug workflow: ${skill.name}`,
        description: `Debug run for skill workflow \`${skill.name}\`.`,
        status: "todo",
        priority: "none",
      });
      await api.startIssueWorkflowRun(issue.id, { skill_id: skill.id });
      toast.success(t(($) => $.detail.workflow_debug.toast_started));
      navigation.push(paths.issueDetail(issue.id));
    } catch (err) {
      toast.error(err instanceof Error ? err.message : t(($) => $.detail.workflow_debug.toast_failed));
    } finally {
      setDebugStarting(false);
    }
  };

  const handleFileContentChange = (newContent: string) => {
    if (!canEdit) return;
    if (selectedPath === SKILL_MD) {
      setContent(newContent);
    } else {
      setFiles((prev) =>
        prev.map((f) =>
          f.path === selectedPath ? { ...f, content: newContent } : f,
        ),
      );
    }
  };

  const supportingQueryDown =
    !!agentsError || !!membersError || !!runtimesError;

  if (isLoading) {
    return (
      <div className="flex flex-1 min-h-0 flex-col">
        <div className="flex h-12 shrink-0 items-center gap-2 border-b px-4">
          <Skeleton className="h-4 w-16" />
          <Skeleton className="h-3 w-3 rounded" />
          <Skeleton className="h-4 w-40" />
        </div>
        <div className="space-y-3 p-6">
          <Skeleton className="h-4 w-full" />
          <Skeleton className="h-4 w-5/6" />
          <Skeleton className="h-4 w-3/4" />
        </div>
      </div>
    );
  }

  if (error || !skill) {
    return (
      <div className="flex flex-1 min-h-0 flex-col">
        <div className="flex h-12 shrink-0 items-center gap-2 border-b px-3">
          <Button
            variant="ghost"
            size="xs"
            render={<AppLink href={paths.skills()} />}
            nativeButton={false}
          >
            <ArrowLeft className="h-3 w-3" />
            {t(($) => $.detail.all_skills)}
          </Button>
        </div>
        <div className="flex flex-1 flex-col items-center justify-center gap-2 text-center">
          <AlertCircle className="h-8 w-8 text-muted-foreground/40" />
          <p className="text-sm font-medium">{t(($) => $.detail.not_found.title)}</p>
          <p className="max-w-xs text-xs text-muted-foreground">
            {error instanceof Error ? error.message : t(($) => $.detail.not_found.fallback)}
          </p>
          <AppLink
            href={paths.skills()}
            className={`${buttonVariants({ variant: "outline", size: "xs" })} mt-2`}
          >
            {t(($) => $.detail.not_found.back)}
          </AppLink>
        </div>
      </div>
    );
  }

  // --- Sub-line metadata for the header ---
  const originLabel = (() => {
    if (!origin) return null;
    if (origin.type === "runtime_local") {
      return originRuntime
        ? t(($) => $.detail.subline.origin_runtime_named, { name: originRuntime.name })
        : origin.provider
          ? t(($) => $.detail.subline.origin_runtime_provider, { provider: origin.provider })
          : t(($) => $.detail.subline.origin_runtime_unknown);
    }
    if (origin.type === "clawhub") return t(($) => $.detail.subline.origin_clawhub);
    if (origin.type === "skills_sh") return t(($) => $.detail.subline.origin_skills_sh);
    if (origin.type === "github") return t(($) => $.detail.subline.origin_github);
    return t(($) => $.detail.subline.origin_workspace);
  })();

  return (
    <div className="flex flex-1 min-h-0 flex-col">
      <BreadcrumbHeader
        segments={[{ href: paths.skills(), label: t(($) => $.page.title) }]}
        leaf={
          <span className="truncate font-mono text-xs text-foreground">
            {skill.name}
          </span>
        }
        actions={
          <>
            {!canEdit && (
              <span className="inline-flex items-center gap-1 text-xs text-muted-foreground">
                <Lock className="h-3 w-3" />
                {t(($) => $.detail.read_only)}
              </span>
            )}
            {canEdit && (
              <Tooltip>
                <TooltipTrigger
                  render={
                    <Button
                      variant="ghost"
                      size="icon-sm"
                      onClick={() => setConfirmDelete(true)}
                      className="text-muted-foreground hover:text-destructive"
                      aria-label={t(($) => $.detail.delete_aria)}
                    >
                      <Trash2 className="h-3.5 w-3.5" />
                    </Button>
                  }
                />
                <TooltipContent>{t(($) => $.detail.delete_tooltip)}</TooltipContent>
              </Tooltip>
            )}
          </>
        }
      />

      {!canEdit && (
        <div className="px-4 pt-3">
          <CapabilityBanner
            reason={skillPermissions.canEdit.reason}
            resource="skill"
            ownerName={creator?.name}
          />
        </div>
      )}

      {supportingQueryDown && (
        <div
          role="status"
          className="flex shrink-0 items-start gap-2 border-b bg-warning/10 px-4 py-2 text-xs text-muted-foreground"
        >
          <AlertTriangle className="mt-0.5 h-3.5 w-3.5 shrink-0 text-warning" />
          <span>{t(($) => $.detail.supporting_data_warning)}</span>
        </div>
      )}

      <div className="flex h-10 shrink-0 items-center gap-1 border-b px-3">
        {(["content", "workflow"] as const).map((tab) => (
          <Button
            key={tab}
            type="button"
            size="xs"
            variant={activeTab === tab ? "secondary" : "ghost"}
            onClick={() => setActiveTab(tab)}
          >
            {tab === "content" ? "Content" : "Workflow"}
          </Button>
        ))}
      </div>

      {activeTab === "content" ? (
        <div className="flex min-h-0 flex-1 flex-col overflow-y-auto md:flex-row md:overflow-hidden">
          <aside className="flex max-h-44 w-full shrink-0 flex-col border-b bg-muted/10 md:max-h-none md:w-60 md:border-b-0 md:border-r">
            <div className="flex h-10 shrink-0 items-center justify-between border-b px-3">
              <span className="text-xs font-medium uppercase tracking-wider text-muted-foreground">
                {t(($) => $.detail.files_label, { count: totalFileCount(skill) })}
              </span>
              {canEdit && (
                <Tooltip>
                  <TooltipTrigger
                    render={
                      <Button
                        type="button"
                        variant="ghost"
                        size="icon-sm"
                        onClick={() => setAddingFile(true)}
                        className="text-muted-foreground"
                        aria-label={t(($) => $.detail.add_file_aria)}
                      >
                        <Plus className="h-3.5 w-3.5" />
                      </Button>
                    }
                  />
                  <TooltipContent>{t(($) => $.detail.add_file_tooltip)}</TooltipContent>
                </Tooltip>
              )}
            </div>
            {addingFile && (
              <AddFileInline
                existingPaths={filePaths}
                onAdd={handleAddFile}
                onCancel={() => setAddingFile(false)}
              />
            )}
            <div className="flex-1 overflow-y-auto">
              <FileTree
                filePaths={filePaths}
                selectedPath={selectedPath}
                onSelect={setSelectedPath}
              />
            </div>
            {selectedPath !== SKILL_MD && canEdit && (
              <div className="border-t px-3 py-2">
                <Button
                  type="button"
                  variant="ghost"
                  size="xs"
                  onClick={handleDeleteFile}
                  className="text-muted-foreground hover:text-destructive"
                >
                  <Trash2 className="h-3 w-3" />
                  {t(($) => $.detail.delete_file)}
                </Button>
              </div>
            )}
          </aside>

          <section className="flex min-h-[32rem] min-w-0 flex-1 flex-col bg-background md:min-h-0">
            <SkillHeaderCompact
              name={name}
              description={description}
              canEdit={canEdit}
              originLabel={originLabel}
              originType={origin?.type}
              updatedLabel={t(($) => $.detail.subline.updated_label, {
                when: timeAgo(skill.updated_at),
              })}
              createdLabel={`${t(($) => $.detail.sidebar.created)} ${timeAgo(skill.created_at)}`}
              creatorName={creator?.name}
              creatorAvatarUrl={creator ? resolvePublicFileUrl(creator.avatar_url) : null}
              fileCountLabel={`${t(($) => $.detail.sidebar.files)} ${totalFileCount(skill)}`}
              idLabel={`${t(($) => $.detail.sidebar.id)} ${skill.id.slice(0, 8)}...`}
              idTitle={skill.id}
              onNameChange={setName}
              onDescriptionChange={setDescription}
            />

            {/* Conflict banner */}
            {conflictPending && canEdit && (
              <div
                role="status"
                aria-live="polite"
                className="flex shrink-0 items-start gap-2 border-b border-warning/30 bg-warning/10 px-4 py-2 text-xs"
              >
                <AlertTriangle className="mt-0.5 h-3.5 w-3.5 shrink-0 text-warning" />
                <div className="flex-1">
                  <div className="font-medium text-foreground">
                    {t(($) => $.detail.conflict_banner.title)}
                  </div>
                  <div className="mt-0.5 text-muted-foreground">
                    {t(($) => $.detail.conflict_banner.body)}
                  </div>
                </div>
              </div>
            )}

            {/* File viewer */}
            <div className="min-h-0 flex-1">
              <FileViewer
                key={selectedPath}
                path={selectedPath}
                content={selectedContent}
                onChange={handleFileContentChange}
                canEdit={canEdit}
              />
            </div>

            {/* Save bar */}
            {isDirty && canEdit && (
              <div
                role="status"
                aria-live="polite"
                className="flex shrink-0 flex-wrap items-center gap-2 border-t bg-muted/30 px-4 py-2"
              >
                <span className="h-1.5 w-1.5 rounded-full bg-brand" />
                <span className="text-xs text-muted-foreground">
                  {t(($) => $.detail.save_bar.unsaved)}
                </span>
                <div className="ml-auto flex items-center gap-1.5">
                  <Button
                    type="button"
                    variant="ghost"
                    size="xs"
                    onClick={handleDiscard}
                  >
                    {t(($) => $.detail.save_bar.discard)}
                  </Button>
                  <Button
                    type="button"
                    size="xs"
                    onClick={handleSave}
                    disabled={saving || !name.trim()}
                  >
                    {saving ? (
                      <>
                        <Loader2 className="h-3 w-3 animate-spin" />
                        {t(($) => $.detail.save_bar.saving)}
                      </>
                    ) : (
                      <>
                        <Save className="h-3 w-3" />
                        {t(($) => $.detail.save_bar.save)}
                      </>
                    )}
                  </Button>
                </div>
              </div>
            )}
          </section>
        </div>
      ) : (
        <div className="flex flex-1 min-h-0 flex-col bg-background">
          {fileMap.has("workflow.yaml") && (
            <div className="flex min-h-10 shrink-0 items-center justify-between gap-3 border-b bg-muted/20 px-4 py-1.5">
              <div className="flex min-w-0 items-center gap-2">
                <span className="size-1.5 shrink-0 rounded-full bg-brand" aria-hidden="true" />
                <div className="truncate text-xs font-medium text-foreground">
                  {t(($) => $.detail.workflow_debug.title)}
                </div>
                <div className="hidden truncate text-xs text-muted-foreground sm:block">
                  {t(($) => $.detail.workflow_debug.description)}
                </div>
              </div>
              <Button type="button" size="xs" variant="secondary" onClick={handleDebugWorkflow} disabled={debugStarting}>
                {debugStarting ? t(($) => $.detail.workflow_debug.starting) : t(($) => $.detail.workflow_debug.action)}
              </Button>
            </div>
          )}
          <WorkflowEditor
            skillId={skill.id}
            initialYaml={fileMap.get("workflow.yaml")}
            agents={agents}
            onSaved={handleWorkflowSaved}
          />
        </div>
      )}

      {/* Delete confirmation */}
      <Dialog
        open={confirmDelete}
        onOpenChange={(v) => {
          if (!v) setConfirmDelete(false);
        }}
      >
        <DialogContent className="sm:max-w-md">
          <DialogHeader>
            <DialogTitle>{t(($) => $.detail.delete_dialog.title)}</DialogTitle>
            <DialogDescription>
              {skillAgents.length > 0
                ? t(($) => $.detail.delete_dialog.description_with_agents, {
                    name: skill.name,
                    count: skillAgents.length,
                  })
                : t(($) => $.detail.delete_dialog.description_no_agents, {
                    name: skill.name,
                  })}
            </DialogDescription>
          </DialogHeader>
          <div className="rounded-md bg-destructive/10 px-3 py-2 text-xs text-destructive">
            {t(($) => $.detail.delete_dialog.warning)}
          </div>
          <DialogFooter>
            <Button
              type="button"
              variant="ghost"
              onClick={() => setConfirmDelete(false)}
              disabled={deleting}
            >
              {t(($) => $.detail.delete_dialog.cancel)}
            </Button>
            <Button
              type="button"
              variant="destructive"
              onClick={handleDelete}
              disabled={deleting}
            >
              {deleting ? (
                <>
                  <Loader2 className="h-3 w-3 animate-spin" />
                  {t(($) => $.detail.delete_dialog.deleting)}
                </>
              ) : (
                <>
                  <Trash2 className="h-3 w-3" />
                  {t(($) => $.detail.delete_dialog.confirm)}
                </>
              )}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  );
}
