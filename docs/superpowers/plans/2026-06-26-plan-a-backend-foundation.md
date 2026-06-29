# Plan A — Simultica 后端基座(workflow_run + API + WS + skill workflow 存储)

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 在 Simultica Go 后端建立 workflow 编排所需的数据与 API 基座:`workflow_run` 进度真相源表、读写 API、WS 实时事件、以及把 workflow 文件挂到 skill 的存储约定。

**Architecture:** 新增 migration 119 建 `workflow_run` 表;用 sqlc 生成 queries;新增 `workflow_run.go` handler 提供 sidecar 写 / GUI 读 / 触发运行三类端点;新增 WS 事件 `workflow_run:updated`;workflow 文件复用既有 `skill_file` + `skill.config.has_workflow` 标记(无需新表)。这是 Plan B(sidecar)与 Plan C(GUI)的依赖根。

**Tech Stack:** Go (chi router, pgx/v5, sqlc), PostgreSQL, 现有 protocol/WS 广播。

## Global Constraints

- 仓库根: `/Users/bytedance/agent_workspace/simultica`,后端在 `server/`。
- migration 命名: `NNN_<slug>.up.sql` + `NNN_<slug>.down.sql`,下一个编号是 **119**(当前最大 118)。
- sqlc 配置: `server/sqlc.yaml`(queries=`pkg/db/queries/`,schema=`migrations/`,out=`pkg/db/generated`,package `db`,sql_package `pgx/v5`,`emit_json_tags: true`,`emit_empty_slices: true`)。生成命令在 `server/` 下跑 `sqlc generate`。
- handler 辅助: `writeJSON(w, status, v)`、`writeError(w, status, msg)`(`internal/handler/handler.go`);workspace id 用 `h.resolveWorkspaceID(r)`;URL 参数用 `chi.URLParam(r, "name")`。
- 路由注册在 `cmd/server/router.go`,authed + workspace 中间件分组内(参照 `/api/skills` 注册块,约 857 行)。
- 事件常量在 `pkg/protocol/events.go`;广播用 `h.publish(event, workspaceID, actorType, actorID, payload)`(参照 `issue_child_done.go:111`)。
- 测试: 在 `server/` 下 `go test ./internal/handler/ -run <Name>`;handler 测试惯例见 `internal/handler/*_test.go`(用 `newRequest`、`withURLParam`、`testHandler`、`testPool`)。
- 不改原生 Multica;所有改动只在 Simultica 仓库内。
- 不要 `--no-verify`、不跳过 hooks。每个 task 末尾 commit。

---

### Task 1: migration 119 建 workflow_run 表

**Files:**
- Create: `server/migrations/119_workflow_run.up.sql`
- Create: `server/migrations/119_workflow_run.down.sql`

**Interfaces:**
- Produces: 表 `workflow_run`,列 `id uuid pk`、`workspace_id uuid fk`、`root_issue_id uuid fk`、`skill_id uuid null fk`、`status text`、`current_node text`、`nodes_state jsonb`、`definition_snapshot jsonb`、`error text null`、`created_at`、`updated_at`。状态值 `pending|running|done|failed|cancelled`。

- [ ] **Step 1: 写 up migration**

`server/migrations/119_workflow_run.up.sql`:
```sql
-- workflow_run: 语义进度真相源。一条记录 = 一个主 issue 上的一次 workflow 运行。
-- sidecar(agent-runtime)单点写入;GUI 只读渲染。LangGraph 自身的 checkpoint
-- 另由 sidecar 的 PostgresSaver 管理(引擎恢复用),不在此表。
CREATE TABLE workflow_run (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id        UUID NOT NULL REFERENCES workspace(id) ON DELETE CASCADE,
    root_issue_id       UUID NOT NULL REFERENCES issue(id) ON DELETE CASCADE,
    skill_id            UUID REFERENCES skill(id) ON DELETE SET NULL,
    status              TEXT NOT NULL DEFAULT 'pending'
        CHECK (status IN ('pending','running','done','failed','cancelled')),
    current_node        TEXT NOT NULL DEFAULT '',
    -- nodes_state: { "<nodeId>": { "status": "...", "sub_issue_id": "...|null",
    --   "started_at": "...|null", "ended_at": "...|null", "error": "...|null" } }
    nodes_state         JSONB NOT NULL DEFAULT '{}',
    -- definition_snapshot: 本次运行所用 workflow 定义(编译前 JSON),保证回看一致。
    definition_snapshot JSONB NOT NULL DEFAULT '{}',
    error               TEXT,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_workflow_run_root_issue ON workflow_run(root_issue_id);
CREATE INDEX idx_workflow_run_status ON workflow_run(status);
CREATE INDEX idx_workflow_run_workspace ON workflow_run(workspace_id);
```

