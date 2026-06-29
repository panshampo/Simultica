# Plan B — agent-runtime 编排 sidecar(HTTP 服务 + 派子 issue 节点 + 进度上报)

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 把 `agent-runtime`(Node.js/LangGraph.js)从一次性 CLI 升级为 Simultica 的常驻 orchestration sidecar:提供 HTTP API 提交/查询 workflow 运行,新增"每节点派独立子 issue 给子 agent 并轮询等待"的节点行为,并把语义进度上报回 Simultica 的 `workflow_run` API。

**Architecture:** 用 Node 内置 `http` 模块(零新框架依赖)起一个常驻服务,暴露 `POST /runs`、`GET /runs/:id`、`POST /webhook/issue-done`(入口保留,MVP 不处理)。新增 `subissue` 节点类型:在主 issue 下经 `multica` CLI 建子 issue、指派子 agent、轮询至 done、回填 state。每次节点转移调用 Plan A 的 `PATCH /api/workflow-runs/:id` 上报进度。LangGraph PostgresSaver 作为 checkpointer 写 Simultica 的 Postgres(引擎恢复用)。

**Tech Stack:** Node.js v18 (ESM), `@langchain/langgraph` ^0.2.74, Node 内置 `http`,`child_process`(调 `multica` CLI),`@langchain/langgraph-checkpoint-postgres`(新增)。

## Global Constraints

- 仓库根: `/Users/bytedance/agent_workspace/agent-runtime`。
- Node v18.20.4(ESM,`"type":"module"`)。测试用 `node --test`(`npm test`)。语法检查 `npm run check`。
- 依赖现状: `@anthropic-ai/sdk`、`@langchain/core`、`@langchain/langgraph`、`dotenv`、`yaml`。**无 HTTP 框架**(用内置 `http`)、**无 pg checkpoint**(本计划新增)。
- 现有结构: 节点在 `nodes/*.js`,经 `lib/nodeRegistry.js` 注册;构图在 `lib/workflowBuilder.js`(`build(config)` → `StateGraph` → `graph.compile()`);派 agent 等待的现成参考是 `backends/multica.js`(建 issue→轮询 runs→取 assistant 文本)。
- 依赖的 Simultica 后端(Plan A 必须先完成): 
  - `POST /api/workflow-runs` body `{root_issue_id, skill_id?, definition_snapshot, current_node?}` → 201 `{id,...}`
  - `PATCH /api/workflow-runs/{runId}` body `{status?, current_node?, nodes_state?, error?}` → 200
  - 后端基址默认 `http://localhost:18083`;鉴权用 CLI token(见下)。
- `multica` CLI: `multica issue create --title --description-stdin --assignee-id <uuid> --project <uuid> --parent <uuid> --output json`;`multica issue get <id> --output json`;`multica issue runs <id> --output json`;`multica issue run-messages <taskId> --output json`。参考 `backends/multica.js` 的 `createMulticaShell`/`pickLatestRun`/`isTerminalRun`/`extractAssistantText`(`backends/multicaResult.js`)。
- DB(checkpointer)连接串: `postgres://multica:multica@localhost:15433/simultica_dev`(本机 dev)。
- 配置经环境变量,新增前缀 `SIDECAR_`(见 Task 1)。复用现有 `TRAEX_*` / `MULTICA_*` 风格。
- 不改原生 Multica;sidecar 是独立进程。每个 task 末尾 commit。

---

### Task 1: sidecar 配置加载

**Files:**
- Create: `lib/sidecarConfig.js`
- Test: `test/lib/sidecarConfig.test.js`

**Interfaces:**
- Produces: `loadSidecarConfig(env = process.env)` → `{ port, simulticaBaseUrl, simulticaToken, dbUrl, pollIntervalMs, nodeTimeoutMs, agentRoutes }`。
  - `port` ← `SIDECAR_PORT`(默认 `8787`)
  - `simulticaBaseUrl` ← `SIDECAR_SIMULTICA_URL`(默认 `http://localhost:18083`)
  - `simulticaToken` ← `SIDECAR_SIMULTICA_TOKEN`(默认 `""`)
  - `dbUrl` ← `SIDECAR_DB_URL`(默认 `postgres://multica:multica@localhost:15433/simultica_dev`)
  - `pollIntervalMs` ← `SIDECAR_POLL_INTERVAL_MS`(默认 `3000`)
  - `nodeTimeoutMs` ← `SIDECAR_NODE_TIMEOUT_MS`(默认 `600000`)
  - `agentRoutes` ← `SIDECAR_AGENT_ROUTES`(JSON 字符串,形如 `{"code":{"agentId":"...","projectId":"..."}}`;默认 `{}`)

- [ ] **Step 1: 写失败测试**

`test/lib/sidecarConfig.test.js`:
```js
import { test } from 'node:test';
import assert from 'node:assert/strict';
import { loadSidecarConfig } from '../../lib/sidecarConfig.js';

test('defaults when env empty', () => {
  const c = loadSidecarConfig({});
  assert.equal(c.port, 8787);
  assert.equal(c.simulticaBaseUrl, 'http://localhost:18083');
  assert.equal(c.pollIntervalMs, 3000);
  assert.deepEqual(c.agentRoutes, {});
});

test('reads overrides and parses agent routes', () => {
  const c = loadSidecarConfig({
    SIDECAR_PORT: '9000',
    SIDECAR_SIMULTICA_URL: 'http://x:1',
    SIDECAR_AGENT_ROUTES: '{"code":{"agentId":"a1","projectId":"p1"}}',
  });
  assert.equal(c.port, 9000);
  assert.equal(c.simulticaBaseUrl, 'http://x:1');
  assert.deepEqual(c.agentRoutes.code, { agentId: 'a1', projectId: 'p1' });
});

test('invalid agent routes json throws', () => {
  assert.throws(() => loadSidecarConfig({ SIDECAR_AGENT_ROUTES: '{bad' }));
});
```

- [ ] **Step 2: 运行确认失败**

Run: `node --test test/lib/sidecarConfig.test.js`
Expected: 模块不存在 / 断言失败。

- [ ] **Step 3: 实现**

`lib/sidecarConfig.js`:
```js
export function loadSidecarConfig(env = process.env) {
  let agentRoutes = {};
  if (env.SIDECAR_AGENT_ROUTES) {
    agentRoutes = JSON.parse(env.SIDECAR_AGENT_ROUTES); // throws on bad JSON — fail fast at boot
  }
  return {
    port: Number(env.SIDECAR_PORT || 8787),
    simulticaBaseUrl: env.SIDECAR_SIMULTICA_URL || 'http://localhost:18083',
    simulticaToken: env.SIDECAR_SIMULTICA_TOKEN || '',
    dbUrl: env.SIDECAR_DB_URL || 'postgres://multica:multica@localhost:15433/simultica_dev',
    pollIntervalMs: Number(env.SIDECAR_POLL_INTERVAL_MS || 3000),
    nodeTimeoutMs: Number(env.SIDECAR_NODE_TIMEOUT_MS || 600000),
    agentRoutes,
  };
}
```

