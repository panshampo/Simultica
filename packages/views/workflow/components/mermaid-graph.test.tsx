import { render, waitFor } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

vi.mock("mermaid", () => ({
  default: {
    initialize: vi.fn(),
    render: vi.fn(async () => ({ svg: "<svg data-testid=\"graph\"></svg>" })),
  },
}));

import mermaid from "mermaid";
import { MermaidGraph } from "./mermaid-graph";

describe("MermaidGraph", () => {
  it("renders svg", async () => {
    const { container } = render(<MermaidGraph chart="flowchart TD\nA --> B" />);
    await waitFor(() => {
      expect(container.querySelector("svg")).toBeTruthy();
    });
  });

  it("shows render errors", async () => {
    vi.mocked(mermaid.render).mockRejectedValueOnce(new Error("bad chart"));
    const { findByText } = render(<MermaidGraph chart="bad" />);
    expect(await findByText(/bad chart/)).toBeTruthy();
  });
});
