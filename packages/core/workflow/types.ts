export type WorkflowNodeType =
  | "llm"
  | "agent"
  | "main_agent"
  | "subissue"
  | "router"
  | "transform"
  | "code"
  | "http"
  | "inline"
  | "condition"
  | "merge"
  | "final_response"
  | "human_gate";
export type WorkflowDispatch = "subissue" | "direct_subagent" | "inline" | "main_issue_task";
export type WorkflowCarrierKind = "issue" | "issue_task" | "agent_runtime" | "inline";

export type IssueWorkflowRole = "entry_issue" | "node_issue" | "none";

export interface IssueWorkflowContext {
  role: IssueWorkflowRole;
  workflow_case_id?: string | null;
  workflow_run_id?: string | null;
  workflow_node_id?: string | null;
  carrier_kind?: WorkflowCarrierKind | string | null;
}

export interface WorkflowOnCompleteIncrementAction {
  action: "increment";
  field: string;
}

export interface WorkflowNode {
  id: string;
  type: WorkflowNodeType;
  dispatch?: WorkflowDispatch;
  carrier_kind?: WorkflowCarrierKind | string;
  inputs?: string[];
  outputs?: string[];
  agent?: string;
  source_skill_id?: string;
  source_skill_name?: string;
  source_node_id?: string;
  modified_from_template?: boolean;
  on_complete?: WorkflowOnCompleteIncrementAction[];
  config?: {
    agent?: string;
    system?: string;
    done_criteria?: string;
    [key: string]: unknown;
  };
}

export interface WorkflowEdge {
  from: string;
  to: string;
  condition?: string;
  else?: string;
  sourceHandle?: string;
  targetHandle?: string;
}

export interface WorkflowStateField {
  name: string;
  type: string;
  required?: boolean;
  default?: unknown;
  description?: string;
}

export interface WorkflowDefinition {
  meta: { name: string; version?: string; description?: string };
  source_skills?: Array<{ id?: string; name?: string }>;
  state: { fields: WorkflowStateField[] };
  nodes: WorkflowNode[];
  routing: WorkflowEdge[];
  execution?: Record<string, unknown>;
}

export type WorkflowCaseStatus =
  | "draft"
  | "planned"
  | "running"
  | "paused"
  | "succeeded"
  | "failed"
  | "cancelling"
  | "cancelled"
  | "archived";

export interface WorkflowCase {
  id: string;
  workspace_id: string;
  title: string;
  description: string;
  entry_issue_id?: string | null;
  source_issue_id?: string | null;
  owner_agent_id?: string | null;
  status: WorkflowCaseStatus;
  online_version_id?: string | null;
  current_run_id?: string | null;
  created_at: string;
  updated_at: string;
}

export interface WorkflowDefinitionDraft {
  id: string;
  case_id: string;
  draft_json: WorkflowDefinition;
  source_templates: unknown[];
  status: "draft" | "archived";
  created_at: string;
  updated_at: string;
}

export interface WorkflowValidationReport {
  valid: boolean;
  errors: Array<{ code: string; path: string; node_id?: string; message: string }>;
  warnings: Array<{ code: string; path: string; node_id?: string; message: string }>;
}

export interface WorkflowDefinitionVersion {
  id: string;
  workspace_id: string;
  case_id: string;
  definition_id: string;
  version: number;
  snapshot_json: WorkflowDefinition;
  source_skills: Array<{ id?: string; name?: string }>;
  validation_report: WorkflowValidationReport;
  confirmed_by?: string | null;
  confirmed_at: string;
  created_at: string;
}

export type WorkflowNodeStatus = "pending" | "running" | "done" | "blocked" | "failed" | "cancelling" | "cancelled";

export interface WorkflowNodeRunState {
  status: WorkflowNodeStatus;
  sub_issue_id: string | null;
  task_id?: string | null;
  type?: WorkflowNodeType | string;
  agent?: string | null;
  source_skill_id?: string | null;
  source_skill_name?: string | null;
  source_node_id?: string | null;
  modified_from_template?: boolean | null;
  started_at: string | null;
  ended_at: string | null;
  completed_at?: string | null;
  output?: unknown;
  traex_session_id?: string | null;
  traex_log_url?: string | null;
  main_issue_task_id?: string | null;
  route_decision?: {
    condition: string;
    condition_result: boolean;
    selected_route: string;
    else_route?: string | null;
    decided_at?: string;
  } | null;
  error: string | null;
}

export type WorkflowRunStatus =
  | "pending"
  | "planning"
  | "running"
  | "finalizing"
  | "done"
  | "failed"
  | "cancelling"
  | "cancelled";

export type WorkflowRunKind = "primary" | "experiment" | "shadow" | "replay" | "debug";

export interface WorkflowRunNode {
  id: string;
  run_id: string;
  node_id: string;
  node_type: WorkflowNodeType | string;
  dispatch: WorkflowDispatch | string;
  carrier_kind?: WorkflowCarrierKind | string;
  status: "pending" | "running" | "blocked" | "succeeded" | "failed" | "cancelling" | "cancelled" | "skipped";
  attempt: number;
  input_snapshot?: unknown;
  output_snapshot?: unknown;
  error?: unknown;
  logs: unknown[];
  carrier_ref?: unknown;
  started_at?: string | null;
  completed_at?: string | null;
  updated_at: string;
}

export interface WorkflowRunNodeEvent {
  id: string;
  run_id: string;
  node_id: string;
  event_type: string;
  attempt: number;
  input_snapshot?: unknown;
  output_snapshot?: unknown;
  error?: unknown;
  logs?: unknown;
  carrier_ref?: unknown;
  sequence?: number | null;
  occurred_at?: string | null;
  created_at: string;
}

export interface WorkflowRun {
  id: string;
  root_issue_id?: string | null;
  case_id?: string | null;
  definition_version_id?: string | null;
  skill_id: string | null;
  planner_task_id?: string | null;
  source_skills?: Array<{ id?: string; name?: string }>;
  status: WorkflowRunStatus;
  run_kind?: WorkflowRunKind | string;
  label?: string;
  current_node: string;
  nodes_state: Record<string, WorkflowNodeRunState>;
  nodes?: WorkflowRunNode[];
  definition_snapshot: WorkflowDefinition;
  error: string | null;
  cancel_reason?: string | null;
  cancelled_by?: string | null;
  cancelled_at?: string | null;
  started_at?: string | null;
  completed_at?: string | null;
  created_at: string;
  updated_at: string;
}