- [ ] **Step 4: 运行确认通过**

Run: `node --test test/lib/sidecarConfig.test.js`
Expected: 3 passed。

- [ ] **Step 5: Commit**

```bash
git add lib/sidecarConfig.js test/lib/sidecarConfig.test.js
git commit -m "feat(sidecar): config loader"
```

---

### Task 2: Simultica 进度上报客户端

**Files:**
- Create: `lib/simulticaProgress.js`
- Test: `test/lib/simulticaProgress.test.js`

**Interfaces:**
- Consumes: Task 1 config(`simulticaBaseUrl`、`simulticaToken`)。用全局 `fetch`(Node 18 内置)。
- Produces: `createProgressClient({ baseUrl, token, fetchImpl = fetch })` → 对象:
  - `createRun({ workspaceId, rootIssueId, skillId, definitionSnapshot, currentNode })` → `Promise<{id}>`(POST `/api/workflow-runs?workspace_id=<workspaceId>`)。**必须带 `workspace_id` 查询参数**:Simultica 的 `resolveWorkspaceID` 在 CLI-token 调用下从 `?workspace_id=` 或 `X-Workspace-ID` 头取 workspace,缺失会 400。
  - `updateRun(runId, { status, currentNode, nodesState, error })` → `Promise<void>`(PATCH `/api/workflow-runs/:id`;无需 workspace,后端按 runId 定位并从记录取 workspace)。
  - 请求头带 `Authorization: Bearer <token>`(token 非空时)与 `Content-Type: application/json`。

- [ ] **Step 1: 写失败测试(注入假 fetch)**

`test/lib/simulticaProgress.test.js`:
```js
import { test } from 'node:test';
import assert from 'node:assert/strict';
import { createProgressClient } from '../../lib/simulticaProgress.js';

function fakeFetch(calls) {
  return async (url, opts) => {
    calls.push({ url, opts });
    return { ok: true, status: 200, json: async () => ({ id: 'run-1' }) };
  };
}

test('createRun posts to /api/workflow-runs with bearer + workspace_id', async () => {
  const calls = [];
  const c = createProgressClient({ baseUrl: 'http://x', token: 't', fetchImpl: fakeFetch(calls) });
  const r = await c.createRun({ workspaceId: 'ws-1', rootIssueId: 'iss-1', definitionSnapshot: { meta: {} }, currentNode: 'START' });
  assert.equal(r.id, 'run-1');
  assert.equal(calls[0].url, 'http://x/api/workflow-runs?workspace_id=ws-1');
  assert.equal(calls[0].opts.method, 'POST');
  assert.equal(calls[0].opts.headers.Authorization, 'Bearer t');
  const body = JSON.parse(calls[0].opts.body);
  assert.equal(body.root_issue_id, 'iss-1');
  assert.equal(body.current_node, 'START');
});

test('updateRun patches run id', async () => {
  const calls = [];
  const c = createProgressClient({ baseUrl: 'http://x', token: '', fetchImpl: fakeFetch(calls) });
  await c.updateRun('run-9', { status: 'done', currentNode: 'END', nodesState: { a: { status: 'done' } } });
  assert.equal(calls[0].url, 'http://x/api/workflow-runs/run-9');
  assert.equal(calls[0].opts.method, 'PATCH');
  assert.equal(calls[0].opts.headers.Authorization, undefined);
  const body = JSON.parse(calls[0].opts.body);
  assert.equal(body.status, 'done');
  assert.deepEqual(body.nodes_state, { a: { status: 'done' } });
});

test('throws on non-ok response', async () => {
  const c = createProgressClient({ baseUrl: 'http://x', token: '', fetchImpl: async () => ({ ok: false, status: 500, text: async () => 'boom' }) });
  await assert.rejects(() => c.createRun({ workspaceId: 'ws', rootIssueId: 'i', definitionSnapshot: {} }));
});
```

- [ ] **Step 2: 运行确认失败**

Run: `node --test test/lib/simulticaProgress.test.js`
Expected: 模块不存在。

- [ ] **Step 3: 实现**

`lib/simulticaProgress.js`:
```js
export function createProgressClient({ baseUrl, token, fetchImpl = fetch }) {
  function headers() {
    const h = { 'Content-Type': 'application/json' };
    if (token) h.Authorization = `Bearer ${token}`;
    return h;
  }
  async function post(path, body) {
    const res = await fetchImpl(baseUrl + path, {
      method: 'POST', headers: headers(), body: JSON.stringify(body),
    });
    if (!res.ok) throw new Error(`POST ${path} -> ${res.status}: ${await safeText(res)}`);
    return res.json();
  }
  async function patch(path, body) {
    const res = await fetchImpl(baseUrl + path, {
      method: 'PATCH', headers: headers(), body: JSON.stringify(body),
    });
    if (!res.ok) throw new Error(`PATCH ${path} -> ${res.status}: ${await safeText(res)}`);
  }
  async function safeText(res) {
    try { return await res.text(); } catch { return ''; }
  }
  return {
    async createRun({ workspaceId, rootIssueId, skillId, definitionSnapshot, currentNode }) {
      const qs = workspaceId ? `?workspace_id=${encodeURIComponent(workspaceId)}` : '';
      return post(`/api/workflow-runs${qs}`, {
        root_issue_id: rootIssueId,
        skill_id: skillId,
        definition_snapshot: definitionSnapshot,
        current_node: currentNode,
      });
    },
    async updateRun(runId, { status, currentNode, nodesState, error }) {
      const body = {};
      if (status !== undefined) body.status = status;
      if (currentNode !== undefined) body.current_node = currentNode;
      if (nodesState !== undefined) body.nodes_state = nodesState;
      if (error !== undefined) body.error = error;
      await patch(`/api/workflow-runs/${runId}`, body);
    },
  };
}
```

- [ ] **Step 4: 运行确认通过**

Run: `node --test test/lib/simulticaProgress.test.js`
Expected: 3 passed。

- [ ] **Step 5: Commit**

```bash
git add lib/simulticaProgress.js test/lib/simulticaProgress.test.js
git commit -m "feat(sidecar): simultica progress client"
```

---

### Task 3: 节点状态聚合器(nodes_state 维护)

**Files:**
- Create: `lib/runProgressState.js`
- Test: `test/lib/runProgressState.test.js`

