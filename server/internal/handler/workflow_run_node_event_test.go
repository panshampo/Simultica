package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

func TestWorkflowRunNodeEventProjectsCurrentStateAndNodesState(t *testing.T) {
	ctx := context.Background()
	runID := createWorkflowRunNodeEventFixture(t, "node event started")

	req := newRequest("POST", "/api/workflow-runs/"+runID+"/nodes/implement/events?workspace_id="+testWorkspaceID, map[string]any{
		"event_type": "node_started",
		"attempt":    1,
		"logs": []map[string]any{
			{"message": "started"},
		},
	})
	req = withURLParam(req, "runId", runID)
	req = withURLParam(req, "nodeId", "implement")
	w := httptest.NewRecorder()

	testHandler.CreateWorkflowRunNodeEvent(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var resp WorkflowRunResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(resp.Nodes) != 1 || resp.Nodes[0].NodeID != "implement" || resp.Nodes[0].Status != "running" {
		t.Fatalf("unexpected response nodes: %+v", resp.Nodes)
	}
	if resp.Nodes[0].RunID != runID {
		t.Fatalf("response node run_id = %q, want %s", resp.Nodes[0].RunID, runID)
	}

	var nodeStatus string
	if err := testPool.QueryRow(ctx, `
		SELECT status
		FROM workflow_run_node
		WHERE run_id = $1 AND node_id = 'implement'
	`, runID).Scan(&nodeStatus); err != nil {
		t.Fatalf("query workflow_run_node: %v", err)
	}
	if nodeStatus != "running" {
		t.Fatalf("workflow_run_node.status = %q, want running", nodeStatus)
	}

	var nodesStateRaw []byte
	if err := testPool.QueryRow(ctx, `SELECT nodes_state FROM workflow_run WHERE id = $1`, runID).Scan(&nodesStateRaw); err != nil {
		t.Fatalf("query nodes_state: %v", err)
	}
	var nodesState map[string]map[string]any
	if err := json.Unmarshal(nodesStateRaw, &nodesState); err != nil {
		t.Fatalf("decode nodes_state: %v", err)
	}
	if nodesState["implement"]["status"] != "running" {
		t.Fatalf("nodes_state implement = %#v, want running", nodesState["implement"])
	}

	var eventCount int
	if err := testPool.QueryRow(ctx, `
		SELECT count(*)
		FROM workflow_run_node_event
		WHERE run_id = $1 AND node_id = 'implement' AND event_type = 'node_started'
	`, runID).Scan(&eventCount); err != nil {
		t.Fatalf("count node events: %v", err)
	}
	if eventCount != 1 {
		t.Fatalf("event count = %d, want 1", eventCount)
	}
}

func TestWorkflowRunNodeSucceededMapsLegacyDone(t *testing.T) {
	ctx := context.Background()
	runID := createWorkflowRunNodeEventFixture(t, "node event succeeded")

	req := newRequest("POST", "/api/workflow-runs/"+runID+"/nodes/implement/events?workspace_id="+testWorkspaceID, map[string]any{
		"event_type":      "node_succeeded",
		"attempt":         1,
		"output_snapshot": map[string]any{"summary": "ok"},
	})
	req = withURLParam(req, "runId", runID)
	req = withURLParam(req, "nodeId", "implement")
	w := httptest.NewRecorder()

	testHandler.CreateWorkflowRunNodeEvent(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var nodeStatus string
	var nodesStateRaw []byte
	if err := testPool.QueryRow(ctx, `
		SELECT n.status, r.nodes_state
		FROM workflow_run_node n
		JOIN workflow_run r ON r.id = n.run_id
		WHERE n.run_id = $1 AND n.node_id = 'implement'
	`, runID).Scan(&nodeStatus, &nodesStateRaw); err != nil {
		t.Fatalf("query projected state: %v", err)
	}
	if nodeStatus != "succeeded" {
		t.Fatalf("workflow_run_node.status = %q, want succeeded", nodeStatus)
	}
	var nodesState map[string]map[string]any
	if err := json.Unmarshal(nodesStateRaw, &nodesState); err != nil {
		t.Fatalf("decode nodes_state: %v", err)
	}
	if nodesState["implement"]["status"] != "done" {
		t.Fatalf("legacy nodes_state status = %#v, want done", nodesState["implement"])
	}
}

func TestWorkflowRunNodeEventAcceptsOutputSnapshotWithoutLogs(t *testing.T) {
	ctx := context.Background()
	runID := createWorkflowRunNodeEventFixture(t, "node event output snapshot")

	req := newRequest("POST", "/api/workflow-runs/"+runID+"/nodes/implement/events?workspace_id="+testWorkspaceID, map[string]any{
		"event_type":      "node_succeeded",
		"attempt":         1,
		"output_snapshot": map[string]any{"summary": "ok"},
	})
	req = withURLParam(req, "runId", runID)
	req = withURLParam(req, "nodeId", "implement")
	w := httptest.NewRecorder()

	testHandler.CreateWorkflowRunNodeEvent(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var outputRaw []byte
	if err := testPool.QueryRow(ctx, `
		SELECT output_snapshot
		FROM workflow_run_node_event
		WHERE run_id = $1 AND node_id = 'implement' AND event_type = 'node_succeeded'
	`, runID).Scan(&outputRaw); err != nil {
		t.Fatalf("query workflow_run_node_event output_snapshot: %v", err)
	}
	var output map[string]any
	if err := json.Unmarshal(outputRaw, &output); err != nil {
		t.Fatalf("decode output_snapshot: %v", err)
	}
	if output["summary"] != "ok" {
		t.Fatalf("output_snapshot = %#v, want summary ok", output)
	}
}

func TestWorkflowRunNodeEventRejectsCaseOutsideWorkspace(t *testing.T) {
	ctx := context.Background()
	runID := createWorkflowRunNodeEventFixture(t, "node event foreign case")
	foreignCaseID := createForeignWorkflowCaseForNodeEvent(t)
	if _, err := testPool.Exec(ctx, `UPDATE workflow_run SET case_id = $1 WHERE id = $2`, foreignCaseID, runID); err != nil {
		t.Fatalf("point workflow_run at foreign case: %v", err)
	}

	req := newRequest("POST", "/api/workflow-runs/"+runID+"/nodes/implement/events?workspace_id="+testWorkspaceID, map[string]any{
		"event_type": "node_started",
		"attempt":    1,
	})
	req = withURLParam(req, "runId", runID)
	req = withURLParam(req, "nodeId", "implement")
	w := httptest.NewRecorder()

	testHandler.CreateWorkflowRunNodeEvent(w, req)

	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d: %s", w.Code, w.Body.String())
	}
	assertWorkflowRunNodeEventCount(t, runID, "implement", 0)
}

func TestWorkflowRunNodeEventRejectsInvalidNodeWithoutAppend(t *testing.T) {
	runID := createWorkflowRunNodeEventFixture(t, "node event invalid node")

	req := newRequest("POST", "/api/workflow-runs/"+runID+"/nodes/missing/events?workspace_id="+testWorkspaceID, map[string]any{
		"event_type": "node_started",
		"attempt":    1,
	})
	req = withURLParam(req, "runId", runID)
	req = withURLParam(req, "nodeId", "missing")
	w := httptest.NewRecorder()

	testHandler.CreateWorkflowRunNodeEvent(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
	assertWorkflowRunNodeEventCount(t, runID, "missing", 0)
}

func TestWorkflowRunNodeCarrierAttachedProjectsLegacyCarrierRefs(t *testing.T) {
	ctx := context.Background()
	runID := createWorkflowRunNodeEventFixture(t, "node event carrier attached")
	if _, err := testPool.Exec(ctx, `
		UPDATE workflow_run
		SET nodes_state = '{"implement":{"status":"running"}}'::jsonb
		WHERE id = $1
	`, runID); err != nil {
		t.Fatalf("seed running nodes_state: %v", err)
	}
	carrierRef := map[string]any{
		"type":         "subissue",
		"sub_issue_id": "11111111-1111-1111-1111-111111111111",
		"task_id":      "22222222-2222-2222-2222-222222222222",
	}

	req := newRequest("POST", "/api/workflow-runs/"+runID+"/nodes/implement/events?workspace_id="+testWorkspaceID, map[string]any{
		"event_type":  "node_carrier_attached",
		"attempt":     1,
		"carrier_ref": carrierRef,
	})
	req = withURLParam(req, "runId", runID)
	req = withURLParam(req, "nodeId", "implement")
	w := httptest.NewRecorder()

	testHandler.CreateWorkflowRunNodeEvent(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var nodesState map[string]map[string]any
	readWorkflowRunNodesState(t, runID, &nodesState)
	node := nodesState["implement"]
	if node["status"] != "running" {
		t.Fatalf("status = %#v, want running", node["status"])
	}
	if node["sub_issue_id"] != carrierRef["sub_issue_id"] || node["task_id"] != carrierRef["task_id"] {
		t.Fatalf("legacy carrier ids not projected: %#v", node)
	}
	if _, ok := node["carrier_ref"].(map[string]any); !ok {
		t.Fatalf("carrier_ref not projected into nodes_state: %#v", node)
	}

	var rawCarrier []byte
	if err := testPool.QueryRow(ctx, `
		SELECT carrier_ref
		FROM workflow_run_node
		WHERE run_id = $1 AND node_id = 'implement'
	`, runID).Scan(&rawCarrier); err != nil {
		t.Fatalf("query workflow_run_node carrier_ref: %v", err)
	}
	var projected map[string]any
	if err := json.Unmarshal(rawCarrier, &projected); err != nil {
		t.Fatalf("decode workflow_run_node carrier_ref: %v", err)
	}
	if projected["type"] != "subissue" {
		t.Fatalf("workflow_run_node carrier_ref = %#v, want subissue", projected)
	}
}

func TestWorkflowRunNodeLogAppendedPreservesStatusAndAppendsLogs(t *testing.T) {
	ctx := context.Background()
	runID := createWorkflowRunNodeEventFixture(t, "node event log appended")
	if _, err := testPool.Exec(ctx, `
		UPDATE workflow_run
		SET nodes_state = '{"implement":{"status":"running","logs":[{"message":"existing"}]}}'::jsonb
		WHERE id = $1
	`, runID); err != nil {
		t.Fatalf("seed running nodes_state: %v", err)
	}
	if _, err := testPool.Exec(ctx, `
		UPDATE workflow_run_node
		SET status = 'running', logs = '[{"message":"existing"}]'::jsonb
		WHERE run_id = $1 AND node_id = 'implement'
	`, runID); err != nil {
		t.Fatalf("seed running workflow_run_node: %v", err)
	}

	req := newRequest("POST", "/api/workflow-runs/"+runID+"/nodes/implement/events?workspace_id="+testWorkspaceID, map[string]any{
		"event_type": "node_log_appended",
		"attempt":    1,
		"logs": []map[string]any{
			{"message": "new"},
		},
	})
	req = withURLParam(req, "runId", runID)
	req = withURLParam(req, "nodeId", "implement")
	w := httptest.NewRecorder()

	testHandler.CreateWorkflowRunNodeEvent(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var nodesState map[string]struct {
		Status string           `json:"status"`
		Logs   []map[string]any `json:"logs"`
	}
	readWorkflowRunNodesState(t, runID, &nodesState)
	if nodesState["implement"].Status != "running" {
		t.Fatalf("status = %q, want running", nodesState["implement"].Status)
	}
	if got := len(nodesState["implement"].Logs); got != 2 {
		t.Fatalf("legacy logs length = %d, want 2: %#v", got, nodesState["implement"].Logs)
	}

	var nodeStatus string
	var nodeLogs []byte
	if err := testPool.QueryRow(ctx, `
		SELECT status, logs
		FROM workflow_run_node
		WHERE run_id = $1 AND node_id = 'implement'
	`, runID).Scan(&nodeStatus, &nodeLogs); err != nil {
		t.Fatalf("query workflow_run_node: %v", err)
	}
	if nodeStatus != "running" {
		t.Fatalf("workflow_run_node.status = %q, want running", nodeStatus)
	}
	var logs []map[string]any
	if err := json.Unmarshal(nodeLogs, &logs); err != nil {
		t.Fatalf("decode workflow_run_node logs: %v", err)
	}
	if len(logs) != 2 {
		t.Fatalf("workflow_run_node logs length = %d, want 2: %#v", len(logs), logs)
	}
}

func TestWorkflowRunNodeEventMergesNodesStateAfterConcurrentUpdate(t *testing.T) {
	ctx := context.Background()
	runID := createWorkflowRunNodeEventFixture(t, "node event concurrent nodes state")

	tx, err := testPool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin lock transaction: %v", err)
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, `SELECT id FROM workflow_run WHERE id = $1 FOR UPDATE`, runID); err != nil {
		t.Fatalf("lock workflow_run: %v", err)
	}

	done := make(chan *httptest.ResponseRecorder, 1)
	go func() {
		req := newRequest("POST", "/api/workflow-runs/"+runID+"/nodes/implement/events?workspace_id="+testWorkspaceID, map[string]any{
			"event_type": "node_succeeded",
			"attempt":    1,
		})
		req = withURLParam(req, "runId", runID)
		req = withURLParam(req, "nodeId", "implement")
		w := httptest.NewRecorder()
		testHandler.CreateWorkflowRunNodeEvent(w, req)
		done <- w
	}()

	time.Sleep(100 * time.Millisecond)
	if _, err := tx.Exec(ctx, `
		UPDATE workflow_run
		SET nodes_state = jsonb_set(nodes_state, '{review}', '{"status":"running"}'::jsonb, true)
		WHERE id = $1
	`, runID); err != nil {
		t.Fatalf("apply concurrent nodes_state update: %v", err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatalf("commit concurrent nodes_state update: %v", err)
	}

	select {
	case w := <-done:
		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
		}
	case <-time.After(5 * time.Second):
		t.Fatal("node event request did not finish")
	}

	var nodesState map[string]map[string]any
	readWorkflowRunNodesState(t, runID, &nodesState)
	if nodesState["implement"]["status"] != "done" {
		t.Fatalf("implement status = %#v, want done in %#v", nodesState["implement"]["status"], nodesState)
	}
	if nodesState["review"]["status"] != "running" {
		t.Fatalf("concurrent review node was lost: %#v", nodesState)
	}
}

func createWorkflowRunNodeEventFixture(t *testing.T, title string) string {
	t.Helper()
	ctx := context.Background()
	queries := db.New(testPool)
	workspaceID := parseUUID(testWorkspaceID)

	workflowCase, err := queries.CreateWorkflowCase(ctx, db.CreateWorkflowCaseParams{
		WorkspaceID: workspaceID,
		Title:       title,
		Description: "node event fixture",
		Status:      "running",
		CreatedBy:   pgtype.Text{String: "test", Valid: true},
		UpdatedBy:   pgtype.Text{String: "test", Valid: true},
	})
	if err != nil {
		t.Fatalf("create workflow_case: %v", err)
	}
	t.Cleanup(func() {
		_, _ = testPool.Exec(ctx, `DELETE FROM workflow_case WHERE id = $1`, workflowCase.ID)
	})

	run, err := queries.CreateWorkflowRun(ctx, db.CreateWorkflowRunParams{
		WorkspaceID:        workspaceID,
		Status:             "running",
		CurrentNode:        "implement",
		NodesState:         []byte(`{"implement":{"status":"pending"}}`),
		DefinitionSnapshot: []byte(`{"nodes":[{"id":"implement","type":"llm","dispatch":"inline"}]}`),
		SourceSkills:       []byte(`[]`),
		CaseID:             workflowCase.ID,
	})
	if err != nil {
		t.Fatalf("create workflow_run: %v", err)
	}
	if _, err := queries.CreateWorkflowRunNode(ctx, db.CreateWorkflowRunNodeParams{
		WorkspaceID: workspaceID,
		CaseID:      workflowCase.ID,
		RunID:       run.ID,
		NodeID:      "implement",
		NodeType:    "llm",
		Dispatch:    "inline",
		CarrierKind: "inline",
		Status:      "pending",
	}); err != nil {
		t.Fatalf("create workflow_run_node: %v", err)
	}
	return uuidToString(run.ID)
}

func createForeignWorkflowCaseForNodeEvent(t *testing.T) string {
	t.Helper()
	ctx := context.Background()
	var workspaceID string
	if err := testPool.QueryRow(ctx, `
		INSERT INTO workspace (name, slug, description, issue_prefix)
		VALUES ($1, $2, $3, $4)
		RETURNING id
	`, "Node Event Foreign Workspace", "node-event-foreign-"+time.Now().Format("150405.000000000"), "foreign", "NEF").Scan(&workspaceID); err != nil {
		t.Fatalf("create foreign workspace: %v", err)
	}
	t.Cleanup(func() {
		_, _ = testPool.Exec(ctx, `DELETE FROM workspace WHERE id = $1`, workspaceID)
	})

	queries := db.New(testPool)
	workflowCase, err := queries.CreateWorkflowCase(ctx, db.CreateWorkflowCaseParams{
		WorkspaceID: parseUUID(workspaceID),
		Title:       "foreign node event case",
		Description: "foreign case",
		Status:      "running",
		CreatedBy:   pgtype.Text{String: "test", Valid: true},
		UpdatedBy:   pgtype.Text{String: "test", Valid: true},
	})
	if err != nil {
		t.Fatalf("create foreign workflow_case: %v", err)
	}
	return uuidToString(workflowCase.ID)
}

func assertWorkflowRunNodeEventCount(t *testing.T, runID, nodeID string, want int) {
	t.Helper()
	var got int
	if err := testPool.QueryRow(context.Background(), `
		SELECT count(*)
		FROM workflow_run_node_event
		WHERE run_id = $1 AND node_id = $2
	`, runID, nodeID).Scan(&got); err != nil {
		t.Fatalf("count workflow_run_node_event: %v", err)
	}
	if got != want {
		t.Fatalf("workflow_run_node_event count = %d, want %d", got, want)
	}
}

func readWorkflowRunNodesState(t *testing.T, runID string, dest any) {
	t.Helper()
	var raw []byte
	if err := testPool.QueryRow(context.Background(), `SELECT nodes_state FROM workflow_run WHERE id = $1`, runID).Scan(&raw); err != nil {
		t.Fatalf("query nodes_state: %v", err)
	}
	if err := json.Unmarshal(raw, dest); err != nil {
		t.Fatalf("decode nodes_state: %v", err)
	}
}
