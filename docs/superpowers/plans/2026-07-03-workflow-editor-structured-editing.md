# Workflow Editor Structured Editing Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Upgrade the skill workflow editor from a thin YAML form into a structured, schema-aware editor that still allows raw YAML and advanced raw expressions.

**Architecture:** Keep `workflow.yaml` as the single source of truth. Add a frontend workflow schema registry and editor model helpers so the UI can constrain common edits without losing unknown YAML fields. Runtime/backend validation remains the final authority; the frontend mirrors enough of those rules to explain mistakes early.

**Tech Stack:** React/TypeScript in `packages/views`, shared workflow types in `packages/core/workflow`, YAML parser in `packages/views/workflow/lib/serialize.ts`, React Flow canvas in `packages/views/workflow/components`, Vitest + Testing Library with Node 22.

## Global Constraints

- Keep raw YAML editing available.
- Do not introduce a second workflow storage format; persist back to `workflow.yaml`.
- `state.fields` is the workflow data contract; structured UI must not create undeclared `inputs`, `outputs`, condition fields, or increment fields.
- Field pickers must support inline field creation to avoid forcing users to switch panels.
- Condition builder V1 supports common RouterDSL only: field truthy/falsy, `==`, `!=`, `<`, `>`, `<=`, `>=`, one-level AND/OR groups.
- Complex conditions remain editable through raw expression mode.
- Unknown YAML fields must be preserved where possible; the UI may expose them through Advanced sections instead of deleting them.
- Use plain language UI copy: explain what a setting does, why it matters, and what happens if it is wrong.
- Verify with Node 22: `PATH="$HOME/.nvm/versions/node/v22.22.2/bin:$PATH" ...`.

---

## File Structure

- Create `packages/views/workflow/lib/schema-registry.ts`
  - Owns node type UI rules: labels, defaults, visible config sections, allowed dispatches, outputs strategy.
- Create `packages/views/workflow/lib/editor-model.ts`
  - Owns immutable update helpers for `WorkflowDefinition`, field references, inline field creation, and unknown field preservation helpers.
- Create `packages/views/workflow/lib/condition-builder.ts`
  - Owns V1 condition builder model, RouterDSL string generation, and best-effort parsing from raw expression to builder rows.
- Create `packages/views/workflow/components/state-fields-editor.tsx`
  - Renders and edits `state.fields`.
- Create `packages/views/workflow/components/field-picker.tsx`
  - Shared picker for inputs, outputs, condition fields, and increment fields with inline create.
- Create `packages/views/workflow/components/node-inspector.tsx`
  - Replaces the current inline selected-node form in `workflow-editor.tsx`.
- Create `packages/views/workflow/components/edge-inspector.tsx`
  - Replaces the current inline selected-edge form and adds condition builder + raw expression.
- Create `packages/views/workflow/components/workflow-yaml-editor.tsx`
  - Raw YAML tab with parse error handling.
- Create `packages/views/workflow/components/workflow-validation-panel.tsx`
  - Shows frontend validation errors and warnings in plain language.
- Modify `packages/views/workflow/components/workflow-editor.tsx`
  - Becomes composition/root state owner instead of holding all inspector UI inline.
- Modify `packages/core/workflow/types.ts`
  - Add optional fields only if needed by editor model. Do not change runtime semantics casually.
- Modify `packages/views/workflow/lib/serialize.ts`
  - Add parse result helpers that preserve last valid definition and expose parse errors.
- Tests:
  - `packages/views/workflow/lib/schema-registry.test.ts`
  - `packages/views/workflow/lib/editor-model.test.ts`
  - `packages/views/workflow/lib/condition-builder.test.ts`
  - `packages/views/workflow/components/workflow-editor.test.tsx`
  - Extend existing `workflow-canvas.test.tsx` and `to-react-flow.test.ts` only when canvas behavior changes.

---

## Task 1: Add Workflow Schema Registry

**Why:** Right now every node shows nearly the same fields. That is confusing because many fields do nothing for some node types. The registry is a small table that tells the UI what each node type supports.

**Files:**
- Create: `packages/views/workflow/lib/schema-registry.ts`
- Test: `packages/views/workflow/lib/schema-registry.test.ts`

**Interfaces:**
- Produces:
  - `type WorkflowNodeTypeConfig`
  - `const NODE_TYPE_CONFIGS: Record<WorkflowNodeType, WorkflowNodeTypeConfig>`
  - `function getNodeTypeConfig(type: WorkflowNodeType | string): WorkflowNodeTypeConfig`
  - `function isAgentLikeNode(type: WorkflowNodeType | string): boolean`

- [ ] **Step 1: Write failing tests**

Create `packages/views/workflow/lib/schema-registry.test.ts`:

```ts
import { describe, expect, it } from "vitest";
import { getNodeTypeConfig, isAgentLikeNode } from "./schema-registry";

describe("workflow schema registry", () => {
  it("shows system prompt only for agent-like nodes", () => {
    expect(isAgentLikeNode("llm")).toBe(true);
    expect(isAgentLikeNode("agent")).toBe(true);
    expect(isAgentLikeNode("subissue")).toBe(true);
    expect(isAgentLikeNode("main_agent")).toBe(true);
    expect(isAgentLikeNode("final_response")).toBe(true);

    expect(isAgentLikeNode("transform")).toBe(false);
    expect(isAgentLikeNode("condition")).toBe(false);
    expect(isAgentLikeNode("merge")).toBe(false);
    expect(isAgentLikeNode("router")).toBe(false);
  });

  it("returns sensible defaults for known and unknown node types", () => {
    expect(getNodeTypeConfig("llm")).toMatchObject({
      label: "LLM",
      defaultDispatch: "subissue",
      supportsSystemPrompt: true,
      supportsOutputs: true,
    });

    expect(getNodeTypeConfig("transform")).toMatchObject({
      label: "Transform",
      defaultDispatch: "inline",
      supportsSystemPrompt: false,
      supportsTransformMap: true,
    });

    expect(getNodeTypeConfig("custom_runtime_node")).toMatchObject({
      label: "custom_runtime_node",
      defaultDispatch: "inline",
      supportsAdvancedConfig: true,
    });
  });
});
```

- [ ] **Step 2: Run red test**

Run:

```bash
PATH="$HOME/.nvm/versions/node/v22.22.2/bin:$PATH" pnpm --filter @multica/views test -- workflow/lib/schema-registry.test.ts
```

Expected: FAIL because `schema-registry.ts` does not exist.

- [ ] **Step 3: Implement registry**

