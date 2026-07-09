package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGetWorkflowCaseDefinitionVersionReturnsOwnedVersion(t *testing.T) {
	ctx := context.Background()
	caseID := createWorkflowCaseForTest(t, "case get owned version")
	versionID := createWorkflowDefinitionVersionFixture(t, ctx, caseID, workflowCaseConfirmDefinition())

	req := newRequest(http.MethodGet, "/api/workflow-cases/"+caseID+"/definition/versions/"+versionID+"?workspace_id="+testWorkspaceID, nil)
	req = withURLParams(req, "caseId", caseID, "versionId", versionID)
	rec := httptest.NewRecorder()

	testHandler.GetWorkflowCaseDefinitionVersion(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp WorkflowDefinitionVersionResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode version: %v", err)
	}
	if resp.ID != versionID {
		t.Fatalf("id = %q, want %s", resp.ID, versionID)
	}
	if resp.CaseID != caseID {
		t.Fatalf("case_id = %q, want %s", resp.CaseID, caseID)
	}
	if resp.Version != 1 {
		t.Fatalf("version = %d, want 1", resp.Version)
	}
}

func TestGetWorkflowCaseDefinitionVersionRejectsVersionOutsideCase(t *testing.T) {
	ctx := context.Background()
	ownerCaseID := createWorkflowCaseForTest(t, "case get version owner")
	otherCaseID := createWorkflowCaseForTest(t, "case get version other")
	versionID := createWorkflowDefinitionVersionFixture(t, ctx, ownerCaseID, workflowCaseConfirmDefinition())

	req := newRequest(http.MethodGet, "/api/workflow-cases/"+otherCaseID+"/definition/versions/"+versionID+"?workspace_id="+testWorkspaceID, nil)
	req = withURLParams(req, "caseId", otherCaseID, "versionId", versionID)
	rec := httptest.NewRecorder()

	testHandler.GetWorkflowCaseDefinitionVersion(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d: %s", rec.Code, rec.Body.String())
	}
}
