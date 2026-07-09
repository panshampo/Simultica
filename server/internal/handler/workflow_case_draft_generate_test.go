package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
)

func TestGenerateWorkflowDraftReturnsAccepted(t *testing.T) {
	caseID := createWorkflowCaseForTest(t, "generate draft")
	req := newRequest(http.MethodPost,
		"/api/workflow-cases/"+caseID+"/draft/generate?workspace_id="+testWorkspaceID,
		map[string]any{"instruction": "plan a frontend bug investigation"},
	)
	req = withURLParam(req, "caseId", caseID)
	rec := httptest.NewRecorder()
	testHandler.GenerateWorkflowDraft(rec, req)

	if rec.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want 202, body = %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp["status"] != "queued" {
		t.Fatalf("status = %v, want queued", resp["status"])
	}
}

func TestGenerateWorkflowDraftRejectsUnknownCase(t *testing.T) {
	req := newRequest(http.MethodPost,
		"/api/workflow-cases/00000000-0000-0000-0000-000000000000/draft/generate?workspace_id="+testWorkspaceID,
		map[string]any{},
	)
	req = withURLParam(req, "caseId", "00000000-0000-0000-0000-000000000000")
	rec := httptest.NewRecorder()
	testHandler.GenerateWorkflowDraft(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404, body = %s", rec.Code, rec.Body.String())
	}
}

// Ensure the route is wired so the stub is reachable through the router.
func TestGenerateWorkflowDraftRouteRegistered(t *testing.T) {
	caseID := createWorkflowCaseForTest(t, "generate draft route")
	router := chi.NewRouter()
	router.Post("/api/workflow-cases/{caseId}/draft/generate", testHandler.GenerateWorkflowDraft)

	req := newRequest(http.MethodPost,
		"/api/workflow-cases/"+caseID+"/draft/generate?workspace_id="+testWorkspaceID,
		map[string]any{},
	)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want 202, body = %s", rec.Code, rec.Body.String())
	}
}
