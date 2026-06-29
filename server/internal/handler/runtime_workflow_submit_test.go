package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSubmitRuntimeWorkflowRejectsUnsupportedNodeType(t *testing.T) {
	issueID := createIssueForTimeline(t, "runtime workflow invalid node")
	req := newRequest("POST", "/api/issues/"+issueID+"/runtime-workflows?workspace_id="+testWorkspaceID, map[string]any{
		"definition": map[string]any{
			"meta": map[string]any{"name": "bad"},
			"nodes": []map[string]any{
				{"id": "danger", "type": "shell"},
			},
			"routing": []map[string]any{{"from": "START", "to": "danger"}},
		},
	})
	req = withURLParam(req, "id", issueID)
	w := httptest.NewRecorder()

	testHandler.SubmitRuntimeWorkflow(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestSubmitRuntimeWorkflowRejectsDirectSubagentUntilImplemented(t *testing.T) {
	issueID := createIssueForTimeline(t, "runtime workflow direct subagent")
	req := newRequest("POST", "/api/issues/"+issueID+"/runtime-workflows?workspace_id="+testWorkspaceID, map[string]any{
		"definition": map[string]any{
			"meta":  map[string]any{"name": "direct"},
			"state": map[string]any{"fields": []map[string]any{}},
			"nodes": []map[string]any{
				{"id": "review", "type": "agent", "dispatch": "direct_subagent", "agent": "architect"},
			},
			"routing": []map[string]any{{"from": "START", "to": "review"}},
		},
	})
	req = withURLParam(req, "id", issueID)
	w := httptest.NewRecorder()

	testHandler.SubmitRuntimeWorkflow(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "direct_subagent") {
		t.Fatalf("expected direct_subagent error, got %s", w.Body.String())
	}
}

func TestSubmitRuntimeWorkflowAcceptsMainIssueTaskNode(t *testing.T) {
	issueID := createIssueForTimeline(t, "runtime workflow main issue task")
	sidecar := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusAccepted)
		_, _ = w.Write([]byte(`{"status":"accepted"}`))
	}))
	defer sidecar.Close()
	t.Setenv("MULTICA_WORKFLOW_SIDECAR_URL", sidecar.URL)
	body := validRuntimeWorkflowSubmitBody()
	def := body["definition"].(map[string]any)
	def["nodes"] = append(def["nodes"].([]map[string]any), map[string]any{
		"id":       "final",
		"type":     "main_agent",
		"dispatch": "main_issue_task",
		"config":   map[string]any{"purpose": "summarize"},
	})
	def["routing"] = []map[string]any{
		{"from": "START", "to": "implement"},
		{"from": "implement", "to": "final"},
		{"from": "final", "to": "END"},
	}
	req := newRequest("POST", "/api/issues/"+issueID+"/runtime-workflows?workspace_id="+testWorkspaceID, body)
	req = withURLParam(req, "id", issueID)
	w := httptest.NewRecorder()

	testHandler.SubmitRuntimeWorkflow(w, req)

	if w.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d: %s", w.Code, w.Body.String())
	}
}

func TestSubmitRuntimeWorkflowCreatesRunAndStartsSidecar(t *testing.T) {
	issueID := createIssueForTimeline(t, "runtime workflow valid")
	var sidecarPayload map[string]any
	sidecar := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/runs" {
			t.Fatalf("unexpected sidecar request: %s %s", r.Method, r.URL.Path)
		}
		if err := json.NewDecoder(r.Body).Decode(&sidecarPayload); err != nil {
			t.Fatalf("decode sidecar payload: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusAccepted)
		_, _ = w.Write([]byte(`{"status":"accepted"}`))
	}))
	defer sidecar.Close()
	t.Setenv("MULTICA_WORKFLOW_SIDECAR_URL", sidecar.URL)

	req := newRequest("POST", "/api/issues/"+issueID+"/runtime-workflows?workspace_id="+testWorkspaceID, validRuntimeWorkflowSubmitBody())
	req = withURLParam(req, "id", issueID)
	w := httptest.NewRecorder()

	testHandler.SubmitRuntimeWorkflow(w, req)

	if w.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d: %s", w.Code, w.Body.String())
	}
	var resp WorkflowRunResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.ID == "" || resp.Status != "running" || resp.CurrentNode != "START" {
		t.Fatalf("unexpected workflow response: %+v", resp)
	}
	if got := sidecarPayload["run_id"]; got != resp.ID {
		t.Fatalf("sidecar run_id = %v, want %s", got, resp.ID)
	}
	if got := sidecarPayload["root_issue_id"]; got != issueID {
		t.Fatalf("sidecar root_issue_id = %v, want %s", got, issueID)
	}
	if _, ok := sidecarPayload["definition"].(map[string]any); !ok {
		t.Fatalf("sidecar definition missing or wrong type: %#v", sidecarPayload["definition"])
	}
}

