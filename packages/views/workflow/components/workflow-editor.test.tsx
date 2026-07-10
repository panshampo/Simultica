import { fireEvent, render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { WorkflowEditor } from "./workflow-editor";

vi.mock("sonner", () => ({
  toast: { success: vi.fn(), error: vi.fn() },
}));

vi.mock("./workflow-canvas", () => ({
  WorkflowCanvas: () => <div data-testid="workflow-canvas" />,
}));

describe("WorkflowEditor structured editing", () => {
  let onSave: ReturnType<typeof vi.fn<(yaml: string) => Promise<void>>>;

  beforeEach(() => {
    onSave = vi.fn<(yaml: string) => Promise<void>>().mockResolvedValue(undefined);
  });

  it("keeps state fields hidden until the toolbar toggle is opened", () => {
    render(<WorkflowEditor agents={[]} onSave={onSave} />);

    expect(screen.queryByLabelText("New field name")).not.toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: "State fields" }));

    expect(screen.getByLabelText("New field name")).toBeInTheDocument();
  });

  it("adds a state field and uses it as a node output", () => {
    render(<WorkflowEditor agents={[]} onSave={onSave} />);

    fireEvent.click(screen.getByRole("button", { name: "State fields" }));
    fireEvent.change(screen.getByLabelText("New field name"), { target: { value: "review_result" } });
    fireEvent.change(screen.getByLabelText("New field type"), { target: { value: "string" } });
    fireEvent.click(screen.getByRole("button", { name: "Add field" }));

    fireEvent.click(screen.getByRole("button", { name: "Add node" }));
    fireEvent.click(screen.getByLabelText("Output review_result"));

    expect(screen.getByLabelText<HTMLInputElement>("Output review_result").checked).toBe(true);
  });

  it("saves unapplied YAML edits from the YAML tab via onSave", async () => {
    const yaml = "meta:\n  name: changed\nstate:\n  fields: []\nnodes: []\nrouting: []\n";
    render(<WorkflowEditor agents={[]} onSave={onSave} />);

    fireEvent.click(screen.getByRole("button", { name: "YAML" }));
    fireEvent.change(screen.getByLabelText("Workflow YAML"), { target: { value: yaml } });
    fireEvent.click(screen.getByRole("button", { name: "Save workflow" }));

    await vi.waitFor(() => expect(onSave).toHaveBeenCalledWith(yaml));
  });

  it("renders workflow in read-only mode without edit or save controls", () => {
    const yaml = "meta:\n  name: readonly\nstate:\n  fields: []\nnodes:\n  - id: plan\n    type: llm\nrouting:\n  - from: START\n    to: plan\n";
    render(<WorkflowEditor agents={[]} onSave={onSave} initialYaml={yaml} readOnly />);

    expect(screen.getByTestId("workflow-canvas")).toBeInTheDocument();
    expect(screen.getByText("Read-only")).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Add node" })).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Add edge" })).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Save workflow" })).not.toBeInTheDocument();
  });

  it("uses a custom save label when provided", () => {
    render(<WorkflowEditor agents={[]} onSave={onSave} saveLabel="Save draft" />);
    expect(screen.getByRole("button", { name: "Save draft" })).toBeInTheDocument();
  });
});
