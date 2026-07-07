package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCancelWorkflowCaseRunCancelsOwnedRun(t *testing.T) {
	ctx := context.Background()
	caseID := createWorkflowCaseForTest(t, "case cancel owned run")
	versionID := createWorkflowDefinitionVersionFixture(t, ctx, caseID, workflowCaseConfirmDefinition())
	var runID string
	if err := testPool.QueryRow(ctx, `
		INSERT INTO workflow_run (
			workspace_id, case_id, definition_version_id, status, current_node,
			nodes_state, definition_snapshot, source_skills
		)
		VALUES ($1, $2, $3, 'running', 'prepare', '{}'::jsonb, '{"meta":{"name":"cancel owned"}}'::jsonb, '[]'::jsonb)
		RETURNING id
	`, testWorkspaceID, caseID, versionID).Scan(&runID); err != nil {
		t.Fatalf("seed workflow_run: %v", err)
	}

	req := newRequest(http.MethodPost, "/api/workflow-cases/"+caseID+"/runs/"+runID+"/cancel?workspace_id="+testWorkspaceID, map[string]any{
		"reason": "case-scoped stop",
	})
	req = withURLParams(req, "caseId", caseID, "runId", runID)
	rec := httptest.NewRecorder()

	testHandler.CancelWorkflowCaseRun(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp WorkflowRunResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode run: %v", err)
	}
	if resp.Status != "cancelled" {
		t.Fatalf("status = %q, want cancelled", resp.Status)
	}
	if resp.CancelReason == nil || *resp.CancelReason != "case-scoped stop" {
		t.Fatalf("cancel_reason = %v, want case-scoped stop", resp.CancelReason)
	}

	// Cancelling a run must not project run status onto the case.
	updatedCase, err := testHandler.Queries.GetWorkflowCase(ctx, parseUUID(caseID))
	if err != nil {
		t.Fatalf("load workflow case: %v", err)
	}
	if updatedCase.Status == "cancelled" {
		t.Fatalf("case status = %q, cancel must not project run status onto case", updatedCase.Status)
	}
}

func TestCancelWorkflowCaseRunRejectsRunOutsideCase(t *testing.T) {
	ctx := context.Background()
	ownerCaseID := createWorkflowCaseForTest(t, "case cancel owner")
	otherCaseID := createWorkflowCaseForTest(t, "case cancel other")
	versionID := createWorkflowDefinitionVersionFixture(t, ctx, ownerCaseID, workflowCaseConfirmDefinition())
	var runID string
	if err := testPool.QueryRow(ctx, `
		INSERT INTO workflow_run (
			workspace_id, case_id, definition_version_id, status, current_node,
			nodes_state, definition_snapshot, source_skills
		)
		VALUES ($1, $2, $3, 'running', 'prepare', '{}'::jsonb, '{"meta":{"name":"cancel outside"}}'::jsonb, '[]'::jsonb)
		RETURNING id
	`, testWorkspaceID, ownerCaseID, versionID).Scan(&runID); err != nil {
		t.Fatalf("seed workflow_run: %v", err)
	}

	req := newRequest(http.MethodPost, "/api/workflow-cases/"+otherCaseID+"/runs/"+runID+"/cancel?workspace_id="+testWorkspaceID, map[string]any{
		"reason": "wrong case",
	})
	req = withURLParams(req, "caseId", otherCaseID, "runId", runID)
	rec := httptest.NewRecorder()

	testHandler.CancelWorkflowCaseRun(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d: %s", rec.Code, rec.Body.String())
	}
	var status string
	if err := testPool.QueryRow(ctx, `SELECT status FROM workflow_run WHERE id = $1`, runID).Scan(&status); err != nil {
		t.Fatalf("load workflow_run status: %v", err)
	}
	if status != "running" {
		t.Fatalf("status = %q, want running", status)
	}
}
