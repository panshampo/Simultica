import { useMemo, useState } from "react";
import type { WorkflowEdge, WorkflowStateField } from "@multica/core/workflow/types";
import { Button } from "@multica/ui/components/ui/button";
import { Input } from "@multica/ui/components/ui/input";
import { tryParseConditionExpression } from "../lib/condition-builder";

function nodeOptionLabel(id: string, nodeLabels: Record<string, string>): string {
  if (id === "START" || id === "END") return id;
  return nodeLabels[id] ? `${id} · ${nodeLabels[id]}` : id;
}

export function EdgeInspector({
  edge,
  stateFields,
  nodeIds,
  nodeLabels = {},
  onChange,
  onRemove,
}: {
  edge: WorkflowEdge;
  stateFields: WorkflowStateField[];
  nodeIds: string[];
  nodeLabels?: Record<string, string>;
  onChange: (patch: Partial<WorkflowEdge>) => void;
  onRemove: () => void;
}) {
  const [mode, setMode] = useState<"builder" | "raw">(() => (edge.condition && !tryParseConditionExpression(edge.condition) ? "raw" : "builder"));
  const parsed = useMemo(() => (edge.condition ? tryParseConditionExpression(edge.condition) : null), [edge.condition]);
  const fromTargets = ["START", ...nodeIds];
  const toTargets = [...nodeIds, "END"];

  return (
    <div className="space-y-3">
      <label className="space-y-1 text-xs text-muted-foreground">
        <span>From</span>
        <select aria-label="From" className="h-9 w-full rounded-md border bg-background px-2 text-sm text-foreground" value={edge.from} onChange={(event) => onChange({ from: event.target.value })}>
          {fromTargets.map((id) => <option key={id} value={id}>{nodeOptionLabel(id, nodeLabels)}</option>)}
        </select>
      </label>

      <label className="space-y-1 text-xs text-muted-foreground">
        <span>To</span>
        <select aria-label="To" className="h-9 w-full rounded-md border bg-background px-2 text-sm text-foreground" value={edge.to} onChange={(event) => onChange({ to: event.target.value })}>
          {toTargets.map((id) => <option key={id} value={id}>{nodeOptionLabel(id, nodeLabels)}</option>)}
        </select>
      </label>

      <div className="flex gap-2">
        <Button type="button" size="xs" variant={mode === "builder" ? "default" : "outline"} onClick={() => setMode("builder")}>
          Builder
        </Button>
        <Button type="button" size="xs" variant={mode === "raw" ? "default" : "outline"} onClick={() => setMode("raw")}>
          Raw expression
        </Button>
      </div>

      {mode === "builder" ? (
        <div className="rounded-md border p-2 text-xs">
          <div className="font-medium">Condition builder</div>
          <p className="mt-1 text-muted-foreground">
            Builder supports common one-level rules. Use raw expression for nested logic.
          </p>
          <div className="mt-2 text-muted-foreground">
            {parsed ? edge.condition : "No builder-compatible condition yet."}
          </div>
          <select
            className="mt-2 h-9 w-full rounded-md border bg-background px-2 text-sm text-foreground"
            onChange={(event) => {
              const field = event.target.value;
              if (field) onChange({ condition: `${field} == ""` });
            }}
          >
            <option value="">Start with field</option>
            {stateFields.map((field) => <option key={field.name} value={field.name}>{field.name} · {field.type}</option>)}
          </select>
        </div>
      ) : (
        <label className="space-y-1 text-xs text-muted-foreground">
          <span>Condition expression</span>
          <Input aria-label="Condition expression" value={edge.condition ?? ""} onChange={(event) => onChange({ condition: event.target.value || undefined })} />
        </label>
      )}

      <label className="space-y-1 text-xs text-muted-foreground">
        <span>Else target</span>
        <select aria-label="Else target" className="h-9 w-full rounded-md border bg-background px-2 text-sm text-foreground" value={edge.else ?? ""} onChange={(event) => onChange({ else: event.target.value || undefined })}>
          <option value="">No else target</option>
          {toTargets.map((id) => <option key={id} value={id}>{nodeOptionLabel(id, nodeLabels)}</option>)}
        </select>
      </label>

      <Button type="button" variant="destructive" size="sm" onClick={onRemove}>
        Remove edge
      </Button>
    </div>
  );
}