Create `packages/views/workflow/lib/schema-registry.ts`:

```ts
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
```

- [ ] **Step 4: Run green test**

Run same command. Expected: PASS.

---

## Task 2: Add Editor Model Helpers For State Fields

**Why:** `state.fields` is the contract. If outputs or conditions can reference undeclared fields, the workflow looks okay in UI but fails later. These helpers keep field references consistent.

**Files:**
- Create: `packages/views/workflow/lib/editor-model.ts`
- Test: `packages/views/workflow/lib/editor-model.test.ts`

**Interfaces:**
- Produces:
  - `function createStateField(name: string, type: WorkflowStateField["type"]): WorkflowStateField`
  - `function addStateField(definition: WorkflowDefinition, field: WorkflowStateField): WorkflowDefinition`
  - `function removeStateField(definition: WorkflowDefinition, fieldName: string): WorkflowDefinition`
  - `function findStateFieldReferences(definition: WorkflowDefinition, fieldName: string): string[]`
  - `function upsertNodeOutputs(definition: WorkflowDefinition, nodeId: string, outputs: string[]): WorkflowDefinition`

- [ ] **Step 1: Write failing tests**

Create `packages/views/workflow/lib/editor-model.test.ts`:

```ts
import { describe, expect, it } from "vitest";
import type { WorkflowDefinition } from "@multica/core/workflow/types";
import { addStateField, createStateField, findStateFieldReferences, removeStateField, upsertNodeOutputs } from "./editor-model";

const baseDefinition: WorkflowDefinition = {
  meta: { name: "test" },
  state: { fields: [{ name: "task", type: "string" }] },
  nodes: [{ id: "review", type: "llm", outputs: ["result"] }],
  routing: [{ from: "review", condition: 'result == "ok"', to: "END", else: "review" }],
};

describe("workflow editor model", () => {
  it("creates and adds unique state fields", () => {
    const next = addStateField(baseDefinition, createStateField("result", "string"));

    expect(next.state.fields).toEqual([
      { name: "task", type: "string" },
      { name: "result", type: "string" },
    ]);
    expect(addStateField(next, createStateField("result", "string"))).toBe(next);
  });

  it("finds references before deleting a field", () => {
    expect(findStateFieldReferences(baseDefinition, "result")).toEqual([
      "node review outputs",
      "route review -> END condition",
    ]);
  });

  it("removes fields without mutating the original definition", () => {
    const next = removeStateField(baseDefinition, "task");

    expect(next.state.fields).toEqual([]);
    expect(baseDefinition.state.fields).toEqual([{ name: "task", type: "string" }]);
  });

  it("updates node outputs", () => {
    const next = upsertNodeOutputs(baseDefinition, "review", ["result", "summary"]);

    expect(next.nodes[0]?.outputs).toEqual(["result", "summary"]);
  });
});
```

- [ ] **Step 2: Run red test**

Run:

```bash
PATH="$HOME/.nvm/versions/node/v22.22.2/bin:$PATH" pnpm --filter @multica/views test -- workflow/lib/editor-model.test.ts
```

Expected: FAIL because `editor-model.ts` does not exist.

- [ ] **Step 3: Implement helpers**

Create `packages/views/workflow/lib/editor-model.ts`:

```ts
import type { WorkflowDefinition, WorkflowStateField } from "@multica/core/workflow/types";

export function createStateField(name: string, type: WorkflowStateField["type"]): WorkflowStateField {
  return { name: name.trim(), type };
}

export function addStateField(definition: WorkflowDefinition, field: WorkflowStateField): WorkflowDefinition {
  if (!field.name || definition.state.fields.some((existing) => existing.name === field.name)) {
    return definition;
  }
  return {
    ...definition,
    state: {
      ...definition.state,
      fields: [...definition.state.fields, field],
    },
  };
}

export function removeStateField(definition: WorkflowDefinition, fieldName: string): WorkflowDefinition {
  return {
    ...definition,
    state: {
      ...definition.state,
      fields: definition.state.fields.filter((field) => field.name !== fieldName),
    },
  };
}

export function findStateFieldReferences(definition: WorkflowDefinition, fieldName: string): string[] {
  const refs: string[] = [];
  for (const node of definition.nodes) {
    if (node.inputs?.includes(fieldName)) refs.push(`node ${node.id} inputs`);
    if (node.outputs?.includes(fieldName)) refs.push(`node ${node.id} outputs`);
    if (node.on_complete?.some((action) => action.action === "increment" && action.field === fieldName)) {
      refs.push(`node ${node.id} on_complete`);
    }
  }
  for (const route of definition.routing) {
    if (route.condition && conditionReferencesField(route.condition, fieldName)) {
      refs.push(`route ${route.from} -> ${route.to} condition`);
    }
  }
  return refs;
}

export function upsertNodeOutputs(definition: WorkflowDefinition, nodeId: string, outputs: string[]): WorkflowDefinition {
  return {
    ...definition,
    nodes: definition.nodes.map((node) => (node.id === nodeId ? { ...node, outputs } : node)),
  };
}

function conditionReferencesField(condition: string, fieldName: string): boolean {
  const escaped = fieldName.replace(/[.*+?^${}()|[\]\\]/g, "\\$&");
  return new RegExp(`(^|[^A-Za-z0-9_])${escaped}([^A-Za-z0-9_]|$)`).test(condition);
}
```

- [ ] **Step 4: Extend shared types if needed**

If TypeScript reports `on_complete` is missing on `WorkflowNode`, add to `packages/core/workflow/types.ts`:

```ts
export interface WorkflowOnCompleteIncrementAction {
  action: "increment";
  field: string;
}
```

Then add to `WorkflowNode`:

```ts
on_complete?: WorkflowOnCompleteIncrementAction[];
```

- [ ] **Step 5: Run green test**

Run same command. Expected: PASS.

---

## Task 3: Add Condition Builder Model

**Why:** Conditions are currently text. That is powerful but easy to get wrong. The builder should cover common cases while leaving raw expressions available for complex logic.

**Files:**
- Create: `packages/views/workflow/lib/condition-builder.ts`
- Test: `packages/views/workflow/lib/condition-builder.test.ts`

**Interfaces:**
- Produces:
  - `type ConditionRule`
  - `type ConditionGroup`
  - `function conditionGroupToExpression(group: ConditionGroup): string`
  - `function tryParseConditionExpression(expression: string): ConditionGroup | null`

- [ ] **Step 1: Write failing tests**

Create `packages/views/workflow/lib/condition-builder.test.ts`:

```ts
import { describe, expect, it } from "vitest";
import { conditionGroupToExpression, tryParseConditionExpression } from "./condition-builder";

describe("condition builder", () => {
  it("serializes a single equality rule", () => {
    expect(conditionGroupToExpression({
      join: "AND",
      rules: [{ field: "workflow_status", operator: "==", value: "done", valueType: "string" }],
    })).toBe('workflow_status == "done"');
  });

  it("serializes boolean and number comparisons", () => {
    expect(conditionGroupToExpression({
      join: "AND",
      rules: [
        { field: "approved", operator: "==", value: true, valueType: "boolean" },
        { field: "revisionCount", operator: "<", value: 2, valueType: "number" },
      ],
    })).toBe("approved == true && revisionCount < 2");
  });

  it("serializes truthy and falsy checks", () => {
    expect(conditionGroupToExpression({
      join: "OR",
      rules: [
        { field: "ready", operator: "truthy" },
        { field: "error", operator: "falsy" },
      ],
    })).toBe("ready || !error");
  });

  it("parses supported expressions and returns null for advanced expressions", () => {
    expect(tryParseConditionExpression('workflow_status == "done" && revisionCount < 2')).toEqual({
      join: "AND",
      rules: [
        { field: "workflow_status", operator: "==", value: "done", valueType: "string" },
        { field: "revisionCount", operator: "<", value: 2, valueType: "number" },
      ],
    });

    expect(tryParseConditionExpression('(a == "x" && b == "y") || c == true')).toBe(null);
  });
});
```

- [ ] **Step 2: Run red test**

Run:

```bash
PATH="$HOME/.nvm/versions/node/v22.22.2/bin:$PATH" pnpm --filter @multica/views test -- workflow/lib/condition-builder.test.ts
```

Expected: FAIL because file does not exist.

- [ ] **Step 3: Implement builder model**

Create `packages/views/workflow/lib/condition-builder.ts`:

```ts
export type ConditionOperator = "==" | "!=" | "<" | ">" | "<=" | ">=" | "truthy" | "falsy";

export type ConditionRule = {
  field: string;
  operator: ConditionOperator;
  value?: string | number | boolean;
  valueType?: "string" | "number" | "boolean";
};

export type ConditionGroup = {
  join: "AND" | "OR";
  rules: ConditionRule[];
};

export function conditionGroupToExpression(group: ConditionGroup): string {
  const joiner = group.join === "AND" ? " && " : " || ";
  return group.rules.map(ruleToExpression).join(joiner);
}

export function tryParseConditionExpression(expression: string): ConditionGroup | null {
  const trimmed = expression.trim();
  if (!trimmed || trimmed.includes("(") || trimmed.includes(")")) return null;
  const join = trimmed.includes(" || ") ? "OR" : "AND";
  const splitter = join === "OR" ? " || " : " && ";
  const parts = trimmed.split(splitter).map((part) => part.trim()).filter(Boolean);
  const rules = parts.map(parseRule);
  if (rules.some((rule) => rule == null)) return null;
  return { join, rules: rules as ConditionRule[] };
}

function ruleToExpression(rule: ConditionRule): string {
  if (rule.operator === "truthy") return rule.field;
  if (rule.operator === "falsy") return `!${rule.field}`;
  return `${rule.field} ${rule.operator} ${formatValue(rule.value, rule.valueType)}`;
}

function formatValue(value: ConditionRule["value"], valueType: ConditionRule["valueType"]): string {
  if (valueType === "number") return String(Number(value ?? 0));
  if (valueType === "boolean") return String(Boolean(value));
  return `"${String(value ?? "").replaceAll('"', '\\"')}"`;
}

function parseRule(part: string): ConditionRule | null {
  if (/^![A-Za-z_][A-Za-z0-9_]*$/.test(part)) {
    return { field: part.slice(1), operator: "falsy" };
  }
  if (/^[A-Za-z_][A-Za-z0-9_]*$/.test(part)) {
    return { field: part, operator: "truthy" };
  }
  const match = part.match(/^([A-Za-z_][A-Za-z0-9_]*)\s*(==|!=|<=|>=|<|>)\s*(.+)$/);
  if (!match) return null;
  const [, field, operator, rawValue] = match;
  const parsed = parseValue(rawValue.trim());
  return { field, operator: operator as ConditionOperator, ...parsed };
}

function parseValue(raw: string): Pick<ConditionRule, "value" | "valueType"> {
  if (raw === "true") return { value: true, valueType: "boolean" };
  if (raw === "false") return { value: false, valueType: "boolean" };
  if (/^-?\d+(\.\d+)?$/.test(raw)) return { value: Number(raw), valueType: "number" };
  const quoted = raw.match(/^["'](.*)["']$/);
  return { value: quoted ? quoted[1] : raw, valueType: "string" };
}
```

- [ ] **Step 4: Run green test**

Run same command. Expected: PASS.

---

## Task 4: Build Field Picker And State Fields Editor

**Why:** Outputs and conditions should use declared fields. A picker makes that obvious. Inline creation keeps the UI fast.

**Files:**
- Create: `packages/views/workflow/components/field-picker.tsx`
- Create: `packages/views/workflow/components/state-fields-editor.tsx`
- Test: `packages/views/workflow/components/state-fields-editor.test.tsx`

**Interfaces:**
- Consumes:
  - `createStateField`, `addStateField`, `removeStateField`, `findStateFieldReferences`
- Produces:
  - `FieldPicker`
  - `StateFieldsEditor`

- [ ] **Step 1: Write failing component test**

Create `packages/views/workflow/components/state-fields-editor.test.tsx`:

```tsx
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
```

- [ ] **Step 2: Run red test**

Run:

```bash
PATH="$HOME/.nvm/versions/node/v22.22.2/bin:$PATH" pnpm --filter @multica/views test -- workflow/components/state-fields-editor.test.tsx
```

Expected: FAIL because components do not exist.

- [ ] **Step 3: Implement `FieldPicker`**

Create `packages/views/workflow/components/field-picker.tsx`:

