import { describe, expect, it } from "vitest";
import type { WorkflowDefinition } from "@multica/core/workflow/types";
import { parseWorkflow, serializeWorkflow } from "./serialize";

const definition: WorkflowDefinition = {
  meta: { name: "demo" },
  state: { fields: [{ name: "task", type: "string", required: true }] },
  nodes: [{ id: "impl", type: "llm", outputs: ["execution"], config: { agent: "code", system: "do" } }],
  routing: [
    { from: "START", to: "impl" },
    { from: "impl", to: "END" },
  ],
};

describe("workflow yaml serialization", () => {
  it("round-trips a definition", () => {
    const text = serializeWorkflow(definition);
    expect(text).toContain("name: demo");
    const back = parseWorkflow(text);
    expect(back.nodes[0]?.id).toBe("impl");
    expect(back.routing[1]?.to).toBe("END");
  });

  it("throws on invalid yaml", () => {
    expect(() => parseWorkflow(":\n  - bad: [unclosed")).toThrow();
  });
});
