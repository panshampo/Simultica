import type { WorkflowDispatch, WorkflowNodeType } from "@multica/core/workflow/types";

export type WorkflowNodeTypeConfig = {
  type: string;
  label: string;
  defaultDispatch: WorkflowDispatch;
  supportsAgentRoute: boolean;
  supportsSystemPrompt: boolean;
  supportsDoneCriteria: boolean;
  supportsInputs: boolean;
  supportsOutputs: boolean;
  supportsTransformMap: boolean;
  supportsConditionConfig: boolean;
  supportsMergeConfig: boolean;
  supportsAdvancedConfig: boolean;
};

const base = {
  supportsAgentRoute: false,
  supportsSystemPrompt: false,
  supportsDoneCriteria: false,
  supportsInputs: true,
  supportsOutputs: true,
  supportsTransformMap: false,
  supportsConditionConfig: false,
  supportsMergeConfig: false,
  supportsAdvancedConfig: true,
} satisfies Omit<WorkflowNodeTypeConfig, "type" | "label" | "defaultDispatch">;

export const NODE_TYPE_CONFIGS: Partial<Record<WorkflowNodeType, WorkflowNodeTypeConfig>> = {
  llm: {
    ...base,
    type: "llm",
    label: "LLM",
    defaultDispatch: "subissue",
    supportsAgentRoute: true,
    supportsSystemPrompt: true,
    supportsDoneCriteria: true,
  },
  agent: {
    ...base,
    type: "agent",
    label: "Agent",
    defaultDispatch: "subissue",
    supportsAgentRoute: true,
    supportsSystemPrompt: true,
    supportsDoneCriteria: true,
  },
  subissue: {
    ...base,
    type: "subissue",
    label: "Sub-issue",
    defaultDispatch: "subissue",
    supportsAgentRoute: true,
    supportsSystemPrompt: true,
    supportsDoneCriteria: true,
  },
  main_agent: {
    ...base,
    type: "main_agent",
    label: "Main agent",
    defaultDispatch: "main_issue_task",
    supportsSystemPrompt: true,
    supportsDoneCriteria: true,
  },
  final_response: {
    ...base,
    type: "final_response",
    label: "Final response",
    defaultDispatch: "main_issue_task",
    supportsSystemPrompt: true,
    supportsOutputs: false,
  },
  transform: {
    ...base,
    type: "transform",
    label: "Transform",
    defaultDispatch: "inline",
    supportsTransformMap: true,
  },
  condition: {
    ...base,
    type: "condition",
    label: "Condition",
    defaultDispatch: "inline",
    supportsConditionConfig: true,
    supportsOutputs: false,
  },
  merge: {
    ...base,
    type: "merge",
    label: "Merge",
    defaultDispatch: "inline",
    supportsMergeConfig: true,
  },
  router: {
    ...base,
    type: "router",
    label: "Router",
    defaultDispatch: "inline",
    supportsOutputs: false,
  },
  code: {
    ...base,
    type: "code",
    label: "Code",
    defaultDispatch: "inline",
  },
  http: {
    ...base,
    type: "http",
    label: "HTTP",
    defaultDispatch: "inline",
  },
};

export function getNodeTypeConfig(type: WorkflowNodeType | string): WorkflowNodeTypeConfig {
  return NODE_TYPE_CONFIGS[type as WorkflowNodeType] ?? {
    ...base,
    type,
    label: type,
    defaultDispatch: "inline",
  };
}

export function isAgentLikeNode(type: WorkflowNodeType | string): boolean {
  return getNodeTypeConfig(type).supportsSystemPrompt;
}