- [ ] **Step 2: 写 down migration**

`server/migrations/119_workflow_run.down.sql`:
```sql
DROP TABLE IF EXISTS workflow_run;
```

- [ ] **Step 3: 应用 migration 到本地 dev 库验证**

Run(在 `server/`):
```bash
PGPASSWORD=multica psql -h localhost -p 15433 -U multica -d simultica_dev -f migrations/119_workflow_run.up.sql
```
Expected: `CREATE TABLE` + 3×`CREATE INDEX`,无错误。

- [ ] **Step 4: 验证表结构**

Run:
```bash
PGPASSWORD=multica psql -h localhost -p 15433 -U multica -d simultica_dev -c "\d workflow_run"
```
Expected: 列出全部列、CHECK 约束、3 个索引、2 个外键。

- [ ] **Step 5: Commit**

```bash
git add server/migrations/119_workflow_run.up.sql server/migrations/119_workflow_run.down.sql
git commit -m "feat(db): add workflow_run table for orchestration progress"
```

---

### Task 2: workflow_run 的 sqlc queries + 生成代码

**Files:**
- Create: `server/pkg/db/queries/workflow_run.sql`
- Modify(生成产物,勿手写): `server/pkg/db/generated/workflow_run.sql.go`、`server/pkg/db/generated/models.go`

**Interfaces:**
- Consumes: Task 1 的 `workflow_run` 表。
- Produces(sqlc 生成的 Go 方法,Task 3 依赖):
  - `CreateWorkflowRun(ctx, CreateWorkflowRunParams) (WorkflowRun, error)`
  - `GetWorkflowRun(ctx, id pgtype.UUID) (WorkflowRun, error)`
  - `GetWorkflowRunByRootIssue(ctx, rootIssueID pgtype.UUID) (WorkflowRun, error)`
  - `UpdateWorkflowRunProgress(ctx, UpdateWorkflowRunProgressParams) (WorkflowRun, error)`
  - 结构体 `db.WorkflowRun`,字段 `ID/WorkspaceID/RootIssueID/SkillID/Status/CurrentNode/NodesState/DefinitionSnapshot/Error/CreatedAt/UpdatedAt`(类型遵循 sqlc:`pgtype.UUID`、`string`、`[]byte`(jsonb)、`pgtype.Text`、`pgtype.Timestamptz`)。

- [ ] **Step 1: 写 queries 文件**

`server/pkg/db/queries/workflow_run.sql`:
```sql
-- name: CreateWorkflowRun :one
INSERT INTO workflow_run (
    workspace_id, root_issue_id, skill_id, status,
    current_node, nodes_state, definition_snapshot
) VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING *;

-- name: GetWorkflowRun :one
SELECT * FROM workflow_run WHERE id = $1;

-- name: GetWorkflowRunByRootIssue :one
-- 一个主 issue 最多关注一次"当前/最近"运行;取最新一条。
SELECT * FROM workflow_run
WHERE root_issue_id = $1
ORDER BY created_at DESC
LIMIT 1;

-- name: UpdateWorkflowRunProgress :one
-- sidecar 单点写入:推进当前节点、节点状态、整体状态、错误。
UPDATE workflow_run SET
    status       = COALESCE(sqlc.narg('status'), status),
    current_node = COALESCE(sqlc.narg('current_node'), current_node),
    nodes_state  = COALESCE(sqlc.narg('nodes_state'), nodes_state),
    error        = sqlc.narg('error'),
    updated_at   = now()
WHERE id = $1
RETURNING *;
```

