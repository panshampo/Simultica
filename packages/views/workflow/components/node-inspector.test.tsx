import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import type { WorkflowStateField } from "@multica/core/workflow/types";
import { NodeInspector } from "./node-inspector";

const fields: WorkflowStateField[] = [
  { name: "task", type: "string" },
  { name: "result", type: "string" },
];

describe("NodeInspector", () => {
  it("shows system prompt for llm nodes", () => {
    render(<NodeInspector node={{ id: "review", type: "llm", config: {} }} stateFields={fields} agents={[]} onChange={vi.fn()} onRemove={vi.fn()} />);

    expect(screen.getByLabelText("System prompt")).toBeInTheDocument();
  });

  it("hides system prompt for transform nodes", () => {
    render(<NodeInspector node={{ id: "map", type: "transform", config: {} }} stateFields={fields} agents={[]} onChange={vi.fn()} onRemove={vi.fn()} />);

    expect(screen.queryByLabelText("System prompt")).not.toBeInTheDocument();
    expect(screen.getByText("Transform map")).toBeInTheDocument();
  });

  it("updates outputs from declared state fields", () => {
    const onChange = vi.fn();
    render(<NodeInspector node={{ id: "review", type: "llm", outputs: ["task"], config: {} }} stateFields={fields} agents={[]} onChange={onChange} onRemove={vi.fn()} />);

    fireEvent.click(screen.getByLabelText("Output result"));

    expect(onChange).toHaveBeenCalledWith(expect.objectContaining({ outputs: ["task", "result"] }));
  });

  it("preserves multiple outputs while editing", () => {
    const onChange = vi.fn();
    render(<NodeInspector node={{ id: "review", type: "llm", outputs: ["task", "result"], config: {} }} stateFields={fields} agents={[]} onChange={onChange} onRemove={vi.fn()} />);

    fireEvent.click(screen.getByLabelText("Output task"));

    expect(onChange).toHaveBeenCalledWith(expect.objectContaining({ outputs: ["result"] }));
  });
});
