# Plan C — 前端 GUI(workflow 编排器 + issue workflow 进度视图)

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 在 Simultica 前端实现两块 GUI:(1) skill 详情页里的 workflow 编排器(表单增删节点/配子 agent/条件边 + mermaid 只读预览,产出并存 `workflow.yaml`);(2) issue 详情页里的 workflow 进度视图(读 `workflow_run`、mermaid 高亮当前节点/各节点状态、链接子 issue、WS 实时刷新)。

**Architecture:** 复用前端既有栈:`packages/core/api/client.ts` 加 workflow 相关方法,`packages/core` 加 workflow 的 types + tanstack query options,`packages/views` 加编排器与进度视图组件,挂进现有 skill 详情页与 issue 详情页。mermaid 走已装的 `catalog:` 依赖。WS 实时用 `useWSEvent("workflow_run:updated", ...)`。

**Tech Stack:** Next.js/React + TypeScript,TanStack Query,`mermaid`,`yaml`(若前端需序列化 workflow;否则后端只存原文),既有 `useWS`/`useWSEvent`。

## Global Constraints

- 仓库根: `/Users/bytedance/agent_workspace/simultica`。前端 monorepo:`packages/core`(数据/api/types)、`packages/views`(组件)、`apps/web`(Next 应用)。
- API client: `packages/core/api/client.ts`,私有 `this.fetch<T>(path, init)`(自动加 JSON header、处理 204、非 ok 抛 `ApiError`)。新方法照搬现有 `getIssue`/`updateSkill` 风格。
- API 暴露: client 方法经 `packages/core/api/index.ts` 的 `api` 对象使用(组件里 `api.xxx()`)。
- Query options: 仿 `packages/core/issues/queries.ts`(`xxxOptions(wsId, ...)` 返回 `{queryKey, queryFn}`,组件用 `useQuery(...)`)。
- WS: `useWSEvent(event, handler)`(`packages/core/realtime/hooks.ts`);事件类型 union 在 `packages/core/types/events.ts` 的 `WSEventType`。
- mermaid 已在 `packages/views/package.json`(`"mermaid": "catalog:"`),但**尚无任何 import**(本计划首次接线)。
- skill 详情页: `packages/views/skills/components/skill-detail-page.tsx`(已有 `FileTree`/`FileViewer`、`api.updateSkill`、tanstack `useQuery(skillDetailOptions(...))`)。
- issue 详情页: `packages/views/issues/components/issue-detail.tsx`。
- 依赖后端(Plan A 完成): `GET /api/issues/{id}/workflow-run`、`POST /api/workflow-runs`、`PATCH /api/workflow-runs/{runId}`,WS 事件 `workflow_run:updated`(payload `{workflow_run: {...}}`),skill workflow 存为 `skill_file path=workflow.yaml` + `skill.config.has_workflow`。
- 前端测试: 组件测试用既有 vitest setup(`*.test.tsx`,见 `packages/views/issues/components/*.test.tsx`)。运行 `pnpm -C packages/views test <pattern>` 或仓库根 `pnpm test`(先 `grep -n "\"test\"" packages/views/package.json` 确认脚本)。
- MVP 不做拖拽画布(React Flow 留迭代)。子 issue 详情页**不**展示 workflow。
- 不改原生 Multica。每个 task 末尾 commit。

---

### Task 1: workflow 类型 + WSEventType 扩展

**Files:**
- Create: `packages/core/workflow/types.ts`
- Modify: `packages/core/types/events.ts`(WSEventType 加 `"workflow_run:updated"`)
- Modify: `packages/core/workflow/index.ts`(新建,re-export)

**Interfaces:**
- Produces:
  - `WorkflowNode { id: string; type: "llm"|"subissue"|"router"|"transform"|"code"|"http"; dispatch?: "subissue"|"inline"; inputs?: string[]; outputs?: string[]; config?: { agent?: string; system?: string; done_criteria?: string; [k:string]: unknown } }`
  - `WorkflowEdge { from: string; to: string; condition?: string; else?: string }`
  - `WorkflowStateField { name: string; type: string; required?: boolean; default?: unknown; description?: string }`
  - `WorkflowDefinition { meta: { name: string; version?: string; description?: string }; state: { fields: WorkflowStateField[] }; nodes: WorkflowNode[]; routing: WorkflowEdge[]; execution?: Record<string, unknown> }`
  - `WorkflowNodeRunState { status: "pending"|"running"|"done"|"failed"; sub_issue_id: string|null; started_at: string|null; ended_at: string|null; error: string|null }`
  - `WorkflowRun { id: string; root_issue_id: string; skill_id: string|null; status: "pending"|"running"|"done"|"failed"|"cancelled"; current_node: string; nodes_state: Record<string, WorkflowNodeRunState>; definition_snapshot: WorkflowDefinition; error: string|null; created_at: string; updated_at: string }`

- [ ] **Step 1: 写类型文件**