- [ ] **Step 2: 生成 sqlc 代码**

Run(在 `server/`):
```bash
sqlc generate
```
Expected: 无错误;新增 `pkg/db/generated/workflow_run.sql.go`,`models.go` 出现 `type WorkflowRun struct`。

- [ ] **Step 3: 编译确认生成代码可用**

Run(在 `server/`):
```bash
go build ./pkg/db/generated/
```
Expected: 退出码 0。

- [ ] **Step 4: 确认生成的方法名与字段**

Run:
```bash
grep -nE "func \(q \*Queries\) (Create|Get|Update)WorkflowRun" server/pkg/db/generated/workflow_run.sql.go
```
Expected: 出现 `CreateWorkflowRun`、`GetWorkflowRun`、`GetWorkflowRunByRootIssue`、`UpdateWorkflowRunProgress`。

- [ ] **Step 5: Commit**

```bash
git add server/pkg/db/queries/workflow_run.sql server/pkg/db/generated/workflow_run.sql.go server/pkg/db/generated/models.go
git commit -m "feat(db): generate workflow_run queries"
```

---

### Task 3: WS 事件常量 workflow_run:updated

**Files:**
- Modify: `server/pkg/protocol/events.go`

**Interfaces:**
- Produces: 常量 `protocol.EventWorkflowRunUpdated = "workflow_run:updated"`(Task 5 广播用,Plan C 订阅用)。

- [ ] **Step 1: 加事件常量**

在 `server/pkg/protocol/events.go` 的 issue 事件附近(`EventIssueMetadataChanged` 之后)加:
```go
	EventWorkflowRunUpdated = "workflow_run:updated"
```

- [ ] **Step 2: 编译确认**

Run(在 `server/`):
```bash
go build ./pkg/protocol/
```
Expected: 退出码 0。

- [ ] **Step 3: Commit**

```bash
git add server/pkg/protocol/events.go
git commit -m "feat(protocol): add workflow_run:updated event"
```

---

### Task 4: workflow_run handler — GUI 读端点

**Files:**
- Create: `server/internal/handler/workflow_run.go`
- Test: `server/internal/handler/workflow_run_test.go`

**Interfaces:**
- Consumes: `db.GetWorkflowRunByRootIssue`、`writeJSON`、`writeError`、`h.resolveWorkspaceID`、`chi.URLParam`。
- Produces:
  - `func (h *Handler) GetIssueWorkflowRun(w http.ResponseWriter, r *http.Request)` — 处理 `GET /api/issues/{id}/workflow-run`,返回该 issue 最新 workflow_run(无则 404)。
  - 响应结构 `WorkflowRunResponse{ID,Status,CurrentNode,NodesState(json.RawMessage),DefinitionSnapshot(json.RawMessage),Error,RootIssueID,SkillID,CreatedAt,UpdatedAt}`。
  - `func workflowRunToResponse(db.WorkflowRun) WorkflowRunResponse`(Task 5 复用)。

- [ ] **Step 1: 写失败测试**

`server/internal/handler/workflow_run_test.go`:
```go
package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
)

func TestGetIssueWorkflowRun_NotFound(t *testing.T) {
	issueID := createIssueForTimeline(t, "wf run none")
	req := newRequest("GET", "/api/issues/"+issueID+"/workflow-run", nil)
	req = withURLParam(req, "id", issueID)
	w := newRecorder()
	testHandler.GetIssueWorkflowRun(w, req)
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for issue with no workflow run, got %d: %s", w.Code, w.Body.String())
	}
}

func TestGetIssueWorkflowRun_ReturnsLatest(t *testing.T) {
	ctx := context.Background()
	issueID := createIssueForTimeline(t, "wf run present")
	var runID string
	if err := testPool.QueryRow(ctx, `
		INSERT INTO workflow_run (workspace_id, root_issue_id, status, current_node, nodes_state, definition_snapshot)
		VALUES ($1, $2, 'running', 'implement', '{"implement":{"status":"running"}}', '{"meta":{"name":"x"}}')
		RETURNING id
	`, testWorkspaceID, issueID).Scan(&runID); err != nil {
		t.Fatalf("seed workflow_run: %v", err)
	}
	t.Cleanup(func() { testPool.Exec(ctx, `DELETE FROM workflow_run WHERE id=$1`, runID) })

	req := newRequest("GET", "/api/issues/"+issueID+"/workflow-run", nil)
	req = withURLParam(req, "id", issueID)
	w := newRecorder()
	testHandler.GetIssueWorkflowRun(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var resp WorkflowRunResponse
	json.NewDecoder(w.Body).Decode(&resp)
	if resp.Status != "running" || resp.CurrentNode != "implement" {
		t.Fatalf("unexpected resp: %+v", resp)
	}
}
```

