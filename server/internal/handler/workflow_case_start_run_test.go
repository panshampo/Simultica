package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func startWorkflowCaseRunViaHandler(t *testing.T, caseID string, body map[string]any) *httptest.ResponseRecorder {
	t.Helper()
	req := newRequest(http.MethodPost, "/api/workflow-cases/"+caseID+"/runs?workspace_id="+testWorkspaceID, body)
	req = withURLParams(req, "caseId", caseID)
	rec := httptest.NewRecorder()
	testHandler.StartWorkflowCaseRun(rec, req)
	return rec
}

func newRunSidecar(t *testing.T) *httptest.Server {
	t.Helper()
	sidecar := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/runs" {
			t.Fatalf("unexpected sidecar request: %s %s", r.Method, r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusAccepted)
		_, _ = w.Write([]byte(`{"status":"accepted"}`))
	}))
	t.Cleanup(sidecar.Close)
	return sidecar
}

func TestStartWorkflowCaseRunUsesOnlineVersion(t *testing.T) {
	ctx := context.Background()
	sidecar := newRunSidecar(t)
	t.Setenv("MULTICA_WORKFLOW_SIDECAR_URL", sidecar.URL)

	caseID := createWorkflowCaseForTest(t, "start from online version")
	upsertWorkflowCaseDefinitionForTest(t, caseID, workflowCaseConfirmDefinition())
	onlineVersionID := publishWorkflowCaseForTest(t, caseID)

	rec := startWorkflowCaseRunViaHandler(t, caseID, map[string]any{
		"run_kind":      "experiment",
		"label":         "Approach A",
		"initial_state": map[string]any{"task": "online run"},
	})
	if rec.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp WorkflowRunResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode run: %v", err)
	}
	if resp.DefinitionVersionID == nil || *resp.DefinitionVersionID != onlineVersionID {
		t.Fatalf("definition_version_id = %v, want online %s", resp.DefinitionVersionID, onlineVersionID)
	}
	if resp.RunKind != "experiment" {
		t.Fatalf("run_kind = %q, want experiment", resp.RunKind)
	}
	if resp.Label != "Approach A" {
		t.Fatalf("label = %q, want Approach A", resp.Label)
	}

	// Start run must not project run status onto the case.
	updatedCase, err := testHandler.Queries.GetWorkflowCase(ctx, parseUUID(caseID))
	if err != nil {
		t.Fatalf("load workflow case: %v", err)
	}
	if updatedCase.Status == "running" {
		t.Fatalf("case status = %q, start run must not project run status", updatedCase.Status)
	}
}

func TestStartWorkflowCaseRunRejectsWithoutOnlineVersion(t *testing.T) {
	ctx := context.Background()
	caseID := createWorkflowCaseForTest(t, "start without online version")
	upsertWorkflowCaseDefinitionForTest(t, caseID, workflowCaseConfirmDefinition())

	rec := startWorkflowCaseRunViaHandler(t, caseID, map[string]any{"run_kind": "primary"})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rec.Code, rec.Body.String())
	}
	var runCount int
	if err := testPool.QueryRow(ctx, `SELECT count(*) FROM workflow_run WHERE case_id = $1`, caseID).Scan(&runCount); err != nil {
		t.Fatalf("count runs: %v", err)
	}
	if runCount != 0 {
		t.Fatalf("workflow_run count = %d, want 0", runCount)
	}
}

func TestStartWorkflowCaseRunRejectsHistoricalVersionRequest(t *testing.T) {
	sidecar := newRunSidecar(t)
	t.Setenv("MULTICA_WORKFLOW_SIDECAR_URL", sidecar.URL)

	caseID := createWorkflowCaseForTest(t, "reject historical version request")
	upsertWorkflowCaseDefinitionForTest(t, caseID, workflowCaseConfirmDefinition())
	historicalVersionID := publishWorkflowCaseForTest(t, caseID)
	_ = publishWorkflowCaseForTest(t, caseID) // new online version

	rec := startWorkflowCaseRunViaHandler(t, caseID, map[string]any{
		"definition_version_id": historicalVersionID,
	})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for explicit definition_version_id, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestStartWorkflowCaseRunAllowsParallelActiveRuns(t *testing.T) {
	ctx := context.Background()
	sidecar := newRunSidecar(t)
	t.Setenv("MULTICA_WORKFLOW_SIDECAR_URL", sidecar.URL)

	caseID := createWorkflowCaseForTest(t, "parallel active runs")
	upsertWorkflowCaseDefinitionForTest(t, caseID, workflowCaseConfirmDefinition())
	publishWorkflowCaseForTest(t, caseID)

	first := startWorkflowCaseRunViaHandler(t, caseID, map[string]any{"run_kind": "primary"})
	if first.Code != http.StatusAccepted {
		t.Fatalf("first run: expected 202, got %d: %s", first.Code, first.Body.String())
	}
	second := startWorkflowCaseRunViaHandler(t, caseID, map[string]any{"run_kind": "experiment"})
	if second.Code != http.StatusAccepted {
		t.Fatalf("second run: expected 202, got %d: %s", second.Code, second.Body.String())
	}

	var runningCount int
	if err := testPool.QueryRow(ctx, `SELECT count(*) FROM workflow_run WHERE case_id = $1 AND status = 'running'`, caseID).Scan(&runningCount); err != nil {
		t.Fatalf("count running runs: %v", err)
	}
	if runningCount != 2 {
		t.Fatalf("running run count = %d, want 2 parallel runs", runningCount)
	}
}

func TestStartWorkflowCaseRunRejectsUnknownRunKind(t *testing.T) {
	caseID := createWorkflowCaseForTest(t, "reject unknown run kind")
	upsertWorkflowCaseDefinitionForTest(t, caseID, workflowCaseConfirmDefinition())
	publishWorkflowCaseForTest(t, caseID)

	rec := startWorkflowCaseRunViaHandler(t, caseID, map[string]any{"run_kind": "bogus"})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for unknown run_kind, got %d: %s", rec.Code, rec.Body.String())
	}
}