`packages/core/workflow/types.ts`:
```ts
export type WorkflowNodeType = "llm" | "subissue" | "router" | "transform" | "code" | "http";
export type WorkflowDispatch = "subissue" | "inline";

export interface WorkflowNode {
  id: string;
  type: WorkflowNodeType;
  dispatch?: WorkflowDispatch;
  inputs?: string[];
  outputs?: string[];
  config?: {
    agent?: string;
    system?: string;
    done_criteria?: string;
    [k: string]: unknown;
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
  state: { fields: WorkflowStateField[] };
  nodes: WorkflowNode[];
  routing: WorkflowEdge[];
  execution?: Record<string, unknown>;
}

export type WorkflowNodeStatus = "pending" | "running" | "done" | "failed";

export interface WorkflowNodeRunState {
  status: WorkflowNodeStatus;
  sub_issue_id: string | null;
  started_at: string | null;
  ended_at: string | null;
  error: string | null;
}

export type WorkflowRunStatus = "pending" | "running" | "done" | "failed" | "cancelled";

export interface WorkflowRun {
  id: string;
  root_issue_id: string;
  skill_id: string | null;
  status: WorkflowRunStatus;
  current_node: string;
  nodes_state: Record<string, WorkflowNodeRunState>;
  definition_snapshot: WorkflowDefinition;
  error: string | null;
  created_at: string;
  updated_at: string;
}
```

- [ ] **Step 2: re-export**

`packages/core/workflow/index.ts`:
```ts
export * from "./types";
```

- [ ] **Step 3: WSEventType 加事件**

`packages/core/types/events.ts`:在 `WSEventType` union 里(`"comment:created"` 附近)加一行:
```ts
  | "workflow_run:updated"
```

- [ ] **Step 4: 类型检查**

Run(仓库根):
```bash
pnpm -C packages/core typecheck || pnpm -C packages/core exec tsc --noEmit
```
Expected: 无类型错误(先 `grep -n "\"typecheck\"\|\"tsc\"" packages/core/package.json` 确认脚本名;无则用 `pnpm -C packages/core exec tsc --noEmit`)。

- [ ] **Step 5: Commit**

```bash
git add packages/core/workflow/types.ts packages/core/workflow/index.ts packages/core/types/events.ts
git commit -m "feat(core): workflow types + workflow_run:updated event"
```

---

### Task 2: API client — workflow 方法

**Files:**
- Modify: `packages/core/api/client.ts`
- Test: `packages/core/api/client.workflow.test.ts`

**Interfaces:**
- Consumes: 私有 `this.fetch<T>(path, init)`、`this.fetchRaw`。Plan A 端点。
- Produces(在 ApiClient 类上):
  - `async getIssueWorkflowRun(issueId: string): Promise<WorkflowRun | null>` — GET `/api/issues/{id}/workflow-run`;404 时返回 `null`(不抛)。
  - `async startWorkflowRun(data: { root_issue_id: string; skill_id?: string; definition_snapshot: WorkflowDefinition; current_node?: string }): Promise<WorkflowRun>` — POST `/api/workflow-runs`。字段名 `definition_snapshot` 必须与 Plan A 的 `CreateWorkflowRun` body 完全一致。
  - 经 `api` 对象暴露(`packages/core/api/index.ts` 若是 `export const api = new ApiClient(...)` 则自动可用;确认后无需改)。

- [ ] **Step 1: 写失败测试**

`packages/core/api/client.workflow.test.ts`:
```ts
import { describe, it, expect, vi, beforeEach } from "vitest";
import { ApiClient } from "./client";

function makeClient(fetchImpl: typeof fetch) {
  vi.stubGlobal("fetch", fetchImpl);
  return new ApiClient({ baseUrl: "http://x", getToken: () => "t" } as any);
}

describe("workflow api", () => {
  beforeEach(() => vi.unstubAllGlobals());

  it("getIssueWorkflowRun returns run on 200", async () => {
    const run = { id: "r1", root_issue_id: "i1", status: "running", current_node: "a", nodes_state: {}, definition_snapshot: {}, skill_id: null, error: null, created_at: "", updated_at: "" };
    const client = makeClient(vi.fn(async () => new Response(JSON.stringify(run), { status: 200 })) as any);
    const got = await client.getIssueWorkflowRun("i1");
    expect(got?.id).toBe("r1");
  });

  it("getIssueWorkflowRun returns null on 404", async () => {
    const client = makeClient(vi.fn(async () => new Response(JSON.stringify({ error: "none" }), { status: 404 })) as any);
    const got = await client.getIssueWorkflowRun("i1");
    expect(got).toBeNull();
  });

  it("startWorkflowRun posts definition", async () => {
    const calls: any[] = [];
    const client = makeClient(vi.fn(async (url: string, init: any) => {
      calls.push({ url, init });
      return new Response(JSON.stringify({ id: "r2", status: "running" }), { status: 201 });
    }) as any);
    const res = await client.startWorkflowRun({ root_issue_id: "i1", definition_snapshot: { meta: { name: "w" }, state: { fields: [] }, nodes: [], routing: [] } });
    expect(res.id).toBe("r2");
    expect(calls[0].url).toContain("/api/workflow-runs");
    expect(calls[0].init.method).toBe("POST");
    expect(JSON.parse(calls[0].init.body).definition_snapshot).toBeDefined();
  });
});
```

Note: `ApiClient` 构造签名以现有为准(`grep -n "constructor" packages/core/api/client.ts`),对齐 `makeClient`。

- [ ] **Step 2: 运行确认失败**

Run: `pnpm -C packages/core test client.workflow`
Expected: 方法未定义 / 失败。

- [ ] **Step 3: 实现 client 方法**

