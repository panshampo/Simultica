import { readFileSync } from "node:fs";
import { join } from "node:path";
import { describe, expect, it } from "vitest";

describe("Mermaid", () => {
  it("keeps fullscreen diagram controls accessible", () => {
    const source = readFileSync(join(__dirname, "mermaid.tsx"), "utf8");

    expect(source).toContain('aria-label="Expand diagram"');
    expect(source).toContain('role="dialog"');
    expect(source).toContain('aria-label="Expanded diagram"');
    expect(source).toContain('aria-label="Close expanded diagram"');
  });
});