```tsx
import type { WorkflowStateField } from "@multica/core/workflow/types";

export function FieldPicker({
  label,
  value,
  fields,
  allowedTypes,
  onChange,
  onCreateField,
}: {
  label: string;
  value: string;
  fields: WorkflowStateField[];
  allowedTypes?: string[];
  onChange: (fieldName: string) => void;
  onCreateField?: (name: string) => void;
}) {
  const visibleFields = allowedTypes?.length
    ? fields.filter((field) => allowedTypes.includes(field.type))
    : fields;

  return (
    <label className="space-y-1 text-xs text-muted-foreground">
      <span>{label}</span>
      <select
        className="h-9 w-full rounded-md border bg-background px-2 text-sm text-foreground"
        value={value}
        onChange={(event) => onChange(event.target.value)}
      >
        <option value="">Select field</option>
        {visibleFields.map((field) => (
          <option key={field.name} value={field.name}>
            {field.name} · {field.type}
          </option>
        ))}
      </select>
      {onCreateField && value && !fields.some((field) => field.name === value) && (
        <button type="button" className="text-xs text-primary" onClick={() => onCreateField(value)}>
          Create field "{value}"
        </button>
      )}
    </label>
  );
}
```

- [ ] **Step 4: Implement `StateFieldsEditor`**

Create `packages/views/workflow/components/state-fields-editor.tsx`:

```tsx
import { useState } from "react";
import type { WorkflowDefinition, WorkflowStateField } from "@multica/core/workflow/types";
import { Button } from "@multica/ui/components/ui/button";
import { Input } from "@multica/ui/components/ui/input";
import { addStateField, createStateField, findStateFieldReferences, removeStateField } from "../lib/editor-model";

const FIELD_TYPES: WorkflowStateField["type"][] = ["string", "number", "boolean", "object", "array", "enum"];

export function StateFieldsEditor({
  definition,
  onChange,
}: {
  definition: WorkflowDefinition;
  onChange: (definition: WorkflowDefinition) => void;
}) {
  const [name, setName] = useState("");
  const [type, setType] = useState<WorkflowStateField["type"]>("string");
  const [pendingDelete, setPendingDelete] = useState<{ field: string; references: string[] } | null>(null);

  return (
    <section className="space-y-3 rounded-md border p-3">
      <div>
        <h4 className="text-sm font-medium">State fields</h4>
        <p className="text-xs text-muted-foreground">
          State fields are the workflow's shared data. Nodes write outputs here, and conditions read from here.
        </p>
      </div>

      <div className="space-y-2">
        {definition.state.fields.map((field) => (
          <div key={field.name} className="flex items-center gap-2 rounded-md border bg-muted/10 px-2 py-1.5">
            <span className="min-w-0 flex-1 truncate font-mono text-xs">{field.name}</span>
            <span className="rounded bg-muted px-1.5 py-0.5 text-[10px] uppercase text-muted-foreground">{field.type}</span>
            <Button
              type="button"
              size="xs"
              variant="ghost"
              aria-label={`Delete field ${field.name}`}
              onClick={() => {
                const references = findStateFieldReferences(definition, field.name);
                if (references.length > 0) {
                  setPendingDelete({ field: field.name, references });
                  return;
                }
                onChange(removeStateField(definition, field.name));
              }}
            >
              Delete
            </Button>
          </div>
        ))}
      </div>

      {pendingDelete && (
        <div className="rounded-md border border-amber-300 bg-amber-50 p-2 text-xs text-amber-900">
          <p className="font-medium">This field is still used:</p>
          <ul className="mt-1 list-disc pl-4">
            {pendingDelete.references.map((reference) => <li key={reference}>{reference}</li>)}
          </ul>
          <Button type="button" size="xs" variant="outline" className="mt-2" onClick={() => setPendingDelete(null)}>
            Keep field
          </Button>
        </div>
      )}

      <div className="grid grid-cols-[minmax(0,1fr)_8rem_auto] gap-2">
        <Input aria-label="New field name" value={name} onChange={(event) => setName(event.target.value)} placeholder="review_result" />
        <select aria-label="New field type" className="h-9 rounded-md border bg-background px-2 text-sm" value={type} onChange={(event) => setType(event.target.value as WorkflowStateField["type"])}>
          {FIELD_TYPES.map((fieldType) => <option key={fieldType} value={fieldType}>{fieldType}</option>)}
        </select>
        <Button type="button" size="sm" onClick={() => {
          const field = createStateField(name, type);
          const next = addStateField(definition, field);
          onChange(next);
          if (next !== definition) setName("");
        }}>
          Add field
        </Button>
      </div>
    </section>
  );
}
```

- [ ] **Step 5: Run green test**

Run same command. Expected: PASS.

---

## Task 5: Replace Inline Node Form With Node Inspector

**Why:** The current node form shows `system` for every node. That is like showing an engine oil field for a bicycle. It makes users think a setting matters when it may be ignored.

**Files:**
- Create: `packages/views/workflow/components/node-inspector.tsx`
- Test: `packages/views/workflow/components/node-inspector.test.tsx`
- Modify: `packages/views/workflow/components/workflow-editor.tsx`

**Interfaces:**
- Consumes:
  - `getNodeTypeConfig`
  - `FieldPicker`
- Produces:
  - `NodeInspector`

- [ ] **Step 1: Write failing tests**

Create `packages/views/workflow/components/node-inspector.test.tsx`:

```tsx
import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import type { WorkflowNode, WorkflowStateField } from "@multica/core/workflow/types";
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

    fireEvent.change(screen.getByLabelText("Outputs"), { target: { value: "result" } });

    expect(onChange).toHaveBeenCalledWith(expect.objectContaining({ outputs: ["result"] }));
  });
});
```

- [ ] **Step 2: Run red test**

Run:

```bash
PATH="$HOME/.nvm/versions/node/v22.22.2/bin:$PATH" pnpm --filter @multica/views test -- workflow/components/node-inspector.test.tsx
```

Expected: FAIL because `node-inspector.tsx` does not exist.

- [ ] **Step 3: Implement NodeInspector**

Create `packages/views/workflow/components/node-inspector.tsx`:

