import type { WorkflowDefinition, WorkflowNodeRunState } from "@multica/core/workflow/types";

function safeId(id: string): string {
  return id.replace(/[^a-zA-Z0-9_]/g, "_");
}

export function workflowToMermaid(
  definition: WorkflowDefinition,
  runState?: Record<string, WorkflowNodeRunState>,
  currentNode?: string,
): string {
  const lines = ["flowchart TD"];

  for (const node of definition.nodes) {
    lines.push(`  ${safeId(node.id)}["${node.id}\\n(${node.type})"]`);
  }

  for (const edge of definition.routing) {
    const from = edge.from === "START" ? "START" : safeId(edge.from);
    const to = edge.to === "END" ? "END" : safeId(edge.to);
    if (edge.condition) {
      lines.push(`  ${from} -->|${edge.condition}| ${to}`);
      if (edge.else) {
        const elseTo = edge.else === "END" ? "END" : safeId(edge.else);
        lines.push(`  ${from} -->|else| ${elseTo}`);
      }
    } else {
      lines.push(`  ${from} --> ${to}`);
    }
  }

  lines.push("  classDef running fill:#fde68a,stroke:#d97706;");
  lines.push("  classDef done fill:#bbf7d0,stroke:#16a34a;");
  lines.push("  classDef failed fill:#fecaca,stroke:#dc2626;");
  lines.push("  classDef pending fill:#e5e7eb,stroke:#9ca3af;");
  lines.push("  classDef current stroke-width:3px;");

  if (runState) {
    for (const node of definition.nodes) {
      const status = runState[node.id]?.status;
      if (status) lines.push(`  class ${safeId(node.id)} ${status};`);
    }
  }
  if (currentNode && currentNode !== "START" && currentNode !== "END") {
    lines.push(`  class ${safeId(currentNode)} current;`);
  }

  return lines.join("\n");
}