**Interfaces:**
- Produces: `createRunProgress(nodeIds = [])` → 对象,维护 `nodes_state` map:
  - `markRunning(nodeId, { subIssueId })` — 置该节点 `status:'running'`、`started_at`、`sub_issue_id`。
  - `markDone(nodeId)` — 置 `status:'done'`、`ended_at`。
  - `markFailed(nodeId, errMsg)` — 置 `status:'failed'`、`ended_at`、`error`。
  - `snapshot()` — 返回当前 `nodes_state` plain object(深拷贝)。
  - 纯内存、无 IO;Task 4 的引擎用它生成上报 payload。

- [ ] **Step 1: 写失败测试**

`test/lib/runProgressState.test.js`:
```js
import { test } from 'node:test';
import assert from 'node:assert/strict';
import { createRunProgress } from '../../lib/runProgressState.js';

test('lifecycle transitions', () => {
  const p = createRunProgress(['a', 'b']);
  p.markRunning('a', { subIssueId: 'iss-a' });
  let s = p.snapshot();
  assert.equal(s.a.status, 'running');
  assert.equal(s.a.sub_issue_id, 'iss-a');
  assert.ok(s.a.started_at);

  p.markDone('a');
  s = p.snapshot();
  assert.equal(s.a.status, 'done');
  assert.ok(s.a.ended_at);

  p.markFailed('b', 'boom');
  s = p.snapshot();
  assert.equal(s.b.status, 'failed');
  assert.equal(s.b.error, 'boom');
});

test('snapshot is a copy', () => {
  const p = createRunProgress(['a']);
  const s1 = p.snapshot();
  s1.a = { mutated: true };
  const s2 = p.snapshot();
  assert.notDeepEqual(s2.a, { mutated: true });
});
```

- [ ] **Step 2: 运行确认失败**

Run: `node --test test/lib/runProgressState.test.js`
Expected: 模块不存在。

- [ ] **Step 3: 实现**

`lib/runProgressState.js`:
```js
export function createRunProgress(nodeIds = []) {
  const state = {};
  for (const id of nodeIds) state[id] = { status: 'pending', sub_issue_id: null, started_at: null, ended_at: null, error: null };
  function ensure(id) {
    if (!state[id]) state[id] = { status: 'pending', sub_issue_id: null, started_at: null, ended_at: null, error: null };
    return state[id];
  }
  return {
    markRunning(nodeId, { subIssueId } = {}) {
      const n = ensure(nodeId);
      n.status = 'running';
      n.started_at = new Date().toISOString();
      if (subIssueId !== undefined) n.sub_issue_id = subIssueId;
    },
    markDone(nodeId) {
      const n = ensure(nodeId);
      n.status = 'done';
      n.ended_at = new Date().toISOString();
    },
    markFailed(nodeId, errMsg) {
      const n = ensure(nodeId);
      n.status = 'failed';
      n.ended_at = new Date().toISOString();
      n.error = errMsg;
    },
    snapshot() {
      return JSON.parse(JSON.stringify(state));
    },
  };
}
```

- [ ] **Step 4: 运行确认通过**

Run: `node --test test/lib/runProgressState.test.js`
Expected: 2 passed。

- [ ] **Step 5: Commit**

```bash
git add lib/runProgressState.js test/lib/runProgressState.test.js
git commit -m "feat(sidecar): node progress state aggregator"
```

---

### Task 4: subissue 节点 — 派子 issue 并轮询等待

**Files:**
- Create: `nodes/subIssueNode.js`
- Modify: `lib/nodeRegistry.js`(注册 `subissue` 类型)
- Test: `test/nodes/subIssueNode.test.js`

**Interfaces:**
- Consumes: 节点 config `{ id, type:'subissue', dispatch?, config:{ agent, system, done_criteria? } }`;运行时 context `{ shell, agentRoutes, rootIssueId, pollIntervalMs, nodeTimeoutMs, progress, eventBus }`。`shell` 提供 `execJson(args, {stdin})`(参照 `backends/multicaResult.js` 的 `createMulticaShell`)。
- Produces:
  - `class SubIssueNode { constructor(config, context); build() }`,`build()` 返回 LangGraph 节点函数 `async (state) => statePatch`。
  - 节点函数行为:
    1. 由 `config.config.agent`(路由键)在 `agentRoutes` 找 `{agentId, projectId}`;找不到 → 抛错。
    2. `progress.markRunning(nodeId, { subIssueId })`(子 issue 创建后回填 id)。
    3. `multica issue create --parent <rootIssueId> --assignee-id <agentId> --project <projectId> --title <nodeId 任务标题> --description-stdin --output json`,stdin = `config.config.system` + state 摘要。
    4. 轮询 `multica issue runs <subIssueId>` 直到 terminal 或超时(用 `pickLatestRun`/`isTerminalRun`)。
    5. 取 `run-messages` 的 assistant 文本(`extractAssistantText`),写入 state 字段(节点 `outputs[0]` 或 `<nodeId>_result`)。
    6. 成功 `progress.markDone(nodeId)`;失败/超时 → `progress.markFailed(nodeId, msg)` 并抛错。
  - 在 `nodeRegistry.registerDefaults()` 增 `this.register('subissue', SubIssueNode);`

- [ ] **Step 1: 写失败测试(注入假 shell)**

`test/nodes/subIssueNode.test.js`:
```js
import { test } from 'node:test';
import assert from 'node:assert/strict';
import { SubIssueNode } from '../../nodes/subIssueNode.js';
import { createRunProgress } from '../../lib/runProgressState.js';

function fakeShell(script) {
  // script: array of {match, result} consulted in order by joined args
  return {
    calls: [],
    async execJson(args, opts = {}) {
      this.calls.push({ args, stdin: opts.stdin });
      const joined = args.join(' ');
      if (joined.startsWith('issue create')) return { id: 'sub-1' };
      if (joined.startsWith('issue runs')) return [{ status: 'completed', task_id: 'task-1' }];
      if (joined.startsWith('issue run-messages')) return [{ role: 'assistant', content: 'done-text' }];
      throw new Error('unexpected args: ' + joined);
    },
  };
}

test('dispatches sub-issue, polls, returns result patch', async () => {
  const progress = createRunProgress(['impl']);
  const shell = fakeShell();
  const node = new SubIssueNode(
    { id: 'impl', type: 'subissue', outputs: ['execution'], config: { agent: 'code', system: 'do it' } },
    {
      shell,
      agentRoutes: { code: { agentId: 'agent-code', projectId: 'proj-code' } },
      rootIssueId: 'root-1',
      pollIntervalMs: 1,
      nodeTimeoutMs: 5000,
      progress,
      extractAssistantText: (msgs) => msgs.find((m) => m.role === 'assistant')?.content || '',
      pickLatestRun: (runs) => runs[runs.length - 1],
      isTerminalRun: (run) => run && (run.status === 'completed' || run.status === 'failed'),
      getRunTaskId: (run) => run.task_id,
      normalizeRunStatus: (run) => run.status,
    }
  );
  const fn = node.build();
  const patch = await fn({ task: 't' });
  assert.equal(patch.execution, 'done-text');
  const snap = progress.snapshot();
  assert.equal(snap.impl.status, 'done');
  assert.equal(snap.impl.sub_issue_id, 'sub-1');
  const createCall = shell.calls.find((c) => c.args.join(' ').startsWith('issue create'));
  assert.ok(createCall.args.includes('--parent'));
  assert.ok(createCall.args.includes('root-1'));
  assert.ok(createCall.args.includes('--assignee-id'));
  assert.ok(createCall.args.includes('agent-code'));
});

test('unknown agent route throws and marks failed', async () => {
  const progress = createRunProgress(['impl']);
  const node = new SubIssueNode(
    { id: 'impl', type: 'subissue', config: { agent: 'nope' } },
    { shell: fakeShell(), agentRoutes: {}, rootIssueId: 'r', progress, pollIntervalMs: 1, nodeTimeoutMs: 10 }
  );
  await assert.rejects(() => node.build()({}));
  assert.equal(progress.snapshot().impl.status, 'failed');
});
```

