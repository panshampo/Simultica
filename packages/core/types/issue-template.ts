import type { Issue, IssueAssigneeType, IssueMetadata } from "./issue";

export type IssueTemplateAssigneeType = Extract<IssueAssigneeType, "agent" | "squad">;

export interface IssueTemplate {
  id: string;
  workspace_id: string;
  project_id: string | null;
  title: string;
  issue_title_template: string;
  issue_body_template: string | null;
  assignee_type: IssueTemplateAssigneeType;
  assignee_id: string;
  priority: string;
  labels?: unknown;
  default_metadata?: IssueMetadata | null;
  execution_spec?: unknown;
  created_by_type: string;
  created_by_id: string;
  created_at: string;
  updated_at: string;
  automation_ref_count?: number;
  issue_ref_count?: number;
}

export interface CreateIssueTemplateRequest {
  project_id?: string | null;
  title: string;
  issue_title_template: string;
  issue_body_template?: string | null;
  assignee_type: IssueTemplateAssigneeType;
  assignee_id: string;
  priority?: string;
  labels?: unknown;
  default_metadata?: IssueMetadata | null;
  execution_spec?: unknown;
}

export interface UpdateIssueTemplateRequest {
  project_id?: string | null;
  title?: string;
  issue_title_template?: string;
  issue_body_template?: string | null;
  assignee_type?: IssueTemplateAssigneeType;
  assignee_id?: string;
  priority?: string;
  labels?: unknown;
  default_metadata?: IssueMetadata | null;
  execution_spec?: unknown;
}

export type ListIssueTemplatesResponse = IssueTemplate[];

export type InstantiateIssueTemplateResponse = Issue;
