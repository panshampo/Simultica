import { describe, expect, it } from "vitest";
import type { WorkflowDefinition } from "@multica/core/workflow/types";
import { workflowToMermaid } from "./to-mermaid";

const definition: WorkflowDefinition = {
  meta: { name: "w" },
  state: { fields: [] },
  nodes: [
    { id: "a", type: "llm" },
    { id: "b", type: "router" },
  ],
  routing: [
    { from: "START", to: "a" },
    { from: "a", to: "b" },
    { from: "b", to: "END", condition: "ok == true" },
  ],
};

describe("workflowToMermaid", () => {
  it("renders flowchart with nodes and edges", () => {
    const chart = workflowToMermaid(definition);
    expect(chart).toContain("flowchart TD");
    expect(chart).toContain("a");
    expect(chart).toContain("b");
    expect(chart).toContain("-->");
  });

  it("labels conditional edges", () => {
    expect(workflowToMermaid(definition)).toMatch(/ok == true/);
  });

  it("applies status classes", () => {
    const chart = workflowToMermaid(
      definition,
      { a: { status: "done", sub_issue_id: null, started_at: null, ended_at: null, error: null } },
      "b",
    );
    expect(chart).toContain("classDef done");
    expect(chart).toMatch(/class a done/);
    expect(chart).toMatch(/class b current/);
  });
});