在 `packages/core/api/client.ts` 顶部 import:
```ts
import type { WorkflowRun, WorkflowDefinition } from "../workflow/types";
```
在 ApiClient 类内(issue 方法附近)加:
```ts
  async getIssueWorkflowRun(issueId: string): Promise<WorkflowRun | null> {
    try {
      return await this.fetch<WorkflowRun>(`/api/issues/${issueId}/workflow-run`);
    } catch (err) {
      if (err instanceof ApiError && err.status === 404) return null;
      throw err;
    }
  }

  async startWorkflowRun(data: {
    root_issue_id: string;
    skill_id?: string;
    definition_snapshot: WorkflowDefinition;
    current_node?: string;
  }): Promise<WorkflowRun> {
    return this.fetch<WorkflowRun>(`/api/workflow-runs`, {
      method: "POST",
      body: JSON.stringify(data),
    });
  }
```

Note: `ApiError` 已在该文件定义/导入(测试里非 ok 抛它)。确认其 `status` 字段名。

- [ ] **Step 4: 运行确认通过**

Run: `pnpm -C packages/core test client.workflow`
Expected: 3 passed。

- [ ] **Step 5: Commit**

```bash
git add packages/core/api/client.ts packages/core/api/client.workflow.test.ts
git commit -m "feat(core): api client workflow methods"
```

---

### Task 3: workflow query options + mermaid 渲染工具

**Files:**
- Create: `packages/core/workflow/queries.ts`
- Create: `packages/views/workflow/lib/to-mermaid.ts`
- Test: `packages/views/workflow/lib/to-mermaid.test.ts`

**Interfaces:**
- Produces:
  - `workflowRunKeys` + `issueWorkflowRunOptions(wsId: string, issueId: string)` → tanstack options(`queryFn: () => api.getIssueWorkflowRun(issueId)`)。
  - `workflowToMermaid(def: WorkflowDefinition, runState?: Record<string, WorkflowNodeRunState>, currentNode?: string): string` — 生成 mermaid `flowchart TD`,节点按 runState 上色(running=黄、done=绿、failed=红、pending=灰),边带 condition label。

- [ ] **Step 1: 写 mermaid 工具失败测试**

`packages/views/workflow/lib/to-mermaid.test.ts`:
```ts
import { describe, it, expect } from "vitest";
import { workflowToMermaid } from "./to-mermaid";
import type { WorkflowDefinition } from "@multica/core/workflow/types";

const def: WorkflowDefinition = {
  meta: { name: "w" },
  state: { fields: [] },
  nodes: [
    { id: "a", type: "llm" },
    { id: "b", type: "router" },
  ],
  routing: [
    { from: "START", to: "a" },
    { from: "a", to: "b" },
    { from: "b", to: "END", condition: "ok == true" },
  ],
};

describe("workflowToMermaid", () => {
  it("renders flowchart with nodes and edges", () => {
    const m = workflowToMermaid(def);
    expect(m).toContain("flowchart TD");
    expect(m).toContain("a");
    expect(m).toContain("b");
    expect(m).toContain("-->");
  });

  it("labels conditional edges", () => {
    const m = workflowToMermaid(def);
    expect(m).toMatch(/ok == true/);
  });

  it("applies status classes from runState", () => {
    const m = workflowToMermaid(def, { a: { status: "done", sub_issue_id: null, started_at: null, ended_at: null, error: null } }, "b");
    expect(m).toContain("classDef done");
    expect(m).toMatch(/class a done/);
  });
});
```

- [ ] **Step 2: 运行确认失败**

Run: `pnpm -C packages/views test to-mermaid`
Expected: 模块不存在。

- [ ] **Step 3: 实现 mermaid 工具**

`packages/views/workflow/lib/to-mermaid.ts`:
```ts
import type { WorkflowDefinition, WorkflowNodeRunState } from "@multica/core/workflow/types";

function safeId(id: string): string {
  return id.replace(/[^a-zA-Z0-9_]/g, "_");
}

export function workflowToMermaid(
  def: WorkflowDefinition,
  runState?: Record<string, WorkflowNodeRunState>,
  currentNode?: string,
): string {
  const lines: string[] = ["flowchart TD"];

  // Nodes
  for (const node of def.nodes) {
    const nid = safeId(node.id);
    const label = `${node.id}\\n(${node.type})`;
    lines.push(`  ${nid}["${label}"]`);
  }

  // Edges
  for (const edge of def.routing) {
    const from = edge.from === "START" ? "START" : safeId(edge.from);
    const to = edge.to === "END" ? "END" : safeId(edge.to);
    if (edge.condition) {
      lines.push(`  ${from} -->|${edge.condition}| ${to}`);
      if (edge.else) {
        const elseTo = edge.else === "END" ? "END" : safeId(edge.else);
        lines.push(`  ${from} -->|else| ${elseTo}`);
      }
    } else {
      lines.push(`  ${from} --> ${to}`);
    }
  }

  // Status classes
  lines.push("  classDef running fill:#fde68a,stroke:#d97706;");
  lines.push("  classDef done fill:#bbf7d0,stroke:#16a34a;");
  lines.push("  classDef failed fill:#fecaca,stroke:#dc2626;");
  lines.push("  classDef pending fill:#e5e7eb,stroke:#9ca3af;");
  lines.push("  classDef current stroke-width:3px;");

  if (runState) {
    for (const node of def.nodes) {
      const st = runState[node.id]?.status;
      if (st) lines.push(`  class ${safeId(node.id)} ${st};`);
    }
  }
  if (currentNode && currentNode !== "START" && currentNode !== "END") {
    lines.push(`  class ${safeId(currentNode)} current;`);
  }

  return lines.join("\n");
}
```

