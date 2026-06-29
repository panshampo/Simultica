package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestGetIssueWorkflowRun_NotFound(t *testing.T) {
	issueID := createIssueForTimeline(t, "wf run none")
	req := newRequest("GET", "/api/issues/"+issueID+"/workflow-run", nil)
	req = withURLParam(req, "id", issueID)
	w := httptest.NewRecorder()

	testHandler.GetIssueWorkflowRun(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for issue with no workflow run, got %d: %s", w.Code, w.Body.String())
	}
}

func TestGetIssueWorkflowRun_ReturnsLatest(t *testing.T) {
	ctx := context.Background()
	issueID := createIssueForTimeline(t, "wf run present")
	var olderID, newerID string
	if err := testPool.QueryRow(ctx, `
		INSERT INTO workflow_run (workspace_id, root_issue_id, status, current_node, nodes_state, definition_snapshot, created_at)
		VALUES ($1, $2, 'running', 'old', '{"old":{"status":"running"}}', '{"meta":{"name":"old"}}', now() - interval '1 minute')
		RETURNING id
	`, testWorkspaceID, issueID).Scan(&olderID); err != nil {
		t.Fatalf("seed older workflow_run: %v", err)
	}
	if err := testPool.QueryRow(ctx, `
		INSERT INTO workflow_run (workspace_id, root_issue_id, status, current_node, nodes_state, definition_snapshot)
		VALUES ($1, $2, 'running', 'implement', '{"implement":{"status":"running"}}', '{"meta":{"name":"x"}}')
		RETURNING id
	`, testWorkspaceID, issueID).Scan(&newerID); err != nil {
		t.Fatalf("seed newer workflow_run: %v", err)
	}
	t.Cleanup(func() {
		testPool.Exec(ctx, `DELETE FROM workflow_run WHERE id = ANY($1::uuid[])`, []string{olderID, newerID})
	})

	req := newRequest("GET", "/api/issues/"+issueID+"/workflow-run", nil)
	req = withURLParam(req, "id", issueID)
	w := httptest.NewRecorder()

	testHandler.GetIssueWorkflowRun(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var resp WorkflowRunResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.ID != newerID || resp.Status != "running" || resp.CurrentNode != "implement" {
		t.Fatalf("unexpected response: %+v", resp)
	}
}

func TestCreateAndUpdateWorkflowRun(t *testing.T) {
	ctx := context.Background()
	issueID := createIssueForTimeline(t, "wf create update")
	t.Cleanup(func() {
		testPool.Exec(ctx, `DELETE FROM workflow_run WHERE root_issue_id=$1`, issueID)
	})

	createBody := map[string]any{
		"root_issue_id":         issueID,
		"definition_snapshot":   map[string]any{"meta": map[string]any{"name": "wf"}},
		"current_node":          "START",
		"unexpected_ignored_by": "json",
	}
	createReq := newRequest("POST", "/api/workflow-runs?workspace_id="+testWorkspaceID, createBody)
	createW := httptest.NewRecorder()

	testHandler.CreateWorkflowRun(createW, createReq)

	if createW.Code != http.StatusCreated {
		t.Fatalf("create: expected 201, got %d: %s", createW.Code, createW.Body.String())
	}
	var created WorkflowRunResponse
	if err := json.NewDecoder(createW.Body).Decode(&created); err != nil {
		t.Fatalf("decode create response: %v", err)
	}
	if created.Status != "running" || created.CurrentNode != "START" {
		t.Fatalf("unexpected created response: %+v", created)
	}

	updateReq := newRequest("PATCH", "/api/workflow-runs/"+created.ID+"?workspace_id="+testWorkspaceID, map[string]any{
		"status":       "done",
		"current_node": "END",
		"nodes_state":  map[string]any{"implement": map[string]any{"status": "done"}},
	})
	updateReq = withURLParam(updateReq, "runId", created.ID)
	updateW := httptest.NewRecorder()

	testHandler.UpdateWorkflowRun(updateW, updateReq)

	if updateW.Code != http.StatusOK {
		t.Fatalf("update: expected 200, got %d: %s", updateW.Code, updateW.Body.String())
	}
	var updated WorkflowRunResponse
	if err := json.NewDecoder(updateW.Body).Decode(&updated); err != nil {
		t.Fatalf("decode update response: %v", err)
	}
	if updated.Status != "done" || updated.CurrentNode != "END" {
		t.Fatalf("unexpected updated response: %+v", updated)
	}
	if !json.Valid(updated.NodesState) || string(updated.NodesState) == "{}" {
		t.Fatalf("expected non-empty valid nodes_state, got %s", string(updated.NodesState))
	}
}

func TestStartIssueWorkflowRunRejectsInvalidWorkflowSkill(t *testing.T) {
	issueID := createIssueForTimeline(t, "wf invalid skill start")
	skillID := insertHandlerTestSkill(t, "wf-invalid-start", "# skill")
	if _, err := testPool.Exec(context.Background(), `
		INSERT INTO skill_file (skill_id, path, content)
		VALUES ($1, $2, $3)
	`, skillID, workflowFilePath, "meta:\n  name: invalid\n"); err != nil {
		t.Fatalf("seed workflow file: %v", err)
	}
	if _, err := testPool.Exec(context.Background(), `
		UPDATE skill
		SET config='{"has_workflow": true, "workflow_validation": {"valid": false, "errors": ["bad route"], "warnings": []}}'::jsonb
		WHERE id=$1
	`, skillID); err != nil {
		t.Fatalf("seed workflow validation: %v", err)
	}

	req := newRequest("POST", "/api/issues/"+issueID+"/workflow-run/start?workspace_id="+testWorkspaceID, map[string]any{
		"skill_id": skillID,
	})
	req = withURLParam(req, "id", issueID)
	w := httptest.NewRecorder()

	testHandler.StartIssueWorkflowRun(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid workflow, got %d: %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "workflow validation failed") {
		t.Fatalf("expected validation error, got %s", w.Body.String())
	}
}

func TestContinueWorkflowRunStartsContinuationWithPreviousContext(t *testing.T) {
	ctx := context.Background()
	issueID := createIssueForTimeline(t, "wf continuation")
	skillID := insertHandlerTestSkill(t, "wf-continuation", "# skill")
	if _, err := testPool.Exec(ctx, `
		INSERT INTO skill_file (skill_id, path, content)
		VALUES ($1, $2, $3)
	`, skillID, workflowFilePath, validWorkflowYAML()); err != nil {
		t.Fatalf("seed workflow file: %v", err)
	}
	if _, err := testPool.Exec(ctx, `
		UPDATE skill
		SET config='{"has_workflow": true, "workflow_validation": {"valid": true, "errors": [], "warnings": []}}'::jsonb
		WHERE id=$1
	`, skillID); err != nil {
		t.Fatalf("seed workflow validation: %v", err)
	}
	var previousID string
	if err := testPool.QueryRow(ctx, `
		INSERT INTO workflow_run (workspace_id, root_issue_id, skill_id, status, current_node, nodes_state, definition_snapshot, source_skills)
		VALUES ($1, $2, $3, 'done', 'review_result', '{"review_result":{"status":"done"}}', '{"meta":{"name":"wf"}}', '[]')
		RETURNING id
	`, testWorkspaceID, issueID, skillID).Scan(&previousID); err != nil {
		t.Fatalf("seed previous workflow run: %v", err)
	}
	t.Cleanup(func() { testPool.Exec(ctx, `DELETE FROM workflow_run WHERE root_issue_id=$1`, issueID) })

	var sidecarPayload map[string]any
	sidecar := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&sidecarPayload); err != nil {
			t.Fatalf("decode sidecar payload: %v", err)
		}
		w.WriteHeader(http.StatusAccepted)
		_, _ = w.Write([]byte(`{"status":"accepted"}`))
	}))
	defer sidecar.Close()
	t.Setenv("MULTICA_WORKFLOW_SIDECAR_URL", sidecar.URL)

	req := newRequest("POST", "/api/workflow-runs/"+previousID+"/continue?workspace_id="+testWorkspaceID, map[string]any{
		"decision": "continue one more round",
	})
	req = withURLParam(req, "runId", previousID)
	w := httptest.NewRecorder()

	testHandler.ContinueWorkflowRun(w, req)

	if w.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d: %s", w.Code, w.Body.String())
	}
	initial, _ := sidecarPayload["initial_state"].(map[string]any)
	if initial["previous_run_id"] != previousID || initial["continuation_decision"] != "continue one more round" {
		t.Fatalf("unexpected continuation initial state: %#v", initial)
	}
}

