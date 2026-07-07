package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestCreateWorkflowCaseStandalone(t *testing.T) {
	ctx := context.Background()
	t.Cleanup(func() {
		testPool.Exec(ctx, `DELETE FROM workflow_case WHERE workspace_id = $1 AND title = $2`, testWorkspaceID, "Standalone workflow case")
	})

	req := newRequest(http.MethodPost, "/api/workflow-cases?workspace_id="+testWorkspaceID, map[string]any{
		"title":       "Standalone workflow case",
		"description": "created without a source issue",
	})
	w := httptest.NewRecorder()

	testHandler.CreateWorkflowCase(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}
	var resp WorkflowCaseResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.ID == "" || resp.WorkspaceID != testWorkspaceID {
		t.Fatalf("unexpected workflow case identity: %+v", resp)
	}
	if resp.Title != "Standalone workflow case" || resp.Description != "created without a source issue" {
		t.Fatalf("unexpected workflow case body: %+v", resp)
	}
	if resp.SourceIssueID != nil {
		t.Fatalf("source_issue_id = %v, want nil", *resp.SourceIssueID)
	}
	if resp.EntryIssueID != nil {
		t.Fatalf("entry_issue_id = %v, want nil", *resp.EntryIssueID)
	}
	if resp.Status != "draft" {
		t.Fatalf("status = %q, want draft", resp.Status)
	}
}

