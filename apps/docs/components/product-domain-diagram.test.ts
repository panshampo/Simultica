import { readFileSync } from "node:fs";
import { join } from "node:path";
import { describe, expect, it } from "vitest";

describe("ProductDomainDiagram", () => {
  it("keeps the domain diagram content and fullscreen controls explicit", () => {
    const source = readFileSync(join(__dirname, "product-domain-diagram.tsx"), "utf8");

    expect(source).toContain("ProductDomainDiagram");
    expect(source).toContain('aria-label="Expand product domain diagram"');
    expect(source).toContain('aria-label="Expanded product domain diagram"');
    expect(source).toContain("IssueTemplate");
    expect(source).toContain("WorkflowCase");
    expect(source).toContain("WorkflowRun");
  });
});