```tsx
import type { Agent } from "@multica/core/types";
import type { WorkflowNode, WorkflowStateField } from "@multica/core/workflow/types";
import { Button } from "@multica/ui/components/ui/button";
import { Input } from "@multica/ui/components/ui/input";
import { Textarea } from "@multica/ui/components/ui/textarea";
import { getNodeTypeConfig } from "../lib/schema-registry";

export function NodeInspector({
  node,
  stateFields,
  agents,
  onChange,
  onRemove,
}: {
  node: WorkflowNode;
  stateFields: WorkflowStateField[];
  agents: Pick<Agent, "id" | "name">[];
  onChange: (patch: Partial<WorkflowNode>) => void;
  onRemove: () => void;
}) {
  const config = getNodeTypeConfig(node.type);

  return (
    <div className="space-y-3">
      <label className="space-y-1 text-xs text-muted-foreground">
        <span>ID</span>
        <Input value={node.id} onChange={(event) => onChange({ id: event.target.value })} />
      </label>

      <label className="space-y-1 text-xs text-muted-foreground">
        <span>Type</span>
        <select className="h-9 w-full rounded-md border bg-background px-2 text-sm text-foreground" value={node.type} onChange={(event) => onChange({ type: event.target.value as WorkflowNode["type"] })}>
          {["llm", "agent", "subissue", "main_agent", "final_response", "router", "transform", "condition", "merge", "code", "http"].map((type) => (
            <option key={type} value={type}>{type}</option>
          ))}
        </select>
      </label>

      <label className="space-y-1 text-xs text-muted-foreground">
        <span>Dispatch</span>
        <select className="h-9 w-full rounded-md border bg-background px-2 text-sm text-foreground" value={node.dispatch ?? config.defaultDispatch} onChange={(event) => onChange({ dispatch: event.target.value as WorkflowNode["dispatch"] })}>
          {["subissue", "inline", "main_issue_task"].map((dispatch) => <option key={dispatch} value={dispatch}>{dispatch}</option>)}
        </select>
      </label>

      {config.supportsAgentRoute && (
        <label className="space-y-1 text-xs text-muted-foreground">
          <span>Agent</span>
          <select className="h-9 w-full rounded-md border bg-background px-2 text-sm text-foreground" value={node.config?.agent ?? node.agent ?? ""} onChange={(event) => onChange({ agent: event.target.value || undefined, config: { ...node.config, agent: event.target.value || undefined } })}>
            <option value="">No agent</option>
            {agents.map((agent) => <option key={agent.id} value={agent.name}>{agent.name}</option>)}
          </select>
        </label>
      )}

      {config.supportsOutputs && (
        <label className="space-y-1 text-xs text-muted-foreground">
          <span>Outputs</span>
          <select
            aria-label="Outputs"
            className="h-9 w-full rounded-md border bg-background px-2 text-sm text-foreground"
            value={node.outputs?.[0] ?? ""}
            onChange={(event) => onChange({ outputs: event.target.value ? [event.target.value] : [] })}
          >
            <option value="">No output</option>
            {stateFields.map((field) => <option key={field.name} value={field.name}>{field.name} · {field.type}</option>)}
          </select>
        </label>
      )}

      {config.supportsSystemPrompt && (
        <label className="space-y-1 text-xs text-muted-foreground">
          <span>System prompt</span>
          <Textarea aria-label="System prompt" className="min-h-28" value={node.config?.system ?? ""} onChange={(event) => onChange({ config: { ...node.config, system: event.target.value } })} />
        </label>
      )}

      {config.supportsTransformMap && (
        <div className="rounded-md border p-2 text-xs">
          <div className="font-medium">Transform map</div>
          <p className="mt-1 text-muted-foreground">Set fixed state values in this inline step. Advanced key/value editing comes after the first structured pass.</p>
        </div>
      )}

      <Button type="button" variant="destructive" size="sm" onClick={onRemove}>
        Remove node
      </Button>
    </div>
  );
}
```

- [ ] **Step 4: Wire into WorkflowEditor**

Modify `packages/views/workflow/components/workflow-editor.tsx`:

```ts
import { NodeInspector } from "./node-inspector";
```

Replace the selected-node branch inside `selectionDetailPanel` with:

```tsx
{selectedNode && selectedNodeIndex >= 0 ? (
  <NodeInspector
    node={selectedNode}
    stateFields={definition.state.fields}
    agents={agents}
    onChange={(patch) => {
      const oldId = selectedNode.id;
      const nextId = patch.id ?? oldId;
      updateNode(selectedNodeIndex, patch);
      if (nextId !== oldId) {
        setDefinition((prev) => ({
          ...prev,
          routing: prev.routing.map((edge) => ({
            ...edge,
            from: edge.from === oldId ? nextId : edge.from,
            to: edge.to === oldId ? nextId : edge.to,
            else: edge.else === oldId ? nextId : edge.else,
          })),
        }));
        setSelection({ type: "node", id: nextId });
      }
    }}
    onRemove={removeSelectedNode}
  />
) : ...}
```

- [ ] **Step 5: Run tests**

Run:

```bash
PATH="$HOME/.nvm/versions/node/v22.22.2/bin:$PATH" pnpm --filter @multica/views test -- workflow/components/node-inspector.test.tsx workflow/components/workflow-canvas.test.tsx
```

Expected: PASS.

---

## Task 6: Build Edge Inspector With Condition Builder And Raw Mode

**Why:** Edges are how the workflow decides where to go next. A plain text input hides the rule and makes mistakes easy. The builder makes common rules obvious; raw mode keeps power.

**Files:**
- Create: `packages/views/workflow/components/edge-inspector.tsx`
- Test: `packages/views/workflow/components/edge-inspector.test.tsx`
- Modify: `packages/views/workflow/components/workflow-editor.tsx`

**Interfaces:**
- Consumes:
  - `conditionGroupToExpression`
  - `tryParseConditionExpression`
  - `FieldPicker`
- Produces:
  - `EdgeInspector`

- [ ] **Step 1: Write failing tests**

Create `packages/views/workflow/components/edge-inspector.test.tsx`:

```tsx
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
```

- [ ] **Step 2: Run red test**

Run:

```bash
PATH="$HOME/.nvm/versions/node/v22.22.2/bin:$PATH" pnpm --filter @multica/views test -- workflow/components/edge-inspector.test.tsx
```

Expected: FAIL because `edge-inspector.tsx` does not exist.

- [ ] **Step 3: Implement EdgeInspector**

Create `packages/views/workflow/components/edge-inspector.tsx`:

