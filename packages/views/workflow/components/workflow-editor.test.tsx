import { fireEvent, render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { api } from "@multica/core/api";
import { WorkflowEditor } from "./workflow-editor";

vi.mock("@multica/core/api", () => ({
  api: {
    upsertSkillFile: vi.fn().mockResolvedValue({}),
  },
}));

vi.mock("sonner", () => ({
  toast: { success: vi.fn(), error: vi.fn() },
}));

vi.mock("./workflow-canvas", () => ({
  WorkflowCanvas: () => <div data-testid="workflow-canvas" />,
}));

describe("WorkflowEditor structured editing", () => {
  beforeEach(() => {
    vi.mocked(api.upsertSkillFile).mockClear();
  });

  it("keeps state fields hidden until the toolbar toggle is opened", () => {
    render(<WorkflowEditor skillId="skill-1" agents={[]} />);

    expect(screen.queryByLabelText("New field name")).not.toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: "State fields" }));

    expect(screen.getByLabelText("New field name")).toBeInTheDocument();
  });

  it("adds a state field and uses it as a node output", () => {
    render(<WorkflowEditor skillId="skill-1" agents={[]} />);

    fireEvent.click(screen.getByRole("button", { name: "State fields" }));
    fireEvent.change(screen.getByLabelText("New field name"), { target: { value: "review_result" } });
    fireEvent.change(screen.getByLabelText("New field type"), { target: { value: "string" } });
    fireEvent.click(screen.getByRole("button", { name: "Add field" }));

    fireEvent.click(screen.getByRole("button", { name: "Add node" }));
    fireEvent.click(screen.getByLabelText("Output review_result"));

    expect(screen.getByLabelText<HTMLInputElement>("Output review_result").checked).toBe(true);
  });

  it("saves unapplied YAML edits from the YAML tab", async () => {
    const yaml = "meta:\n  name: changed\nstate:\n  fields: []\nnodes: []\nrouting: []\n";
    render(<WorkflowEditor skillId="skill-1" agents={[]} />);

    fireEvent.click(screen.getByRole("button", { name: "YAML" }));
    fireEvent.change(screen.getByLabelText("Workflow YAML"), { target: { value: yaml } });
    fireEvent.click(screen.getByRole("button", { name: "Save workflow" }));

    await vi.waitFor(() => expect(api.upsertSkillFile).toHaveBeenCalledWith("skill-1", { path: "workflow.yaml", content: yaml }));
  });

  it("renders workflow in read-only mode without edit or save controls", () => {
    const yaml = "meta:\n  name: readonly\nstate:\n  fields: []\nnodes:\n  - id: plan\n    type: llm\nrouting:\n  - from: START\n    to: plan\n";
    render(<WorkflowEditor skillId="skill-1" agents={[]} initialYaml={yaml} readOnly />);

    expect(screen.getByTestId("workflow-canvas")).toBeInTheDocument();
    expect(screen.getByText("Read-only")).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Add node" })).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Add edge" })).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Save workflow" })).not.toBeInTheDocument();
  });
});
