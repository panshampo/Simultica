package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// seedReviewRunForTest creates a workflow_run whose definition snapshot contains
// a human_review node, plus a workflow_run_node row in the given status. It
// returns the run id. Cleanup is registered on the case created by the caller.
func seedReviewRunForTest(t *testing.T, caseID, nodeID, nodeStatus string) string {
	t.Helper()
	ctx := context.Background()
	snapshot := `{"meta":{"name":"review-flow","version":"1"},"nodes":[{"id":"` + nodeID + `","type":"human_review","dispatch":"human_gate"}]}`
	nodesState := `{"` + nodeID + `":{"status":"` + nodeStatus + `","input":{"proposed_fix":"change builder"}}}`
	var runID string
	if err := testPool.QueryRow(ctx, `
		INSERT INTO workflow_run (
			workspace_id, root_issue_id, case_id, status, current_node,
			nodes_state, definition_snapshot, source_skills
		)
		VALUES ($1, NULL, $2, 'waiting_for_review', $3, $4::jsonb, $5::jsonb, '[]'::jsonb)
		RETURNING id
	`, testWorkspaceID, caseID, nodeID, nodesState, snapshot).Scan(&runID); err != nil {
		t.Fatalf("seed review workflow_run: %v", err)
	}
	if _, err := testPool.Exec(ctx, `
		INSERT INTO workflow_run_node (
			workspace_id, case_id, run_id, node_id, node_type, dispatch, carrier_kind, status
		)
		VALUES ($1, $2, $3, $4, 'human_review', 'human_gate', 'inline', $5)
	`, testWorkspaceID, caseID, runID, nodeID, nodeStatus); err != nil {
		t.Fatalf("seed review workflow_run_node: %v", err)
	}
	return runID
}

func reviewRequest(caseID, runID, stepID string, body map[string]any) *http.Request {
	req := newRequest(http.MethodPost,
		"/api/workflow-cases/"+caseID+"/runs/"+runID+"/steps/"+stepID+"/review?workspace_id="+testWorkspaceID,
		body,
	)
	req = withURLParam(req, "caseId", caseID)
	req = withURLParam(req, "runId", runID)
	req = withURLParam(req, "stepId", stepID)
	return req
}

func TestReviewWorkflowRunStepApprovesPendingReviewStep(t *testing.T) {
	caseID := createWorkflowCaseForTest(t, "review approve")
	runID := seedReviewRunForTest(t, caseID, "review_before_mutation", "pending_review")

	req := reviewRequest(caseID, runID, "review_before_mutation", map[string]any{
		"decision": "approved",
		"comment":  "safe to continue",
	})
	rec := httptest.NewRecorder()
	testHandler.ReviewWorkflowRunStep(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp["decision"] != "approved" {
		t.Fatalf("decision = %v, want approved", resp["decision"])
	}
	if resp["reviewed_by"] == "" || resp["reviewed_by"] == nil {
		t.Fatalf("reviewed_by missing: %v", resp["reviewed_by"])
	}

	// The node output snapshot must carry the decision for the sidecar to resume.
	var nodesStateRaw []byte
	if err := testPool.QueryRow(context.Background(),
		`SELECT nodes_state FROM workflow_run WHERE id = $1`, runID).Scan(&nodesStateRaw); err != nil {
		t.Fatalf("query nodes_state: %v", err)
	}
	var nodesState map[string]map[string]any
	if err := json.Unmarshal(nodesStateRaw, &nodesState); err != nil {
		t.Fatalf("decode nodes_state: %v", err)
	}
	node := nodesState["review_before_mutation"]
	if node["status"] != "done" {
		t.Fatalf("node status = %v, want done", node["status"])
	}
	output, _ := node["output"].(map[string]any)
	if output["review_decision"] != "approved" {
		t.Fatalf("output.review_decision = %v, want approved", output["review_decision"])
	}
}

func TestReviewWorkflowRunStepRejectsNonPendingStep(t *testing.T) {
	caseID := createWorkflowCaseForTest(t, "review non-pending")
	runID := seedReviewRunForTest(t, caseID, "review_before_mutation", "succeeded")

	req := reviewRequest(caseID, runID, "review_before_mutation", map[string]any{"decision": "approved"})
	rec := httptest.NewRecorder()
	testHandler.ReviewWorkflowRunStep(rec, req)

	if rec.Code != http.StatusConflict {
		t.Fatalf("status = %d, want 409, body = %s", rec.Code, rec.Body.String())
	}
}

func TestReviewWorkflowRunStepRejectsInvalidDecision(t *testing.T) {
	caseID := createWorkflowCaseForTest(t, "review bad decision")
	runID := seedReviewRunForTest(t, caseID, "review_before_mutation", "pending_review")

	req := reviewRequest(caseID, runID, "review_before_mutation", map[string]any{"decision": "maybe"})
	rec := httptest.NewRecorder()
	testHandler.ReviewWorkflowRunStep(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400, body = %s", rec.Code, rec.Body.String())
	}
}

func TestReviewWorkflowRunStepRejectsUnknownStep(t *testing.T) {
	caseID := createWorkflowCaseForTest(t, "review unknown step")
	runID := seedReviewRunForTest(t, caseID, "review_before_mutation", "pending_review")

	req := reviewRequest(caseID, runID, "not_a_step", map[string]any{"decision": "approved"})
	rec := httptest.NewRecorder()
	testHandler.ReviewWorkflowRunStep(rec, req)

	if rec.Code != http.StatusConflict && rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404/409, body = %s", rec.Code, rec.Body.String())
	}
}

