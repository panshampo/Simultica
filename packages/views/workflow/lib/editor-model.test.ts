import { describe, expect, it } from "vitest";
import type { WorkflowDefinition } from "@multica/core/workflow/types";
import { addStateField, createStateField, findStateFieldReferences, removeStateField, upsertNodeOutputs } from "./editor-model";

const baseDefinition: WorkflowDefinition = {
  meta: { name: "test" },
  state: { fields: [{ name: "task", type: "string" }] },
  nodes: [{ id: "review", type: "llm", outputs: ["result"] }],
  routing: [{ from: "review", condition: 'result == "ok"', to: "END", else: "review" }],
};

describe("workflow editor model", () => {
  it("creates and adds unique state fields", () => {
    const next = addStateField(baseDefinition, createStateField("result", "string"));

    expect(next.state.fields).toEqual([
      { name: "task", type: "string" },
      { name: "result", type: "string" },
    ]);
    expect(addStateField(next, createStateField("result", "string"))).toBe(next);
  });

  it("finds references before deleting a field", () => {
    expect(findStateFieldReferences(baseDefinition, "result")).toEqual([
      "node review outputs",
      "route review -> END condition",
    ]);
  });

  it("removes fields without mutating the original definition", () => {
    const next = removeStateField(baseDefinition, "task");

    expect(next.state.fields).toEqual([]);
    expect(baseDefinition.state.fields).toEqual([{ name: "task", type: "string" }]);
  });

  it("updates node outputs", () => {
    const next = upsertNodeOutputs(baseDefinition, "review", ["result", "summary"]);

    expect(next.nodes[0]?.outputs).toEqual(["result", "summary"]);
  });
});
