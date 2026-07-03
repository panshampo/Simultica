import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

vi.mock("../../i18n", () => ({
  useT: () => ({
    t: (selector: (value: any) => string) => selector({
      detail: {
        name_aria: "Skill name",
        name_placeholder: "skill-name",
        description_label: "Description",
        description_placeholder: "One sentence describing when an agent should use this skill...",
      },
    }),
  }),
}));

vi.mock("@multica/ui/components/common/actor-avatar", () => ({
  ActorAvatar: ({ name }: { name: string }) => <span data-testid="avatar">{name.slice(0, 2)}</span>,
}));

import { SkillHeaderCompact } from "./skill-detail-page";

describe("SkillHeaderCompact", () => {
  it("renders compact metadata and keeps description overflow bounded", () => {
    const onNameChange = vi.fn();
    const onDescriptionChange = vi.fn();
    const longDescription = `Use this skill for ${"very-long-description-without-spaces".repeat(8)} https://example.com/${"long".repeat(20)}`;

    render(
      <SkillHeaderCompact
        name="review-helper"
        description={longDescription}
        canEdit
        originLabel="Imported · GitHub"
        originType="github"
        updatedLabel="Updated today"
        createdLabel="Created yesterday"
        creatorName="Alice"
        creatorAvatarUrl={null}
        fileCountLabel="Files 4"
        idLabel="ID abc12345..."
        idTitle="abc12345-full"
        onNameChange={onNameChange}
        onDescriptionChange={onDescriptionChange}
      />,
    );

    expect(screen.getByDisplayValue("review-helper")).toBeInTheDocument();
    expect(screen.getByDisplayValue(longDescription)).toHaveClass("max-h-16", "overflow-y-auto", "break-words");
    expect(screen.getByText("Imported · GitHub")).toBeInTheDocument();
    expect(screen.getByText("Updated today")).toBeInTheDocument();
    expect(screen.getByText("Created yesterday")).toBeInTheDocument();
    expect(screen.getByText("Files 4")).toBeInTheDocument();
    expect(screen.queryByText("Metadata")).not.toBeInTheDocument();
  });

  it("emits name and description edits", () => {
    const onNameChange = vi.fn();
    const onDescriptionChange = vi.fn();

    render(
      <SkillHeaderCompact
        name="review-helper"
        description="Review code"
        canEdit
        originLabel={null}
        updatedLabel="Updated today"
        createdLabel="Created yesterday"
        fileCountLabel="Files 1"
        idLabel="ID abc12345..."
        idTitle="abc12345-full"
        onNameChange={onNameChange}
        onDescriptionChange={onDescriptionChange}
      />,
    );

    fireEvent.change(screen.getByRole("textbox", { name: "Skill name" }), { target: { value: "new-name" } });
    fireEvent.change(screen.getByLabelText("Description"), { target: { value: "New description" } });

    expect(onNameChange).toHaveBeenCalledWith("new-name");
    expect(onDescriptionChange).toHaveBeenCalledWith("New description");
  });
});