func TestCancelWorkflowRunPersistsCancelMetadata(t *testing.T) {
	ctx := context.Background()
	issueID := createIssueForTimeline(t, "wf cancel metadata")
	var runID string
	if err := testPool.QueryRow(ctx, `
		INSERT INTO workflow_run (workspace_id, root_issue_id, status, current_node, nodes_state, definition_snapshot)
		VALUES ($1, $2, 'running', 'implement', '{"implement":{"status":"running"}}', '{"meta":{"name":"cancel"}}')
		RETURNING id
	`, testWorkspaceID, issueID).Scan(&runID); err != nil {
		t.Fatalf("seed workflow_run: %v", err)
	}
	t.Cleanup(func() {
		testPool.Exec(ctx, `DELETE FROM workflow_run WHERE id = $1`, runID)
	})

	req := newRequest("POST", "/api/workflow-runs/"+runID+"/cancel?workspace_id="+testWorkspaceID, map[string]any{
		"reason": "user stopped workflow",
	})
	req = withURLParam(req, "runId", runID)
	w := httptest.NewRecorder()

	testHandler.CancelWorkflowRun(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var resp WorkflowRunResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.Status != "cancelled" {
		t.Fatalf("status = %q, want cancelled", resp.Status)
	}
	if resp.CancelReason == nil || *resp.CancelReason != "user stopped workflow" {
		t.Fatalf("cancel_reason = %v, want user stopped workflow", resp.CancelReason)
	}

	var status, cancelReason string
	var cancelledAt any
	if err := testPool.QueryRow(ctx, `
		SELECT status, cancel_reason, cancelled_at
		FROM workflow_run
		WHERE id = $1
	`, runID).Scan(&status, &cancelReason, &cancelledAt); err != nil {
		t.Fatalf("query workflow_run: %v", err)
	}
	if status != "cancelled" || cancelReason != "user stopped workflow" || cancelledAt == nil {
		t.Fatalf("unexpected persisted cancel fields: status=%q reason=%q cancelledAt=%v", status, cancelReason, cancelledAt)
	}
}

func TestCancelWorkflowRunCancelsActiveChildIssueTasks(t *testing.T) {
	ctx := context.Background()
	agentID := createHandlerTestAgent(t, "Workflow Cancel Child Agent", nil)
	rootIssueID := createIssueForTimeline(t, "wf cancel child root")
	childIssueID := createIssueForTimeline(t, "wf cancel child")
	if _, err := testPool.Exec(ctx, `
		UPDATE issue
		SET parent_issue_id = $1, assignee_type = 'agent', assignee_id = $2
		WHERE id = $3
	`, rootIssueID, agentID, childIssueID); err != nil {
		t.Fatalf("link child issue: %v", err)
	}
	taskID := createHandlerTestTaskForAgentOnIssue(t, agentID, childIssueID)

	var runID string
	if err := testPool.QueryRow(ctx, `
		INSERT INTO workflow_run (workspace_id, root_issue_id, status, current_node, nodes_state, definition_snapshot)
		VALUES ($1, $2, 'running', 'implement', $3, '{"meta":{"name":"cancel child"}}')
		RETURNING id
	`, testWorkspaceID, rootIssueID, map[string]any{
		"implement": map[string]any{
			"status":       "running",
			"sub_issue_id": childIssueID,
			"task_id":      taskID,
		},
	}).Scan(&runID); err != nil {
		t.Fatalf("seed workflow_run: %v", err)
	}
	t.Cleanup(func() {
		testPool.Exec(ctx, `DELETE FROM workflow_run WHERE id = $1`, runID)
	})

	req := newRequest("POST", "/api/workflow-runs/"+runID+"/cancel?workspace_id="+testWorkspaceID, map[string]any{
		"reason": "stop active child",
	})
	req = withURLParam(req, "runId", runID)
	w := httptest.NewRecorder()

	testHandler.CancelWorkflowRun(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var taskStatus string
	if err := testPool.QueryRow(ctx, `SELECT status FROM agent_task_queue WHERE id = $1`, taskID).Scan(&taskStatus); err != nil {
		t.Fatalf("query task status: %v", err)
	}
	if taskStatus != "cancelled" {
		t.Fatalf("task status = %q, want cancelled", taskStatus)
	}
}

func TestCancelWorkflowRunCancelsOriginMarkedChildIssueTasks(t *testing.T) {
	ctx := context.Background()
	agentID := createHandlerTestAgent(t, "Workflow Cancel Origin Child Agent", nil)
	rootIssueID := createIssueForTimeline(t, "wf cancel origin child root")

	var runID string
	if err := testPool.QueryRow(ctx, `
		INSERT INTO workflow_run (workspace_id, root_issue_id, status, current_node, nodes_state, definition_snapshot)
		VALUES ($1, $2, 'running', 'START', '{}'::jsonb, '{"meta":{"name":"cancel origin child"}}')
		RETURNING id
	`, testWorkspaceID, rootIssueID).Scan(&runID); err != nil {
		t.Fatalf("seed workflow_run: %v", err)
	}
	t.Cleanup(func() {
		testPool.Exec(ctx, `DELETE FROM workflow_run WHERE id = $1`, runID)
	})

	childIssueID := createIssueForTimeline(t, "wf cancel origin child")
	if _, err := testPool.Exec(ctx, `
		UPDATE issue
		SET parent_issue_id = $1,
		    assignee_type = 'agent',
		    assignee_id = $2,
		    origin_type = 'workflow_node',
		    origin_id = $3
		WHERE id = $4
	`, rootIssueID, agentID, runID, childIssueID); err != nil {
		t.Fatalf("link origin child issue: %v", err)
	}
	taskID := createHandlerTestTaskForAgentOnIssue(t, agentID, childIssueID)

	req := newRequest("POST", "/api/workflow-runs/"+runID+"/cancel?workspace_id="+testWorkspaceID, map[string]any{
		"reason": "stop origin child",
	})
	req = withURLParam(req, "runId", runID)
	w := httptest.NewRecorder()

	testHandler.CancelWorkflowRun(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var taskStatus string
	if err := testPool.QueryRow(ctx, `SELECT status FROM agent_task_queue WHERE id = $1`, taskID).Scan(&taskStatus); err != nil {
		t.Fatalf("query task status: %v", err)
	}
	if taskStatus != "cancelled" {
		t.Fatalf("task status = %q, want cancelled", taskStatus)
	}
}

func TestCreateWorkflowMainNodeTask(t *testing.T) {
	ctx := context.Background()
	agentID := createHandlerTestAgent(t, "Workflow Main Node API Agent", nil)
	rootIssueID := createIssueForTimeline(t, "wf main node api root")
	if _, err := testPool.Exec(ctx,
		`UPDATE issue SET assignee_type = 'agent', assignee_id = $1 WHERE id = $2`,
		agentID, rootIssueID,
	); err != nil {
		t.Fatalf("assign root issue: %v", err)
	}
	var runID string
	if err := testPool.QueryRow(ctx, `
		INSERT INTO workflow_run (workspace_id, root_issue_id, status, current_node, nodes_state, definition_snapshot)
		VALUES ($1, $2, 'running', 'final', '{}'::jsonb, '{"meta":{"name":"main node api"}}'::jsonb)
		RETURNING id
	`, testWorkspaceID, rootIssueID).Scan(&runID); err != nil {
		t.Fatalf("seed workflow_run: %v", err)
	}
	t.Cleanup(func() {
		testPool.Exec(ctx, `DELETE FROM workflow_run WHERE id = $1`, runID)
	})

	req := newRequest("POST", "/api/workflow-runs/"+runID+"/main-node-task?workspace_id="+testWorkspaceID, map[string]any{
		"node_id":   "final",
		"node_type": "final_response",
	})
	req = withURLParam(req, "runId", runID)
	w := httptest.NewRecorder()

	testHandler.CreateWorkflowMainNodeTask(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}
	var resp map[string]string
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp["task_id"] == "" {
		t.Fatalf("expected task_id in response: %+v", resp)
	}
	var contextRaw string
	if err := testPool.QueryRow(ctx, `SELECT context::text FROM agent_task_queue WHERE id = $1`, resp["task_id"]).Scan(&contextRaw); err != nil {
		t.Fatalf("query task context: %v", err)
	}
	for _, want := range []string{`"workflow_main_node"`, `"workflow_run_id": "` + runID + `"`, `"node_id": "final"`} {
		if !strings.Contains(contextRaw, want) {
			t.Fatalf("context %s missing %s", contextRaw, want)
		}
	}
}

func TestCreateWorkflowMainNodeTaskUsesRequestAgentIDWhenRootUnassigned(t *testing.T) {
	ctx := context.Background()
	agentID := createHandlerTestAgent(t, "Workflow Main Node Request Agent", nil)
	rootIssueID := createIssueForTimeline(t, "wf main node request agent root")
	var runID string
	if err := testPool.QueryRow(ctx, `
		INSERT INTO workflow_run (workspace_id, root_issue_id, status, current_node, nodes_state, definition_snapshot)
		VALUES ($1, $2, 'running', 'final', '{}'::jsonb, '{"meta":{"name":"main node request agent"}}'::jsonb)
		RETURNING id
	`, testWorkspaceID, rootIssueID).Scan(&runID); err != nil {
		t.Fatalf("seed workflow_run: %v", err)
	}
	t.Cleanup(func() {
		testPool.Exec(ctx, `DELETE FROM workflow_run WHERE id = $1`, runID)
	})

	req := newRequest("POST", "/api/workflow-runs/"+runID+"/main-node-task?workspace_id="+testWorkspaceID, map[string]any{
		"node_id":   "final",
		"node_type": "final_response",
		"agent_id":  agentID,
	})
	req = withURLParam(req, "runId", runID)
	w := httptest.NewRecorder()

	testHandler.CreateWorkflowMainNodeTask(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}
	var resp map[string]string
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	var gotAgentID string
	if err := testPool.QueryRow(ctx, `SELECT agent_id FROM agent_task_queue WHERE id = $1`, resp["task_id"]).Scan(&gotAgentID); err != nil {
		t.Fatalf("query task agent_id: %v", err)
	}
	if gotAgentID != agentID {
		t.Fatalf("task agent_id = %s, want %s", gotAgentID, agentID)
	}
}

func TestCreateWorkflowRunRejectsCrossWorkspaceRootIssue(t *testing.T) {
	otherWorkspaceID := createWorkflowRunOtherWorkspace(t)
	issueID := createIssueForTimeline(t, "wf cross workspace")

	req := newRequest("POST", "/api/workflow-runs", map[string]any{
		"root_issue_id": issueID,
	})
	req.Header.Set("X-Workspace-ID", otherWorkspaceID)
	w := httptest.NewRecorder()

	testHandler.CreateWorkflowRun(w, req)

	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for cross-workspace root issue, got %d: %s", w.Code, w.Body.String())
	}
}

func createWorkflowRunOtherWorkspace(t *testing.T) string {
	t.Helper()
	ctx := context.Background()
	var workspaceID string
	if err := testPool.QueryRow(ctx, `
		INSERT INTO workspace (name, slug, issue_prefix)
		VALUES ('workflow-run-other', 'workflow-run-other-' || gen_random_uuid(), 'WFO')
		RETURNING id
	`).Scan(&workspaceID); err != nil {
		t.Fatalf("create other workspace: %v", err)
	}
	t.Cleanup(func() {
		testPool.Exec(ctx, `DELETE FROM workspace WHERE id=$1`, workspaceID)
	})
	return workspaceID
}