func validRuntimeWorkflowSubmitBody() map[string]any {
	return map[string]any{
		"initial_state": map[string]any{"task": "implement runtime workflow"},
		"definition": map[string]any{
			"meta": map[string]any{"name": "valid runtime workflow", "version": "1"},
			"source_skills": []map[string]any{
				{"id": "skill-1", "name": "implementation"},
			},
			"state": map[string]any{
				"fields": []map[string]any{{"name": "task", "type": "string"}},
			},
			"nodes": []map[string]any{
				{
					"id":                     "implement",
					"type":                   "agent",
					"dispatch":               "subissue",
					"agent":                  "code",
					"source_skill_id":        "skill-1",
					"source_skill_name":      "implementation",
					"source_node_id":         "implement",
					"modified_from_template": false,
					"config":                 map[string]any{"system": "implement the request"},
					"outputs":                []string{"implementation_summary"},
				},
			},
			"routing": []map[string]any{
				{"from": "START", "to": "implement"},
				{"from": "implement", "to": "END"},
			},
		},
	}
}

func TestEnqueueWorkflowPlannerTaskSetsContext(t *testing.T) {
	ctx := context.Background()
	agentID := createHandlerTestAgent(t, "Workflow Planner Agent", nil)
	issueID := createIssueForTimeline(t, "workflow planner task issue")
	if _, err := testPool.Exec(ctx,
		`UPDATE issue SET assignee_type = 'agent', assignee_id = $1 WHERE id = $2`,
		agentID, issueID,
	); err != nil {
		t.Fatalf("assign issue: %v", err)
	}
	issue, err := testHandler.Queries.GetIssue(ctx, parseUUID(issueID))
	if err != nil {
		t.Fatalf("load issue: %v", err)
	}

	task, err := testHandler.TaskService.EnqueueWorkflowPlannerTask(ctx, issue, parseUUID(agentID))
	if err != nil {
		t.Fatalf("enqueue workflow planner task: %v", err)
	}

	var contextRaw string
	if err := testPool.QueryRow(ctx, `SELECT context::text FROM agent_task_queue WHERE id = $1`, task.ID).Scan(&contextRaw); err != nil {
		t.Fatalf("query planner task context: %v", err)
	}
	if !strings.Contains(contextRaw, `"workflow_planner"`) {
		t.Fatalf("context = %s, want workflow_planner marker", contextRaw)
	}
}

func TestEnqueueWorkflowMainNodeTaskSetsContext(t *testing.T) {
	ctx := context.Background()
	agentID := createHandlerTestAgent(t, "Workflow Main Node Agent", nil)
	issueID := createIssueForTimeline(t, "workflow main node task issue")
	if _, err := testPool.Exec(ctx,
		`UPDATE issue SET assignee_type = 'agent', assignee_id = $1 WHERE id = $2`,
		agentID, issueID,
	); err != nil {
		t.Fatalf("assign issue: %v", err)
	}
	issue, err := testHandler.Queries.GetIssue(ctx, parseUUID(issueID))
	if err != nil {
		t.Fatalf("load issue: %v", err)
	}
	var runID string
	if err := testPool.QueryRow(ctx, `
		INSERT INTO workflow_run (workspace_id, root_issue_id, status, current_node, nodes_state, definition_snapshot)
		VALUES ($1, $2, 'running', 'final', '{}'::jsonb, '{"meta":{"name":"main node"}}'::jsonb)
		RETURNING id
	`, testWorkspaceID, issueID).Scan(&runID); err != nil {
		t.Fatalf("create workflow run: %v", err)
	}

	task, err := testHandler.TaskService.EnqueueWorkflowMainNodeTask(ctx, issue, parseUUID(agentID), parseUUID(runID), "final", "final_response")
	if err != nil {
		t.Fatalf("enqueue workflow main node task: %v", err)
	}

	var contextRaw string
	if err := testPool.QueryRow(ctx, `SELECT context::text FROM agent_task_queue WHERE id = $1`, task.ID).Scan(&contextRaw); err != nil {
		t.Fatalf("query main node task context: %v", err)
	}
	for _, want := range []string{`"workflow_main_node"`, `"workflow_run_id": "` + runID + `"`, `"node_id": "final"`, `"node_type": "final_response"`} {
		if !strings.Contains(contextRaw, want) {
			t.Fatalf("context = %s, missing %s", contextRaw, want)
		}
	}
}
