import type { WorkflowDefinition, WorkflowStateField } from "@multica/core/workflow/types";

export function createStateField(name: string, type: WorkflowStateField["type"]): WorkflowStateField {
  return { name: name.trim(), type };
}

export function addStateField(definition: WorkflowDefinition, field: WorkflowStateField): WorkflowDefinition {
  if (!field.name || definition.state.fields.some((existing) => existing.name === field.name)) {
    return definition;
  }
  return {
    ...definition,
    state: {
      ...definition.state,
      fields: [...definition.state.fields, field],
    },
  };
}

export function removeStateField(definition: WorkflowDefinition, fieldName: string): WorkflowDefinition {
  return {
    ...definition,
    state: {
      ...definition.state,
      fields: definition.state.fields.filter((field) => field.name !== fieldName),
    },
  };
}

export function findStateFieldReferences(definition: WorkflowDefinition, fieldName: string): string[] {
  const refs: string[] = [];
  for (const node of definition.nodes) {
    if (node.inputs?.includes(fieldName)) refs.push(`node ${node.id} inputs`);
    if (node.outputs?.includes(fieldName)) refs.push(`node ${node.id} outputs`);
    if (node.on_complete?.some((action) => action.action === "increment" && action.field === fieldName)) {
      refs.push(`node ${node.id} on_complete`);
    }
  }
  for (const route of definition.routing) {
    if (route.condition && conditionReferencesField(route.condition, fieldName)) {
      refs.push(`route ${route.from} -> ${route.to} condition`);
    }
  }
  return refs;
}

export function upsertNodeOutputs(definition: WorkflowDefinition, nodeId: string, outputs: string[]): WorkflowDefinition {
  return {
    ...definition,
    nodes: definition.nodes.map((node) => (node.id === nodeId ? { ...node, outputs } : node)),
  };
}

function conditionReferencesField(condition: string, fieldName: string): boolean {
  const escaped = fieldName.replace(/[.*+?^${}()|[\]\\]/g, "\\$&");
  return new RegExp(`(^|[^A-Za-z0-9_])${escaped}([^A-Za-z0-9_]|$)`).test(condition);
}
