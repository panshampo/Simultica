"use client";

import type { WorkflowRunNode } from "@multica/core/workflow/types";
import { AppLink } from "../../navigation";

export function WorkflowRunNodeDetailPanel({
  node,
  issueHref,
}: {
  node?: WorkflowRunNode | null;
  issueHref?: (issueId: string) => string;
}) {
  if (!node) {
    return (
      <section className="rounded-lg border border-dashed bg-background/60 p-4 text-sm text-muted-foreground">
        Select a run node to inspect its projection.
      </section>
    );
  }

  const carrierIssueId = node.carrier_kind === "issue" ? issueIdFromCarrierRef(node.carrier_ref) : null;

  return (
    <section className="overflow-hidden rounded-lg border bg-card">
      <div className="border-b px-3 py-2">
        <h2 className="text-sm font-medium">Node detail</h2>
      </div>
      <div className="space-y-3 p-3">
        <div className="grid grid-cols-2 gap-2 text-xs">
          <Detail label="Node" value={node.node_id} mono />
          <Detail label="Status" value={node.status} />
          <Detail label="Type" value={node.node_type} />
          <Detail label="Dispatch" value={node.dispatch} />
          <Detail label="Attempt" value={String(node.attempt)} />
          <Detail label="Updated" value={formatDateTime(node.updated_at)} />
        </div>
        {carrierIssueId && issueHref && (
          <AppLink
            href={issueHref(carrierIssueId)}
            className="inline-flex h-7 items-center justify-center rounded-[min(var(--radius-md),12px)] border border-border bg-background px-2.5 text-[0.8rem] font-medium hover:bg-muted hover:text-foreground"
          >
            Open sub-issue
          </AppLink>
        )}
        <JsonBlock label="Carrier ref" value={node.carrier_ref} />
        <JsonBlock label="Input" value={node.input_snapshot} />
        <JsonBlock label="Output" value={node.output_snapshot} />
        <JsonBlock label="Error" value={node.error} />
        <JsonBlock label="Logs" value={node.logs} />
      </div>
    </section>
  );
}

function issueIdFromCarrierRef(ref: unknown): string | null {
  if (!ref || typeof ref !== "object") return null;
  const rec = ref as Record<string, unknown>;
  for (const key of ["issue_id", "sub_issue_id", "issueId", "subIssueId"]) {
    const value = rec[key];
    if (typeof value === "string" && value.trim() !== "") return value;
  }
  return null;
}

function Detail({ label, value, mono = false }: { label: string; value?: string | null; mono?: boolean }) {
  return (
    <div className="min-w-0 rounded-md bg-muted/40 px-2 py-1.5">
      <div className="text-[11px] uppercase text-muted-foreground">{label}</div>
      <div className={mono ? "truncate font-mono text-xs" : "truncate text-xs"}>{value || "-"}</div>
    </div>
  );
}

function JsonBlock({ label, value }: { label: string; value: unknown }) {
  return (
    <div>
      <div className="mb-1 text-[11px] font-medium uppercase text-muted-foreground">{label}</div>
      <pre className="max-h-40 overflow-auto rounded-md bg-muted/45 p-2 font-mono text-[11px] leading-4 text-muted-foreground">
        {formatJson(value)}
      </pre>
    </div>
  );
}

function formatJson(value: unknown): string {
  if (value === undefined || value === null || (Array.isArray(value) && value.length === 0)) return "-";
  if (typeof value === "string") return value;
  return JSON.stringify(value, null, 2);
}

function formatDateTime(value: string): string {
  return new Intl.DateTimeFormat(undefined, {
    month: "short",
    day: "numeric",
    hour: "2-digit",
    minute: "2-digit",
  }).format(new Date(value));
}
