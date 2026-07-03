import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { WorkflowYamlEditor } from "./workflow-yaml-editor";

describe("WorkflowYamlEditor", () => {
  it("emits parsed YAML and shows parse errors", () => {
    const onParsed = vi.fn();
    render(<WorkflowYamlEditor initialYaml={"meta:\n  name: test\nstate:\n  fields: []\nnodes: []\nrouting: []\n"} onParsed={onParsed} />);

    fireEvent.change(screen.getByLabelText("Workflow YAML"), { target: { value: "meta: [" } });
    fireEvent.click(screen.getByRole("button", { name: "Apply YAML" }));

    expect(screen.getByText(/YAML parse failed/)).toBeInTheDocument();
    expect(onParsed).not.toHaveBeenCalled();
  });
});
