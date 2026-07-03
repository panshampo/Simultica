import { describe, expect, it } from "vitest";
import { getSkillFileRenderer, isMarkdownFile } from "./skill-file-renderers";

describe("skill file renderer registry", () => {
  it("selects renderers by file extension", () => {
    expect(getSkillFileRenderer("SKILL.md").id).toBe("markdown");
    expect(getSkillFileRenderer("README.mdx").id).toBe("markdown");
    expect(getSkillFileRenderer("workflow.yaml").id).toBe("yaml");
    expect(getSkillFileRenderer("config.yml").id).toBe("yaml");
    expect(getSkillFileRenderer("package.json").id).toBe("json");
    expect(getSkillFileRenderer("notes.txt").id).toBe("text");
  });

  it("detects markdown files case-insensitively", () => {
    expect(isMarkdownFile("SKILL.MD")).toBe(true);
    expect(isMarkdownFile("docs/Guide.MDX")).toBe(true);
    expect(isMarkdownFile("workflow.yaml")).toBe(false);
  });
});
