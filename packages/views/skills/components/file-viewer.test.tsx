import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

vi.mock("../../common/markdown", () => ({
  Markdown: ({ children }: { children: React.ReactNode }) => <div data-testid="markdown-preview">{children}</div>,
}));

vi.mock("../../i18n", () => ({
  useT: () => ({
    t: (selector: (value: any) => string) => selector({
      file_viewer: {
        edit_tooltip: "Edit",
        preview_tooltip: "Preview",
      },
    }),
  }),
}));

import { FileViewer } from "./file-viewer";

describe("FileViewer", () => {
  it("renders markdown in preview mode first and can switch to edit mode", () => {
    const onChange = vi.fn();
    render(
      <FileViewer
        path="SKILL.md"
        content={"---\ndescription: A useful skill\n---\n# Skill\nBody"}
        onChange={onChange}
      />,
    );

    expect(screen.getByText("Markdown")).toBeInTheDocument();
    expect(screen.getByText("Frontmatter")).toBeInTheDocument();
    expect(screen.getByTestId("markdown-preview")).toHaveTextContent("# Skill");

    fireEvent.click(screen.getByRole("button", { name: "Edit" }));

    const editor = screen.getByRole("textbox");
    expect(editor).toHaveValue("---\ndescription: A useful skill\n---\n# Skill\nBody");
    fireEvent.change(editor, { target: { value: "# Updated" } });
    expect(onChange).toHaveBeenCalledWith("# Updated");
  });

  it("does not expose an enabled edit action when read-only", () => {
    render(
      <FileViewer
        path="SKILL.md"
        content="# Read only"
        onChange={vi.fn()}
        canEdit={false}
      />,
    );

    expect(screen.getByRole("button", { name: "Edit" })).toBeDisabled();
    expect(screen.queryByRole("textbox")).not.toBeInTheDocument();
  });

  it("renders yaml with the yaml renderer", () => {
    render(
      <FileViewer
        path="workflow.yaml"
        content={"nodes:\n  - id: start"}
        onChange={vi.fn()}
      />,
    );

    expect(screen.getByText("YAML")).toBeInTheDocument();
  });
});
