package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
)

// TestReviewStepEndToEndLoop exercises the full Review Step loop against the real
// database and real router, simulating the sidecar <-> backend interaction:
//
//  1. sidecar requests review  -> POST node event node_review_requested
//     => node status becomes pending_review, run reported waiting_for_review
//  2. sidecar polls decision   -> GET .../review returns pending_review
//  3. human approves           -> POST .../review returns approved
//  4. sidecar polls decision   -> GET .../review returns approved
//     => run resumes to running, nodes_state carries the decision
//
// This is the end-to-end smoke for the review-step closed loop on simultica_dev.
func TestReviewStepEndToEndLoop(t *testing.T) {
	ctx := context.Background()
	caseID := createWorkflowCaseForTest(t, "review e2e loop")

	// Seed a run + review node in the pre-review state the sidecar would create:
	// definition has the human_review node, node row exists as running.
	snapshot := `{"meta":{"name":"review-e2e","version":"1"},"nodes":[{"id":"review_before_mutation","type":"human_review","dispatch":"human_gate"}],"routing":[{"from":"START","to":"review_before_mutation"},{"from":"review_before_mutation","condition":"review_decision == \"approved\"","to":"END","else":"END"}]}`
	var runID string
	if err := testPool.QueryRow(ctx, `
		INSERT INTO workflow_run (
			workspace_id, root_issue_id, case_id, status, current_node,
			nodes_state, definition_snapshot, source_skills
		)
		VALUES ($1, NULL, $2, 'running', 'review_before_mutation', '{}'::jsonb, $3::jsonb, '[]'::jsonb)
		RETURNING id
	`, testWorkspaceID, caseID, snapshot).Scan(&runID); err != nil {
		t.Fatalf("seed e2e run: %v", err)
	}
	if _, err := testPool.Exec(ctx, `
		INSERT INTO workflow_run_node (workspace_id, case_id, run_id, node_id, node_type, dispatch, carrier_kind, status)
		VALUES ($1, $2, $3, 'review_before_mutation', 'human_review', 'human_gate', 'inline', 'running')
	`, testWorkspaceID, caseID, runID); err != nil {
		t.Fatalf("seed e2e node: %v", err)
	}

	router := chi.NewRouter()
	router.Post("/api/workflow-runs/{runId}/nodes/{nodeId}/events", testHandler.CreateWorkflowRunNodeEvent)
	router.Post("/api/workflow-cases/{caseId}/runs/{runId}/steps/{stepId}/review", testHandler.ReviewWorkflowRunStep)
	router.Get("/api/workflow-cases/{caseId}/runs/{runId}/steps/{stepId}/review", testHandler.GetWorkflowRunStepReview)

	do := func(method, path string, body any) *httptest.ResponseRecorder {
		req := newRequest(method, path, body)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		return rec
	}

	// Step 1: sidecar requests review via node event.
	rec := do(http.MethodPost,
		"/api/workflow-runs/"+runID+"/nodes/review_before_mutation/events?workspace_id="+testWorkspaceID,
		map[string]any{"event_type": "node_review_requested", "attempt": 1, "input_snapshot": map[string]any{"proposed_fix": "change builder"}},
	)
	if rec.Code != http.StatusOK {
		t.Fatalf("request review: status = %d, body = %s", rec.Code, rec.Body.String())
	}

	// Node must now be pending_review.
	var nodeStatus string
	if err := testPool.QueryRow(ctx,
		`SELECT status FROM workflow_run_node WHERE run_id = $1 AND node_id = 'review_before_mutation'`, runID).Scan(&nodeStatus); err != nil {
		t.Fatalf("read node status: %v", err)
	}
	if nodeStatus != "pending_review" {
		t.Fatalf("node status after request = %q, want pending_review", nodeStatus)
	}

	// Step 2: sidecar polls decision -> still pending.
	rec = do(http.MethodGet,
		"/api/workflow-cases/"+caseID+"/runs/"+runID+"/steps/review_before_mutation/review?workspace_id="+testWorkspaceID, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("poll pending: status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var pollPending map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &pollPending)
	if pollPending["status"] != "pending_review" {
		t.Fatalf("poll status = %v, want pending_review", pollPending["status"])
	}
	if d, ok := pollPending["decision"]; ok && d != "" {
		t.Fatalf("decision present while pending: %v", d)
	}

	// Step 3: human approves.
	rec = do(http.MethodPost,
		"/api/workflow-cases/"+caseID+"/runs/"+runID+"/steps/review_before_mutation/review?workspace_id="+testWorkspaceID,
		map[string]any{"decision": "approved", "comment": "safe"})
	if rec.Code != http.StatusOK {
		t.Fatalf("approve: status = %d, body = %s", rec.Code, rec.Body.String())
	}

	// Step 4: sidecar polls decision -> approved.
	rec = do(http.MethodGet,
		"/api/workflow-cases/"+caseID+"/runs/"+runID+"/steps/review_before_mutation/review?workspace_id="+testWorkspaceID, nil)
	var pollDecided map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &pollDecided)
	if pollDecided["decision"] != "approved" {
		t.Fatalf("final poll decision = %v, want approved", pollDecided["decision"])
	}

	// The run resumed to running and nodes_state carries the decision.
	var runStatus string
	var nodesStateRaw []byte
	if err := testPool.QueryRow(ctx,
		`SELECT status, nodes_state FROM workflow_run WHERE id = $1`, runID).Scan(&runStatus, &nodesStateRaw); err != nil {
		t.Fatalf("read run: %v", err)
	}
	if runStatus != "running" {
		t.Fatalf("run status after approve = %q, want running", runStatus)
	}
	var nodesState map[string]map[string]any
	_ = json.Unmarshal(nodesStateRaw, &nodesState)
	output, _ := nodesState["review_before_mutation"]["output"].(map[string]any)
	if output["review_decision"] != "approved" {
		t.Fatalf("nodes_state decision = %v, want approved", output["review_decision"])
	}
}
