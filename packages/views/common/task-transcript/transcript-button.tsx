"use client";

import { useCallback, useEffect, useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { Loader2, ScrollText } from "lucide-react";
import { cn } from "@multica/ui/lib/utils";
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from "@multica/ui/components/ui/tooltip";
import { api } from "@multica/core/api";
import { taskMessagesOptions } from "@multica/core/chat/queries";
import type { AgentTask } from "@multica/core/types/agent";
import { AgentTranscriptDialog } from "./agent-transcript-dialog";
import { buildTimeline, type TimelineItem } from "./build-timeline";

interface TranscriptButtonProps {
  task: AgentTask;
  agentName: string;
  /**
   * Pre-loaded timeline. When provided the button skips the fetch and opens
   * the dialog immediately — used by the live card where `items` already
   * accumulate via WS. Omit for terminal tasks; the button will fetch via
   * `api.listTaskMessages` on the first click and cache the result.
   */
  items?: TimelineItem[];
  isLive?: boolean;
  className?: string;
  title?: string;
  /**
   * Optional content rendered above the transcript event list. Used to
   * surface autopilot webhook payloads inline with the run history.
   */
  headerSlot?: React.ReactNode;
}

/**
 * Compact icon-button that opens the full transcript dialog. Used on any
 * surface that lists agent tasks (issue activity card, agent detail
 * activity tab). Owns its own dialog state and lazy-load — the parent
 * just drops it in.
 */
export function TranscriptButton({
  task,
  agentName,
  items: providedItems,
  isLive = false,
  className,
  title = "View transcript",
  headerSlot,
}: TranscriptButtonProps) {
  const [open, setOpen] = useState(false);
  const [loading, setLoading] = useState(false);
  const [loadedItems, setLoadedItems] = useState<TimelineItem[] | null>(null);
  const shouldUseLiveQuery = isLive && providedItems === undefined;
  const { data: liveTaskMessages } = useQuery({
    ...taskMessagesOptions(task.id),
    enabled: shouldUseLiveQuery && open,
  });

  // Provided-items mode: parent owns the timeline, we just render it.
  // Running task rows subscribe to the shared task-messages cache so
  // task:message WS updates keep an already-open transcript live.
  // Lazy mode for terminal tasks still fetches once and caches.
  const items =
    providedItems ??
    (shouldUseLiveQuery ? buildTimeline(liveTaskMessages ?? []) : loadedItems) ??
    [];

  const handleClick = useCallback(
    (e: React.MouseEvent) => {
      e.preventDefault();
      e.stopPropagation();
      if (providedItems !== undefined || shouldUseLiveQuery || loadedItems !== null) {
        setOpen(true);
        return;
      }
      setLoading(true);
      api
        .listTaskMessages(task.id)
        .then((msgs) => {
          setLoadedItems(buildTimeline(msgs));
          setOpen(true);
        })
        .catch((err) => {
          console.error(err);
          setLoadedItems([]);
          setOpen(true);
        })
        .finally(() => setLoading(false));
    },
    [providedItems, shouldUseLiveQuery, loadedItems, task.id],
  );

  useEffect(() => {
    if (!open) return;

    const handleGlobalNavigate = () => {
      setOpen(false);
    };

    window.addEventListener("multica:navigate", handleGlobalNavigate);
    return () => {
      window.removeEventListener("multica:navigate", handleGlobalNavigate);
    };
  }, [open]);

  return (
    <>
      <Tooltip>
        <TooltipTrigger
          render={<button type="button" />}
          onClick={handleClick}
          disabled={loading}
          aria-label={title}
          className={cn(
            "flex items-center justify-center rounded p-1 text-muted-foreground hover:text-foreground hover:bg-accent/50 transition-colors disabled:opacity-50",
            className,
          )}
        >
          {loading ? (
            <Loader2 className="h-3.5 w-3.5 animate-spin" />
          ) : (
            <ScrollText className="h-3.5 w-3.5" />
          )}
        </TooltipTrigger>
        <TooltipContent>{title}</TooltipContent>
      </Tooltip>

      {open && (
        <AgentTranscriptDialog
          open={open}
          onOpenChange={setOpen}
          task={task}
          items={items}
          agentName={agentName}
          isLive={isLive}
          headerSlot={headerSlot}
        />
      )}
    </>
  );
}
