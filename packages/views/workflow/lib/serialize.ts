import YAML from "yaml";
import type { WorkflowDefinition } from "@multica/core/workflow/types";

export function serializeWorkflow(definition: WorkflowDefinition): string {
  return YAML.stringify(definition);
}

export function parseWorkflow(text: string): WorkflowDefinition {
  const parsed = YAML.parse(text);
  if (!parsed || typeof parsed !== "object") {
    throw new Error("invalid workflow yaml: not an object");
  }
  const candidate = parsed as Partial<WorkflowDefinition>;
  if (!candidate.meta || typeof candidate.meta !== "object") {
    throw new Error("invalid workflow yaml: meta is required");
  }
  if (!candidate.state || typeof candidate.state !== "object" || !Array.isArray(candidate.state.fields)) {
    throw new Error("invalid workflow yaml: state.fields must be an array");
  }
  if (!Array.isArray(candidate.nodes)) {
    throw new Error("invalid workflow yaml: nodes must be an array");
  }
  if (!Array.isArray(candidate.routing)) {
    throw new Error("invalid workflow yaml: routing must be an array");
  }
  return parsed as WorkflowDefinition;
}