- [ ] **Step 4: 运行确认通过**

Run: `pnpm -C packages/views test to-mermaid`
Expected: 3 passed。

- [ ] **Step 5: 写 query options**

`packages/core/workflow/queries.ts`:
```ts
import { api } from "../api";
import type { WorkflowRun } from "./types";

export const workflowRunKeys = {
  all: ["workflow-run"] as const,
  byIssue: (wsId: string, issueId: string) => ["workflow-run", wsId, issueId] as const,
};

export function issueWorkflowRunOptions(wsId: string, issueId: string) {
  return {
    queryKey: workflowRunKeys.byIssue(wsId, issueId),
    queryFn: (): Promise<WorkflowRun | null> => api.getIssueWorkflowRun(issueId),
  };
}
```

Note: `api` 的导入路径以 `packages/core/api/index.ts` 实际导出为准(`grep -n "export" packages/core/api/index.ts`)。

- [ ] **Step 6: 类型检查 + Commit**

Run: `pnpm -C packages/core exec tsc --noEmit && pnpm -C packages/views exec tsc --noEmit`
Expected: 无错误。
```bash
git add packages/core/workflow/queries.ts packages/views/workflow/lib/to-mermaid.ts packages/views/workflow/lib/to-mermaid.test.ts
git commit -m "feat(workflow): query options + mermaid renderer"
```

---

### Task 4: mermaid 预览组件(只读)

**Files:**
- Create: `packages/views/workflow/components/mermaid-graph.tsx`
- Test: `packages/views/workflow/components/mermaid-graph.test.tsx`

**Interfaces:**
- Consumes: `mermaid` 包、`workflowToMermaid`(Task 3)。
- Produces: `<MermaidGraph chart={string} />` — 客户端组件,`useEffect` 里调用 `mermaid.render` 把 chart 渲染成 SVG 注入容器。渲染失败显示错误文本而非崩溃。`"use client"`。

- [ ] **Step 1: 写测试(渲染不崩 + 调用 mermaid.render)**

`packages/views/workflow/components/mermaid-graph.test.tsx`:
```tsx
import { describe, it, expect, vi } from "vitest";
import { render, waitFor } from "@testing-library/react";

vi.mock("mermaid", () => ({
  default: {
    initialize: vi.fn(),
    render: vi.fn(async (_id: string, _chart: string) => ({ svg: "<svg data-testid='m'></svg>" })),
  },
}));

import { MermaidGraph } from "./mermaid-graph";

describe("MermaidGraph", () => {
  it("renders svg from mermaid.render", async () => {
    const { container } = render(<MermaidGraph chart={"flowchart TD\n A --> B"} />);
    await waitFor(() => {
      expect(container.querySelector("svg")).toBeTruthy();
    });
  });

  it("shows error text when render throws", async () => {
    const mermaid = (await import("mermaid")).default as any;
    mermaid.render.mockRejectedValueOnce(new Error("bad chart"));
    const { findByText } = render(<MermaidGraph chart={"bad"} />);
    expect(await findByText(/bad chart|渲染失败/)).toBeTruthy();
  });
});
```

Note: 测试用的 `@testing-library/react` 是否已配置见 `packages/views` 既有 `*.test.tsx`;若用别的渲染工具,对齐既有惯例。

- [ ] **Step 2: 运行确认失败**

Run: `pnpm -C packages/views test mermaid-graph`
Expected: 模块不存在。

- [ ] **Step 3: 实现组件**

`packages/views/workflow/components/mermaid-graph.tsx`:
```tsx
"use client";

import { useEffect, useRef, useState } from "react";
import mermaid from "mermaid";

let initialized = false;
function ensureInit() {
  if (!initialized) {
    mermaid.initialize({ startOnLoad: false, securityLevel: "strict", theme: "neutral" });
    initialized = true;
  }
}

let renderSeq = 0;

export function MermaidGraph({ chart }: { chart: string }) {
  const ref = useRef<HTMLDivElement>(null);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    let cancelled = false;
    ensureInit();
    const id = `wf-mermaid-${renderSeq++}`;
    mermaid
      .render(id, chart)
      .then(({ svg }) => {
        if (!cancelled && ref.current) {
          ref.current.innerHTML = svg;
          setError(null);
        }
      })
      .catch((e: unknown) => {
        if (!cancelled) setError(e instanceof Error ? e.message : "渲染失败");
      });
    return () => {
      cancelled = true;
    };
  }, [chart]);

  if (error) {
    return <div className="text-sm text-destructive">图渲染失败: {error}</div>;
  }
  return <div ref={ref} className="overflow-auto" />;
}
```

- [ ] **Step 4: 运行确认通过**

Run: `pnpm -C packages/views test mermaid-graph`
Expected: 2 passed。

- [ ] **Step 5: Commit**

```bash
git add packages/views/workflow/components/mermaid-graph.tsx packages/views/workflow/components/mermaid-graph.test.tsx
git commit -m "feat(workflow): mermaid graph preview component"
```

---

### Task 5: workflow 编排器(表单 + 预览),挂进 skill 详情页

