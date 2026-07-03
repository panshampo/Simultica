import type { WorkflowDefinition } from "@multica/core/workflow/types";

export type WorkflowValidationIssue = {
  severity: "error" | "warning";
  path: string;
  message: string;
};

export function validateWorkflowDefinition(definition: WorkflowDefinition): WorkflowValidationIssue[] {
  const issues: WorkflowValidationIssue[] = [];
  const fieldNames = new Set(definition.state.fields.map((field) => field.name));
  const nodeIds = new Set(["START", "END", ...definition.nodes.map((node) => node.id)]);

  for (const node of definition.nodes) {
    for (const output of node.outputs ?? []) {
      if (!fieldNames.has(output)) {
        issues.push({
          severity: "error",
          path: `nodes.${node.id}.outputs`,
          message: `Output "${output}" is not declared in state fields.`,
        });
      }
    }
  }

  definition.routing.forEach((route, index) => {
    if (!nodeIds.has(route.from)) {
      issues.push({ severity: "error", path: `routing.${index}.from`, message: `Route source "${route.from}" does not exist.` });
    }
    if (!nodeIds.has(route.to)) {
      issues.push({ severity: "error", path: `routing.${index}.to`, message: `Route target "${route.to}" does not exist.` });
    }
    if (route.condition && !route.else) {
      issues.push({ severity: "error", path: `routing.${index}.else`, message: "Conditional routes need an else target." });
    }
  });

  return issues;
}
