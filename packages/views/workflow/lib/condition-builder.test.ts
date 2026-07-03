import { describe, expect, it } from "vitest";
import { conditionGroupToExpression, tryParseConditionExpression } from "./condition-builder";

describe("condition builder", () => {
  it("serializes a single equality rule", () => {
    expect(conditionGroupToExpression({
      join: "AND",
      rules: [{ field: "workflow_status", operator: "==", value: "done", valueType: "string" }],
    })).toBe('workflow_status == "done"');
  });

  it("serializes boolean and number comparisons", () => {
    expect(conditionGroupToExpression({
      join: "AND",
      rules: [
        { field: "approved", operator: "==", value: true, valueType: "boolean" },
        { field: "revisionCount", operator: "<", value: 2, valueType: "number" },
      ],
    })).toBe("approved == true && revisionCount < 2");
  });

  it("serializes truthy and falsy checks", () => {
    expect(conditionGroupToExpression({
      join: "OR",
      rules: [
        { field: "ready", operator: "truthy" },
        { field: "error", operator: "falsy" },
      ],
    })).toBe("ready || !error");
  });

  it("parses supported expressions and returns null for advanced expressions", () => {
    expect(tryParseConditionExpression('workflow_status == "done" && revisionCount < 2')).toEqual({
      join: "AND",
      rules: [
        { field: "workflow_status", operator: "==", value: "done", valueType: "string" },
        { field: "revisionCount", operator: "<", value: 2, valueType: "number" },
      ],
    });

    expect(tryParseConditionExpression('(a == "x" && b == "y") || c == true')).toBe(null);
  });
});
