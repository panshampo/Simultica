package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestDeleteWorkflowCaseCascadesAndDetachesCarrierIssues(t *testing.T) {
	ctx := context.Background()
	caseID := createWorkflowCaseForTest(t, "delete cascade case")
	versionID := createWorkflowDefinitionVersionFixture(t, ctx, caseID, workflowCaseConfirmDefinition())

	var runID string
	if err := testPool.QueryRow(ctx, `
		INSERT INTO workflow_run (
			workspace_id, case_id, definition_version_id, status, current_node,
			nodes_state, definition_snapshot, source_skills
		)
		VALUES ($1, $2, $3, 'done', 'prepare', '{}'::jsonb, '{"meta":{"name":"del"}}'::jsonb, '[]'::jsonb)
		RETURNING id
	`, testWorkspaceID, caseID, versionID).Scan(&runID); err != nil {
		t.Fatalf("seed workflow_run: %v", err)
	}
	if _, err := testPool.Exec(ctx, `
		INSERT INTO workflow_run_node (workspace_id, case_id, run_id, node_id, node_type, dispatch, carrier_kind, status)
		VALUES ($1, $2, $3, 'prepare', 'condition', 'inline', 'inline', 'succeeded')
	`, testWorkspaceID, caseID, runID); err != nil {
		t.Fatalf("seed workflow_run_node: %v", err)
	}

	// A carrier sub-issue bound to the run via origin_type/origin_id (no FK).
	var carrierIssueID string
	if err := testPool.QueryRow(ctx, `
		INSERT INTO issue (workspace_id, title, status, priority, creator_type, creator_id, number, origin_type, origin_id)
		VALUES ($1, 'carrier sub-issue', 'todo', 'medium', 'member', $2,
		        COALESCE((SELECT MAX(number) FROM issue WHERE workspace_id = $1), 0) + 1,
		        'workflow_node', $3)
		RETURNING id
	`, testWorkspaceID, testUserID, runID).Scan(&carrierIssueID); err != nil {
		t.Fatalf("seed carrier issue: %v", err)
	}
	t.Cleanup(func() {
		testPool.Exec(context.Background(), `DELETE FROM issue WHERE id = $1`, carrierIssueID)
	})

	req := newRequest(http.MethodDelete, "/api/workflow-cases/"+caseID+"?workspace_id="+testWorkspaceID, nil)
	req = withURLParam(req, "caseId", caseID)
	rec := httptest.NewRecorder()

	testHandler.DeleteWorkflowCase(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d: %s", rec.Code, rec.Body.String())
	}

	var caseCount int
	if err := testPool.QueryRow(ctx, `SELECT count(*) FROM workflow_case WHERE id = $1`, caseID).Scan(&caseCount); err != nil {
		t.Fatalf("count case: %v", err)
	}
	if caseCount != 0 {
		t.Fatalf("workflow_case count = %d, want 0", caseCount)
	}
	var runCount, nodeCount, versionCount int
	if err := testPool.QueryRow(ctx, `SELECT count(*) FROM workflow_run WHERE case_id = $1`, caseID).Scan(&runCount); err != nil {
		t.Fatalf("count run: %v", err)
	}
	if err := testPool.QueryRow(ctx, `SELECT count(*) FROM workflow_run_node WHERE run_id = $1`, runID).Scan(&nodeCount); err != nil {
		t.Fatalf("count node: %v", err)
	}
	if err := testPool.QueryRow(ctx, `SELECT count(*) FROM workflow_definition_version WHERE case_id = $1`, caseID).Scan(&versionCount); err != nil {
		t.Fatalf("count version: %v", err)
	}
	if runCount != 0 || nodeCount != 0 || versionCount != 0 {
		t.Fatalf("cascade incomplete: runs=%d nodes=%d versions=%d", runCount, nodeCount, versionCount)
	}

	// Carrier issue must survive but be detached from its workflow binding.
	var stillExists int
	var originType, originID *string
	if err := testPool.QueryRow(ctx, `
		SELECT count(*) FROM issue WHERE id = $1
	`, carrierIssueID).Scan(&stillExists); err != nil {
		t.Fatalf("count carrier issue: %v", err)
	}
	if stillExists != 1 {
		t.Fatalf("carrier issue must survive case deletion, got count=%d", stillExists)
	}
	if err := testPool.QueryRow(ctx, `
		SELECT origin_type::text, origin_id::text FROM issue WHERE id = $1
	`, carrierIssueID).Scan(&originType, &originID); err != nil {
		t.Fatalf("load carrier issue origin: %v", err)
	}
	if originType != nil || originID != nil {
		t.Fatalf("carrier issue workflow binding must be cleared, got origin_type=%v origin_id=%v", originType, originID)
	}
}

func TestDeleteWorkflowCaseRejectsActiveRun(t *testing.T) {
	ctx := context.Background()
	caseID := createWorkflowCaseForTest(t, "delete active run case")
	versionID := createWorkflowDefinitionVersionFixture(t, ctx, caseID, workflowCaseConfirmDefinition())
	if _, err := testPool.Exec(ctx, `
		INSERT INTO workflow_run (
			workspace_id, case_id, definition_version_id, status, current_node,
			nodes_state, definition_snapshot, source_skills
		)
		VALUES ($1, $2, $3, 'running', 'prepare', '{}'::jsonb, '{"meta":{"name":"active"}}'::jsonb, '[]'::jsonb)
	`, testWorkspaceID, caseID, versionID); err != nil {
		t.Fatalf("seed running run: %v", err)
	}

	req := newRequest(http.MethodDelete, "/api/workflow-cases/"+caseID+"?workspace_id="+testWorkspaceID, nil)
	req = withURLParam(req, "caseId", caseID)
	rec := httptest.NewRecorder()

	testHandler.DeleteWorkflowCase(rec, req)

	if rec.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d: %s", rec.Code, rec.Body.String())
	}
	var caseCount int
	if err := testPool.QueryRow(ctx, `SELECT count(*) FROM workflow_case WHERE id = $1`, caseID).Scan(&caseCount); err != nil {
		t.Fatalf("count case: %v", err)
	}
	if caseCount != 1 {
		t.Fatalf("case must survive rejected delete, count=%d", caseCount)
	}
}

func TestDeleteWorkflowCaseNotFound(t *testing.T) {
	missingID := "00000000-0000-0000-0000-000000000000"
	req := newRequest(http.MethodDelete, "/api/workflow-cases/"+missingID+"?workspace_id="+testWorkspaceID, nil)
	req = withURLParam(req, "caseId", missingID)
	rec := httptest.NewRecorder()

	testHandler.DeleteWorkflowCase(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", rec.Code, rec.Body.String())
	}
}
