import { describe, expect, it } from "vitest";
import type { WorkflowDefinition } from "@multica/core/workflow/types";
import { validateWorkflowDefinition } from "./validation";

describe("workflow validation", () => {
  it("reports outputs that are not declared state fields", () => {
    const definition: WorkflowDefinition = {
      meta: { name: "test" },
      state: { fields: [{ name: "task", type: "string" }] },
      nodes: [{ id: "review", type: "llm", outputs: ["missing"] }],
      routing: [],
    };

    expect(validateWorkflowDefinition(definition)).toEqual([
      expect.objectContaining({
        severity: "error",
        path: "nodes.review.outputs",
        message: 'Output "missing" is not declared in state fields.',
      }),
    ]);
  });

  it("requires else for conditional routes", () => {
    const definition: WorkflowDefinition = {
      meta: { name: "test" },
      state: { fields: [{ name: "approved", type: "boolean" }] },
      nodes: [{ id: "review", type: "llm" }],
      routing: [{ from: "review", to: "END", condition: "approved == true" }],
    };

    expect(validateWorkflowDefinition(definition)).toEqual([
      expect.objectContaining({
        severity: "error",
        path: "routing.0.else",
      }),
    ]);
  });

  it("reports blank and duplicate node ids", () => {
    const definition: WorkflowDefinition = {
      meta: { name: "test" },
      state: { fields: [] },
      nodes: [
        { id: "", type: "llm" },
        { id: "review", type: "llm" },
        { id: "review", type: "agent" },
      ],
      routing: [],
    };

    expect(validateWorkflowDefinition(definition)).toEqual(expect.arrayContaining([
      expect.objectContaining({ path: "nodes.0.id", message: "Node id is required." }),
      expect.objectContaining({ path: "nodes.review.id", message: 'Duplicate node id "review".' }),
    ]));
  });

  it("reports invalid else targets", () => {
    const definition: WorkflowDefinition = {
      meta: { name: "test" },
      state: { fields: [{ name: "approved", type: "boolean" }] },
      nodes: [{ id: "review", type: "llm" }],
      routing: [{ from: "review", to: "END", condition: "approved == true", else: "missing" }],
    };

    expect(validateWorkflowDefinition(definition)).toEqual([
      expect.objectContaining({
        severity: "error",
        path: "routing.0.else",
        message: 'Route else target "missing" does not exist.',
      }),
    ]);
  });
});