Note: 若 `newRecorder` 在测试包里叫别的名字,先 `grep -n "func newRecorder\|httptest.NewRecorder" server/internal/handler/*_test.go` 确认惯例并对齐。

- [ ] **Step 2: 运行测试确认失败**

Run(在 `server/`):
```bash
go test ./internal/handler/ -run TestGetIssueWorkflowRun -v
```
Expected: 编译失败 / `GetIssueWorkflowRun` 未定义。

- [ ] **Step 3: 写 handler 实现**

`server/internal/handler/workflow_run.go`:
```go
package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

type WorkflowRunResponse struct {
	ID                 string          `json:"id"`
	RootIssueID        string          `json:"root_issue_id"`
	SkillID            *string         `json:"skill_id"`
	Status             string          `json:"status"`
	CurrentNode        string          `json:"current_node"`
	NodesState         json.RawMessage `json:"nodes_state"`
	DefinitionSnapshot json.RawMessage `json:"definition_snapshot"`
	Error              *string         `json:"error"`
	CreatedAt          time.Time       `json:"created_at"`
	UpdatedAt          time.Time       `json:"updated_at"`
}

func workflowRunToResponse(run db.WorkflowRun) WorkflowRunResponse {
	resp := WorkflowRunResponse{
		ID:                 uuidToString(run.ID),
		RootIssueID:        uuidToString(run.RootIssueID),
		Status:             run.Status,
		CurrentNode:        run.CurrentNode,
		NodesState:         json.RawMessage(run.NodesState),
		DefinitionSnapshot: json.RawMessage(run.DefinitionSnapshot),
		CreatedAt:          run.CreatedAt.Time,
		UpdatedAt:          run.UpdatedAt.Time,
	}
	if run.SkillID.Valid {
		s := uuidToString(run.SkillID)
		resp.SkillID = &s
	}
	if run.Error.Valid {
		e := run.Error.String
		resp.Error = &e
	}
	return resp
}

func (h *Handler) GetIssueWorkflowRun(w http.ResponseWriter, r *http.Request) {
	issueID := chi.URLParam(r, "id")
	run, err := h.Queries.GetWorkflowRunByRootIssue(r.Context(), parseUUID(issueID))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeError(w, http.StatusNotFound, "no workflow run for this issue")
			return
		}
		writeError(w, http.StatusInternalServerError, "load workflow run: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, workflowRunToResponse(run))
}
```

Note: `uuidToString` / `parseUUID` 已存在于 handler 包(见 `issue_child_done.go`、`handler.go`)。`run.NodesState` 是 sqlc 对 jsonb 生成的 `[]byte`;若生成类型不是 `[]byte` 而是 `pgtype.*`,按实际生成类型调整(`grep -n "NodesState" server/pkg/db/generated/models.go`)。

- [ ] **Step 4: 运行测试确认通过**

Run(在 `server/`):
```bash
go test ./internal/handler/ -run TestGetIssueWorkflowRun -v
```
Expected: PASS(两个子测试)。

- [ ] **Step 5: Commit**

```bash
git add server/internal/handler/workflow_run.go server/internal/handler/workflow_run_test.go
git commit -m "feat(api): GET issue workflow-run endpoint"
```

---