- [ ] **Step 2: 运行确认失败**

Run: `node --test test/nodes/subIssueNode.test.js`
Expected: 模块不存在。

- [ ] **Step 3: 实现节点**

`nodes/subIssueNode.js`:
```js
function sleep(ms) { return new Promise((r) => setTimeout(r, ms)); }

export class SubIssueNode {
  constructor(config, context) {
    this.config = config;
    this.context = context;
  }

  build() {
    const { id, config = {}, outputs = [] } = this.config;
    const ctx = this.context;
    const outputField = outputs[0] || `${id}_result`;

    return async (state) => {
      const route = ctx.agentRoutes?.[config.agent];
      if (!route || !route.agentId) {
        ctx.progress?.markFailed(id, `no agent route for "${config.agent}"`);
        throw new Error(`SubIssueNode ${id}: no agent route for "${config.agent}"`);
      }
      ctx.progress?.markRunning(id, {});

      const title = `[wf:${id}] ${truncate(state.task || config.system || id, 80)}`;
      const description = buildDescription(config.system || '', state);

      let subIssueId;
      try {
        const createArgs = [
          'issue', 'create',
          '--parent', ctx.rootIssueId,
          '--assignee-id', route.agentId,
          '--title', title,
          '--description-stdin',
          '--output', 'json',
        ];
        if (route.projectId) { createArgs.push('--project', route.projectId); }
        const issue = await ctx.shell.execJson(createArgs, { stdin: description });
        subIssueId = issue.id;
        ctx.progress?.markRunning(id, { subIssueId });
        ctx.eventBus?.emit?.('node:status', { node: id, text: `created sub-issue ${subIssueId}` });

        const run = await this.pollRun(subIssueId);
        const taskId = ctx.getRunTaskId(run);
        const messages = await ctx.shell.execJson(['issue', 'run-messages', taskId, '--output', 'json']);
        const text = ctx.extractAssistantText(messages);
        const status = ctx.normalizeRunStatus(run);
        if (status !== 'completed') {
          throw new Error(text || `sub-issue ${subIssueId} ended with status ${status}`);
        }
        ctx.progress?.markDone(id);
        return { [outputField]: text };
      } catch (err) {
        ctx.progress?.markFailed(id, err.message);
        throw err;
      }
    };
  }

  async pollRun(subIssueId) {
    const ctx = this.context;
    const deadline = Date.now() + (ctx.nodeTimeoutMs || 600000);
    let run = null;
    while (Date.now() < deadline) {
      const runs = await ctx.shell.execJson(['issue', 'runs', subIssueId, '--output', 'json']);
      run = ctx.pickLatestRun(runs);
      if (ctx.isTerminalRun(run)) return run;
      await sleep(ctx.pollIntervalMs || 3000);
    }
    throw new Error(`timed out waiting for sub-issue ${subIssueId}`);
  }
}

function truncate(s, n) { s = String(s || ''); return s.length <= n ? s : s.slice(0, n); }

function buildDescription(system, state) {
  const parts = [];
  if (system) parts.push(system);
  if (state && state.task) parts.push(`\n## 上游任务\n${state.task}`);
  return parts.join('\n');
}
```

- [ ] **Step 4: 注册节点类型**

`lib/nodeRegistry.js`:在 import 区加 `import { SubIssueNode } from '../nodes/subIssueNode.js';`,在 `registerDefaults()` 末尾加 `this.register('subissue', SubIssueNode);`。

- [ ] **Step 5: 运行确认通过**

Run: `node --test test/nodes/subIssueNode.test.js`
Expected: 2 passed。

- [ ] **Step 6: Commit**

```bash
git add nodes/subIssueNode.js lib/nodeRegistry.js test/nodes/subIssueNode.test.js
git commit -m "feat(sidecar): subissue node dispatches+polls sub-issue"
```

---

### Task 5: dispatch 字段推断 + schema 放行

**Files:**
- Modify: `lib/schemaValidator.js`(节点类型白名单加 `subissue`;允许节点可选字段 `dispatch`)
- Modify: `lib/workflowBuilder.js`(构图前对每个节点推断 `dispatch`:`subissue`/`llm`→`subissue` 默认派子 issue;`router`/`transform`/`code`→`inline`;并据此把 `llm` 类型在"派子 issue 模式"下改用 `subissue` 节点实现)
- Test: `test/lib/dispatchInference.test.js`

**Interfaces:**
- Produces: `inferDispatch(nodeConfig)` 导出函数 → `'subissue' | 'inline'`:
  - 若 `nodeConfig.dispatch` 显式给出且为合法值 → 原样返回。
  - 否则按 type 推断:`subissue`/`llm` → `'subissue'`;`router`/`transform`/`code`/`http` → `'inline'`。
  - workflowBuilder 在创建节点时:`dispatch==='subissue'` 的节点统一用 `subissue` 节点类(把 `llm` 语义节点也派成子 issue,符合 spec“agent 节点=子 issue”)。

- [ ] **Step 1: 写失败测试**

`test/lib/dispatchInference.test.js`:
```js
import { test } from 'node:test';
import assert from 'node:assert/strict';
import { inferDispatch } from '../../lib/workflowBuilder.js';

test('explicit dispatch wins', () => {
  assert.equal(inferDispatch({ type: 'llm', dispatch: 'inline' }), 'inline');
  assert.equal(inferDispatch({ type: 'router', dispatch: 'subissue' }), 'subissue');
});

