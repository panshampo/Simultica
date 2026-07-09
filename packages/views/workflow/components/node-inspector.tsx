import type { Agent } from "@multica/core/types";
import type { WorkflowDispatch, WorkflowNode, WorkflowNodeType, WorkflowStateField } from "@multica/core/workflow/types";
import { Button } from "@multica/ui/components/ui/button";
import { Input } from "@multica/ui/components/ui/input";
import { Textarea } from "@multica/ui/components/ui/textarea";
import { getNodeTypeConfig } from "../lib/schema-registry";

const NODE_TYPE_OPTIONS: WorkflowNodeType[] = [
  "llm",
  "agent",
  "subissue",
  "main_agent",
  "final_response",
  "router",
  "transform",
  "condition",
  "merge",
  "code",
  "http",
];

const DISPATCH_LABELS: Record<WorkflowDispatch, string> = {
  subissue: "Sub-issue",
  direct_subagent: "Direct sub-agent",
  inline: "Inline",
  main_issue_task: "Main issue task",
  human_gate: "Review step",
};

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
  const currentAgentValue = node.config?.agent ?? node.agent ?? "";
  const selectedAgent = agents.find((agent) => agent.id === currentAgentValue)
    ?? agents.find((agent) => agent.name === currentAgentValue);
  const hasCurrentAgentOption = !currentAgentValue || agents.some((agent) => agent.id === currentAgentValue);

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
          {NODE_TYPE_OPTIONS.map((type) => (
            <option key={type} value={type}>{getNodeTypeConfig(type).label}</option>
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
          {(["subissue", "inline", "main_issue_task"] as WorkflowDispatch[]).map((dispatch) => (
            <option key={dispatch} value={dispatch}>{DISPATCH_LABELS[dispatch]}</option>
          ))}
        </select>
      </label>

      {config.supportsAgentRoute && (
        <label className="space-y-1 text-xs text-muted-foreground">
          <span>Agent</span>
          <select
            className="h-9 w-full rounded-md border bg-background px-2 text-sm text-foreground"
            value={currentAgentValue}
            onChange={(event) => onChange({ agent: event.target.value || undefined, config: { ...node.config, agent: event.target.value || undefined } })}
          >
            <option value="">No agent</option>
            {!hasCurrentAgentOption && (
              <option value={currentAgentValue}>{selectedAgent?.name ?? currentAgentValue}</option>
            )}
            {agents.map((agent) => <option key={agent.id} value={agent.id}>{agent.name}</option>)}
          </select>
        </label>
      )}

      {config.supportsOutputs && (
        <div className="space-y-2 text-xs text-muted-foreground">
          <div>Outputs</div>
          <div className="space-y-1 rounded-md border p-2">
            {stateFields.length === 0 ? (
              <div>No state fields yet.</div>
            ) : stateFields.map((field) => {
              const checked = Boolean(node.outputs?.includes(field.name));
              return (
                <label key={field.name} className="flex items-center gap-2 text-xs text-foreground">
                  <input
                    aria-label={`Output ${field.name}`}
                    type="checkbox"
                    checked={checked}
                    onChange={() => {
                      const current = node.outputs ?? [];
                      const outputs = checked ? current.filter((output) => output !== field.name) : [...current, field.name];
                      onChange({ outputs });
                    }}
                  />
                  <span className="min-w-0 flex-1 truncate font-mono">{field.name}</span>
                  <span className="text-muted-foreground">{field.type}</span>
                </label>
              );
            })}
          </div>
        </div>
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