```tsx
import { useMemo, useState } from "react";
import type { WorkflowEdge, WorkflowStateField } from "@multica/core/workflow/types";
import { Button } from "@multica/ui/components/ui/button";
import { Input } from "@multica/ui/components/ui/input";
import { tryParseConditionExpression } from "../lib/condition-builder";

export function EdgeInspector({
  edge,
  stateFields,
  nodeIds,
  onChange,
  onRemove,
}: {
  edge: WorkflowEdge;
  stateFields: WorkflowStateField[];
  nodeIds: string[];
  onChange: (patch: Partial<WorkflowEdge>) => void;
  onRemove: () => void;
}) {
  const [mode, setMode] = useState<"builder" | "raw">(() => (edge.condition && !tryParseConditionExpression(edge.condition) ? "raw" : "builder"));
  const parsed = useMemo(() => (edge.condition ? tryParseConditionExpression(edge.condition) : null), [edge.condition]);

  return (
    <div className="space-y-3">
      <label className="space-y-1 text-xs text-muted-foreground">
        <span>From</span>
        <select className="h-9 w-full rounded-md border bg-background px-2 text-sm text-foreground" value={edge.from} onChange={(event) => onChange({ from: event.target.value })}>
          {nodeIds.map((id) => <option key={id} value={id}>{id}</option>)}
        </select>
      </label>

      <label className="space-y-1 text-xs text-muted-foreground">
        <span>To</span>
        <select className="h-9 w-full rounded-md border bg-background px-2 text-sm text-foreground" value={edge.to} onChange={(event) => onChange({ to: event.target.value })}>
          {nodeIds.map((id) => <option key={id} value={id}>{id}</option>)}
        </select>
      </label>

      <div className="flex gap-2">
        <Button type="button" size="xs" variant={mode === "builder" ? "default" : "outline"} onClick={() => setMode("builder")}>
          Builder
        </Button>
        <Button type="button" size="xs" variant={mode === "raw" ? "default" : "outline"} onClick={() => setMode("raw")}>
          Raw expression
        </Button>
      </div>

      {mode === "builder" ? (
        <div className="rounded-md border p-2 text-xs">
          <div className="font-medium">Condition builder</div>
          <p className="mt-1 text-muted-foreground">
            Builder supports common one-level rules. Use raw expression for nested logic.
          </p>
          <div className="mt-2 text-muted-foreground">
            {parsed ? edge.condition : "No builder-compatible condition yet."}
          </div>
          <select className="mt-2 h-9 w-full rounded-md border bg-background px-2 text-sm text-foreground" onChange={(event) => {
            const field = event.target.value;
            if (field) onChange({ condition: `${field} == ""` });
          }}>
            <option value="">Start with field</option>
            {stateFields.map((field) => <option key={field.name} value={field.name}>{field.name} · {field.type}</option>)}
          </select>
        </div>
      ) : (
        <label className="space-y-1 text-xs text-muted-foreground">
          <span>Condition expression</span>
          <Input aria-label="Condition expression" value={edge.condition ?? ""} onChange={(event) => onChange({ condition: event.target.value || undefined })} />
        </label>
      )}

      <label className="space-y-1 text-xs text-muted-foreground">
        <span>Else target</span>
        <select aria-label="Else target" className="h-9 w-full rounded-md border bg-background px-2 text-sm text-foreground" value={edge.else ?? ""} onChange={(event) => onChange({ else: event.target.value || undefined })}>
          <option value="">No else target</option>
          {nodeIds.map((id) => <option key={id} value={id}>{id}</option>)}
        </select>
      </label>

      <Button type="button" variant="destructive" size="sm" onClick={onRemove}>
        Remove edge
      </Button>
    </div>
  );
}
```

- [ ] **Step 4: Wire into WorkflowEditor**

Import:

```ts
import { EdgeInspector } from "./edge-inspector";
```

Replace selected-edge branch with:

```tsx
<EdgeInspector
  edge={selectedEdge}
  stateFields={definition.state.fields}
  nodeIds={definition.nodes.map((node) => node.id)}
  onChange={(patch) => updateEdge(selection.index, patch)}
  onRemove={removeSelectedEdge}
/>
```

- [ ] **Step 5: Run tests**

Run:

```bash
PATH="$HOME/.nvm/versions/node/v22.22.2/bin:$PATH" pnpm --filter @multica/views test -- workflow/components/edge-inspector.test.tsx workflow/lib/condition-builder.test.ts
```

Expected: PASS.

---

## Task 7: Add YAML Tab With Parse Safety

**Why:** YAML is still the escape hatch. The user should be able to edit raw workflow YAML without losing the last valid structured model.

**Files:**
- Create: `packages/views/workflow/components/workflow-yaml-editor.tsx`
- Test: `packages/views/workflow/components/workflow-yaml-editor.test.tsx`
- Modify: `packages/views/workflow/components/workflow-editor.tsx`

**Interfaces:**
- Produces:
  - `WorkflowYamlEditor`

- [ ] **Step 1: Write failing test**

Create `packages/views/workflow/components/workflow-yaml-editor.test.tsx`:

```tsx
import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { WorkflowYamlEditor } from "./workflow-yaml-editor";

describe("WorkflowYamlEditor", () => {
  it("emits parsed YAML and shows parse errors", () => {
    const onParsed = vi.fn();
    render(<WorkflowYamlEditor initialYaml={"meta:\n  name: test\nstate:\n  fields: []\nnodes: []\nrouting: []\n"} onParsed={onParsed} />);

    fireEvent.change(screen.getByLabelText("Workflow YAML"), { target: { value: "meta: [" } });
    fireEvent.click(screen.getByRole("button", { name: "Apply YAML" }));

    expect(screen.getByText(/YAML parse failed/)).toBeInTheDocument();
    expect(onParsed).not.toHaveBeenCalled();
  });
});
```

- [ ] **Step 2: Run red test**

Run:

```bash
PATH="$HOME/.nvm/versions/node/v22.22.2/bin:$PATH" pnpm --filter @multica/views test -- workflow/components/workflow-yaml-editor.test.tsx
```

Expected: FAIL because component does not exist.

- [ ] **Step 3: Implement component**

Create `packages/views/workflow/components/workflow-yaml-editor.tsx`:

```tsx
import { useState } from "react";
import type { WorkflowDefinition } from "@multica/core/workflow/types";
import { Button } from "@multica/ui/components/ui/button";
import { Textarea } from "@multica/ui/components/ui/textarea";
import { parseWorkflow } from "../lib/serialize";

export function WorkflowYamlEditor({
  initialYaml,
  onParsed,
}: {
  initialYaml: string;
  onParsed: (definition: WorkflowDefinition, yaml: string) => void;
}) {
  const [yaml, setYaml] = useState(initialYaml);
  const [error, setError] = useState<string | null>(null);

  return (
    <div className="flex h-full min-h-0 flex-col gap-2">
      <Textarea
        aria-label="Workflow YAML"
        className="min-h-[24rem] flex-1 font-mono text-xs"
        value={yaml}
        onChange={(event) => setYaml(event.target.value)}
      />
      {error && <div className="rounded-md border border-destructive/40 bg-destructive/5 px-3 py-2 text-sm text-destructive">{error}</div>}
      <Button type="button" size="sm" onClick={() => {
        try {
          const parsed = parseWorkflow(yaml);
          setError(null);
          onParsed(parsed, yaml);
        } catch (err) {
          setError(`YAML parse failed: ${err instanceof Error ? err.message : "unknown error"}`);
        }
      }}>
        Apply YAML
      </Button>
    </div>
  );
}
```