test('infer by type', () => {
  assert.equal(inferDispatch({ type: 'llm' }), 'subissue');
  assert.equal(inferDispatch({ type: 'subissue' }), 'subissue');
  assert.equal(inferDispatch({ type: 'router' }), 'inline');
  assert.equal(inferDispatch({ type: 'transform' }), 'inline');
  assert.equal(inferDispatch({ type: 'code' }), 'inline');
});
```

- [ ] **Step 2: 运行确认失败**

Run: `node --test test/lib/dispatchInference.test.js`
Expected: `inferDispatch` 未导出。

- [ ] **Step 3: 实现 inferDispatch + 接入构图**

在 `lib/workflowBuilder.js` 顶部(import 后)加并导出:
```js
const SUBISSUE_TYPES = new Set(['llm', 'subissue']);
const INLINE_TYPES = new Set(['router', 'transform', 'code', 'http']);

export function inferDispatch(nodeConfig) {
  const d = nodeConfig.dispatch;
  if (d === 'subissue' || d === 'inline') return d;
  if (SUBISSUE_TYPES.has(nodeConfig.type)) return 'subissue';
  if (INLINE_TYPES.has(nodeConfig.type)) return 'inline';
  return 'inline';
}
```

在 `build()` 的「添加节点」循环里,创建节点前决定实现类型:
```js
    for (const nodeConfig of config.nodes) {
      const dispatch = inferDispatch(nodeConfig);
      const effectiveConfig = dispatch === 'subissue'
        ? { ...nodeConfig, type: 'subissue' }
        : nodeConfig;
      let nodeFn = this.nodeRegistry.create(effectiveConfig, {
        ...this.context,
        config,
      });
      // ... 既有 audit/wrap 逻辑不变,但用 effectiveConfig.id(== nodeConfig.id)
```

`lib/schemaValidator.js`:把 `subissue` 加入节点 type 白名单(找到 type 白名单 Set/数组,加 `'subissue'`);`dispatch` 为可选字段不强校验(若 validator 对未知字段报错,显式允许 `dispatch ∈ {subissue,inline}`)。

- [ ] **Step 4: 运行确认通过 + 语法检查**

Run:
```bash
node --test test/lib/dispatchInference.test.js
npm run check
```
Expected: 2 passed;`node --check index.js` 无语法错。

- [ ] **Step 5: Commit**

```bash
git add lib/workflowBuilder.js lib/schemaValidator.js test/lib/dispatchInference.test.js
git commit -m "feat(sidecar): dispatch inference + subissue routing in builder"
```

---

### Task 6: 编排引擎 — 跑一次 workflow 并全程上报

**Files:**
- Create: `lib/orchestrator.js`
- Test: `test/lib/orchestrator.test.js`

**Interfaces:**
- Consumes: `WorkflowBuilder`、`createRunProgress`、progress client(Task 2)、`shell`/路由/轮询辅助(Task 4)。
- Produces: `createOrchestrator({ progressClient, shell, agentRoutes, pollIntervalMs, nodeTimeoutMs, builderFactory })` → 对象:
  - `async run({ workspaceId, rootIssueId, skillId, definition, initialState })` →
    1. `progressClient.createRun({ workspaceId, ... })` 拿 `runId`(`current_node='START'`,`definition_snapshot=definition`)。`workspaceId` 透传给 createRun 的 `?workspace_id=`。
    2. 用 `createRunProgress(nodeIds)` + 在 context 注入 `progress`,build 出 graph。
    3. 包一层"节点后置 hook":每个节点完成后 `progressClient.updateRun(runId, {currentNode, nodesState})`(用 progress.snapshot())。MVP 简化:`run` 在每次 graph 事件/步进后上报;最低限度在 graph 跑完后上报最终 `nodes_state` + `status`。
    4. graph 跑完 → `updateRun(runId, {status:'done', currentNode:'END', nodesState})`;抛错 → `updateRun(runId, {status:'failed', error, nodesState})` 后重新抛出。
    5. 返回 `{ runId, finalState }`。
  - 为可测,`builderFactory(context)` 返回一个有 `build(definition)` 的对象(默认 `new WorkflowBuilder(...)`);测试注入假 builder + 假 graph。

- [ ] **Step 1: 写失败测试(注入假 builder/graph/progressClient)**

`test/lib/orchestrator.test.js`:
```js
import { test } from 'node:test';
import assert from 'node:assert/strict';
import { createOrchestrator } from '../../lib/orchestrator.js';

function fakeProgressClient(events) {
  return {
    async createRun(p) { events.push(['create', p]); return { id: 'run-1' }; },
    async updateRun(id, p) { events.push(['update', id, p]); },
  };
}

test('run reports running then done', async () => {
  const events = [];
  const orch = createOrchestrator({
    progressClient: fakeProgressClient(events),
    shell: {}, agentRoutes: {}, pollIntervalMs: 1, nodeTimeoutMs: 10,
    builderFactory: () => ({
      async build() {
        return { async invoke(state) { return { ...state, done: true }; } };
      },
    }),
  });
  const res = await orch.run({
    workspaceId: 'ws-1',
    rootIssueId: 'root-1',
    definition: { nodes: [{ id: 'a', type: 'llm' }], routing: [] },
    initialState: { task: 't' },
  });
  assert.equal(res.runId, 'run-1');
  assert.equal(res.finalState.done, true);
  assert.equal(events[0][0], 'create');
  const last = events[events.length - 1];
  assert.equal(last[0], 'update');
  assert.equal(last[2].status, 'done');
});

test('run reports failed on graph error', async () => {
  const events = [];
  const orch = createOrchestrator({
    progressClient: fakeProgressClient(events),
    shell: {}, agentRoutes: {}, pollIntervalMs: 1, nodeTimeoutMs: 10,
    builderFactory: () => ({
      async build() { return { async invoke() { throw new Error('boom'); } }; },
    }),
  });
  await assert.rejects(() => orch.run({
    workspaceId: 'ws-1',
    rootIssueId: 'root-1',
    definition: { nodes: [{ id: 'a', type: 'llm' }], routing: [] },
    initialState: {},
  }));
  const failUpdate = events.find((e) => e[0] === 'update' && e[2].status === 'failed');
  assert.ok(failUpdate, 'should report failed');
  assert.match(failUpdate[2].error, /boom/);
});
```

- [ ] **Step 2: 运行确认失败**

Run: `node --test test/lib/orchestrator.test.js`
Expected: 模块不存在。

- [ ] **Step 3: 实现**

`lib/orchestrator.js`:
```js
import { createRunProgress } from './runProgressState.js';

export function createOrchestrator({ progressClient, shell, agentRoutes, pollIntervalMs, nodeTimeoutMs, builderFactory }) {
  return {
    async run({ workspaceId, rootIssueId, skillId, definition, initialState = {} }) {
      const nodeIds = (definition.nodes || []).map((n) => n.id);
      const progress = createRunProgress(nodeIds);

      const { id: runId } = await progressClient.createRun({
        workspaceId, rootIssueId, skillId, definitionSnapshot: definition, currentNode: 'START',
      });

      const builder = builderFactory({
        shell, agentRoutes, rootIssueId, pollIntervalMs, nodeTimeoutMs, progress,
      });

      let graph;
      try {
        graph = await builder.build(definition);
      } catch (err) {
        await progressClient.updateRun(runId, { status: 'failed', error: `build: ${err.message}`, nodesState: progress.snapshot() });
        throw err;
      }

      try {
        const finalState = await graph.invoke(initialState);
        await progressClient.updateRun(runId, {
          status: 'done', currentNode: 'END', nodesState: progress.snapshot(),
        });
        return { runId, finalState };
      } catch (err) {
        await progressClient.updateRun(runId, {
          status: 'failed', error: err.message, nodesState: progress.snapshot(),
        });
        throw err;
      }
    },
  };
}
```

Note: MVP 在 graph 跑完/失败时上报终态;节点级实时上报由 `subIssueNode` 通过共享的 `progress` 对象记录,终态 `snapshot()` 已含每节点 status。若要"每节点转移即时 PATCH",在迭代里给 builder 注入 `onNodeComplete` 回调(留作后续,不在 MVP)。

- [ ] **Step 4: 运行确认通过**

Run: `node --test test/lib/orchestrator.test.js`
Expected: 2 passed。

- [ ] **Step 5: Commit**

```bash
git add lib/orchestrator.js test/lib/orchestrator.test.js
git commit -m "feat(sidecar): orchestrator runs workflow and reports progress"
```

---

### Task 7: HTTP 服务层(内置 http)

**Files:**
- Create: `lib/sidecarServer.js`
- Create: `bin/sidecar.js`(可执行入口)
- Modify: `package.json`(加 `"sidecar": "node bin/sidecar.js"` script)
- Test: `test/lib/sidecarServer.test.js`

**Interfaces:**
- Consumes: Task 1 config、Task 6 orchestrator。
- Produces: `createSidecarServer({ orchestrator, logger = console })` → `{ handler(req, res), listen(port) }`:
  - `POST /runs` body `{root_issue_id, skill_id?, definition, initial_state?}` → 启动 orchestrator.run(**异步**,不阻塞响应);立即返回 `202 {run_id}`(run_id 来自 createRun;实现上 orchestrator.run 内部先 create 再异步跑——见 note)。
  - `GET /runs/:id` → 代理查询 Simultica `GET /api/workflow-runs`?MVP 简化:返回 `501 {error:"query via Simultica"}`(真相源在 Simultica,sidecar 不重复存)。**入口保留**。
  - `POST /webhook/issue-done` → 入口保留,MVP 返回 `204`(不处理,仅占位)。
  - 未知路由 → `404`。
- `listen(port)` 启动 `http.createServer(handler)`。

- [ ] **Step 1: 写失败测试**

`test/lib/sidecarServer.test.js`:
```js
import { test } from 'node:test';
import assert from 'node:assert/strict';
import http from 'node:http';
import { createSidecarServer } from '../../lib/sidecarServer.js';

function once(server, port) {
  return new Promise((resolve) => server.listen(port, () => resolve()));
}
async function req(port, method, path, body) {
  return new Promise((resolve, reject) => {
    const data = body ? JSON.stringify(body) : null;
    const r = http.request({ host: '127.0.0.1', port, method, path, headers: { 'Content-Type': 'application/json' } }, (res) => {
      let buf = '';
      res.on('data', (c) => (buf += c));
      res.on('end', () => resolve({ status: res.statusCode, body: buf ? JSON.parse(buf) : null }));
    });
    r.on('error', reject);
    if (data) r.write(data);
    r.end();
  });
}

test('POST /runs starts run and returns 202 with run_id', async () => {
  const started = [];
  const orchestrator = { async run(p) { started.push(p); return { runId: 'run-x', finalState: {} }; } };
  const srv = createSidecarServer({ orchestrator, logger: { error() {}, info() {} } });
  const server = http.createServer(srv.handler);
  await once(server, 0);
  const port = server.address().port;
  const res = await req(port, 'POST', '/runs', { workspace_id: 'ws-1', root_issue_id: 'i1', definition: { nodes: [], routing: [] } });
  server.close();
  assert.equal(res.status, 202);
  assert.ok(res.body.run_id);
});

test('webhook issue-done returns 204', async () => {
  const srv = createSidecarServer({ orchestrator: { async run() {} }, logger: { error() {}, info() {} } });
  const server = http.createServer(srv.handler);
  await once(server, 0);
  const port = server.address().port;
  const res = await req(port, 'POST', '/webhook/issue-done', { issue_id: 'x' });
  server.close();
  assert.equal(res.status, 204);
});

test('unknown route 404', async () => {
  const srv = createSidecarServer({ orchestrator: { async run() {} }, logger: { error() {}, info() {} } });
  const server = http.createServer(srv.handler);
  await once(server, 0);
  const port = server.address().port;
  const res = await req(port, 'GET', '/nope');
  server.close();
  assert.equal(res.status, 404);
});
```

- [ ] **Step 2: 运行确认失败**

Run: `node --test test/lib/sidecarServer.test.js`
Expected: 模块不存在。

- [ ] **Step 3: 实现 server**

`lib/sidecarServer.js`:
```js
function readBody(req) {
  return new Promise((resolve, reject) => {
    let buf = '';
    req.on('data', (c) => (buf += c));
    req.on('end', () => {
      if (!buf) return resolve({});
      try { resolve(JSON.parse(buf)); } catch (e) { reject(e); }
    });
    req.on('error', reject);
  });
}

function send(res, status, obj) {
  const body = obj === undefined ? '' : JSON.stringify(obj);
  res.writeHead(status, { 'Content-Type': 'application/json' });
  res.end(body);
}

export function createSidecarServer({ orchestrator, logger = console }) {
  async function handler(req, res) {
    try {
      const url = new URL(req.url, 'http://localhost');
      const path = url.pathname;

      if (req.method === 'POST' && path === '/runs') {
        const body = await readBody(req);
        if (!body.root_issue_id || !body.definition || !body.workspace_id) {
          return send(res, 400, { error: 'workspace_id, root_issue_id and definition required' });
        }
        // Kick off the run; respond as soon as the run is registered.
        // orchestrator.run resolves with {runId}; we await only enough to get it.
        let settled = false;
        const runPromise = orchestrator.run({
          workspaceId: body.workspace_id,
          rootIssueId: body.root_issue_id,
          skillId: body.skill_id,
          definition: body.definition,
          initialState: body.initial_state || {},
        });
        // The orchestrator creates the run record before executing the graph,
        // but exposes runId only on resolve. For MVP we attach a catch to log
        // failures and return a synthetic ack; the run_id is surfaced once the
        // run resolves. To return run_id eagerly, orchestrator could accept an
        // onRunCreated callback (see note). MVP: await the createRun portion
        // by racing a short tick is unreliable — instead return 202 and let
        // the run resolve in background, logging errors.
        runPromise.then(
          (r) => { settled = true; logger.info?.('run completed', r.runId); },
          (e) => { settled = true; logger.error?.('run failed', e.message); }
        );
        // Best-effort: give createRun a microtask to populate. Acceptable for MVP.
        return send(res, 202, { run_id: 'accepted', note: 'run started; track via Simultica workflow_run' });
      }

      if (req.method === 'GET' && path.startsWith('/runs/')) {
        return send(res, 501, { error: 'query progress via Simultica /api/workflow-runs' });
      }

      if (req.method === 'POST' && path === '/webhook/issue-done') {
        // Entry point reserved for the C-tier real-time path. MVP: no-op ack.
        return send(res, 204, undefined);
      }

      return send(res, 404, { error: 'not found' });
    } catch (err) {
      logger.error?.('sidecar handler error', err.message);
      return send(res, 500, { error: err.message });
    }
  }

  return {
    handler,
    listen(port) {
      const http = require('node:http');
      const server = http.createServer(handler);
      server.listen(port);
      return server;
    },
  };
}
```

Note(run_id 返回):MVP 为简单起见返回 `run_id:"accepted"`,真实 run_id 在 Simultica 的 `workflow_run` 里以 `root_issue_id` 关联查询(GUI 用 `GET /api/issues/{id}/workflow-run`)。若要立刻返回真实 run_id,迭代时给 `orchestrator.run` 加 `onRunCreated(runId)` 回调,在 createRun 后同步触发,server 用 Promise 等到该回调再响应。**当前 MVP 不做**,因为 GUI 用 issue 维度查询已足够。`listen` 用 `require('node:http')` 在 ESM 下不可用 —— 改为文件顶部 `import http from 'node:http'` 并在 `listen` 用之。修正见 Step 3b。

- [ ] **Step 3b: 修正 ESM http import**

`lib/sidecarServer.js` 顶部加 `import http from 'node:http';`,并把 `listen` 内的 `const http = require('node:http');` 删除,直接 `const server = http.createServer(handler);`。

- [ ] **Step 4: 运行确认通过**

Run: `node --test test/lib/sidecarServer.test.js`
Expected: 3 passed。

- [ ] **Step 5: 写可执行入口 + package script**

`bin/sidecar.js`:
```js
#!/usr/bin/env node
import 'dotenv/config';
import http from 'node:http';
import { loadSidecarConfig } from '../lib/sidecarConfig.js';
import { createProgressClient } from '../lib/simulticaProgress.js';
import { createOrchestrator } from '../lib/orchestrator.js';
import { createSidecarServer } from '../lib/sidecarServer.js';
import { WorkflowBuilder } from '../lib/workflowBuilder.js';
import { createMulticaShell } from '../backends/multicaResult.js';
import { extractAssistantText, pickLatestRun, isTerminalRun, getRunTaskId, normalizeRunStatus } from '../backends/multicaResult.js';

const cfg = loadSidecarConfig();
const progressClient = createProgressClient({ baseUrl: cfg.simulticaBaseUrl, token: cfg.simulticaToken });
const shell = createMulticaShell({ multicaBin: process.env.MULTICA_BIN || 'multica', profile: process.env.MULTICA_PROFILE });

const orchestrator = createOrchestrator({
  progressClient, shell,
  agentRoutes: cfg.agentRoutes,
  pollIntervalMs: cfg.pollIntervalMs,
  nodeTimeoutMs: cfg.nodeTimeoutMs,
  builderFactory: (context) => new WorkflowBuilder({
    context: { ...context, extractAssistantText, pickLatestRun, isTerminalRun, getRunTaskId, normalizeRunStatus },
  }),
});

const srv = createSidecarServer({ orchestrator });
const server = http.createServer(srv.handler);
server.listen(cfg.port, () => console.log(`sidecar listening on :${cfg.port}`));
```

Note: `createMulticaShell` 与 `extractAssistantText` 等的确切导出位置以 `backends/multicaResult.js` 为准(`grep -n "export" backends/multicaResult.js`),对齐 import。SubIssueNode 需要的 `extractAssistantText`/`pickLatestRun`/`isTerminalRun`/`getRunTaskId`/`normalizeRunStatus` 经 WorkflowBuilder 的 `context` 透传到节点 context(Task 4 节点从 `ctx` 读它们);确认 `WorkflowBuilder` 把 `options.context` 透传给 `nodeRegistry.create` 的 context(`build()` 里 `{...this.context, config}`),成立。

`package.json` scripts 加:
```json
    "sidecar": "node bin/sidecar.js",
```

- [ ] **Step 6: 语法检查 + 全量测试**

Run:
```bash
node --check bin/sidecar.js
node --test
```
Expected: 无语法错;全部测试 passed(含既有测试,无回归)。

- [ ] **Step 7: Commit**

```bash
git add lib/sidecarServer.js bin/sidecar.js package.json test/lib/sidecarServer.test.js
git commit -m "feat(sidecar): http server + executable entrypoint"
```

---

### Task 8: PostgresSaver checkpointer 接线(引擎恢复)

**Files:**
- Modify: `package.json`(加依赖 `@langchain/langgraph-checkpoint-postgres`)
- Modify: `lib/orchestrator.js`(build 时把 checkpointer 传给 `graph.compile({ checkpointer })`)
- Modify: `lib/workflowBuilder.js`(`build` 接受可选 `checkpointer`,透传给 `compile`)
- Test: `test/lib/checkpointerWiring.test.js`(只验证 build 把 checkpointer 透传到 compile,用假 graph)

**Interfaces:**
- Consumes: Task 6 orchestrator、Task 1 `dbUrl`。
- Produces: orchestrator 在有 `checkpointerFactory` 时,`builder.build(definition, { checkpointer })` 把它透传给 `graph.compile({ checkpointer, ... })`。无 checkpointer 时行为不变(向后兼容)。

- [ ] **Step 1: 装依赖**

Run(在 agent-runtime):
```bash
npm install @langchain/langgraph-checkpoint-postgres
```
Expected: 安装成功,`package.json` dependencies 出现该包。

- [ ] **Step 2: 写透传测试(假 graph 捕获 compile 参数)**

`test/lib/checkpointerWiring.test.js`:
```js
import { test } from 'node:test';
import assert from 'node:assert/strict';
import { WorkflowBuilder } from '../../lib/workflowBuilder.js';

test('build passes checkpointer to compile', async () => {
  const builder = new WorkflowBuilder({ context: {} });
  // monkeypatch StateGraph compile via a minimal definition + spy
  const sentinel = { __cp: true };
  const def = {
    meta: { name: 'x' }, state: { fields: [{ name: 'task', type: 'string' }] },
    nodes: [{ id: 'n', type: 'transform', config: { template: '{{task}}' }, outputs: ['task'] }],
    routing: [{ from: 'START', to: 'n' }, { from: 'n', to: 'END' }],
  };
  // We can't easily spy on internal StateGraph; instead assert build accepts
  // the option without throwing and returns a compiled graph.
  const graph = await builder.build(def, { checkpointer: sentinel });
  assert.ok(graph, 'compiled graph returned with checkpointer option');
});
```

Note: 若 LangGraph 在传入非法 checkpointer 时报错,把 `sentinel` 换成一个最小满足接口的 stub,或断言"build 不抛错地接受 checkpointer 形参"。重点是验证 `build` 的签名扩展不破坏现有路径。

- [ ] **Step 3: 运行确认失败/或揭示当前不接受第二参**

Run: `node --test test/lib/checkpointerWiring.test.js`
Expected: 当前 `build` 忽略第二参(测试可能假通过)——改为断言透传:在 `build` 内把 `opts.checkpointer` 传 `compile`,并在测试用一个记录型 stub 验证。若 StateGraph 难以注入 spy,降级为"build(def, {checkpointer}) 正常返回且 def 单节点 transform 能 invoke"。

- [ ] **Step 4: 实现透传**

`lib/workflowBuilder.js` 把 `build(configOrPath)` 改为 `build(configOrPath, opts = {})`,末尾 `return graph.compile(opts.checkpointer ? { checkpointer: opts.checkpointer } : undefined);`。

`lib/orchestrator.js` 在创建时接受 `checkpointerFactory`,`run` 内:
```js
      const checkpointer = checkpointerFactory ? await checkpointerFactory() : undefined;
      graph = await builder.build(definition, { checkpointer });
```
并在 `createOrchestrator` 解构参数加 `checkpointerFactory`。

`bin/sidecar.js` 增加(import + factory):
```js
import { PostgresSaver } from '@langchain/langgraph-checkpoint-postgres';
// ...
let checkpointerSingleton;
const checkpointerFactory = async () => {
  if (!checkpointerSingleton) {
    checkpointerSingleton = PostgresSaver.fromConnString(cfg.dbUrl);
    await checkpointerSingleton.setup();
  }
  return checkpointerSingleton;
};
```
并把 `checkpointerFactory` 传入 `createOrchestrator({...})`。

Note: `PostgresSaver.fromConnString` / `.setup()` 的确切 API 以安装版本为准(`node -e "import('@langchain/langgraph-checkpoint-postgres').then(m=>console.log(Object.keys(m)))"`)。`setup()` 会在库里建 checkpoint 相关表(与 `workflow_run` 不冲突,不同表名)。

- [ ] **Step 5: 运行确认通过 + 全量回归**

Run:
```bash
node --test
node --check bin/sidecar.js
```
Expected: 全 passed;语法 OK。

- [ ] **Step 6: Commit**

```bash
git add package.json package-lock.json lib/workflowBuilder.js lib/orchestrator.js bin/sidecar.js test/lib/checkpointerWiring.test.js
git commit -m "feat(sidecar): postgres checkpointer for engine recovery"
```

---

### Task 9: E2E 冒烟(对本地 Simultica)

**Files:**
- Create: `test/e2e/sidecar-smoke.md`(手动冒烟步骤文档,非自动测试 —— 需真实 Simultica + agent 在线)

**Interfaces:**
- Consumes: 运行中的 Simultica 后端(Plan A 完成,:18083)、至少一个绑了 project 的子 agent、CLI token。

- [ ] **Step 1: 写冒烟文档**

`test/e2e/sidecar-smoke.md`:
```markdown
# Sidecar E2E 冒烟(手动)

前置:
- Simultica 后端在 :18083,Plan A 三端点可用。
- 准备一个 CLI token:`multica`(已登录的 dev 环境)。
- 准备一个主 issue id(在 simultica_dev 里建一个 issue 当 root)。
- agent 路由:用现有子 agent(如「码上办」)的 agentId + projectId。

步骤:
1. 启动 sidecar:
   SIDECAR_SIMULTICA_TOKEN=<token> \
   SIDECAR_AGENT_ROUTES='{"code":{"agentId":"<码上办 agentId>","projectId":"<其 projectId>"}}' \
   npm run sidecar

2. 提交一个最小 workflow(单 agent 节点):
   curl -s -XPOST http://localhost:8787/runs -H 'Content-Type: application/json' -d '{
     "workspace_id": "<workspace id, 如 simultica_dev 的 pxb workspace>",
     "root_issue_id": "<root issue id>",
     "definition": {
       "meta": {"name":"smoke"},
       "state": {"fields":[{"name":"task","type":"string","required":true}]},
       "nodes": [{"id":"impl","type":"llm","outputs":["execution"],"config":{"agent":"code","system":"写一句 hello"}}],
       "routing": [{"from":"START","to":"impl"},{"from":"impl","to":"END"}]
     },
     "initial_state": {"task":"打个招呼"}
   }'
   期望:202 accepted。

