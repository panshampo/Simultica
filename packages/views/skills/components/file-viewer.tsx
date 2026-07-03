"use client";

import { useEffect, useMemo, useState } from "react";
import { Pencil, Eye } from "lucide-react";
import { Button } from "@multica/ui/components/ui/button";
import { Tooltip, TooltipTrigger, TooltipContent } from "@multica/ui/components/ui/tooltip";
import { useT } from "../../i18n";
import { getSkillFileRenderer, type SkillFileMode } from "./skill-file-renderers";

// ---------------------------------------------------------------------------
// File viewer
// ---------------------------------------------------------------------------

export function FileViewer({
  path,
  content,
  onChange,
  canEdit = true,
}: {
  path: string;
  content: string;
  onChange: (content: string) => void;
  canEdit?: boolean;
}) {
  const { t } = useT("skills");
  const renderer = useMemo(() => getSkillFileRenderer(path), [path]);
  const [mode, setMode] = useState<SkillFileMode>(renderer.defaultMode);
  const isEditing = mode === "edit";
  const canPreview = renderer.canPreview && Boolean(renderer.Preview);

  useEffect(() => {
    setMode(renderer.defaultMode);
  }, [path, renderer.defaultMode]);

  useEffect(() => {
    if (!canEdit && mode === "edit") setMode("preview");
  }, [canEdit, mode]);

  const Preview = renderer.Preview;
  const Body = isEditing || !Preview ? renderer.Editor : Preview;

  return (
    <div className="flex h-full min-h-0 flex-col overflow-hidden bg-background">
      {/* File header */}
      <div className="flex min-h-10 shrink-0 items-center justify-between gap-3 border-b bg-muted/15 px-4">
        <div className="flex min-w-0 items-center gap-2">
          <span className="truncate font-mono text-xs text-muted-foreground">
            {path}
          </span>
          <span className="shrink-0 rounded bg-muted px-1.5 py-0.5 text-[10px] font-medium uppercase tracking-wide text-muted-foreground">
            {renderer.label}
          </span>
        </div>
        <div className="flex items-center gap-1">
          {canPreview && (
            <Tooltip>
              <TooltipTrigger
                render={
                  <Button
                    variant="ghost"
                    size="icon-sm"
                    onClick={() => setMode(isEditing ? "preview" : "edit")}
                    disabled={!canEdit && !isEditing}
                    className="text-muted-foreground"
                    aria-label={
                      isEditing
                        ? t(($) => $.file_viewer.preview_tooltip)
                        : t(($) => $.file_viewer.edit_tooltip)
                    }
                  >
                    {isEditing ? (
                      <Eye className="h-3.5 w-3.5" />
                    ) : (
                      <Pencil className="h-3.5 w-3.5" />
                    )}
                  </Button>
                }
              />
              <TooltipContent>
                {isEditing
                  ? t(($) => $.file_viewer.preview_tooltip)
                  : t(($) => $.file_viewer.edit_tooltip)}
              </TooltipContent>
            </Tooltip>
          )}
        </div>
      </div>

      {/* File content */}
      <div className="min-h-0 flex-1">
        <Body path={path} content={content} canEdit={canEdit} onChange={onChange} />
      </div>
    </div>
  );
}
