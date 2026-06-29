// @vitest-environment jsdom

import { act, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import type { ReactNode } from "react";
import type { AgentTask } from "@multica/core/types/agent";
import type { TaskMessagePayload } from "@multica/core/types/events";
import { describe, expect, it, vi } from "vitest";
import { TranscriptButton } from "./transcript-button";
import type { TimelineItem } from "./build-timeline";

vi.mock("@multica/core/api", () => ({
  api: {
    listTaskMessages: vi.fn(),
  },
}));

vi.mock("./agent-transcript-dialog", () => ({
  AgentTranscriptDialog: ({
    open,
    onOpenChange,
    items,
  }: {
    open: boolean;
    onOpenChange: (open: boolean) => void;
    items: TimelineItem[];
  }) =>
    open ? (
      <div role="dialog">
        <div data-testid="event-count">{items.length}</div>
        <button type="button" onClick={() => onOpenChange(false)}>
          Close
        </button>
      </div>
    ) : null,
}));

const task: AgentTask = {
  id: "task-1",
  agent_id: "agent-1",
  runtime_id: "",
  issue_id: "issue-1",
  status: "completed",
  priority: 0,
  dispatched_at: "2026-05-15T10:00:05.000Z",
  started_at: "2026-05-15T10:00:06.000Z",
  completed_at: "2026-05-15T10:00:10.000Z",
  result: null,
  error: null,
  created_at: "2026-05-15T10:00:00.000Z",
};

const items: TimelineItem[] = [
  {
    seq: 1,
    type: "text",
    content: "hello world",
  },
];

function renderWithQuery(ui: ReactNode) {
  const queryClient = new QueryClient({
    defaultOptions: {
      queries: { retry: false },
    },
  });
  return {
    queryClient,
    ...render(
      <QueryClientProvider client={queryClient}>
        {ui}
      </QueryClientProvider>,
    ),
  };
}

describe("TranscriptButton", () => {
  it("closes the transcript dialog when desktop navigation starts", async () => {
    renderWithQuery(<TranscriptButton task={task} agentName="Codex" items={items} />);

    fireEvent.click(screen.getByRole("button", { name: "View transcript" }));
    expect(screen.getByRole("dialog")).toBeInTheDocument();

    act(() => {
      window.dispatchEvent(
        new CustomEvent("multica:navigate", {
          detail: { path: "/acme/inbox?issue=MUL-123" },
        }),
      );
    });

    await waitFor(() => {
      expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
    });
  });

  it("keeps a live transcript subscribed to task message cache updates", async () => {
    const liveTask = {
      ...task,
      id: "4a2e8d1c-7f9b-4e2a-9c1d-123456789abc",
      status: "running" as const,
      completed_at: null,
    };
    const { queryClient } = renderWithQuery(
      <TranscriptButton task={liveTask} agentName="Codex" isLive />,
    );

    queryClient.setQueryData<TaskMessagePayload[]>(
      ["task-messages", liveTask.id],
      [
        {
          task_id: liveTask.id,
          issue_id: liveTask.issue_id,
          seq: 1,
          type: "tool_use",
          tool: "exec_command",
        },
      ],
    );

    fireEvent.click(screen.getByRole("button", { name: "View transcript" }));
    expect(await screen.findByRole("dialog")).toBeInTheDocument();
    expect(screen.getByTestId("event-count")).toHaveTextContent("1");

    act(() => {
      queryClient.setQueryData<TaskMessagePayload[]>(
        ["task-messages", liveTask.id],
        [
          {
            task_id: liveTask.id,
            issue_id: liveTask.issue_id,
            seq: 1,
            type: "tool_use",
            tool: "exec_command",
          },
          {
            task_id: liveTask.id,
            issue_id: liveTask.issue_id,
            seq: 2,
            type: "tool_result",
            tool: "exec_command",
          },
        ],
      );
    });

    await waitFor(() => {
      expect(screen.getByTestId("event-count")).toHaveTextContent("2");
    });
  });
});
