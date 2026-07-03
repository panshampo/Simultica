import type { Agent } from "@multica/core/types";
import type { WorkflowNode, WorkflowStateField } from "@multica/core/workflow/types";
import { Button } from "@multica/ui/components/ui/button";
import { Input } from "@multica/ui/components/ui/input";
import { Textarea } from "@multica/ui/components/ui/textarea";
import { getNodeTypeConfig } from "../lib/schema-registry";

export function NodeInspector({
  node,
  stateFields,
  agents,
  onChange,
  onRemove,
}: {
  node: WorkflowNode;
  stateFields: WorkflowStateField[];
  agents: Pick<Agent, "id" | "name">[];
  onChange: (patch: Partial<WorkflowNode>) => void;
  onRemove: () => void;
}) {
  const config = getNodeTypeConfig(node.type);

  return (
    <div className="space-y-3">
      <label className="space-y-1 text-xs text-muted-foreground">
        <span>ID</span>
        <Input value={node.id} onChange={(event) => onChange({ id: event.target.value })} />
      </label>

      <label className="space-y-1 text-xs text-muted-foreground">
        <span>Type</span>
        <select
          className="h-9 w-full rounded-md border bg-background px-2 text-sm text-foreground"
          value={node.type}
          onChange={(event) => onChange({ type: event.target.value as WorkflowNode["type"] })}
        >
          {["llm", "agent", "subissue", "main_agent", "final_response", "router", "transform", "condition", "merge", "code", "http"].map((type) => (
            <option key={type} value={type}>{type}</option>
          ))}
        </select>
      </label>

      <label className="space-y-1 text-xs text-muted-foreground">
        <span>Dispatch</span>
        <select
          className="h-9 w-full rounded-md border bg-background px-2 text-sm text-foreground"
          value={node.dispatch ?? config.defaultDispatch}
          onChange={(event) => onChange({ dispatch: event.target.value as WorkflowNode["dispatch"] })}
        >
          {["subissue", "inline", "main_issue_task"].map((dispatch) => <option key={dispatch} value={dispatch}>{dispatch}</option>)}
        </select>
      </label>

      {config.supportsAgentRoute && (
        <label className="space-y-1 text-xs text-muted-foreground">
          <span>Agent</span>
          <select
            className="h-9 w-full rounded-md border bg-background px-2 text-sm text-foreground"
            value={node.config?.agent ?? node.agent ?? ""}
            onChange={(event) => onChange({ agent: event.target.value || undefined, config: { ...node.config, agent: event.target.value || undefined } })}
          >
            <option value="">No agent</option>
            {agents.map((agent) => <option key={agent.id} value={agent.name}>{agent.name}</option>)}
          </select>
        </label>
      )}

      {config.supportsOutputs && (
        <label className="space-y-1 text-xs text-muted-foreground">
          <span>Outputs</span>
          <select
            aria-label="Outputs"
            className="h-9 w-full rounded-md border bg-background px-2 text-sm text-foreground"
            value={node.outputs?.[0] ?? ""}
            onChange={(event) => onChange({ outputs: event.target.value ? [event.target.value] : [] })}
          >
            <option value="">No output</option>
            {stateFields.map((field) => <option key={field.name} value={field.name}>{field.name} · {field.type}</option>)}
          </select>
        </label>
      )}

      {config.supportsSystemPrompt && (
        <label className="space-y-1 text-xs text-muted-foreground">
          <span>System prompt</span>
          <Textarea
            aria-label="System prompt"
            className="min-h-28"
            value={node.config?.system ?? ""}
            onChange={(event) => onChange({ config: { ...node.config, system: event.target.value } })}
          />
        </label>
      )}

      {config.supportsTransformMap && (
        <div className="rounded-md border p-2 text-xs">
          <div className="font-medium">Transform map</div>
          <p className="mt-1 text-muted-foreground">
            Set fixed state values in this inline step. Advanced key/value editing comes after the first structured pass.
          </p>
        </div>
      )}

      <Button type="button" variant="destructive" size="sm" onClick={onRemove}>
        Remove node
      </Button>
    </div>
  );
}
