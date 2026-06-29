import { describe, expect, it } from "vitest";
import type { WorkflowDefinition } from "@multica/core/workflow/types";
import { workflowToReactFlow } from "./to-react-flow";

const definition: WorkflowDefinition = {
  meta: { name: "canvas" },
  state: { fields: [] },
  nodes: [
    { id: "plan", type: "llm" },
    { id: "impl", type: "llm" },
    { id: "review", type: "router" },
  ],
  routing: [
    { from: "START", to: "plan" },
    { from: "plan", to: "impl" },
    { from: "impl", to: "review", condition: "ok == true" },
    { from: "review", to: "END" },
  ],
};

describe("workflowToReactFlow", () => {
  it("creates positioned nodes and edges", () => {
    const { nodes, edges } = workflowToReactFlow(definition);
    expect(nodes.map((n) => n.id)).toEqual(["plan", "impl", "review"]);
    expect(nodes.find((n) => n.id === "impl")?.position.x).toBeGreaterThan(nodes.find((n) => n.id === "plan")!.position.x);
    expect(edges).toHaveLength(2);
    expect(edges[1]?.label).toBe("ok == true");
  });

  it("maps runtime state onto node data", () => {
    const { nodes } = workflowToReactFlow(definition, {
      impl: { status: "running", sub_issue_id: "sub-1", started_at: null, ended_at: null, error: null },
    });
    const impl = nodes.find((n) => n.id === "impl")!;
    expect(impl.data.status).toBe("running");
    expect(impl.data.subIssueId).toBe("sub-1");
  });

  it("handles loop-back routes without unbounded layer growth", () => {
    const loopDefinition: WorkflowDefinition = {
      meta: { name: "loop" },
      state: { fields: [] },
      nodes: [
        { id: "author", type: "agent" },
        { id: "review", type: "agent" },
        { id: "gate", type: "transform" },
        { id: "final", type: "final_response" },
      ],
      routing: [
        { from: "START", to: "author" },
        { from: "author", to: "review" },
        { from: "review", to: "gate" },
        { from: "gate", condition: "needs_fix == true", to: "author", else: "final" },
        { from: "final", to: "END" },
      ],
    };

    const { nodes, edges } = workflowToReactFlow(loopDefinition);

    expect(nodes.map((node) => node.id).sort()).toEqual(["author", "final", "gate", "review"]);
    expect(Math.max(...nodes.map((node) => node.position.x))).toBeLessThan(2000);
    expect(edges).toHaveLength(3);
  });
});
