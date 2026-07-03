import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
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
  it("adds a state field and uses it as a node output", () => {
    render(<WorkflowEditor skillId="skill-1" agents={[]} />);

    fireEvent.change(screen.getByLabelText("New field name"), { target: { value: "review_result" } });
    fireEvent.change(screen.getByLabelText("New field type"), { target: { value: "string" } });
    fireEvent.click(screen.getByRole("button", { name: "Add field" }));

    fireEvent.click(screen.getByRole("button", { name: "Add node" }));
    fireEvent.change(screen.getByLabelText("Outputs"), { target: { value: "review_result" } });

    expect(screen.getByLabelText<HTMLSelectElement>("Outputs").value).toBe("review_result");
  });
});
