import { describe, expect, it } from "vitest";
import { getNodeTypeConfig, isAgentLikeNode } from "./schema-registry";

describe("workflow schema registry", () => {
  it("shows system prompt only for agent-like nodes", () => {
    expect(isAgentLikeNode("llm")).toBe(true);
    expect(isAgentLikeNode("agent")).toBe(true);
    expect(isAgentLikeNode("subissue")).toBe(true);
    expect(isAgentLikeNode("main_agent")).toBe(true);
    expect(isAgentLikeNode("final_response")).toBe(true);

    expect(isAgentLikeNode("transform")).toBe(false);
    expect(isAgentLikeNode("condition")).toBe(false);
    expect(isAgentLikeNode("merge")).toBe(false);
    expect(isAgentLikeNode("router")).toBe(false);
  });

  it("returns sensible defaults for known and unknown node types", () => {
    expect(getNodeTypeConfig("llm")).toMatchObject({
      label: "LLM",
      defaultDispatch: "subissue",
      supportsSystemPrompt: true,
      supportsOutputs: true,
    });

    expect(getNodeTypeConfig("transform")).toMatchObject({
      label: "Transform",
      defaultDispatch: "inline",
      supportsSystemPrompt: false,
      supportsTransformMap: true,
    });

    expect(getNodeTypeConfig("custom_runtime_node")).toMatchObject({
      label: "custom_runtime_node",
      defaultDispatch: "inline",
      supportsAdvancedConfig: true,
    });
  });
});