**Files:**
- Create: `packages/views/workflow/components/workflow-editor.tsx`
- Create: `packages/views/workflow/lib/serialize.ts`(definition ↔ YAML 文本)
- Test: `packages/views/workflow/lib/serialize.test.ts`
- Modify: `packages/views/skills/components/skill-detail-page.tsx`(加一个 "Workflow" 区/标签页,挂 `<WorkflowEditor>`)

**Interfaces:**
- Consumes: `WorkflowDefinition` 类型、`MermaidGraph`、`workflowToMermaid`、`yaml` 包、`api.upsertSkillFile`(确认现有 client 方法名:`grep -n "upsertSkillFile\|SkillFile" packages/core/api/client.ts`)。
- Produces:
  - `serializeWorkflow(def): string`(YAML)、`parseWorkflow(text): WorkflowDefinition`。
  - `<WorkflowEditor skillId initialYaml agents onSaved />`:
    - 表单:节点列表(增删、改 id/type/dispatch、选 agent(下拉=传入 agents)、填 system、done_criteria),边列表(from/to/condition/else)。
    - 右侧 `<MermaidGraph chart={workflowToMermaid(def)} />` 实时预览。
    - "保存"按钮:`api.upsertSkillFile(skillId, { path: "workflow.yaml", content: serializeWorkflow(def) })` → `onSaved()`。

- [ ] **Step 1: 写 serialize 失败测试**

`packages/views/workflow/lib/serialize.test.ts`:
```ts
import { describe, it, expect } from "vitest";
import { serializeWorkflow, parseWorkflow } from "./serialize";
import type { WorkflowDefinition } from "@multica/core/workflow/types";

const def: WorkflowDefinition = {
  meta: { name: "demo" },
  state: { fields: [{ name: "task", type: "string", required: true }] },
  nodes: [{ id: "impl", type: "llm", outputs: ["execution"], config: { agent: "code", system: "do" } }],
  routing: [{ from: "START", to: "impl" }, { from: "impl", to: "END" }],
};

describe("serialize", () => {
  it("round-trips definition through yaml", () => {
    const text = serializeWorkflow(def);
    expect(text).toContain("name: demo");
    const back = parseWorkflow(text);
    expect(back.nodes[0].id).toBe("impl");
    expect(back.routing[1].to).toBe("END");
  });

  it("parse throws on invalid yaml", () => {
    expect(() => parseWorkflow(":\n  - bad: [unclosed")).toThrow();
  });
});
```

- [ ] **Step 2: 运行确认失败**

Run: `pnpm -C packages/views test serialize`
Expected: 模块不存在。

- [ ] **Step 3: 实现 serialize**

`packages/views/workflow/lib/serialize.ts`:
```ts
import YAML from "yaml";
import type { WorkflowDefinition } from "@multica/core/workflow/types";

export function serializeWorkflow(def: WorkflowDefinition): string {
  return YAML.stringify(def);
}

export function parseWorkflow(text: string): WorkflowDefinition {
  const parsed = YAML.parse(text);
  if (!parsed || typeof parsed !== "object") {
    throw new Error("invalid workflow yaml: not an object");
  }
  return parsed as WorkflowDefinition;
}
```

Note: `yaml` 包是否在 `packages/views` 依赖里 —— `grep -n "\"yaml\"" packages/views/package.json`;无则 `pnpm -C packages/views add yaml`(它在 agent-runtime 已用,前端需自己声明)。

- [ ] **Step 4: 运行确认通过**

Run: `pnpm -C packages/views test serialize`
Expected: 2 passed。

- [ ] **Step 5: 实现编排器组件**

