package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGetIssueWorkflowContextResolvesEntryIssue(t *testing.T) {
	ctx := context.Background()
	issueID := createIssueForTimeline(t, "workflow context entry issue")
	caseID := createWorkflowCaseForIssueFixture(t, ctx, issueID)
	runID := createWorkflowRunForCaseFixture(t, ctx, caseID, issueID)

	req := newRequest(http.MethodGet, "/api/issues/"+issueID+"/workflow-context?workspace_id="+testWorkspaceID, nil)
	req = withURLParams(req, "id", issueID)
	rec := httptest.NewRecorder()

	testHandler.GetIssueWorkflowContext(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp IssueWorkflowContextResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.Role != "entry_issue" {
		t.Fatalf("role = %q, want entry_issue", resp.Role)
	}
	if resp.WorkflowCaseID == nil || *resp.WorkflowCaseID != caseID {
		t.Fatalf("case id = %v, want %s", resp.WorkflowCaseID, caseID)
	}
	if resp.WorkflowRunID == nil || *resp.WorkflowRunID != runID {
		t.Fatalf("run id = %v, want %s", resp.WorkflowRunID, runID)
	}
	if resp.CarrierKind == nil || *resp.CarrierKind != "issue" {
		t.Fatalf("carrier kind = %v, want issue", resp.CarrierKind)
	}
}

func TestGetIssueWorkflowContextResolvesNodeIssueFromOrigin(t *testing.T) {
	ctx := context.Background()
	entryIssueID := createIssueForTimeline(t, "workflow context node entry")
	nodeIssueID := createIssueForTimeline(t, "workflow context node issue")
	caseID := createWorkflowCaseForIssueFixture(t, ctx, entryIssueID)
	runID := createWorkflowRunForCaseFixture(t, ctx, caseID, entryIssueID)
	seedWorkflowRunNodeFixture(t, ctx, caseID, runID, "implement", "agent", "subissue", map[string]any{
		"type":     "issue",
		"issue_id": nodeIssueID,
	})
	setIssueWorkflowOriginFixture(t, ctx, nodeIssueID, runID)

	req := newRequest(http.MethodGet, "/api/issues/"+nodeIssueID+"/workflow-context?workspace_id="+testWorkspaceID, nil)
	req = withURLParams(req, "id", nodeIssueID)
	rec := httptest.NewRecorder()

	testHandler.GetIssueWorkflowContext(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp IssueWorkflowContextResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.Role != "node_issue" {
		t.Fatalf("role = %q, want node_issue", resp.Role)
	}
	if resp.WorkflowCaseID == nil || *resp.WorkflowCaseID != caseID {
		t.Fatalf("case id = %v, want %s", resp.WorkflowCaseID, caseID)
	}
	if resp.WorkflowRunID == nil || *resp.WorkflowRunID != runID {
		t.Fatalf("run id = %v, want %s", resp.WorkflowRunID, runID)
	}
	if resp.WorkflowNodeID == nil || *resp.WorkflowNodeID != "implement" {
		t.Fatalf("node id = %v, want implement", resp.WorkflowNodeID)
	}
	if resp.CarrierKind == nil || *resp.CarrierKind != "issue" {
		t.Fatalf("carrier kind = %v, want issue", resp.CarrierKind)
	}
}

func TestGetIssueWorkflowContextResolvesNodeIssueFromMetadataWithoutProjection(t *testing.T) {
	ctx := context.Background()
	entryIssueID := createIssueForTimeline(t, "workflow context metadata entry")
	nodeIssueID := createIssueForTimeline(t, "workflow context metadata node issue")
	caseID := createWorkflowCaseForIssueFixture(t, ctx, entryIssueID)
	runID := createWorkflowRunForCaseFixture(t, ctx, caseID, entryIssueID)
	setIssueWorkflowMetadataFixture(t, ctx, nodeIssueID, map[string]any{
		"workflow_node": map[string]any{
			"workflow_case_id": caseID,
			"workflow_run_id":  runID,
			"workflow_node_id": "implement",
			"carrier_kind":     "issue",
		},
	})

	req := newRequest(http.MethodGet, "/api/issues/"+nodeIssueID+"/workflow-context?workspace_id="+testWorkspaceID, nil)
	req = withURLParams(req, "id", nodeIssueID)
	rec := httptest.NewRecorder()

	testHandler.GetIssueWorkflowContext(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp IssueWorkflowContextResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.Role != "node_issue" {
		t.Fatalf("role = %q, want node_issue", resp.Role)
	}
	if resp.WorkflowCaseID == nil || *resp.WorkflowCaseID != caseID {
		t.Fatalf("case id = %v, want %s", resp.WorkflowCaseID, caseID)
	}
	if resp.WorkflowRunID == nil || *resp.WorkflowRunID != runID {
		t.Fatalf("run id = %v, want %s", resp.WorkflowRunID, runID)
	}
	if resp.WorkflowNodeID == nil || *resp.WorkflowNodeID != "implement" {
		t.Fatalf("node id = %v, want implement", resp.WorkflowNodeID)
	}
	if resp.CarrierKind == nil || *resp.CarrierKind != "issue" {
		t.Fatalf("carrier kind = %v, want issue", resp.CarrierKind)
	}
}

func createWorkflowCaseForIssueFixture(t *testing.T, ctx context.Context, issueID string) string {
	t.Helper()
	var caseID string
	if err := testPool.QueryRow(ctx, `
		INSERT INTO workflow_case (workspace_id, title, description, source_issue_id, status, created_by, updated_by)
		VALUES ($1, 'Workflow context fixture', '', $2, 'running', 'test', 'test')
		RETURNING id
	`, testWorkspaceID, issueID).Scan(&caseID); err != nil {
		t.Fatalf("seed workflow_case: %v", err)
	}
	t.Cleanup(func() {
		testPool.Exec(ctx, `DELETE FROM workflow_case WHERE id = $1`, caseID)
	})
	return caseID
}

func createWorkflowRunForCaseFixture(t *testing.T, ctx context.Context, caseID, issueID string) string {
	t.Helper()
	var runID string
	if err := testPool.QueryRow(ctx, `
		INSERT INTO workflow_run (
			workspace_id, root_issue_id, case_id, status, current_node,
			nodes_state, definition_snapshot, source_skills
		)
		VALUES ($1, $2, $3, 'running', 'START', '{}'::jsonb, '{"meta":{"name":"context"}}'::jsonb, '[]'::jsonb)
		RETURNING id
	`, testWorkspaceID, issueID, caseID).Scan(&runID); err != nil {
		t.Fatalf("seed workflow_run: %v", err)
	}
	if _, err := testPool.Exec(ctx, `
		UPDATE workflow_case
		SET current_run_id = $1
		WHERE id = $2
	`, runID, caseID); err != nil {
		t.Fatalf("set workflow_case current_run_id: %v", err)
	}
	return runID
}

func seedWorkflowRunNodeFixture(t *testing.T, ctx context.Context, caseID, runID, nodeID, nodeType, dispatch string, carrierRef map[string]any) {
	t.Helper()
	rawCarrierRef, err := json.Marshal(carrierRef)
	if err != nil {
		t.Fatalf("marshal carrier_ref: %v", err)
	}
	if _, err := testPool.Exec(ctx, `
		INSERT INTO workflow_run_node (
			workspace_id, case_id, run_id, node_id, node_type, dispatch, carrier_kind, status, carrier_ref
		)
		VALUES ($1, $2, $3, $4, $5, $6, 'issue', 'running', $7::jsonb)
	`, testWorkspaceID, caseID, runID, nodeID, nodeType, dispatch, rawCarrierRef); err != nil {
		t.Fatalf("seed workflow_run_node: %v", err)
	}
}

func setIssueWorkflowOriginFixture(t *testing.T, ctx context.Context, issueID, runID string) {
	t.Helper()
	if _, err := testPool.Exec(ctx, `
		UPDATE issue
		SET origin_type = 'workflow_node', origin_id = $1
		WHERE id = $2
	`, runID, issueID); err != nil {
		t.Fatalf("set issue workflow origin: %v", err)
	}
}

func setIssueWorkflowMetadataFixture(t *testing.T, ctx context.Context, issueID string, metadata map[string]any) {
	t.Helper()
	raw, err := json.Marshal(metadata)
	if err != nil {
		t.Fatalf("marshal issue metadata: %v", err)
	}
	if _, err := testPool.Exec(ctx, `
		UPDATE issue
		SET metadata = $1::jsonb
		WHERE id = $2
	`, raw, issueID); err != nil {
		t.Fatalf("set issue workflow metadata: %v", err)
	}
}
