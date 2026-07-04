import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { WorkflowYamlEditor } from "./workflow-yaml-editor";

describe("WorkflowYamlEditor", () => {
  it("emits parsed YAML and shows parse errors", () => {
    const onParsed = vi.fn();
    const onError = vi.fn();
    render(
      <WorkflowYamlEditor
        yaml="meta: ["
        error="YAML parse failed: bad shape"
        onYamlChange={vi.fn()}
        onError={onError}
        onParsed={onParsed}
      />,
    );

    fireEvent.click(screen.getByRole("button", { name: "Apply YAML" }));

    expect(screen.getByText(/YAML parse failed/)).toBeInTheDocument();
    expect(onParsed).not.toHaveBeenCalled();
  });

  it("rejects YAML that is not shaped like a workflow", () => {
    const onParsed = vi.fn();
    const onError = vi.fn();
    render(
      <WorkflowYamlEditor
        yaml={`meta:
  name: bad
`}
        error={null}
        onYamlChange={vi.fn()}
        onError={onError}
        onParsed={onParsed}
      />,
    );

    fireEvent.click(screen.getByRole("button", { name: "Apply YAML" }));

    expect(onError).toHaveBeenCalledWith(expect.stringMatching(/state.fields/));
    expect(onParsed).not.toHaveBeenCalled();
  });
});