`packages/views/workflow/components/workflow-editor.tsx`:
```tsx
"use client";

import { useMemo, useState } from "react";
import type { WorkflowDefinition, WorkflowNode, WorkflowEdge } from "@multica/core/workflow/types";
import { api } from "@multica/core/api";
import { MermaidGraph } from "./mermaid-graph";
import { workflowToMermaid } from "../lib/to-mermaid";
import { serializeWorkflow, parseWorkflow } from "../lib/serialize";

const EMPTY_DEF: WorkflowDefinition = {
  meta: { name: "workflow" },
  state: { fields: [{ name: "task", type: "string", required: true }] },
  nodes: [],
  routing: [{ from: "START", to: "END" }],
};

export function WorkflowEditor({
  skillId,
  initialYaml,
  agents,
  onSaved,
}: {
  skillId: string;
  initialYaml?: string;
  agents: { id: string; name: string }[];
  onSaved?: () => void;
}) {
  const [def, setDef] = useState<WorkflowDefinition>(() => {
    if (initialYaml) {
      try { return parseWorkflow(initialYaml); } catch { /* fall through */ }
    }
    return EMPTY_DEF;
  });
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const chart = useMemo(() => workflowToMermaid(def), [def]);

  function updateNode(idx: number, patch: Partial<WorkflowNode>) {
    setDef((d) => {
      const nodes = d.nodes.map((n, i) => (i === idx ? { ...n, ...patch, config: { ...n.config, ...(patch.config ?? {}) } } : n));
      return { ...d, nodes };
    });
  }
  function addNode() {
    setDef((d) => ({ ...d, nodes: [...d.nodes, { id: `node${d.nodes.length + 1}`, type: "llm", config: {} }] }));
  }
  function removeNode(idx: number) {
    setDef((d) => ({ ...d, nodes: d.nodes.filter((_, i) => i !== idx) }));
  }
  function updateEdge(idx: number, patch: Partial<WorkflowEdge>) {
    setDef((d) => ({ ...d, routing: d.routing.map((e, i) => (i === idx ? { ...e, ...patch } : e)) }));
  }
  function addEdge() {
    setDef((d) => ({ ...d, routing: [...d.routing, { from: "START", to: "END" }] }));
  }
  function removeEdge(idx: number) {
    setDef((d) => ({ ...d, routing: d.routing.filter((_, i) => i !== idx) }));
  }

  async function save() {
    setSaving(true);
    setError(null);
    try {
      await api.upsertSkillFile(skillId, { path: "workflow.yaml", content: serializeWorkflow(def) });
      onSaved?.();
    } catch (e) {
      setError(e instanceof Error ? e.message : "保存失败");
    } finally {
      setSaving(false);
    }
  }

  return (
    <div className="grid grid-cols-1 gap-4 md:grid-cols-2">
      <div className="space-y-4">
        <section>
          <div className="mb-2 flex items-center justify-between">
            <h4 className="font-medium">节点</h4>
            <button className="text-sm underline" onClick={addNode} type="button">+ 节点</button>
          </div>
          {def.nodes.map((node, i) => (
            <div key={i} className="mb-2 rounded border p-2 text-sm">
              <div className="flex gap-2">
                <input className="w-24 border px-1" value={node.id} onChange={(e) => updateNode(i, { id: e.target.value })} placeholder="id" />
                <select className="border px-1" value={node.type} onChange={(e) => updateNode(i, { type: e.target.value as WorkflowNode["type"] })}>
                  {["llm", "subissue", "router", "transform", "code", "http"].map((t) => <option key={t} value={t}>{t}</option>)}
                </select>
                <select className="border px-1" value={node.config?.agent ?? ""} onChange={(e) => updateNode(i, { config: { agent: e.target.value } })}>
                  <option value="">(无子 agent)</option>
                  {agents.map((a) => <option key={a.id} value={a.name}>{a.name}</option>)}
                </select>
                <button className="text-destructive" onClick={() => removeNode(i)} type="button">删除</button>
              </div>
              <textarea className="mt-1 w-full border px-1" rows={2} value={node.config?.system ?? ""} onChange={(e) => updateNode(i, { config: { system: e.target.value } })} placeholder="system / 约束" />
            </div>
          ))}
        </section>
        <section>
          <div className="mb-2 flex items-center justify-between">
            <h4 className="font-medium">边</h4>
            <button className="text-sm underline" onClick={addEdge} type="button">+ 边</button>
          </div>
          {def.routing.map((edge, i) => (
            <div key={i} className="mb-2 flex gap-2 text-sm">
              <input className="w-20 border px-1" value={edge.from} onChange={(e) => updateEdge(i, { from: e.target.value })} placeholder="from" />
              <input className="w-20 border px-1" value={edge.to} onChange={(e) => updateEdge(i, { to: e.target.value })} placeholder="to" />
              <input className="flex-1 border px-1" value={edge.condition ?? ""} onChange={(e) => updateEdge(i, { condition: e.target.value || undefined })} placeholder="condition (可选)" />
              <button className="text-destructive" onClick={() => removeEdge(i)} type="button">删除</button>
            </div>
          ))}
        </section>
        {error && <div className="text-sm text-destructive">{error}</div>}
        <button className="rounded bg-primary px-3 py-1 text-primary-foreground" disabled={saving} onClick={save} type="button">
          {saving ? "保存中…" : "保存 workflow"}
        </button>
      </div>
      <div className="rounded border p-2">
        <h4 className="mb-2 font-medium">预览</h4>
        <MermaidGraph chart={chart} />
      </div>
    </div>
  );
}
```

Note: 样式 class 用项目既有 Tailwind/ui 约定即可,上面是占位风格;关键是行为正确。`api.upsertSkillFile` 的入参形状以现有 client 为准(可能是 `(skillId, {path, content})` 或 `(skillId, path, content)`)——先 grep 对齐。

- [ ] **Step 6: 挂进 skill 详情页**

在 `packages/views/skills/components/skill-detail-page.tsx`:import `WorkflowEditor`,在文件区/标签页旁加一个 "Workflow" 区块:
```tsx
import { WorkflowEditor } from "../../workflow/components/workflow-editor";
// ...在渲染区合适位置(如 files 区下方):
{skill && (
  <section className="mt-4">
    <h3 className="mb-2 text-sm font-semibold">Workflow 编排</h3>
    <WorkflowEditor
      skillId={skill.id}
      initialYaml={(skill.files ?? []).find((f: { path: string; content?: string }) => f.path === "workflow.yaml")?.content}
      agents={agents.map((a: { id: string; name: string }) => ({ id: a.id, name: a.name }))}
      onSaved={() => qc.invalidateQueries({ queryKey: skillDetailOptions(wsId, skillId).queryKey })}
    />
  </section>
)}
```

Note: `agents`、`qc`、`wsId`、`skillId`、`skillDetailOptions`、`skill.files` 均已在该页存在(见前文 grep:`agents`(258 行)、`qc`(249)、`skillDetailOptions`(257)、`skill.files`(326))。若 `skill.files` 的元素无 `content`(列表接口省略),改为按需拉取单文件内容(`api.getSkillFile`?先 grep 确认);若无单文件读接口,MVP 用 `FileViewer` 已有的内容获取路径取 `workflow.yaml`。

- [ ] **Step 7: 类型检查 + 组件测试**

