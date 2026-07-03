import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import type { WorkflowEdge, WorkflowStateField } from "@multica/core/workflow/types";
import { EdgeInspector } from "./edge-inspector";

const fields: WorkflowStateField[] = [
  { name: "workflow_status", type: "string" },
  { name: "revisionCount", type: "number" },
];

describe("EdgeInspector", () => {
  it("edits a raw condition expression", () => {
    const edge: WorkflowEdge = { from: "review", to: "fix", condition: 'workflow_status == "done"', else: "final" };
    const onChange = vi.fn();

    render(<EdgeInspector edge={edge} stateFields={fields} nodeIds={["review", "fix", "final"]} onChange={onChange} onRemove={vi.fn()} />);

    fireEvent.click(screen.getByRole("button", { name: "Raw expression" }));
    fireEvent.change(screen.getByLabelText("Condition expression"), { target: { value: 'workflow_status == "fixable_auto"' } });

    expect(onChange).toHaveBeenCalledWith({ condition: 'workflow_status == "fixable_auto"' });
  });

  it("sets else target for conditional routes", () => {
    const onChange = vi.fn();
    render(<EdgeInspector edge={{ from: "review", to: "fix", condition: 'workflow_status == "done"' }} stateFields={fields} nodeIds={["review", "fix", "final"]} onChange={onChange} onRemove={vi.fn()} />);

    fireEvent.change(screen.getByLabelText("Else target"), { target: { value: "final" } });

    expect(onChange).toHaveBeenCalledWith({ else: "final" });
  });
});
