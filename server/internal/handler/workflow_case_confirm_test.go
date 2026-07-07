package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestPublishWorkflowCaseDefinitionCreatesOnlineVersionWithoutRun(t *testing.T) {
	ctx := context.Background()
	caseID := createWorkflowCaseForTest(t, "Publish standalone workflow case")
	definition := workflowCaseConfirmDefinition()
	upsertWorkflowCaseDefinitionForTest(t, caseID, definition)

	req := newRequest(http.MethodPost, "/api/workflow-cases/"+caseID+"/definition/publish?workspace_id="+testWorkspaceID, map[string]any{
		"note": "ready to execute",
	})
	req = withURLParam(req, "caseId", caseID)
	w := httptest.NewRecorder()

	testHandler.PublishWorkflowCaseDefinition(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var resp publishWorkflowDefinitionResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.Version.ID == "" || resp.Version.CaseID != caseID || resp.Version.Version != 1 {
		t.Fatalf("unexpected version response: %+v", resp.Version)
	}

	updatedCase, err := testHandler.Queries.GetWorkflowCase(ctx, parseUUID(caseID))
	if err != nil {
		t.Fatalf("load workflow case: %v", err)
	}
	if uuidToString(updatedCase.OnlineVersionID) != resp.Version.ID {
		t.Fatalf("online_version_id = %q, want %s", uuidToString(updatedCase.OnlineVersionID), resp.Version.ID)
	}
	if updatedCase.Status == "planned" || updatedCase.Status == "running" {
		t.Fatalf("case status = %q, publish must not project run/lifecycle status", updatedCase.Status)
	}

	var runCount int
	if err := testPool.QueryRow(ctx, `SELECT count(*) FROM workflow_run WHERE case_id = $1`, caseID).Scan(&runCount); err != nil {
		t.Fatalf("count workflow_run rows: %v", err)
	}
	if runCount != 0 {
		t.Fatalf("workflow_run count = %d, want 0", runCount)
	}
}

func TestPublishWorkflowCaseDefinitionReplacesOnlineVersion(t *testing.T) {
	ctx := context.Background()
	caseID := createWorkflowCaseForTest(t, "Publish twice workflow case")
	definition := workflowCaseConfirmDefinition()
	upsertWorkflowCaseDefinitionForTest(t, caseID, definition)

	first := publishWorkflowCaseForTest(t, caseID)
	second := publishWorkflowCaseForTest(t, caseID)
	if first == second {
		t.Fatalf("expected distinct versions, got %s twice", first)
	}

	updatedCase, err := testHandler.Queries.GetWorkflowCase(ctx, parseUUID(caseID))
	if err != nil {
		t.Fatalf("load workflow case: %v", err)
	}
	if uuidToString(updatedCase.OnlineVersionID) != second {
		t.Fatalf("online_version_id = %q, want latest %s", uuidToString(updatedCase.OnlineVersionID), second)
	}
}

func TestPublishWorkflowCaseDefinitionRejectsInvalidDraft(t *testing.T) {
	ctx := context.Background()
	caseID := createWorkflowCaseForTest(t, "Publish invalid workflow case")
	upsertWorkflowCaseDefinitionForTest(t, caseID, map[string]any{
		"meta": map[string]any{"name": "invalid"},
		"nodes": []map[string]any{
			{"id": "danger", "type": "shell"},
		},
		"routing": []map[string]any{{"from": "START", "to": "danger"}},
	})

	req := newRequest(http.MethodPost, "/api/workflow-cases/"+caseID+"/definition/publish?workspace_id="+testWorkspaceID, map[string]any{})
	req = withURLParam(req, "caseId", caseID)
	w := httptest.NewRecorder()

	testHandler.PublishWorkflowCaseDefinition(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
	var versionCount int
	if err := testPool.QueryRow(ctx, `SELECT count(*) FROM workflow_definition_version WHERE case_id = $1`, caseID).Scan(&versionCount); err != nil {
		t.Fatalf("count workflow_definition_version rows: %v", err)
	}
	if versionCount != 0 {
		t.Fatalf("workflow_definition_version count = %d, want 0", versionCount)
	}
	updatedCase, err := testHandler.Queries.GetWorkflowCase(ctx, parseUUID(caseID))
	if err != nil {
		t.Fatalf("load workflow case: %v", err)
	}
	if updatedCase.OnlineVersionID.Valid {
		t.Fatalf("online_version_id must remain unset after invalid publish")
	}
}

func publishWorkflowCaseForTest(t *testing.T, caseID string) string {
	t.Helper()
	req := newRequest(http.MethodPost, "/api/workflow-cases/"+caseID+"/definition/publish?workspace_id="+testWorkspaceID, map[string]any{})
	req = withURLParam(req, "caseId", caseID)
	w := httptest.NewRecorder()
	testHandler.PublishWorkflowCaseDefinition(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("publish: expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var resp publishWorkflowDefinitionResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode publish response: %v", err)
	}
	return resp.Version.ID
}

func workflowCaseConfirmDefinition() map[string]any {
	return map[string]any{
		"meta": map[string]any{"name": "confirm case workflow", "version": "1"},
		"source_skills": []map[string]any{
			{"id": "confirm-skill", "name": "confirm"},
		},
		"state": map[string]any{
			"fields": []map[string]any{{"name": "task", "type": "string"}},
		},
		"nodes": []map[string]any{
			{
				"id":       "prepare",
				"type":     "condition",
				"dispatch": "inline",
			},
			{
				"id":       "implement",
				"type":     "subissue",
				"dispatch": "subissue",
				"agent":    "code",
			},
		},
		"routing": []map[string]any{
			{"from": "START", "to": "prepare"},
			{"from": "prepare", "to": "implement"},
			{"from": "implement", "to": "END"},
		},
	}
}

func createWorkflowDefinitionVersionFixture(t *testing.T, ctx context.Context, caseID string, definition map[string]any) string {
	t.Helper()
	rawDefinition, err := json.Marshal(definition)
	if err != nil {
		t.Fatalf("marshal workflow definition fixture: %v", err)
	}
	sourceSkills := []byte(`[]`)
	if raw, err := json.Marshal(definition["source_skills"]); err == nil && string(raw) != "null" {
		sourceSkills = raw
	}
	var definitionID string
	if err := testPool.QueryRow(ctx, `
		INSERT INTO workflow_definition (
			workspace_id, case_id, draft_json, source_templates, status, created_by, updated_by
		)
		VALUES ($1, $2, $3::jsonb, '[]'::jsonb, 'draft', 'test', 'test')
		RETURNING id
	`, testWorkspaceID, caseID, rawDefinition).Scan(&definitionID); err != nil {
		t.Fatalf("seed workflow_definition: %v", err)
	}
	var versionID string
	if err := testPool.QueryRow(ctx, `
		INSERT INTO workflow_definition_version (
			workspace_id, case_id, definition_id, version, snapshot_json, source_skills,
			validation_report, confirmed_by, confirmed_at
		)
		VALUES ($1, $2, $3, 1, $4::jsonb, $5::jsonb, '{"valid":true,"errors":[],"warnings":[]}'::jsonb, 'test', now())
		RETURNING id
	`, testWorkspaceID, caseID, definitionID, rawDefinition, sourceSkills).Scan(&versionID); err != nil {
		t.Fatalf("seed workflow_definition_version: %v", err)
	}
	return versionID
}