Run:
```bash
pnpm -C packages/views exec tsc --noEmit
pnpm -C packages/views test workflow
```
Expected: 无类型错误;serialize + to-mermaid + mermaid-graph 测试通过。

- [ ] **Step 8: Commit**

```bash
git add packages/views/workflow/components/workflow-editor.tsx packages/views/workflow/lib/serialize.ts packages/views/workflow/lib/serialize.test.ts packages/views/skills/components/skill-detail-page.tsx packages/views/package.json
git commit -m "feat(workflow): skill workflow editor with mermaid preview"
```

---

### Task 6: issue workflow 进度视图,挂进 issue 详情页 + WS 实时

**Files:**
- Create: `packages/views/workflow/components/issue-workflow-panel.tsx`
- Test: `packages/views/workflow/components/issue-workflow-panel.test.tsx`
- Modify: `packages/views/issues/components/issue-detail.tsx`(仅在"非子 issue"即无 parent 时渲染该面板)

**Interfaces:**
- Consumes: `issueWorkflowRunOptions`、`workflowToMermaid`、`MermaidGraph`、`useWSEvent`、`useQueryClient`、`workflowRunKeys`。
- Produces:
  - `<IssueWorkflowPanel wsId issueId />`:
    - `useQuery(issueWorkflowRunOptions(wsId, issueId))` 拿 run;为 `null` 时渲染"无 workflow 运行"占位(或不渲染)。
    - `useWSEvent("workflow_run:updated", ...)`:当 payload `workflow_run.root_issue_id === issueId` 时,把缓存更新为新 run(`qc.setQueryData(workflowRunKeys.byIssue(...), payload.workflow_run)`)。
    - 渲染 `<MermaidGraph chart={workflowToMermaid(run.definition_snapshot, run.nodes_state, run.current_node)} />` + 节点列表(状态徽标 + 链接到 `sub_issue_id`)。

- [ ] **Step 1: 写测试(有 run 时渲染节点状态;无 run 时占位)**

`packages/views/workflow/components/issue-workflow-panel.test.tsx`:
```tsx
import { describe, it, expect, vi } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";

vi.mock("mermaid", () => ({ default: { initialize: vi.fn(), render: vi.fn(async () => ({ svg: "<svg></svg>" })) } }));
vi.mock("@multica/core/realtime", () => ({ useWSEvent: vi.fn() }));

const run = {
  id: "r1", root_issue_id: "i1", skill_id: null, status: "running", current_node: "impl",
  nodes_state: { impl: { status: "running", sub_issue_id: "sub-1", started_at: null, ended_at: null, error: null } },
  definition_snapshot: { meta: { name: "w" }, state: { fields: [] }, nodes: [{ id: "impl", type: "llm" }], routing: [{ from: "START", to: "impl" }, { from: "impl", to: "END" }] },
  error: null, created_at: "", updated_at: "",
};

vi.mock("@multica/core/api", () => ({
  api: { getIssueWorkflowRun: vi.fn(async (id: string) => (id === "i1" ? run : null)) },
}));

import { IssueWorkflowPanel } from "./issue-workflow-panel";

function wrap(ui: React.ReactNode) {
  const qc = new QueryClient();
  return render(<QueryClientProvider client={qc}>{ui}</QueryClientProvider>);
}

describe("IssueWorkflowPanel", () => {
  it("renders node status when run exists", async () => {
    wrap(<IssueWorkflowPanel wsId="ws" issueId="i1" />);
    await waitFor(() => expect(screen.getByText(/impl/)).toBeTruthy());
    expect(screen.getByText(/running/i)).toBeTruthy();
  });

  it("renders placeholder when no run", async () => {
    wrap(<IssueWorkflowPanel wsId="ws" issueId="none" />);
    await waitFor(() => expect(screen.queryByText(/无.*workflow|no workflow/i)).toBeTruthy());
  });
});
```

Note: `useWSEvent` 的导入路径以实际为准(`packages/core/realtime` 还是 `@multica/core/realtime/hooks`);对齐 mock 路径。

- [ ] **Step 2: 运行确认失败**

Run: `pnpm -C packages/views test issue-workflow-panel`
Expected: 模块不存在。

- [ ] **Step 3: 实现面板**