func TestCreateWorkflowCaseRejectsPublicStatus(t *testing.T) {
	req := newRequest(http.MethodPost, "/api/workflow-cases?workspace_id="+testWorkspaceID, map[string]any{
		"title":  "Forged lifecycle case",
		"status": "running",
	})
	w := httptest.NewRecorder()

	testHandler.CreateWorkflowCase(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestCreateWorkflowCaseFromIssue(t *testing.T) {
	ctx := context.Background()
	issueID := createIssueForTimeline(t, "Workflow case source issue")
	if _, err := testPool.Exec(ctx, `
		UPDATE issue
		SET description = 'source issue description'
		WHERE id = $1
	`, issueID); err != nil {
		t.Fatalf("seed source issue description: %v", err)
	}
	t.Cleanup(func() {
		testPool.Exec(ctx, `DELETE FROM workflow_case WHERE source_issue_id = $1`, issueID)
	})

	req := newRequest(http.MethodPost, "/api/issues/"+issueID+"/workflow-cases?workspace_id="+testWorkspaceID, map[string]any{})
	req = withURLParam(req, "id", issueID)
	w := httptest.NewRecorder()

	testHandler.CreateWorkflowCaseFromIssue(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}
	var resp WorkflowCaseResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.SourceIssueID == nil || *resp.SourceIssueID != issueID {
		t.Fatalf("source_issue_id = %v, want %s", resp.SourceIssueID, issueID)
	}
	if resp.EntryIssueID == nil || *resp.EntryIssueID != issueID {
		t.Fatalf("entry_issue_id = %v, want %s", resp.EntryIssueID, issueID)
	}
	if resp.Title != "Workflow case source issue" {
		t.Fatalf("title = %q, want issue title", resp.Title)
	}
	if resp.Description != "source issue description" {
		t.Fatalf("description = %q, want copied issue description", resp.Description)
	}
}

func TestCreateWorkflowCaseAcceptsEntryIssueID(t *testing.T) {
	ctx := context.Background()
	issueID := createIssueForTimeline(t, "Workflow case entry issue")
	t.Cleanup(func() {
		testPool.Exec(ctx, `DELETE FROM workflow_case WHERE source_issue_id = $1`, issueID)
	})

	req := newRequest(http.MethodPost, "/api/workflow-cases?workspace_id="+testWorkspaceID, map[string]any{
		"title":          "Entry issue workflow case",
		"entry_issue_id": issueID,
	})
	w := httptest.NewRecorder()

	testHandler.CreateWorkflowCase(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}
	var resp WorkflowCaseResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.EntryIssueID == nil || *resp.EntryIssueID != issueID {
		t.Fatalf("entry_issue_id = %v, want %s", resp.EntryIssueID, issueID)
	}
	if resp.SourceIssueID == nil || *resp.SourceIssueID != issueID {
		t.Fatalf("source_issue_id = %v, want %s", resp.SourceIssueID, issueID)
	}
}

func TestCreateWorkflowCaseRejectsConflictingEntryAndSourceIssue(t *testing.T) {
	entryIssueID := createIssueForTimeline(t, "Workflow case entry issue conflict")
	sourceIssueID := createIssueForTimeline(t, "Workflow case source issue conflict")

	req := newRequest(http.MethodPost, "/api/workflow-cases?workspace_id="+testWorkspaceID, map[string]any{
		"title":           "Conflicting issue workflow case",
		"entry_issue_id":  entryIssueID,
		"source_issue_id": sourceIssueID,
	})
	w := httptest.NewRecorder()

	testHandler.CreateWorkflowCase(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestUpdateWorkflowCaseRejectsConflictingEntryAndSourceIssue(t *testing.T) {
	caseID := createWorkflowCaseForTest(t, "Reject conflicting entry source case")
	entryIssueID := createIssueForTimeline(t, "Workflow case update entry issue conflict")
	sourceIssueID := createIssueForTimeline(t, "Workflow case update source issue conflict")
	req := newRequest(http.MethodPatch, "/api/workflow-cases/"+caseID+"?workspace_id="+testWorkspaceID, map[string]any{
		"entry_issue_id":  entryIssueID,
		"source_issue_id": sourceIssueID,
	})
	req = withURLParam(req, "caseId", caseID)
	w := httptest.NewRecorder()

	testHandler.UpdateWorkflowCase(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestCreateWorkflowCaseFromIssueRejectsSecondActiveCase(t *testing.T) {
	ctx := context.Background()
	issueID := createIssueForTimeline(t, "workflowcase unique entry issue")
	t.Cleanup(func() {
		testPool.Exec(ctx, `DELETE FROM workflow_case WHERE source_issue_id = $1`, issueID)
	})

	firstReq := newRequest(http.MethodPost, "/api/issues/"+issueID+"/workflow-cases?workspace_id="+testWorkspaceID, map[string]any{})
	firstReq = withURLParam(firstReq, "id", issueID)
	firstRec := httptest.NewRecorder()
	testHandler.CreateWorkflowCaseFromIssue(firstRec, firstReq)
	if firstRec.Code != http.StatusCreated {
		t.Fatalf("first create expected 201, got %d: %s", firstRec.Code, firstRec.Body.String())
	}

	secondReq := newRequest(http.MethodPost, "/api/issues/"+issueID+"/workflow-cases?workspace_id="+testWorkspaceID, map[string]any{})
	secondReq = withURLParam(secondReq, "id", issueID)
	secondRec := httptest.NewRecorder()
	testHandler.CreateWorkflowCaseFromIssue(secondRec, secondReq)
	if secondRec.Code != http.StatusConflict {
		t.Fatalf("second create expected 409, got %d: %s", secondRec.Code, secondRec.Body.String())
	}
	if !strings.Contains(secondRec.Body.String(), "entry issue already has an active workflow case") {
		t.Fatalf("unexpected conflict body: %s", secondRec.Body.String())
	}
}

func TestCreateWorkflowCaseRejectsSecondActiveEntryIssue(t *testing.T) {
	ctx := context.Background()
	issueID := createIssueForTimeline(t, "workflowcase generic unique entry issue")
	t.Cleanup(func() {
		testPool.Exec(ctx, `DELETE FROM workflow_case WHERE source_issue_id = $1`, issueID)
	})

	firstReq := newRequest(http.MethodPost, "/api/workflow-cases?workspace_id="+testWorkspaceID, map[string]any{
		"title":          "Generic entry workflow case first",
		"entry_issue_id": issueID,
	})
	firstRec := httptest.NewRecorder()
	testHandler.CreateWorkflowCase(firstRec, firstReq)
	if firstRec.Code != http.StatusCreated {
		t.Fatalf("first create expected 201, got %d: %s", firstRec.Code, firstRec.Body.String())
	}

	secondReq := newRequest(http.MethodPost, "/api/workflow-cases?workspace_id="+testWorkspaceID, map[string]any{
		"title":          "Generic entry workflow case second",
		"entry_issue_id": issueID,
	})
	secondRec := httptest.NewRecorder()
	testHandler.CreateWorkflowCase(secondRec, secondReq)
	if secondRec.Code != http.StatusConflict {
		t.Fatalf("second create expected 409, got %d: %s", secondRec.Code, secondRec.Body.String())
	}
	if !strings.Contains(secondRec.Body.String(), "entry issue already has an active workflow case") {
		t.Fatalf("unexpected conflict body: %s", secondRec.Body.String())
	}
}

func TestUpdateWorkflowCaseRejectsExecutionStatus(t *testing.T) {
	caseID := createWorkflowCaseForTest(t, "Reject execution status case")
	req := newRequest(http.MethodPatch, "/api/workflow-cases/"+caseID+"?workspace_id="+testWorkspaceID, map[string]any{
		"status": "running",
	})
	req = withURLParam(req, "caseId", caseID)
	w := httptest.NewRecorder()

	testHandler.UpdateWorkflowCase(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestUpdateWorkflowCaseAllowsDraftAndArchivedStatus(t *testing.T) {
	caseID := createWorkflowCaseForTest(t, "Allowed public status case")
	req := newRequest(http.MethodPatch, "/api/workflow-cases/"+caseID+"?workspace_id="+testWorkspaceID, map[string]any{
		"status": "archived",
	})
	req = withURLParam(req, "caseId", caseID)
	w := httptest.NewRecorder()

	testHandler.UpdateWorkflowCase(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var resp WorkflowCaseResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.Status != "archived" {
		t.Fatalf("status = %q, want archived", resp.Status)
	}
}

func TestListWorkflowCaseRuns(t *testing.T) {
	ctx := context.Background()
	caseID := createWorkflowCaseForTest(t, "List workflow case runs")
	var definitionID string
	if err := testPool.QueryRow(ctx, `
		INSERT INTO workflow_definition (
			workspace_id, case_id, draft_json, source_templates, status, created_by, updated_by
		)
		VALUES ($1, $2, '{"meta":{"name":"list runs"}}'::jsonb, '[]'::jsonb, 'draft', 'test', 'test')
		RETURNING id
	`, testWorkspaceID, caseID).Scan(&definitionID); err != nil {
		t.Fatalf("seed workflow_definition: %v", err)
	}
	var versionID string
	if err := testPool.QueryRow(ctx, `
		INSERT INTO workflow_definition_version (
			workspace_id, case_id, definition_id, version, snapshot_json, source_skills, validation_report, confirmed_by, confirmed_at
		)
		VALUES ($1, $2, $3, 1, '{"meta":{"name":"list runs"}}'::jsonb, '[]'::jsonb, '{"valid":true,"errors":[],"warnings":[]}'::jsonb, 'test', now())
		RETURNING id
	`, testWorkspaceID, caseID, definitionID).Scan(&versionID); err != nil {
		t.Fatalf("seed workflow_definition_version: %v", err)
	}

	var olderID, newerID string
	if err := testPool.QueryRow(ctx, `
		INSERT INTO workflow_run (
			workspace_id, case_id, definition_version_id, status, current_node,
			nodes_state, definition_snapshot, source_skills, created_at
		)
		VALUES ($1, $2, $3, 'done', 'END', '{}'::jsonb, '{"meta":{"name":"old"}}'::jsonb, '[]'::jsonb, now() - interval '1 minute')
		RETURNING id
	`, testWorkspaceID, caseID, versionID).Scan(&olderID); err != nil {
		t.Fatalf("seed older workflow_run: %v", err)
	}
	if err := testPool.QueryRow(ctx, `
		INSERT INTO workflow_run (
			workspace_id, case_id, definition_version_id, status, current_node,
			nodes_state, definition_snapshot, source_skills
		)
		VALUES ($1, $2, $3, 'running', 'START', '{}'::jsonb, '{"meta":{"name":"new"}}'::jsonb, '[]'::jsonb)
		RETURNING id
	`, testWorkspaceID, caseID, versionID).Scan(&newerID); err != nil {
		t.Fatalf("seed newer workflow_run: %v", err)
	}
	if _, err := testPool.Exec(ctx, `
		INSERT INTO workflow_run_node (
			workspace_id, case_id, run_id, node_id, node_type, dispatch, carrier_kind, status, attempt, carrier_ref
		)
		VALUES ($1, $2, $3, 'plan', 'llm', 'inline', 'inline', 'running', 2, '{"kind":"inline"}'::jsonb)
	`, testWorkspaceID, caseID, newerID); err != nil {
		t.Fatalf("seed workflow_run_node: %v", err)
	}

	req := newRequest(http.MethodGet, "/api/workflow-cases/"+caseID+"/runs?workspace_id="+testWorkspaceID, nil)
	req = withURLParam(req, "caseId", caseID)
	w := httptest.NewRecorder()

	testHandler.ListWorkflowCaseRuns(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var resp struct {
		Runs []WorkflowRunResponse `json:"runs"`
	}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(resp.Runs) != 2 {
		t.Fatalf("runs length = %d, want 2: %+v", len(resp.Runs), resp.Runs)
	}
	if resp.Runs[0].ID != newerID || resp.Runs[1].ID != olderID {
		t.Fatalf("runs order = [%s %s], want [%s %s]", resp.Runs[0].ID, resp.Runs[1].ID, newerID, olderID)
	}
	if resp.Runs[0].CaseID == nil || *resp.Runs[0].CaseID != caseID {
		t.Fatalf("case_id = %v, want %s", resp.Runs[0].CaseID, caseID)
	}
	if resp.Runs[0].DefinitionVersionID == nil || *resp.Runs[0].DefinitionVersionID != versionID {
		t.Fatalf("definition_version_id = %v, want %s", resp.Runs[0].DefinitionVersionID, versionID)
	}
	if len(resp.Runs[0].Nodes) != 1 {
		t.Fatalf("newer run nodes length = %d, want 1: %+v", len(resp.Runs[0].Nodes), resp.Runs[0].Nodes)
	}
	if node := resp.Runs[0].Nodes[0]; node.NodeID != "plan" || node.Status != "running" || node.Attempt != 2 {
		t.Fatalf("newer run node = %+v, want plan/running/attempt 2", node)
	}
}

func TestUpsertWorkflowDefinitionDraft(t *testing.T) {
	ctx := context.Background()
	caseID := createWorkflowCaseForTest(t, "Definition draft case")
	t.Cleanup(func() {
		testPool.Exec(ctx, `DELETE FROM workflow_definition WHERE case_id = $1`, caseID)
	})

	draft := map[string]any{
		"meta": map[string]any{"name": "case draft"},
		"nodes": []map[string]any{
			{"id": "implement", "type": "subissue", "agent": "agent-id"},
		},
		"routing": []map[string]any{{"from": "START", "to": "implement"}},
	}
	sourceTemplates := []map[string]any{{"skill_id": "template-skill", "node_id": "implement"}}
	putReq := newRequest(http.MethodPut, "/api/workflow-cases/"+caseID+"/definition/draft?workspace_id="+testWorkspaceID, map[string]any{
		"draft_json":       draft,
		"source_templates": sourceTemplates,
	})
	putReq = withURLParam(putReq, "caseId", caseID)
	putW := httptest.NewRecorder()

	testHandler.UpsertWorkflowCaseDefinitionDraft(putW, putReq)

	if putW.Code != http.StatusOK {
		t.Fatalf("upsert: expected 200, got %d: %s", putW.Code, putW.Body.String())
	}

	getReq := newRequest(http.MethodGet, "/api/workflow-cases/"+caseID+"/definition?workspace_id="+testWorkspaceID, nil)
	getReq = withURLParam(getReq, "caseId", caseID)
	getW := httptest.NewRecorder()

	testHandler.GetWorkflowCaseDefinition(getW, getReq)

	if getW.Code != http.StatusOK {
		t.Fatalf("get: expected 200, got %d: %s", getW.Code, getW.Body.String())
	}
	var resp WorkflowDefinitionResponse
	if err := json.NewDecoder(getW.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.CaseID != caseID {
		t.Fatalf("case_id = %q, want %q", resp.CaseID, caseID)
	}
	var gotDraft map[string]any
	if err := json.Unmarshal(resp.DraftJSON, &gotDraft); err != nil {
		t.Fatalf("decode draft_json: %v", err)
	}
	if gotDraft["meta"].(map[string]any)["name"] != "case draft" {
		t.Fatalf("draft_json did not round-trip: %s", string(resp.DraftJSON))
	}
	var gotTemplates []map[string]any
	if err := json.Unmarshal(resp.SourceTemplates, &gotTemplates); err != nil {
		t.Fatalf("decode source_templates: %v", err)
	}
	if len(gotTemplates) != 1 || gotTemplates[0]["skill_id"] != "template-skill" {
		t.Fatalf("source_templates did not round-trip: %s", string(resp.SourceTemplates))
	}
}

func TestValidateWorkflowDefinitionDraft(t *testing.T) {
	ctx := context.Background()
	caseID := createWorkflowCaseForTest(t, "Definition validate case")
	t.Cleanup(func() {
		testPool.Exec(ctx, `DELETE FROM workflow_definition WHERE case_id = $1`, caseID)
	})

	putReq := newRequest(http.MethodPut, "/api/workflow-cases/"+caseID+"/definition/draft?workspace_id="+testWorkspaceID, map[string]any{
		"draft_json": map[string]any{
			"meta": map[string]any{"name": "invalid"},
			"nodes": []map[string]any{
				{"id": "danger", "type": "shell"},
			},
			"routing": []map[string]any{{"from": "START", "to": "danger"}},
		},
		"source_templates": []map[string]any{},
	})
	putReq = withURLParam(putReq, "caseId", caseID)
	putW := httptest.NewRecorder()
	testHandler.UpsertWorkflowCaseDefinitionDraft(putW, putReq)
	if putW.Code != http.StatusOK {
		t.Fatalf("upsert invalid draft: expected 200, got %d: %s", putW.Code, putW.Body.String())
	}

	validateReq := newRequest(http.MethodPost, "/api/workflow-cases/"+caseID+"/definition/validate?workspace_id="+testWorkspaceID, nil)
	validateReq = withURLParam(validateReq, "caseId", caseID)
	validateW := httptest.NewRecorder()

	testHandler.ValidateWorkflowCaseDefinitionDraft(validateW, validateReq)

	if validateW.Code != http.StatusOK {
		t.Fatalf("validate: expected 200, got %d: %s", validateW.Code, validateW.Body.String())
	}
	var resp struct {
		Valid  bool `json:"valid"`
		Errors []struct {
			Code    string `json:"code"`
			Path    string `json:"path"`
			Message string `json:"message"`
		} `json:"errors"`
		Warnings []any `json:"warnings"`
	}
	if err := json.NewDecoder(validateW.Body).Decode(&resp); err != nil {
		t.Fatalf("decode validation response: %v", err)
	}
	if resp.Valid {
		t.Fatalf("valid = true, want false")
	}
	if len(resp.Errors) == 0 {
		t.Fatalf("expected at least one validation error")
	}
	if resp.Errors[0].Message == "" {
		t.Fatalf("validation error missing message: %+v", resp.Errors[0])
	}
}

func TestValidateWorkflowDefinitionUsesOwnerAgentForStandaloneCase(t *testing.T) {
	agentID := createHandlerTestAgent(t, "Workflow Case Owner Agent", nil)
	skillID := insertHandlerTestSkill(t, "workflow-case-owner-skill", "owned body")
	bindSkillToAgentForWorkflowCase(t, agentID, skillID)

	caseID := createWorkflowCaseViaHandler(t, map[string]any{
		"title":          "Owner authority standalone case",
		"owner_agent_id": agentID,
	})
	upsertWorkflowCaseDefinitionForTest(t, caseID, workflowCaseDefinitionWithSourceSkill(agentID, skillID))

	resp := validateWorkflowCaseDefinitionForTest(t, caseID)

	if !resp.Valid {
		t.Fatalf("valid = false, want true; errors=%+v", resp.Errors)
	}
}

func TestValidateWorkflowDefinitionUsesOwnerAgentOverSourceIssueAssignee(t *testing.T) {
	ownerAgentID := createHandlerTestAgent(t, "Workflow Case Explicit Owner Agent", nil)
	issueAssigneeID := createHandlerTestAgent(t, "Workflow Case Issue Assignee Agent", nil)
	skillID := insertHandlerTestSkill(t, "workflow-case-explicit-owner-skill", "owned body")
	bindSkillToAgentForWorkflowCase(t, ownerAgentID, skillID)
	issueID := createHandlerTestIssueWithAssignee(t, "Owner authority source issue", issueAssigneeID, "in_progress")

	caseID := createWorkflowCaseFromIssueViaHandler(t, issueID, map[string]any{
		"owner_agent_id": ownerAgentID,
	})
	upsertWorkflowCaseDefinitionForTest(t, caseID, workflowCaseDefinitionWithSourceSkill(ownerAgentID, skillID))

	resp := validateWorkflowCaseDefinitionForTest(t, caseID)

	if !resp.Valid {
		t.Fatalf("valid = false, want true; errors=%+v", resp.Errors)
	}
}

func createWorkflowCaseForTest(t *testing.T, title string) string {
	t.Helper()
	ctx := context.Background()
	var caseID string
	if err := testPool.QueryRow(ctx, `
		INSERT INTO workflow_case (workspace_id, title, description, status, created_by, updated_by)
		VALUES ($1, $2, '', 'draft', $3, $3)
		RETURNING id
	`, testWorkspaceID, title, testUserID).Scan(&caseID); err != nil {
		t.Fatalf("create workflow_case fixture: %v", err)
	}
	t.Cleanup(func() {
		testPool.Exec(ctx, `DELETE FROM workflow_case WHERE id = $1`, caseID)
	})
	return caseID
}

func createWorkflowCaseViaHandler(t *testing.T, body map[string]any) string {
	t.Helper()
	req := newRequest(http.MethodPost, "/api/workflow-cases?workspace_id="+testWorkspaceID, body)
	w := httptest.NewRecorder()

	testHandler.CreateWorkflowCase(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("create workflow case: expected 201, got %d: %s", w.Code, w.Body.String())
	}
	var resp WorkflowCaseResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode workflow case response: %v", err)
	}
	t.Cleanup(func() {
		testPool.Exec(context.Background(), `DELETE FROM workflow_case WHERE id = $1`, resp.ID)
	})
	return resp.ID
}

func createWorkflowCaseFromIssueViaHandler(t *testing.T, issueID string, body map[string]any) string {
	t.Helper()
	req := newRequest(http.MethodPost, "/api/issues/"+issueID+"/workflow-cases?workspace_id="+testWorkspaceID, body)
	req = withURLParam(req, "id", issueID)
	w := httptest.NewRecorder()

	testHandler.CreateWorkflowCaseFromIssue(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("create workflow case from issue: expected 201, got %d: %s", w.Code, w.Body.String())
	}
	var resp WorkflowCaseResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode workflow case response: %v", err)
	}
	t.Cleanup(func() {
		testPool.Exec(context.Background(), `DELETE FROM workflow_case WHERE id = $1`, resp.ID)
	})
	return resp.ID
}

func upsertWorkflowCaseDefinitionForTest(t *testing.T, caseID string, draft map[string]any) {
	t.Helper()
	req := newRequest(http.MethodPut, "/api/workflow-cases/"+caseID+"/definition/draft?workspace_id="+testWorkspaceID, map[string]any{
		"draft_json":       draft,
		"source_templates": []map[string]any{},
	})
	req = withURLParam(req, "caseId", caseID)
	w := httptest.NewRecorder()

	testHandler.UpsertWorkflowCaseDefinitionDraft(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("upsert workflow definition: expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func validateWorkflowCaseDefinitionForTest(t *testing.T, caseID string) workflowDefinitionValidationResponse {
	t.Helper()
	req := newRequest(http.MethodPost, "/api/workflow-cases/"+caseID+"/definition/validate?workspace_id="+testWorkspaceID, nil)
	req = withURLParam(req, "caseId", caseID)
	w := httptest.NewRecorder()

	testHandler.ValidateWorkflowCaseDefinitionDraft(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("validate workflow definition: expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var resp workflowDefinitionValidationResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode validation response: %v", err)
	}
	return resp
}

func workflowCaseDefinitionWithSourceSkill(agentID, skillID string) map[string]any {
	return map[string]any{
		"meta": map[string]any{"name": "owner authority"},
		"nodes": []map[string]any{
			{
				"id":              "implement",
				"type":            "subissue",
				"dispatch":        "subissue",
				"agent":           agentID,
				"source_skill_id": skillID,
			},
		},
		"routing": []map[string]any{
			{"from": "START", "to": "implement"},
			{"from": "implement", "to": "END"},
		},
	}
}

func bindSkillToAgentForWorkflowCase(t *testing.T, agentID, skillID string) {
	t.Helper()
	if _, err := testPool.Exec(context.Background(),
		`INSERT INTO agent_skill (agent_id, skill_id) VALUES ($1, $2)`,
		agentID, skillID,
	); err != nil {
		t.Fatalf("bind skill to agent: %v", err)
	}
	t.Cleanup(func() {
		testPool.Exec(context.Background(), `DELETE FROM agent_skill WHERE agent_id = $1 AND skill_id = $2`, agentID, skillID)
	})
}