### Task 5: workflow_run handler — sidecar 写端点 + WS 广播

**Files:**
- Modify: `server/internal/handler/workflow_run.go`
- Test: `server/internal/handler/workflow_run_test.go`

**Interfaces:**
- Consumes: `db.CreateWorkflowRun`、`db.UpdateWorkflowRunProgress`、`db.GetWorkflowRun`、`workflowRunToResponse`、`h.publish`、`protocol.EventWorkflowRunUpdated`。
- Produces:
  - `func (h *Handler) CreateWorkflowRun(w, r)` — `POST /api/workflow-runs`,body `{root_issue_id, skill_id?, definition_snapshot, current_node?}`,创建一条 `status=running` 记录,广播,返回 201 + WorkflowRunResponse。
  - `func (h *Handler) UpdateWorkflowRun(w, r)` — `PATCH /api/workflow-runs/{runId}`,body `{status?, current_node?, nodes_state?, error?}`,更新并广播,返回 200 + WorkflowRunResponse。

- [ ] **Step 1: 写失败测试**

追加到 `server/internal/handler/workflow_run_test.go`:
```go
func TestCreateAndUpdateWorkflowRun(t *testing.T) {
	issueID := createIssueForTimeline(t, "wf create+update")
	t.Cleanup(func() {
		testPool.Exec(context.Background(), `DELETE FROM workflow_run WHERE root_issue_id=$1`, issueID)
	})

	// Create
	body := map[string]any{
		"root_issue_id":       issueID,
		"definition_snapshot": map[string]any{"meta": map[string]any{"name": "wf"}},
		"current_node":        "START",
	}
	cw := newRecorder()
	testHandler.CreateWorkflowRun(cw, newRequest("POST", "/api/workflow-runs?workspace_id="+testWorkspaceID, body))
	if cw.Code != http.StatusCreated {
		t.Fatalf("create: expected 201, got %d: %s", cw.Code, cw.Body.String())
	}
	var created WorkflowRunResponse
	json.NewDecoder(cw.Body).Decode(&created)
	if created.Status != "running" {
		t.Fatalf("expected running, got %q", created.Status)
	}

	// Update
	upd := map[string]any{
		"status":       "done",
		"current_node": "END",
		"nodes_state":  map[string]any{"implement": map[string]any{"status": "done"}},
	}
	uw := newRecorder()
	ur := newRequest("PATCH", "/api/workflow-runs/"+created.ID, upd)
	ur = withURLParam(ur, "runId", created.ID)
	testHandler.UpdateWorkflowRun(uw, ur)
	if uw.Code != http.StatusOK {
		t.Fatalf("update: expected 200, got %d: %s", uw.Code, uw.Body.String())
	}
	var updated WorkflowRunResponse
	json.NewDecoder(uw.Body).Decode(&updated)
	if updated.Status != "done" || updated.CurrentNode != "END" {
		t.Fatalf("unexpected updated: %+v", updated)
	}
}
```

- [ ] **Step 2: 运行测试确认失败**

Run(在 `server/`):
```bash
go test ./internal/handler/ -run TestCreateAndUpdateWorkflowRun -v
```
Expected: `CreateWorkflowRun` / `UpdateWorkflowRun` 未定义。

- [ ] **Step 3: 实现两个 handler**

