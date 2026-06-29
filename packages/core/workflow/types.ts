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

export interface WorkflowNode {
  id: string;
  type: WorkflowNodeType;
  dispatch?: WorkflowDispatch;
  inputs?: string[];
  outputs?: string[];
  agent?: string;
  source_skill_id?: string;
  source_skill_name?: string;
  source_node_id?: string;
  modified_from_template?: boolean;
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

export type WorkflowNodeStatus = "pending" | "running" | "done" | "failed" | "cancelling" | "cancelled";

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

export interface WorkflowRun {
  id: string;
  root_issue_id: string;
  skill_id: string | null;
  planner_task_id?: string | null;
  source_skills?: Array<{ id?: string; name?: string }>;
  status: WorkflowRunStatus;
  current_node: string;
  nodes_state: Record<string, WorkflowNodeRunState>;
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
