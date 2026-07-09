"use client";

import { useState } from "react";
import { Maximize2, X } from "lucide-react";

/**
 * Product-domain diagram for "Product domain model".
 *
 * It follows the same boundary-card visual language as ArchitectureDiagram:
 * large rounded panels, muted server/product surfaces, small typographic
 * labels, and relationship conveyed mostly by layout instead of dense arrows.
 */
export function ProductDomainDiagram() {
  const [expanded, setExpanded] = useState(false);

  return (
    <>
      <figure className="not-prose my-8 overflow-hidden rounded-xl border border-border/70 bg-muted/20">
        <div className="flex items-center justify-between border-b border-border/70 bg-background/80 px-4 py-2">
          <div>
            <div className="text-[10px] font-semibold uppercase tracking-[0.14em] text-muted-foreground">
              Product domain map
            </div>
            <div className="mt-0.5 text-sm font-medium">
              Issue, capability, workflow control, and execution
            </div>
          </div>
          <button
            type="button"
            aria-label="Expand product domain diagram"
            className="inline-flex h-8 items-center gap-1.5 rounded-md px-2.5 text-xs text-muted-foreground hover:bg-muted hover:text-foreground"
            onClick={() => setExpanded(true)}
          >
            <Maximize2 className="size-3.5" aria-hidden="true" />
            Expand
          </button>
        </div>
        <DomainMap compact />
      </figure>

      {expanded && (
        <div
          role="dialog"
          aria-label="Expanded product domain diagram"
          className="fixed inset-0 z-[9999] flex flex-col bg-background"
        >
          <div className="flex h-12 shrink-0 items-center justify-between border-b px-4">
            <div>
              <div className="text-sm font-medium">Product domain map</div>
              <div className="text-xs text-muted-foreground">Expanded diagram</div>
            </div>
            <button
              type="button"
              aria-label="Close product domain diagram"
              className="inline-flex size-8 items-center justify-center rounded-md text-muted-foreground hover:bg-muted hover:text-foreground"
              onClick={() => setExpanded(false)}
            >
              <X className="size-4" aria-hidden="true" />
            </button>
          </div>
          <div className="min-h-0 flex-1 overflow-auto p-6">
            <div className="mx-auto min-w-[1080px] max-w-7xl">
              <DomainMap />
            </div>
          </div>
        </div>
      )}
    </>
  );
}

function DomainMap({ compact = false }: { compact?: boolean }) {
  if (compact) {
    return (
      <div className="p-5 md:p-6">
        <div className="grid gap-4 lg:grid-cols-[minmax(0,0.95fr)_minmax(0,1.35fr)]">
          <GoalPanel compact />
          <WorkflowPanel compact />
        </div>
        <div className="mt-4">
          <ExecutionPanel compact />
        </div>
        <Legend compact />
      </div>
    );
  }

  return (
    <div className="p-6">
      <div className="grid gap-4 md:grid-cols-[1.05fr_auto_1.25fr_auto_1.05fr] md:items-stretch">
        <GoalPanel />
        <Connector label="plans" />
        <WorkflowPanel />
        <Connector label="runs" />
        <ExecutionPanel />
      </div>
      <Legend />
    </div>
  );
}

function GoalPanel({ compact = false }: { compact?: boolean }) {
  return (
    <BoundaryPanel
      eyebrow="Goal intake"
      title="Issue surface"
      tone="warm"
      compact={compact}
      footer="The user goal enters here. Templates only help create issues."
    >
      <Stack>
        <EntityCard title="Issue" subtitle="Goal + discussion + status" />
        <EntityCard title="IssueTemplate" subtitle="Direct issue creation" accent="template" />
        <EntityCard title="Automation" subtitle="References IssueTemplate" accent="template" />
      </Stack>
    </BoundaryPanel>
  );
}

function WorkflowPanel({ compact = false }: { compact?: boolean }) {
  return (
    <BoundaryPanel
      eyebrow="Workflow control plane"
      title="WorkflowCase"
      tone="brand"
      compact={compact}
      footer="The workflow is a managed product object, not a one-off task log."
    >
      <div className="grid gap-3 sm:grid-cols-3">
        <EntityCard title="Metadata" subtitle="title / owner / lifecycle" />
        <EntityCard title="Definition draft" subtitle="editable workflow" />
        <EntityCard title="Definition version" subtitle="immutable snapshot" />
      </div>
      <div className="mt-4 rounded-lg border border-brand/20 bg-brand/[0.04] p-3">
        <SectionLabel>Capability input</SectionLabel>
        <div className="mt-2 grid gap-2 sm:grid-cols-2">
          <Pill>Agent</Pill>
          <Pill>Skill workflow shape</Pill>
        </div>
      </div>
    </BoundaryPanel>
  );
}

