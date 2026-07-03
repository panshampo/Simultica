"use client";

import { useState } from "react";
import type { Connection } from "@xyflow/react";
import { toast } from "sonner";
import type { Agent } from "@multica/core/types";
import type { WorkflowDefinition, WorkflowEdge, WorkflowNode } from "@multica/core/workflow/types";
import { api } from "@multica/core/api";
import { Button } from "@multica/ui/components/ui/button";
import { WorkflowCanvas } from "./workflow-canvas";
import { EdgeInspector } from "./edge-inspector";
import { NodeInspector } from "./node-inspector";
import { StateFieldsEditor } from "./state-fields-editor";
import { WorkflowValidationPanel } from "./workflow-validation-panel";
import { WorkflowYamlEditor } from "./workflow-yaml-editor";
import { parseWorkflow, serializeWorkflow } from "../lib/serialize";
import { validateWorkflowDefinition } from "../lib/validation";
import { useT } from "../../i18n";

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
  const { t } = useT("skills");
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
  const [mode, setMode] = useState<"structured" | "yaml">("structured");
  const [layoutVersion, setLayoutVersion] = useState(0);
  const selectedNodeId = selection?.type === "node" ? selection.id : null;
  const selectedEdgeId = selection?.type === "edge" ? `edge-${selection.index}` : null;
  const selectedNodeIndex = definition.nodes.findIndex((node) => node.id === selectedNodeId);
  const selectedNode = selectedNodeIndex >= 0 ? definition.nodes[selectedNodeIndex] : null;
  const selectedEdge = selection?.type === "edge" ? definition.routing[selection.index] : null;
  const validationIssues = validateWorkflowDefinition(definition);
  const blockingValidation = validationIssues.find((issue) => issue.severity === "error");

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
      routing: [
        ...prev.routing,
        {
          from: connection.source!,
          to: connection.target!,
          sourceHandle: connection.sourceHandle ?? undefined,
          targetHandle: connection.targetHandle ?? undefined,
        },
      ],
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
    if (blockingValidation) {
      setError(blockingValidation.message);
      setSaving(false);
      return;
    }
    try {
      const yaml = serializeWorkflow(definition);
      await api.upsertSkillFile(skillId, { path: "workflow.yaml", content: yaml });
      onSaved?.(yaml);
      toast.success(t(($) => $.detail.toast_workflow_saved));
    } catch (err) {
      const message = err instanceof Error ? err.message : t(($) => $.detail.toast_workflow_save_failed);
      setError(message);
      toast.error(message);
    } finally {
      setSaving(false);
    }
  }

  const nodeIds = definition.nodes.map((node) => node.id);
  const selectionDetailPanel = (selectedNode || selectedEdge) && (
    <aside className="absolute bottom-8 right-8 top-8 z-10 flex w-[380px] min-h-0 flex-col overflow-y-auto rounded-md border bg-background/95 p-4 shadow-xl backdrop-blur">
      <div className="mb-3 flex items-center justify-between">
        <h4 className="text-sm font-medium">{selectedNode ? "Node details" : "Edge details"}</h4>
        <Button type="button" size="xs" variant="ghost" onClick={() => setSelection(null)}>
          Close
        </Button>
      </div>

      {selectedNode && selectedNodeIndex >= 0 ? (
        <NodeInspector
          node={selectedNode}
          stateFields={definition.state.fields}
          agents={agents}
          onChange={(patch) => {
            const oldId = selectedNode.id;
            const nextId = patch.id ?? oldId;
            updateNode(selectedNodeIndex, patch);
            if (nextId !== oldId) {
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
            }
          }}
          onRemove={removeSelectedNode}
        />
      ) : selectedEdge && selection?.type === "edge" ? (
        <EdgeInspector
          edge={selectedEdge}
          stateFields={definition.state.fields}
          nodeIds={nodeIds}
          onChange={(patch) => updateEdge(selection.index, patch)}
          onRemove={removeSelectedEdge}
        />
      ) : (
        <div className="text-sm text-muted-foreground">Select a node on the canvas.</div>
      )}
    </aside>
  );

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
          <Button type="button" size="xs" variant={mode === "structured" ? "default" : "outline"} onClick={() => setMode("structured")}>
            Structured
          </Button>
          <Button type="button" size="xs" variant={mode === "yaml" ? "default" : "outline"} onClick={() => setMode("yaml")}>
            YAML
          </Button>
          <span className="text-xs text-muted-foreground">
            {validationIssues.length === 0 ? "Valid" : validationIssues[0]?.message}
          </span>
          <Button type="button" size="xs" onClick={save} disabled={saving}>
            {saving ? "Saving..." : "Save workflow"}
          </Button>
        </div>
      </div>

      {error && <div className="border-b px-4 py-2 text-sm text-destructive">{error}</div>}

      <div className="relative min-h-0 flex-1 p-4">
        {mode === "structured" ? (
          <div className="grid h-full min-h-[640px] grid-cols-[minmax(0,1fr)_320px] gap-4">
            <div className="h-full min-h-[640px]">
              <WorkflowCanvas
                key={layoutVersion}
                definition={definition}
                editable
                selectedNodeId={selectedNodeId}
                selectedEdgeId={selectedEdgeId}
                onSelectNode={(id) => setSelection({ type: "node", id })}
                onSelectEdge={(edgeId) => {
                  const match = edgeId.match(/^edge-(\d+)(?:-(?:to|else))?$/);
                  if (!match) return;
                  setSelection({ type: "edge", index: Number(match[1]) });
                }}
                onPaneClick={() => setSelection(null)}
                onConnect={connectEdge}
                onAddNextNode={addNextNode}
                className="h-full min-h-[640px]"
                fullscreenTitle="Skill workflow"
                hideSelectionOverlay
                fullscreenExtra={selectionDetailPanel}
              />
            </div>
            <div className="flex min-h-0 flex-col gap-3 overflow-y-auto">
              <StateFieldsEditor definition={definition} onChange={setDefinition} />
              <WorkflowValidationPanel issues={validationIssues} />
            </div>
          </div>
        ) : (
          <WorkflowYamlEditor
            initialYaml={serializeWorkflow(definition)}
            onParsed={(next) => {
              setDefinition(next);
              setSelection(null);
              setMode("structured");
            }}
          />
        )}

        {selectionDetailPanel}
      </div>
    </div>
  );
}
