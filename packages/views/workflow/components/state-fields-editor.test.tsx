import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import type { WorkflowDefinition } from "@multica/core/workflow/types";
import { StateFieldsEditor } from "./state-fields-editor";

const definition: WorkflowDefinition = {
  meta: { name: "test" },
  state: { fields: [{ name: "task", type: "string" }] },
  nodes: [{ id: "review", type: "llm", outputs: ["task"] }],
  routing: [],
};

describe("StateFieldsEditor", () => {
  it("adds a state field and warns before deleting a referenced field", () => {
    const onChange = vi.fn();
    render(<StateFieldsEditor definition={definition} onChange={onChange} />);

    fireEvent.change(screen.getByLabelText("New field name"), { target: { value: "result" } });
    fireEvent.change(screen.getByLabelText("New field type"), { target: { value: "string" } });
    fireEvent.click(screen.getByRole("button", { name: "Add field" }));

    expect(onChange).toHaveBeenCalledWith(expect.objectContaining({
      state: { fields: [{ name: "task", type: "string" }, { name: "result", type: "string" }] },
    }));

    fireEvent.click(screen.getByRole("button", { name: "Delete field task" }));
    expect(screen.getByText("node review outputs")).toBeInTheDocument();
  });
});
