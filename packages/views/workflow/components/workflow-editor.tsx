"use client";

import { useState } from "react";
import type { Connection } from "@xyflow/react";
import { toast } from "sonner";
import type { Agent } from "@multica/core/types";
import type { WorkflowDefinition, WorkflowEdge, WorkflowNode } from "@multica/core/workflow/types";
import { Button } from "@multica/ui/components/ui/button";
import { managementActionButtonClass } from "../../common/management-action-button";
import { WorkflowCanvas } from "./workflow-canvas";
import { EdgeInspector } from "./edge-inspector";
import { NodeInspector } from "./node-inspector";
import { StateFieldsEditor } from "./state-fields-editor";
import { WorkflowValidationPanel } from "./workflow-validation-panel";
import { WorkflowYamlEditor } from "./workflow-yaml-editor";
import { getNodeTypeConfig } from "../lib/schema-registry";
import { parseWorkflow, serializeWorkflow } from "../lib/serialize";
import { validateWorkflowDefinition } from "../lib/validation";

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
  initialYaml,
  agents,
  onSave,
  onSaved,
  readOnly = false,
  title = "Workflow",
  saveLabel = "Save workflow",
  savedToast = "Workflow saved",
}: {
  initialYaml?: string;
  agents: Pick<Agent, "id" | "name">[];
  onSave: (yaml: string) => Promise<void>;
  onSaved?: (yaml: string) => void;
  readOnly?: boolean;
  title?: string;
  saveLabel?: string;
  savedToast?: string;
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
  const [mode, setMode] = useState<"structured" | "yaml">("structured");
  const [yamlDraft, setYamlDraft] = useState(() => initialYaml?.trim() ? initialYaml : serializeWorkflow(EMPTY_WORKFLOW));
  const [yamlError, setYamlError] = useState<string | null>(null);
  const [layoutVersion, setLayoutVersion] = useState(0);
  const [showStateFields, setShowStateFields] = useState(false);
  const selectedNodeId = selection?.type === "node" ? selection.id : null;
  const selectedEdgeId = selection?.type === "edge" ? `edge-${selection.index}` : null;
  const selectedNodeIndex = definition.nodes.findIndex((node) => node.id === selectedNodeId);
  const selectedNode = selectedNodeIndex >= 0 ? definition.nodes[selectedNodeIndex] : null;
  const selectedEdge = selection?.type === "edge" ? definition.routing[selection.index] : null;
  const validationIssues = validateWorkflowDefinition(definition);
  const blockingValidation = validationIssues.find((issue) => issue.severity === "error");
  const showValidation = !readOnly;

  function updateNode(index: number, patch: Partial<WorkflowNode>) {
    updateDefinition((prev) => ({
      ...prev,
      nodes: prev.nodes.map((node, i) =>
        i === index ? { ...node, ...patch, config: { ...node.config, ...patch.config } } : node,
      ),
    }));
  }

  function updateDefinition(next: WorkflowDefinition | ((prev: WorkflowDefinition) => WorkflowDefinition)) {
    setDefinition((prev) => {
      const resolved = typeof next === "function" ? next(prev) : next;
      setYamlDraft(serializeWorkflow(resolved));
      return resolved;
    });
  }

  function addNode() {
    if (readOnly) return;
    updateDefinition((prev) => {
      const node = { id: `node${prev.nodes.length + 1}`, type: "llm" as const, config: {} };
      setSelection({ type: "node", id: node.id });
      return { ...prev, nodes: [...prev.nodes, node] };
    });
  }

  function addNextNode(afterId: string) {
    if (readOnly) return;
    updateDefinition((prev) => {
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
    if (readOnly) return;
    if (!selectedNode) return;
    const removedId = selectedNode.id;
    updateDefinition((prev) => ({
      ...prev,
      nodes: prev.nodes.filter((node) => node.id !== removedId),
      routing: prev.routing.filter((edge) => edge.from !== removedId && edge.to !== removedId && edge.else !== removedId),
    }));
    setSelection(null);
  }

  function updateEdge(index: number, patch: Partial<WorkflowEdge>) {
    if (readOnly) return;
    updateDefinition((prev) => ({
      ...prev,
      routing: prev.routing.map((edge, i) => (i === index ? { ...edge, ...patch } : edge)),
    }));
  }

  function addEdge() {
    if (readOnly) return;
    updateDefinition((prev) => ({
      ...prev,
      routing: [...prev.routing, { from: prev.nodes[0]?.id ?? "START", to: prev.nodes[1]?.id ?? "END" }],
    }));
    setSelection({ type: "edge", index: definition.routing.length });
  }

  function connectEdge(connection: Connection) {
    if (readOnly) return;
    if (!connection.source || !connection.target || connection.source === connection.target) return;
    updateDefinition((prev) => ({
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
    if (readOnly) return;
    if (selection?.type !== "edge") return;
    updateDefinition((prev) => ({ ...prev, routing: prev.routing.filter((_, i) => i !== selection.index) }));
    setSelection(null);
  }

  async function save() {
    if (readOnly) return;
    setSaving(true);
    setError(null);
    try {
      let yaml = serializeWorkflow(definition);
      if (mode === "yaml") {
        const parsed = parseWorkflow(yamlDraft);
        setDefinition(parsed);
        yaml = yamlDraft;
      } else if (blockingValidation) {
        setError(blockingValidation.message);
        setSaving(false);
        return;
      }
      await onSave(yaml);
      onSaved?.(yaml);
      toast.success(savedToast);
    } catch (err) {
      const message = err instanceof Error ? err.message : "Failed to save workflow";
      setError(message);
      toast.error(message);
    } finally {
      setSaving(false);
    }
  }

  const nodeIds = definition.nodes.map((node) => node.id);
  const nodeLabels = Object.fromEntries(
    definition.nodes.map((node) => [node.id, getNodeTypeConfig(node.type).label]),
  );
  const selectionDetailPanel = (selectedNode || selectedEdge) && (
    <aside className="flex min-h-0 flex-col rounded-md border bg-background/95 p-3">
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
              updateDefinition((prev) => ({
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
          nodeLabels={nodeLabels}
          onChange={(patch) => updateEdge(selection.index, patch)}
          onRemove={removeSelectedEdge}
        />
      ) : (
        <div className="text-sm text-muted-foreground">Select a node on the canvas.</div>
      )}
    </aside>
  );
  const sidePanel = selectionDetailPanel ? (
    selectionDetailPanel
  ) : showStateFields ? (
    <aside className="flex min-h-0 flex-col overflow-y-auto rounded-md border bg-background/95 p-3">
      <StateFieldsEditor definition={definition} onChange={updateDefinition} />
    </aside>
  ) : null;

  return (
    <div className="flex min-h-0 flex-1 flex-col">
      <div className="flex h-12 shrink-0 items-center gap-2 border-b px-4">
        <h3 className="text-sm font-medium">{title}</h3>
        <div className="ml-auto flex items-center gap-2">
          <div className="inline-flex rounded-md border bg-background p-0.5">
            <Button type="button" size="xs" variant={mode === "structured" ? "default" : "ghost"} onClick={() => setMode("structured")}>
              Structured
            </Button>
            <Button type="button" size="xs" variant={mode === "yaml" ? "default" : "ghost"} onClick={() => setMode("yaml")}>
              YAML
            </Button>
          </div>
          {mode === "structured" && !readOnly && (
            <Button
              type="button"
              size="xs"
              variant={showStateFields && !selection ? "default" : "outline"}
              className={managementActionButtonClass("configure")}
              aria-pressed={showStateFields && !selection}
              onClick={() => {
                setSelection(null);
                setShowStateFields((value) => !value);
              }}
            >
              State fields
            </Button>
          )}
          {showValidation && (
            <span className="text-xs text-muted-foreground">
              {validationIssues.length === 0 ? "Valid" : validationIssues[0]?.message}
            </span>
          )}
          {readOnly && (
            <span className="text-xs text-muted-foreground">Read-only</span>
          )}
          {!readOnly && (
            <Button type="button" size="xs" className={managementActionButtonClass("save")} onClick={save} disabled={saving}>
              {saving ? "Saving..." : saveLabel}
            </Button>
          )}
          </div>
        </div>

      {error && <div className="border-b px-4 py-2 text-sm text-destructive">{error}</div>}

      <div className="relative min-h-0 flex-1 p-4">
        {mode === "structured" ? (
          <div className={sidePanel ? "grid h-full min-h-[640px] grid-cols-[minmax(0,1fr)_340px] gap-4" : "grid h-full min-h-[640px] grid-cols-1"}>
            <div className="relative h-full min-h-[640px]">
              <div className="absolute left-3 top-3 z-20 flex items-center gap-2 rounded-md border bg-background/95 p-1 shadow-sm backdrop-blur">
                {!readOnly && (
                  <>
                    <Button type="button" size="xs" variant="outline" className={managementActionButtonClass("create")} onClick={addNode}>
                      Add node
                    </Button>
                    <Button type="button" size="xs" variant="outline" className={managementActionButtonClass("create")} onClick={addEdge}>
                      Add edge
                    </Button>
                  </>
                )}
                <Button type="button" size="xs" variant="outline" className={managementActionButtonClass("configure")} onClick={() => setLayoutVersion((v) => v + 1)}>
                  Auto layout
                </Button>
              </div>
              <WorkflowCanvas
                key={layoutVersion}
                definition={definition}
                editable={!readOnly}
                selectedNodeId={selectedNodeId}
                selectedEdgeId={selectedEdgeId}
                onSelectNode={(id) => setSelection({ type: "node", id })}
                onSelectEdge={(edgeId) => {
                  const match = edgeId.match(/^edge-(\d+)(?:-(?:to|else))?$/);
                  if (!match) return;
                  setSelection({ type: "edge", index: Number(match[1]) });
                }}
                onPaneClick={() => setSelection(null)}
                onConnect={readOnly ? undefined : connectEdge}
                onAddNextNode={readOnly ? undefined : addNextNode}
                className="h-full min-h-[640px]"
                fullscreenTitle="Skill workflow"
                hideSelectionOverlay
                fullscreenExtra={selectionDetailPanel ? (
                  <div className="absolute bottom-8 right-8 top-20 z-30 w-[380px] overflow-y-auto">
                    {selectionDetailPanel}
                  </div>
                ) : null}
              />
              {showValidation && validationIssues.length > 0 && (
                <div className="absolute bottom-3 left-3 right-3 z-20 max-h-40 overflow-y-auto rounded-md border bg-background/95 p-2 shadow-sm backdrop-blur">
                  <WorkflowValidationPanel issues={validationIssues} />
                </div>
              )}
            </div>
            {sidePanel}
          </div>
        ) : (
          readOnly ? (
            <pre className="h-full min-h-[24rem] overflow-auto rounded-md border bg-muted/30 p-3 font-mono text-xs text-muted-foreground">
              {yamlDraft}
            </pre>
          ) : (
            <WorkflowYamlEditor
              yaml={yamlDraft}
              error={yamlError}
              onYamlChange={setYamlDraft}
              onError={setYamlError}
              onParsed={(next) => {
                updateDefinition(next);
                setSelection(null);
                setMode("structured");
              }}
            />
          )
        )}
      </div>
    </div>
  );
}