function ExecutionPanel({ compact = false }: { compact?: boolean }) {
  return (
    <BoundaryPanel
      eyebrow="Execution record"
      title="WorkflowRun"
      tone="muted"
      compact={compact}
      footer="UI reads run state here. Runtime checkpoints are internal."
    >
      <div className={compact ? "grid gap-3 md:grid-cols-[1fr_1fr_1.25fr]" : ""}>
        <EntityCard title="WorkflowRun" subtitle="version + status + current node" accent="run" />
        <EntityCard title="Run node" subtitle="input / output / error / carrier" accent="run" />
        <div className={compact ? "grid gap-2 sm:grid-cols-3 md:grid-cols-1 lg:grid-cols-3" : "mt-3 grid gap-2 sm:grid-cols-3"}>
          <Pill>subissue</Pill>
          <Pill>main_issue_task</Pill>
          <Pill>inline</Pill>
        </div>
      </div>
    </BoundaryPanel>
  );
}

function Legend({ compact = false }: { compact?: boolean }) {
  return (
    <div className={compact ? "mt-4 grid gap-3 lg:grid-cols-3" : "mt-4 grid gap-3 md:grid-cols-3"}>
      <LegendItem
        label="IssueTemplate boundary"
        text="Only creates Issues directly or through Automation."
      />
      <LegendItem
        label="Skill boundary"
        text="Reusable capability and workflow shape for agents."
      />
      <LegendItem
        label="Run boundary"
        text="One execution snapshot with user-visible progress."
      />
    </div>
  );
}

function BoundaryPanel({
  eyebrow,
  title,
  tone,
  footer,
  compact = false,
  children,
}: {
  eyebrow: string;
  title: string;
  tone: "warm" | "brand" | "muted";
  footer: string;
  compact?: boolean;
  children: React.ReactNode;
}) {
  return (
    <div className={panelClass(tone, compact)} data-domain-panel={title}>
      <div className="mb-5">
        <div className={eyebrowClass(tone)}>{eyebrow}</div>
        <div className="mt-1 text-lg font-semibold tracking-tight">{title}</div>
      </div>
      <div className={compact ? "flex-1" : "min-h-[12rem] flex-1"}>{children}</div>
      <div className={footerClass(tone)}>{footer}</div>
    </div>
  );
}

function EntityCard({
  title,
  subtitle,
  accent = "default",
}: {
  title: string;
  subtitle: string;
  accent?: "default" | "template" | "run";
}) {
  return (
    <div className={`rounded-lg border bg-background/90 p-3 shadow-sm ${entityAccentClass(accent)}`}>
      <div className="text-sm font-semibold">{title}</div>
      <div className="mt-1 text-xs leading-relaxed text-muted-foreground">{subtitle}</div>
    </div>
  );
}

function Stack({ children }: { children: React.ReactNode }) {
  return <div className="space-y-3">{children}</div>;
}

function Connector({ label }: { label: string }) {
  return (
    <div className="hidden items-center justify-center md:flex">
      <div className="flex flex-col items-center gap-2 text-muted-foreground/70">
        <div className="h-px w-12 bg-border" />
        <div className="rounded-full border bg-background px-2 py-0.5 text-[10px] font-medium uppercase tracking-[0.12em]">
          {label}
        </div>
        <div className="h-px w-12 bg-border" />
      </div>
    </div>
  );
}

function SectionLabel({ children }: { children: React.ReactNode }) {
  return (
    <div className="text-[10px] font-medium uppercase tracking-[0.1em] text-muted-foreground/70">
      {children}
    </div>
  );
}

function Pill({ children }: { children: React.ReactNode }) {
  return (
    <span className="inline-flex items-center justify-center rounded-md border border-border/70 bg-background px-2 py-1 text-[11px] font-medium text-foreground">
      {children}
    </span>
  );
}

function LegendItem({ label, text }: { label: string; text: string }) {
  return (
    <div className="rounded-lg border border-border/70 bg-background/70 p-3">
      <div className="text-xs font-semibold">{label}</div>
      <div className="mt-1 text-xs leading-relaxed text-muted-foreground">{text}</div>
    </div>
  );
}

function panelClass(tone: "warm" | "brand" | "muted", compact = false): string {
  const base = `flex flex-col rounded-lg border p-5 ${compact ? "min-h-[18rem]" : "min-h-[24rem]"}`;
  if (tone === "brand") return `${base} border-brand/30 bg-brand/[0.035]`;
  if (tone === "warm") return `${base} border-amber-500/25 bg-amber-50/30 dark:bg-amber-950/10`;
  return `${base} border-border/70 bg-muted/25`;
}

function eyebrowClass(tone: "warm" | "brand" | "muted"): string {
  const base = "text-[11px] font-semibold uppercase tracking-[0.14em]";
  if (tone === "brand") return `${base} text-brand`;
  if (tone === "warm") return `${base} text-amber-700 dark:text-amber-300`;
  return `${base} text-muted-foreground`;
}

function footerClass(tone: "warm" | "brand" | "muted"): string {
  const border = tone === "brand" ? "border-brand/20" : "border-border/60";
  return `mt-5 border-t ${border} pt-3 text-center text-[11px] uppercase tracking-[0.08em] text-muted-foreground`;
}

function entityAccentClass(accent: "default" | "template" | "run"): string {
  if (accent === "template") return "border-pink-500/30";
  if (accent === "run") return "border-slate-500/25";
  return "border-border/70";
}