追加到 `server/internal/handler/workflow_run.go`(import 增加 `"github.com/jackc/pgx/v5/pgtype"`、`"github.com/multica-ai/multica/server/pkg/protocol"`):
```go
type createWorkflowRunRequest struct {
	RootIssueID        string          `json:"root_issue_id"`
	SkillID            string          `json:"skill_id"`
	DefinitionSnapshot json.RawMessage `json:"definition_snapshot"`
	CurrentNode        string          `json:"current_node"`
}

func (h *Handler) CreateWorkflowRun(w http.ResponseWriter, r *http.Request) {
	workspaceID := h.resolveWorkspaceID(r)
	if workspaceID == "" {
		writeError(w, http.StatusBadRequest, "workspace_id required")
		return
	}
	var req createWorkflowRunRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid body: "+err.Error())
		return
	}
	if req.RootIssueID == "" {
		writeError(w, http.StatusBadRequest, "root_issue_id required")
		return
	}
	snapshot := req.DefinitionSnapshot
	if len(snapshot) == 0 {
		snapshot = json.RawMessage(`{}`)
	}
	params := db.CreateWorkflowRunParams{
		WorkspaceID:        parseUUID(workspaceID),
		RootIssueID:        parseUUID(req.RootIssueID),
		Status:             "running",
		CurrentNode:        req.CurrentNode,
		NodesState:         []byte(`{}`),
		DefinitionSnapshot: []byte(snapshot),
	}
	if req.SkillID != "" {
		params.SkillID = parseUUID(req.SkillID)
	}
	run, err := h.Queries.CreateWorkflowRun(r.Context(), params)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "create workflow run: "+err.Error())
		return
	}
	resp := workflowRunToResponse(run)
	h.publish(protocol.EventWorkflowRunUpdated, workspaceID, "system", "", map[string]any{
		"workflow_run": resp,
	})
	writeJSON(w, http.StatusCreated, resp)
}

type updateWorkflowRunRequest struct {
	Status      *string         `json:"status"`
	CurrentNode *string         `json:"current_node"`
	NodesState  json.RawMessage `json:"nodes_state"`
	Error       *string         `json:"error"`
}

func (h *Handler) UpdateWorkflowRun(w http.ResponseWriter, r *http.Request) {
	runID := chi.URLParam(r, "runId")
	var req updateWorkflowRunRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid body: "+err.Error())
		return
	}
	params := db.UpdateWorkflowRunProgressParams{ID: parseUUID(runID)}
	if req.Status != nil {
		params.Status = pgtype.Text{String: *req.Status, Valid: true}
	}
	if req.CurrentNode != nil {
		params.CurrentNode = pgtype.Text{String: *req.CurrentNode, Valid: true}
	}
	if len(req.NodesState) > 0 {
		params.NodesState = []byte(req.NodesState)
	}
	if req.Error != nil {
		params.Error = pgtype.Text{String: *req.Error, Valid: true}
	}
	run, err := h.Queries.UpdateWorkflowRunProgress(r.Context(), params)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "update workflow run: "+err.Error())
		return
	}
	resp := workflowRunToResponse(run)
	h.publish(protocol.EventWorkflowRunUpdated, uuidToString(run.WorkspaceID), "system", "", map[string]any{
		"workflow_run": resp,
	})
	writeJSON(w, http.StatusOK, resp)
}
```

Note: `UpdateWorkflowRunProgressParams` 中 `NodesState`/`Status`/`CurrentNode` 的具体类型以 sqlc 生成为准。`COALESCE(sqlc.narg(...))` 对 `nodes_state` 生成的参数若是 `[]byte`,空 `[]byte` 会被当 NULL → COALESCE 保留原值,符合预期。`status`/`current_node` 用 `narg` 生成 `pgtype.Text`。生成后 `grep -n "type UpdateWorkflowRunProgressParams" server/pkg/db/generated/workflow_run.sql.go` 核对字段类型并对齐。

- [ ] **Step 4: 运行测试确认通过**

Run(在 `server/`):
```bash
go test ./internal/handler/ -run TestCreateAndUpdateWorkflowRun -v
```
Expected: PASS。

- [ ] **Step 5: Commit**

```bash
git add server/internal/handler/workflow_run.go server/internal/handler/workflow_run_test.go
git commit -m "feat(api): create/update workflow_run with WS broadcast"
```

---

### Task 6: 注册路由

**Files:**
- Modify: `server/cmd/server/router.go`

**Interfaces:**
- Consumes: Task 4/5 的 handler 方法。
- Produces: 三条已接线路由:
  - `GET /api/issues/{id}/workflow-run` → `h.GetIssueWorkflowRun`
  - `POST /api/workflow-runs` → `h.CreateWorkflowRun`
  - `PATCH /api/workflow-runs/{runId}` → `h.UpdateWorkflowRun`

- [ ] **Step 1: 找到现有 issue 路由块**