func TestReviewWorkflowRunStepRejectsRunOutsideCase(t *testing.T) {
	caseID := createWorkflowCaseForTest(t, "review scope a")
	otherCaseID := createWorkflowCaseForTest(t, "review scope b")
	runID := seedReviewRunForTest(t, caseID, "review_before_mutation", "pending_review")

	// Try to review the run through a different case id.
	req := reviewRequest(otherCaseID, runID, "review_before_mutation", map[string]any{"decision": "approved"})
	rec := httptest.NewRecorder()
	testHandler.ReviewWorkflowRunStep(rec, req)

	if rec.Code != http.StatusForbidden && rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 403/404, body = %s", rec.Code, rec.Body.String())
	}
}

func TestReviewWorkflowRunStepRejectsReject(t *testing.T) {
	caseID := createWorkflowCaseForTest(t, "review reject")
	runID := seedReviewRunForTest(t, caseID, "review_before_mutation", "pending_review")

	req := reviewRequest(caseID, runID, "review_before_mutation", map[string]any{"decision": "rejected", "comment": "no"})
	rec := httptest.NewRecorder()
	testHandler.ReviewWorkflowRunStep(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &resp)
	if resp["decision"] != "rejected" {
		t.Fatalf("decision = %v, want rejected", resp["decision"])
	}
}

func TestGetWorkflowRunStepReviewReportsPending(t *testing.T) {
	caseID := createWorkflowCaseForTest(t, "review get pending")
	runID := seedReviewRunForTest(t, caseID, "review_before_mutation", "pending_review")

	req := newRequest(http.MethodGet,
		"/api/workflow-cases/"+caseID+"/runs/"+runID+"/steps/review_before_mutation/review?workspace_id="+testWorkspaceID,
		nil,
	)
	req = withURLParam(req, "caseId", caseID)
	req = withURLParam(req, "runId", runID)
	req = withURLParam(req, "stepId", "review_before_mutation")
	rec := httptest.NewRecorder()
	testHandler.GetWorkflowRunStepReview(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp["status"] != "pending_review" {
		t.Fatalf("status = %v, want pending_review", resp["status"])
	}
	if resp["decision"] != nil && resp["decision"] != "" {
		t.Fatalf("decision should be empty while pending, got %v", resp["decision"])
	}
}

func TestGetWorkflowRunStepReviewReportsDecisionAfterApprove(t *testing.T) {
	caseID := createWorkflowCaseForTest(t, "review get decided")
	runID := seedReviewRunForTest(t, caseID, "review_before_mutation", "pending_review")

	approve := reviewRequest(caseID, runID, "review_before_mutation", map[string]any{"decision": "approved", "comment": "ok"})
	approveRec := httptest.NewRecorder()
	testHandler.ReviewWorkflowRunStep(approveRec, approve)
	if approveRec.Code != http.StatusOK {
		t.Fatalf("approve status = %d, body = %s", approveRec.Code, approveRec.Body.String())
	}

	req := newRequest(http.MethodGet,
		"/api/workflow-cases/"+caseID+"/runs/"+runID+"/steps/review_before_mutation/review?workspace_id="+testWorkspaceID,
		nil,
	)
	req = withURLParam(req, "caseId", caseID)
	req = withURLParam(req, "runId", runID)
	req = withURLParam(req, "stepId", "review_before_mutation")
	rec := httptest.NewRecorder()
	testHandler.GetWorkflowRunStepReview(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp["decision"] != "approved" {
		t.Fatalf("decision = %v, want approved", resp["decision"])
	}
}