`packages/views/workflow/components/issue-workflow-panel.tsx`:
```tsx
"use client";

import { useQuery, useQueryClient } from "@tanstack/react-query";
import { issueWorkflowRunOptions, workflowRunKeys } from "@multica/core/workflow/queries";
import type { WorkflowRun } from "@multica/core/workflow/types";
import { useWSEvent } from "@multica/core/realtime";
import { MermaidGraph } from "./mermaid-graph";
import { workflowToMermaid } from "../lib/to-mermaid";

export function IssueWorkflowPanel({ wsId, issueId }: { wsId: string; issueId: string }) {
  const qc = useQueryClient();
  const { data: run, isLoading } = useQuery(issueWorkflowRunOptions(wsId, issueId));

  useWSEvent("workflow_run:updated", (payload: { workflow_run?: WorkflowRun }) => {
    const incoming = payload?.workflow_run;
    if (incoming && incoming.root_issue_id === issueId) {
      qc.setQueryData(workflowRunKeys.byIssue(wsId, issueId), incoming);
    }
  });

  if (isLoading) return <div className="text-sm text-muted-foreground">加载 workflow…</div>;
  if (!run) return <div className="text-sm text-muted-foreground">无 workflow 运行</div>;

  const chart = workflowToMermaid(run.definition_snapshot, run.nodes_state, run.current_node);

  return (
    <section className="mt-4 rounded border p-3">
      <div className="mb-2 flex items-center justify-between">
        <h3 className="text-sm font-semibold">Workflow 进度</h3>
        <span className="text-xs">{run.status} · 当前: {run.current_node || "—"}</span>
      </div>
      <MermaidGraph chart={chart} />
      <ul className="mt-3 space-y-1 text-sm">
        {run.definition_snapshot.nodes.map((node) => {
          const st = run.nodes_state[node.id];
          return (
            <li key={node.id} className="flex items-center gap-2">
              <span className="font-mono">{node.id}</span>
              <StatusBadge status={st?.status ?? "pending"} />
              {st?.sub_issue_id && (
                <a className="text-xs underline" href={`/issue/${st.sub_issue_id}`}>子 issue</a>
              )}
              {st?.error && <span className="text-xs text-destructive">{st.error}</span>}
            </li>
          );
        })}
      </ul>
    </section>
  );
}

function StatusBadge({ status }: { status: string }) {
  const cls: Record<string, string> = {
    running: "bg-amber-100 text-amber-800",
    done: "bg-green-100 text-green-800",
    failed: "bg-red-100 text-red-800",
    pending: "bg-gray-100 text-gray-700",
  };
  return <span className={`rounded px-1.5 py-0.5 text-xs ${cls[status] ?? cls.pending}`}>{status}</span>;
}
```

Note: 子 issue 链接 `href` 用项目真实 issue 路由(`grep -rn "AppLink\|/issue/\|issues/" packages/views/issues/components/list-row.tsx` 对齐;优先用 `AppLink` + workspace 路径 helper,而非裸 href)。`useWSEvent` 导入路径对齐 Task 1 确认的实际位置。

- [ ] **Step 4: 运行确认通过**

Run: `pnpm -C packages/views test issue-workflow-panel`
Expected: 2 passed。

- [ ] **Step 5: 挂进 issue 详情页(仅非子 issue)**

`packages/views/issues/components/issue-detail.tsx`:import 面板,在详情主体合适位置加(仅当该 issue 无 parent 时渲染,满足"子 issue 不展示 workflow"):
```tsx
import { IssueWorkflowPanel } from "../../workflow/components/issue-workflow-panel";
// ...在渲染区(已有 issue 对象与 wsId):
{!issue.parent_issue_id && <IssueWorkflowPanel wsId={wsId} issueId={issue.id} />}
```

Note: `issue.parent_issue_id` 字段名以前端 `Issue` 类型为准(`grep -n "parent_issue_id\|parentIssueId" packages/core/types/issue.ts`);`wsId` 在该组件的取法对齐既有(`useWorkspaceId()` 或 props)。

- [ ] **Step 6: 类型检查 + Commit**

Run: `pnpm -C packages/views exec tsc --noEmit && pnpm -C packages/views test workflow`
Expected: 无类型错误;workflow 相关测试全过。
```bash
git add packages/views/workflow/components/issue-workflow-panel.tsx packages/views/workflow/components/issue-workflow-panel.test.tsx packages/views/issues/components/issue-detail.tsx
git commit -m "feat(workflow): issue workflow progress panel with realtime"
```

---

### Task 7: 前端构建冒烟 + 浏览器验证

**Files:** 无新增(验证步骤)。

- [ ] **Step 1: 全量类型检查 + 测试**

Run(仓库根):
```bash
pnpm -C packages/core exec tsc --noEmit
pnpm -C packages/views exec tsc --noEmit
pnpm -C packages/views test workflow
```
Expected: 全绿。

- [ ] **Step 2: 起前端(已由 launchctl 跑在 :13083)并在浏览器验证**

用 in-app Browser 打开 `http://localhost:13083`,登录后:
1. 进入某 skill 详情页 → 看到 "Workflow 编排" 区,能加节点/边,右侧 mermaid 预览实时更新,点"保存 workflow"成功(无报错)。
2. 进入一个主 issue(无 parent)详情页 → 若该 issue 有 workflow_run,看到 "Workflow 进度" 面板与节点状态;无则显示占位。

Note: 若前端 dev server 未自动热更新,`launchctl kickstart -k gui/$(id -u)/com.simultica.local-dev.frontend`。无法在浏览器完成时,明确说明"UI 未人工验证",不谎称通过。

- [ ] **Step 3: Commit(若有微调)**

```bash
git add -A
git commit -m "chore(workflow): frontend smoke fixes"
```

---

## Plan C 完成标准

- skill 详情页有 workflow 编排器:表单增删节点/边、选子 agent、mermaid 实时预览、保存为 `workflow.yaml`(触发后端 `has_workflow=true`)。
- 主 issue 详情页有 workflow 进度视图:mermaid 高亮当前节点/各节点状态、链接子 issue、`workflow_run:updated` WS 实时刷新;子 issue 页不展示。
- `pnpm -C packages/{core,views} exec tsc --noEmit` 通过,workflow 相关单测全过。
- 浏览器人工冒烟通过(或明确标注未验证项)。

依赖: 必须先完成 Plan A(读端点 + WS 事件 + skill 存储);进度视图要看到真实数据还需 Plan B 跑出 workflow_run。迭代: 编排器升级为 React Flow 拖拽画布。