Run:
```bash
grep -n "/api/issues\|r.Route(\"/api/skills\"" server/cmd/server/router.go | head
```
确认 issue 与 skill 路由在同一 authed+workspace 分组内。

- [ ] **Step 2: 在该分组内加路由**

在 `/api/skills` 路由块附近(同一 `r.Group`/workspace 中间件作用域内)加:
```go
			// Workflow runs (orchestration progress)
			r.Get("/api/issues/{id}/workflow-run", h.GetIssueWorkflowRun)
			r.Route("/api/workflow-runs", func(r chi.Router) {
				r.Post("/", h.CreateWorkflowRun)
				r.Patch("/{runId}", h.UpdateWorkflowRun)
			})
```

Note: 若现有 issue 详情路由用的是 `r.Route("/api/issues/{id}", ...)` 嵌套块,则把 `GET .../workflow-run` 放进该嵌套块内写成 `r.Get("/workflow-run", h.GetIssueWorkflowRun)`,与既有风格一致。先 `grep -n "api/issues" server/cmd/server/router.go` 判断采用哪种。

- [ ] **Step 3: 编译**

Run(在 `server/`):
```bash
go build ./...
```
Expected: 退出码 0。

- [ ] **Step 4: 起后端冒烟验证读端点**

Run:
```bash
launchctl kickstart -k gui/$(id -u)/com.simultica.local-dev.backend
sleep 6
curl -s -o /dev/null -w "%{http_code}\n" http://localhost:18083/health
```
Expected: `200`(后端正常起;workflow 端点需鉴权,这里只验证编译+启动无回归)。

- [ ] **Step 5: Commit**

```bash
git add server/cmd/server/router.go
git commit -m "feat(api): wire workflow_run routes"
```

---

### Task 7: skill workflow 存储约定(标记 + 校验)

**Files:**
- Modify: `server/internal/handler/skill.go`(在 `UpsertSkillFile` 路径上,当 `path` 为 `workflow.yaml` 时同步置 `skill.config.has_workflow=true`)
- Test: `server/internal/handler/skill_test.go`(或新建 `skill_workflow_test.go`)

**Interfaces:**
- Consumes: 既有 `UpsertSkillFile` handler、`db.UpdateSkill`(支持 `config` narg)、`db.GetSkill`。
- Produces: 约定 —— 当某 skill 存在 `skill_file.path = "workflow.yaml"` 时,其 `skill.config` JSON 含 `"has_workflow": true`;删除该文件时置回 `false`。常量 `const workflowFilePath = "workflow.yaml"`。

- [ ] **Step 1: 写失败测试**

`server/internal/handler/skill_workflow_test.go`:
```go
package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
)

func TestUpsertWorkflowFileSetsHasWorkflowFlag(t *testing.T) {
	ctx := context.Background()
	// 复用测试中创建 skill 的既有辅助;若无,直接插一行。
	var skillID string
	if err := testPool.QueryRow(ctx, `
		INSERT INTO skill (workspace_id, name, description, content, config)
		VALUES ($1, 'wf-skill', '', '', '{}') RETURNING id
	`, testWorkspaceID).Scan(&skillID); err != nil {
		t.Fatalf("seed skill: %v", err)
	}
	t.Cleanup(func() { testPool.Exec(ctx, `DELETE FROM skill WHERE id=$1`, skillID) })

	body := map[string]any{"path": "workflow.yaml", "content": "meta:\n  name: wf\n"}
	req := newRequest("PUT", "/api/skills/"+skillID+"/files", body)
	req = withURLParam(req, "id", skillID)
	w := newRecorder()
	testHandler.UpsertSkillFile(w, req)
	if w.Code != http.StatusOK && w.Code != http.StatusCreated {
		t.Fatalf("upsert file: got %d: %s", w.Code, w.Body.String())
	}

	var cfg []byte
	if err := testPool.QueryRow(ctx, `SELECT config FROM skill WHERE id=$1`, skillID).Scan(&cfg); err != nil {
		t.Fatalf("read config: %v", err)
	}
	var parsed map[string]any
	json.Unmarshal(cfg, &parsed)
	if parsed["has_workflow"] != true {
		t.Fatalf("expected has_workflow=true in config, got %s", string(cfg))
	}
}
```

