import type { IssueTemplate } from "@multica/core/types";

export function formatTemplateDate(date: string): string {
  return new Intl.DateTimeFormat(undefined, {
    month: "short",
    day: "numeric",
  }).format(new Date(date));
}

export function templatePreview(template: IssueTemplate): string {
  return (
    template.issue_body_template?.trim() ||
    template.issue_title_template.trim() ||
    template.title
  );
}
