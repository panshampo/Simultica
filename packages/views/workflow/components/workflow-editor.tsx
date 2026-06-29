"use client";

import { useState } from "react";
import type { Connection } from "@xyflow/react";
import type { Agent } from "@multica/core/types";
import type { WorkflowDefinition, WorkflowEdge, WorkflowNode } from "@multica/core/workflow/types";
import { api } from "@multica/core/api";
import { Button } from "@multica/ui/components/ui/button";
import { Input } from "@multica/ui/components/ui/input";
import { Textarea } from "@multica/ui/components/ui/textarea";
import { WorkflowCanvas } from "./workflow-canvas";
import { parseWorkflow, serializeWorkflow } from "../lib/serialize";

const EMPTY_WORKFLOW: WorkflowDefinition = {
  meta: { name: "workflow" },
  state: { fields: [{ name: "task", type: "string", required: true }] },
  nodes: [],
  routing: [{ from: "START", to: "END" }],
};

type Selection =
  | { type: "node"; id: string }
  | { type: "edge"; index: number }
  | null;

export function WorkflowEditor({
  skillId,
  initialYaml,
  agents,
  onSaved,
}: {
  skillId: string;
  initialYaml?: string;
  agents: Pick<Agent, "id" | "name">[];
  onSaved?: (yaml: string) => void;
}) {
  const [definition, setDefinition] = useState<WorkflowDefinition>(() => {
    if (!initialYaml?.trim()) return EMPTY_WORKFLOW;
    try {
      return parseWorkflow(initialYaml);
    } catch {
      return EMPTY_WORKFLOW;
    }
  });
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [selection, setSelection] = useState<Selection>(null);
  const [layoutVersion, setLayoutVersion] = useState(0);
  const selectedNodeId = selection?.type === "node" ? selection.id : null;
  const selectedEdgeId = selection?.type === "edge" ? `edge-${selection.index}` : null;
  const selectedNodeIndex = definition.nodes.findIndex((node) => node.id === selectedNodeId);
  const selectedNode = selectedNodeIndex >= 0 ? definition.nodes[selectedNodeIndex] : null;
  const selectedEdge = selection?.type === "edge" ? definition.routing[selection.index] : null;
  const validation = validateWorkflow(definition);

  function updateNode(index: number, patch: Partial<WorkflowNode>) {
    setDefinition((prev) => ({
      ...prev,
      nodes: prev.nodes.map((node, i) =>
        i === index ? { ...node, ...patch, config: { ...node.config, ...patch.config } } : node,
      ),
    }));
  }

  function addNode() {
    setDefinition((prev) => {
      const node = { id: `node${prev.nodes.length + 1}`, type: "llm" as const, config: {} };
      setSelection({ type: "node", id: node.id });
      return { ...prev, nodes: [...prev.nodes, node] };
    });
  }

  function addNextNode(afterId: string) {
    setDefinition((prev) => {
      const node = { id: `node${prev.nodes.length + 1}`, type: "llm" as const, config: {} };
      setSelection({ type: "node", id: node.id });
      return {
        ...prev,
        nodes: [...prev.nodes, node],
        routing: [...prev.routing, { from: afterId, to: node.id }],
      };
    });
  }

  function removeSelectedNode() {
    if (!selectedNode) return;
    const removedId = selectedNode.id;
    setDefinition((prev) => ({
      ...prev,
      nodes: prev.nodes.filter((node) => node.id !== removedId),
      routing: prev.routing.filter((edge) => edge.from !== removedId && edge.to !== removedId && edge.else !== removedId),
    }));
    setSelection(null);
  }

  function updateEdge(index: number, patch: Partial<WorkflowEdge>) {
    setDefinition((prev) => ({
      ...prev,
      routing: prev.routing.map((edge, i) => (i === index ? { ...edge, ...patch } : edge)),
    }));
  }

  function addEdge() {
    setDefinition((prev) => ({
      ...prev,
      routing: [...prev.routing, { from: prev.nodes[0]?.id ?? "START", to: prev.nodes[1]?.id ?? "END" }],
    }));
    setSelection({ type: "edge", index: definition.routing.length });
  }

  function connectEdge(connection: Connection) {
    if (!connection.source || !connection.target || connection.source === connection.target) return;
    setDefinition((prev) => ({
      ...prev,
      routing: [...prev.routing, { from: connection.source!, to: connection.target! }],
    }));
    setSelection({ type: "edge", index: definition.routing.length });
  }

  function removeSelectedEdge() {
    if (selection?.type !== "edge") return;
    setDefinition((prev) => ({ ...prev, routing: prev.routing.filter((_, i) => i !== selection.index) }));
    setSelection(null);
  }

  async function save() {
    setSaving(true);
    setError(null);
    if (validation.length > 0) {
      setError(validation[0] ?? "Invalid workflow");
      setSaving(false);
      return;
    }
    try {
      const yaml = serializeWorkflow(definition);
      await api.upsertSkillFile(skillId, { path: "workflow.yaml", content: yaml });
      onSaved?.(yaml);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Save failed");
    } finally {
      setSaving(false);
    }
  }

  return (
    <div className="flex min-h-0 flex-1 flex-col">
      <div className="flex h-12 shrink-0 items-center gap-2 border-b px-4">
        <h3 className="text-sm font-medium">Workflow</h3>
        <div className="ml-auto flex items-center gap-2">
          <Button type="button" size="xs" variant="outline" onClick={addNode}>
            Add node
          </Button>
          <Button
            type="button"
            size="xs"
            variant="outline"
            onClick={addEdge}
          >
            Add edge
          </Button>
          <Button type="button" size="xs" variant="outline" onClick={() => setLayoutVersion((v) => v + 1)}>
            Auto layout
          </Button>
          <span className="text-xs text-muted-foreground">
            {validation.length === 0 ? "Valid" : validation[0]}
          </span>
          <Button type="button" size="xs" onClick={save} disabled={saving}>
            {saving ? "Saving..." : "Save workflow"}
          </Button>
        </div>
      </div>

      {error && <div className="border-b px-4 py-2 text-sm text-destructive">{error}</div>}

      <div className="relative min-h-0 flex-1 p-4">
        <div className="h-full min-h-[640px]">
          <WorkflowCanvas
            key={layoutVersion}
            definition={definition}
            editable
            selectedNodeId={selectedNodeId}
            selectedEdgeId={selectedEdgeId}
            onSelectNode={(id) => setSelection({ type: "node", id })}
            onSelectEdge={(edgeId) => setSelection({ type: "edge", index: Number(edgeId.replace("edge-", "")) })}
            onPaneClick={() => setSelection(null)}
            onConnect={connectEdge}
            onAddNextNode={addNextNode}
            className="h-full min-h-[640px]"
            fullscreenTitle="Skill workflow"
          />
        </div>

        {(selectedNode || selectedEdge) && (
          <aside className="absolute bottom-8 right-8 top-8 z-10 flex w-[380px] min-h-0 flex-col overflow-y-auto rounded-md border bg-background/95 p-4 shadow-xl backdrop-blur">
            <div className="mb-3 flex items-center justify-between">
              <h4 className="text-sm font-medium">{selectedNode ? "Node details" : "Edge details"}</h4>
              <Button type="button" size="xs" variant="ghost" onClick={() => setSelection(null)}>
                Close
              </Button>
            </div>

            {selectedNode && selectedNodeIndex >= 0 ? (
              <div className="space-y-3">
                <div className="space-y-1">
                  <label className="text-xs text-muted-foreground">ID</label>
                  <Input
                    value={selectedNode.id}
                    onChange={(e) => {
                      const nextId = e.target.value;
                      const oldId = selectedNode.id;
                      updateNode(selectedNodeIndex, { id: nextId });
                      setDefinition((prev) => ({
                        ...prev,
                        routing: prev.routing.map((edge) => ({
                          ...edge,
                          from: edge.from === oldId ? nextId : edge.from,
                          to: edge.to === oldId ? nextId : edge.to,
                          else: edge.else === oldId ? nextId : edge.else,
                        })),
                      }));
                      setSelection({ type: "node", id: nextId });
                    }}
                  />
                </div>
                <div className="space-y-1">
                  <label className="text-xs text-muted-foreground">Type</label>
                  <select
                    className="h-9 w-full rounded-md border bg-background px-2 text-sm"
                    value={selectedNode.type}
                    onChange={(e) => updateNode(selectedNodeIndex, { type: e.target.value as WorkflowNode["type"] })}
                  >
                    {["llm", "subissue", "router", "transform", "code", "http"].map((type) => (
                      <option key={type} value={type}>{type}</option>
                    ))}
                  </select>
                </div>
                <div className="space-y-1">
                  <label className="text-xs text-muted-foreground">Agent</label>
                  <select
                    className="h-9 w-full rounded-md border bg-background px-2 text-sm"
                    value={selectedNode.config?.agent ?? ""}
                    onChange={(e) => updateNode(selectedNodeIndex, { config: { agent: e.target.value || undefined } })}
                  >
                    <option value="">No agent</option>
                    {agents.map((agent) => (
                      <option key={agent.id} value={agent.name}>{agent.name}</option>
                    ))}
                  </select>
                </div>
                <div className="space-y-1">
                  <label className="text-xs text-muted-foreground">Outputs</label>
                  <Input
                    value={(selectedNode.outputs ?? []).join(", ")}
                    onChange={(e) =>
                      updateNode(selectedNodeIndex, {
                        outputs: e.target.value.split(",").map((v) => v.trim()).filter(Boolean),
                      })
                    }
                  />
                </div>
                <div className="space-y-1">
                  <label className="text-xs text-muted-foreground">System</label>
                  <Textarea
                    className="min-h-28"
                    value={selectedNode.config?.system ?? ""}
                    onChange={(e) => updateNode(selectedNodeIndex, { config: { system: e.target.value } })}
                  />
                </div>
                <Button type="button" variant="destructive" size="sm" onClick={removeSelectedNode}>
                  Remove node
                </Button>
              </div>
            ) : selectedEdge && selection?.type === "edge" ? (
              <div className="space-y-3">
                <div className="space-y-1">
                  <label className="text-xs text-muted-foreground">From</label>
                  <Input value={selectedEdge.from} onChange={(e) => updateEdge(selection.index, { from: e.target.value })} />
                </div>
                <div className="space-y-1">
                  <label className="text-xs text-muted-foreground">To</label>
                  <Input value={selectedEdge.to} onChange={(e) => updateEdge(selection.index, { to: e.target.value })} />
                </div>
                <div className="space-y-1">
                  <label className="text-xs text-muted-foreground">Condition</label>
                  <Input
                    value={selectedEdge.condition ?? ""}
                    onChange={(e) => updateEdge(selection.index, { condition: e.target.value || undefined })}
                  />
                </div>
                <div className="space-y-1">
                  <label className="text-xs text-muted-foreground">Else</label>
                  <Input value={selectedEdge.else ?? ""} onChange={(e) => updateEdge(selection.index, { else: e.target.value || undefined })} />
                </div>
                <Button type="button" variant="destructive" size="sm" onClick={removeSelectedEdge}>
                  Remove edge
                </Button>
              </div>
            ) : (
              <div className="text-sm text-muted-foreground">Select a node on the canvas.</div>
            )}
          </aside>
        )}
      </div>
    </div>
  );
}

function validateWorkflow(definition: WorkflowDefinition): string[] {
  const errors: string[] = [];
  const ids = new Set<string>();
  for (const node of definition.nodes) {
    if (!node.id.trim()) errors.push("Node id is required");
    if (ids.has(node.id)) errors.push(`Duplicate node id: ${node.id}`);
    ids.add(node.id);
  }
  const validTargets = new Set(["START", "END", ...ids]);
  for (const edge of definition.routing) {
    if (!validTargets.has(edge.from)) errors.push(`Invalid edge source: ${edge.from}`);
    if (!validTargets.has(edge.to)) errors.push(`Invalid edge target: ${edge.to}`);
    if (edge.else && !validTargets.has(edge.else)) errors.push(`Invalid edge else target: ${edge.else}`);
  }
  return errors;
}
