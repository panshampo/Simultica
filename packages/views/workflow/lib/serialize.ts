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
  return parsed as WorkflowDefinition;
}