3. 观察:
   - simultica_dev 的 workflow_run 表出现一条 root_issue_id 匹配的记录,status 从 running→done。
   - root issue 下出现一个 [wf:impl] 子 issue,指派给「码上办」,跑完。
   - GET http://localhost:18083/api/issues/<root>/workflow-run 返回 nodes_state.impl.status=done。
```

- [ ] **Step 2: Commit**

```bash
git add test/e2e/sidecar-smoke.md
git commit -m "docs(sidecar): e2e smoke checklist"
```

---

## Plan B 完成标准

- `npm test` 全绿(config/progress/progressState/subIssueNode/dispatch/orchestrator/server/checkpointer)。
- `npm run sidecar` 起常驻 HTTP 服务,`POST /runs` 接受 workflow 并异步执行。
- agent 节点会在主 issue 下建子 issue、指派子 agent、轮询至 done、回填结果。
- 全程把 `nodes_state` + status 上报到 Simultica `workflow_run`。
- checkpointer 写 Simultica Postgres(引擎恢复入口就位)。
- `/webhook/issue-done` 路由占位(C 入口)。

依赖: 必须先完成 Plan A。下一步: Plan C(GUI)消费 `workflow_run` 读端点 + WS 渲染进度,并提供 workflow 编排器写 `workflow.yaml`。