- [ ] **Step 4: Wire tab into WorkflowEditor**

Add `const [mode, setMode] = useState<"structured" | "yaml">("structured");`

Render two buttons in header:

```tsx
<Button type="button" size="xs" variant={mode === "structured" ? "default" : "outline"} onClick={() => setMode("structured")}>
  Structured
</Button>
<Button type="button" size="xs" variant={mode === "yaml" ? "default" : "outline"} onClick={() => setMode("yaml")}>
  YAML
</Button>
```

When `mode === "yaml"`, render:

```tsx
<WorkflowYamlEditor
  initialYaml={serializeWorkflow(definition)}
  onParsed={(next) => {
    setDefinition(next);
    setSelection(null);
    setMode("structured");
  }}
/>
```

- [ ] **Step 5: Run tests**

Run:

```bash
PATH="$HOME/.nvm/versions/node/v22.22.2/bin:$PATH" pnpm --filter @multica/views test -- workflow/components/workflow-yaml-editor.test.tsx
```

Expected: PASS.

---

## Task 8: Add Validation Panel

**Why:** Validation errors should not feel like random runtime failures. Users need to see what is wrong and where to fix it.

**Files:**
- Create: `packages/views/workflow/lib/validation.ts`
- Create: `packages/views/workflow/components/workflow-validation-panel.tsx`
- Test: `packages/views/workflow/lib/validation.test.ts`
- Modify: `packages/views/workflow/components/workflow-editor.tsx`

**Interfaces:**
- Produces:
  - `type WorkflowValidationIssue`
  - `function validateWorkflowDefinition(definition: WorkflowDefinition): WorkflowValidationIssue[]`
  - `WorkflowValidationPanel`

- [ ] **Step 1: Write failing validation tests**

Create `packages/views/workflow/lib/validation.test.ts`:

```ts
import { describe, expect, it } from "vitest";
import type { WorkflowDefinition } from "@multica/core/workflow/types";
import { validateWorkflowDefinition } from "./validation";

describe("workflow validation", () => {
  it("reports outputs that are not declared state fields", () => {
    const definition: WorkflowDefinition = {
      meta: { name: "test" },
      state: { fields: [{ name: "task", type: "string" }] },
      nodes: [{ id: "review", type: "llm", outputs: ["missing"] }],
      routing: [],
    };

    expect(validateWorkflowDefinition(definition)).toEqual([
      expect.objectContaining({
        severity: "error",
        path: "nodes.review.outputs",
        message: 'Output "missing" is not declared in state fields.',
      }),
    ]);
  });

  it("requires else for conditional routes", () => {
    const definition: WorkflowDefinition = {
      meta: { name: "test" },
      state: { fields: [{ name: "approved", type: "boolean" }] },
      nodes: [{ id: "review", type: "llm" }],
      routing: [{ from: "review", to: "END", condition: "approved == true" }],
    };

    expect(validateWorkflowDefinition(definition)).toEqual([
      expect.objectContaining({
        severity: "error",
        path: "routing.0.else",
      }),
    ]);
  });
});
```

- [ ] **Step 2: Run red test**

Run:

```bash
PATH="$HOME/.nvm/versions/node/v22.22.2/bin:$PATH" pnpm --filter @multica/views test -- workflow/lib/validation.test.ts
```

Expected: FAIL because file does not exist.

- [ ] **Step 3: Implement validation**

Create `packages/views/workflow/lib/validation.ts`:

```ts
import type { WorkflowDefinition } from "@multica/core/workflow/types";

export type WorkflowValidationIssue = {
  severity: "error" | "warning";
  path: string;
  message: string;
};

export function validateWorkflowDefinition(definition: WorkflowDefinition): WorkflowValidationIssue[] {
  const issues: WorkflowValidationIssue[] = [];
  const fieldNames = new Set(definition.state.fields.map((field) => field.name));
  const nodeIds = new Set(["START", "END", ...definition.nodes.map((node) => node.id)]);

  for (const node of definition.nodes) {
    for (const output of node.outputs ?? []) {
      if (!fieldNames.has(output)) {
        issues.push({
          severity: "error",
          path: `nodes.${node.id}.outputs`,
          message: `Output "${output}" is not declared in state fields.`,
        });
      }
    }
  }

  definition.routing.forEach((route, index) => {
    if (!nodeIds.has(route.from)) {
      issues.push({ severity: "error", path: `routing.${index}.from`, message: `Route source "${route.from}" does not exist.` });
    }
    if (!nodeIds.has(route.to)) {
      issues.push({ severity: "error", path: `routing.${index}.to`, message: `Route target "${route.to}" does not exist.` });
    }
    if (route.condition && !route.else) {
      issues.push({ severity: "error", path: `routing.${index}.else`, message: "Conditional routes need an else target." });
    }
  });

  return issues;
}
```

- [ ] **Step 4: Implement panel**

Create `packages/views/workflow/components/workflow-validation-panel.tsx`:

```tsx
import type { WorkflowValidationIssue } from "../lib/validation";

export function WorkflowValidationPanel({ issues }: { issues: WorkflowValidationIssue[] }) {
  if (issues.length === 0) {
    return <div className="rounded-md border border-green-200 bg-green-50 px-3 py-2 text-xs text-green-800">Workflow looks valid.</div>;
  }

  return (
    <div className="rounded-md border border-amber-300 bg-amber-50 px-3 py-2 text-xs text-amber-950">
      <div className="font-medium">{issues.length} workflow issue{issues.length === 1 ? "" : "s"}</div>
      <ul className="mt-1 list-disc space-y-1 pl-4">
        {issues.map((issue) => (
          <li key={`${issue.path}:${issue.message}`}>
            <span className="font-mono">{issue.path}</span>: {issue.message}
          </li>
        ))}
      </ul>
    </div>
  );
}
```

- [ ] **Step 5: Wire into editor**

In `WorkflowEditor`, replace local `validateWorkflow(definition)` with:

```ts
const validationIssues = validateWorkflowDefinition(definition);
const blockingValidation = validationIssues.find((issue) => issue.severity === "error");
```

Before save:

```ts
if (blockingValidation) {
  setError(blockingValidation.message);
  setSaving(false);
  return;
}
```

Render:

```tsx
<WorkflowValidationPanel issues={validationIssues} />
```

- [ ] **Step 6: Run tests**

Run:

```bash
PATH="$HOME/.nvm/versions/node/v22.22.2/bin:$PATH" pnpm --filter @multica/views test -- workflow/lib/validation.test.ts
PATH="$HOME/.nvm/versions/node/v22.22.2/bin:$PATH" pnpm --filter @multica/views typecheck
```

Expected: PASS.

---

## Task 9: Integrate Structured Workflow Editor End-To-End

**Why:** After the pieces exist, the root editor needs to feel like one coherent tool instead of separate panels.

**Files:**
- Modify: `packages/views/workflow/components/workflow-editor.tsx`
- Test: `packages/views/workflow/components/workflow-editor.test.tsx`
- Verify existing:
  - `packages/views/workflow/components/workflow-canvas.test.tsx`
  - `packages/views/workflow/lib/to-react-flow.test.ts`

**Interfaces:**
- Consumes:
  - `StateFieldsEditor`
  - `NodeInspector`
  - `EdgeInspector`
  - `WorkflowYamlEditor`
  - `WorkflowValidationPanel`

- [ ] **Step 1: Write integration test**

Create or extend `packages/views/workflow/components/workflow-editor.test.tsx`:

```tsx
import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { WorkflowEditor } from "./workflow-editor";

vi.mock("@multica/core/api", () => ({
  api: {
    upsertSkillFile: vi.fn().mockResolvedValue({}),
  },
}));

vi.mock("sonner", () => ({
  toast: { success: vi.fn(), error: vi.fn() },
}));

describe("WorkflowEditor structured editing", () => {
  it("adds a state field and uses it as a node output", () => {
    render(<WorkflowEditor skillId="skill-1" agents={[]} />);

    fireEvent.change(screen.getByLabelText("New field name"), { target: { value: "review_result" } });
    fireEvent.change(screen.getByLabelText("New field type"), { target: { value: "string" } });
    fireEvent.click(screen.getByRole("button", { name: "Add field" }));

    fireEvent.click(screen.getByRole("button", { name: "Add node" }));
    fireEvent.change(screen.getByLabelText("Outputs"), { target: { value: "review_result" } });

    expect(screen.getByDisplayValue("review_result")).toBeInTheDocument();
  });
});
```

- [ ] **Step 2: Run red test**

Run:

```bash
PATH="$HOME/.nvm/versions/node/v22.22.2/bin:$PATH" pnpm --filter @multica/views test -- workflow/components/workflow-editor.test.tsx
```

Expected: FAIL until root editor wires new components.

- [ ] **Step 3: Refactor root editor**

Modify `packages/views/workflow/components/workflow-editor.tsx`:

- Keep root-owned state:

```ts
const [definition, setDefinition] = useState<WorkflowDefinition>(...)
const [selection, setSelection] = useState<Selection>(null)
const [mode, setMode] = useState<"structured" | "yaml">("structured")
```

- Render top toolbar:
  - Add node
  - Add edge
  - Auto layout
  - Structured/YAML toggle
  - Save workflow

- Render side panel:
  - `StateFieldsEditor`
  - `WorkflowValidationPanel`
  - `NodeInspector` or `EdgeInspector`

- Render main panel:
  - `WorkflowCanvas` in structured mode
  - `WorkflowYamlEditor` in YAML mode

- [ ] **Step 4: Run integration test**

Run same command. Expected: PASS.

- [ ] **Step 5: Run full focused workflow tests**

Run:

```bash
PATH="$HOME/.nvm/versions/node/v22.22.2/bin:$PATH" pnpm --filter @multica/views test -- workflow/components/workflow-editor.test.tsx workflow/components/workflow-canvas.test.tsx workflow/lib/to-react-flow.test.ts workflow/lib/schema-registry.test.ts workflow/lib/editor-model.test.ts workflow/lib/condition-builder.test.ts workflow/lib/validation.test.ts
```

Expected: PASS.

---

## Task 10: Final Verification And Manual QA

**Why:** This is an editor. Passing unit tests is not enough; the main risk is losing data while switching between structured editing and YAML.

**Files:**
- No required code files unless verification finds bugs.

- [ ] **Step 1: Run typecheck**

Run:

```bash
PATH="$HOME/.nvm/versions/node/v22.22.2/bin:$PATH" pnpm --filter @multica/views typecheck
```

Expected: PASS.

- [ ] **Step 2: Run focused tests**

Run:

```bash
PATH="$HOME/.nvm/versions/node/v22.22.2/bin:$PATH" pnpm --filter @multica/views test -- workflow/components workflow/lib
```

Expected: PASS.

- [ ] **Step 3: Manual QA in local app**

Start/restart local app:

```bash
.multica/local-dev/bin/dev.sh
```

Open:

```text
http://localhost:13083
```

Manual checks:

- Create a skill workflow.
- Add state field `workflow_status`.
- Add `llm` node and confirm System prompt is visible.
- Change node type to `transform` and confirm System prompt disappears.
- Add output using field picker.
- Add an edge and condition using builder.
- Switch to YAML tab and confirm generated YAML includes state fields, node outputs, and edge condition.
- Edit YAML with an unknown `config.custom` field.
- Switch back to structured mode and save.
- Reopen the skill and confirm unknown config is not lost.
- Clear `nodes: []` and save.
- Confirm Skills list no longer shows workflow icon.

- [ ] **Step 4: Record known limitations**

Add a short note to the implementation PR/summary:

```md
Known V1 limits:
- Condition builder supports only one-level AND/OR groups.
- Advanced raw expression remains the path for nested conditions.
- Transform and merge config start with simple structured fields plus Advanced config.
- Runtime/backend validator remains final authority for workflow executability.
```

---

## Self-Review

**Spec coverage:** Covered structured node editing, outputs choices, conditional edge editor, raw YAML, raw condition, state fields, validation, and round-trip safety.

**Placeholder scan:** No `TBD`, `TODO`, or “write tests for above” placeholders. Each task has concrete files, commands, and expected outcomes.

**Type consistency:** Proposed helper names are stable across tasks:
- `getNodeTypeConfig`
- `createStateField`
- `addStateField`
- `findStateFieldReferences`
- `conditionGroupToExpression`
- `tryParseConditionExpression`
- `validateWorkflowDefinition`

**Scope risk:** Task 9 is the largest task. If implementation feels too large, split it into two implementation passes:
- Pass A: State fields + Node inspector integration.
- Pass B: Edge inspector + YAML tab + validation panel integration.