Note: 先 `grep -n "func (h \*Handler) UpsertSkillFile" server/internal/handler/skill.go` 确认入参从 body 读 `path`/`content` 的字段名,对齐测试 body。

- [ ] **Step 2: 运行测试确认失败**

Run(在 `server/`):
```bash
go test ./internal/handler/ -run TestUpsertWorkflowFileSetsHasWorkflowFlag -v
```
Expected: FAIL(`has_workflow` 不存在)。

- [ ] **Step 3: 在 UpsertSkillFile 成功写入后置标记**

在 `UpsertSkillFile` 写入文件成功后、返回响应前插入(具体变量名以现有 handler 为准):
```go
	if filePath == workflowFilePath {
		if err := h.setSkillHasWorkflow(r.Context(), parseUUID(skillID), true); err != nil {
			// 非致命:文件已存,标记失败仅影响"是否有 workflow"的快速判断。
			// 记录日志但不阻断。
			slog.Warn("skill: set has_workflow flag failed", "skill_id", skillID, "err", err.Error())
		}
	}
```

在 `skill.go` 增加常量与辅助函数:
```go
const workflowFilePath = "workflow.yaml"

func (h *Handler) setSkillHasWorkflow(ctx context.Context, skillID pgtype.UUID, has bool) error {
	skill, err := h.Queries.GetSkill(ctx, skillID)
	if err != nil {
		return err
	}
	var cfg map[string]any
	if len(skill.Config) > 0 {
		_ = json.Unmarshal(skill.Config, &cfg)
	}
	if cfg == nil {
		cfg = map[string]any{}
	}
	cfg["has_workflow"] = has
	raw, err := json.Marshal(cfg)
	if err != nil {
		return err
	}
	_, err = h.Queries.UpdateSkill(ctx, db.UpdateSkillParams{
		ID:     skillID,
		Config: raw,
	})
	return err
}
```

Note: `UpdateSkillParams` 各字段以 sqlc 生成为准;`config` 是 `COALESCE(sqlc.narg('config'), config)`,故只传 `Config` 即可,其余 narg 字段留零值(NULL → 保留原值)。`skill.Config` 是 jsonb 的 `[]byte`。确认 `GetSkill`/`UpdateSkill` 的参数结构后对齐字段名。若 `UpdateSkillParams` 把其它列设为非 narg 的必填,改用一条专门的 `SetSkillConfig` query(在 `skill.sql` 加 `-- name: SetSkillConfig :exec` `UPDATE skill SET config=$2, updated_at=now() WHERE id=$1;` 并 `sqlc generate`)。

- [ ] **Step 4: 运行测试确认通过**

Run(在 `server/`):
```bash
go test ./internal/handler/ -run TestUpsertWorkflowFileSetsHasWorkflowFlag -v
```
Expected: PASS。

- [ ] **Step 5: 跑 handler 包全量回归**

Run(在 `server/`):
```bash
go test ./internal/handler/ -count=1
```
Expected: PASS(无回归)。

- [ ] **Step 6: Commit**

```bash
git add server/internal/handler/skill.go server/internal/handler/skill_workflow_test.go server/pkg/db/queries/skill.sql server/pkg/db/generated/
git commit -m "feat(skill): mark has_workflow when workflow.yaml is stored"
```

---

## Plan A 完成标准

- migration 119 已应用,`workflow_run` 表存在。
- `GET /api/issues/{id}/workflow-run`、`POST /api/workflow-runs`、`PATCH /api/workflow-runs/{runId}` 三端点可用并广播 `workflow_run:updated`。
- 存 `workflow.yaml` 到某 skill 会置 `config.has_workflow=true`。
- `go build ./...` + `go test ./internal/handler/ -count=1` 通过,后端 /health 200。

下一步: Plan B(agent-runtime sidecar)消费这些 API;Plan C(GUI)消费读端点与 WS 事件。
