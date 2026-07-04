package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

func TestCreateAndInstantiateIssueTemplate(t *testing.T) {
	if testHandler == nil || testPool == nil {
		t.Skip("handler test database not configured")
	}
	agentID := seedIssueTemplateTestAgent(t)

	createBody := map[string]any{
		"title":                "Weekly triage template",
		"issue_title_template": "Weekly triage",
		"issue_body_template":  "Review inbox and backlog",
		"assignee_type":        "agent",
		"assignee_id":          agentID,
		"priority":             "medium",
		"labels":               []string{"triage"},
	}
	w := httptest.NewRecorder()
	testHandler.CreateIssueTemplate(w, newRequest(http.MethodPost, "/api/issue-templates?workspace_id="+testWorkspaceID, createBody))
	if w.Code != http.StatusCreated {
		t.Fatalf("CreateIssueTemplate status = %d, body = %s", w.Code, w.Body.String())
	}
	var created IssueTemplateResponse
	if err := json.Unmarshal(w.Body.Bytes(), &created); err != nil {
		t.Fatalf("decode create response: %v", err)
	}
	t.Cleanup(func() {
		_, _ = testPool.Exec(context.Background(), `DELETE FROM issue WHERE issue_template_id = $1`, created.ID)
		_, _ = testPool.Exec(context.Background(), `DELETE FROM issue_template WHERE id = $1`, created.ID)
	})

	req := withURLParam(
		newRequest(http.MethodPost, "/api/issue-templates/"+created.ID+"/instantiate?workspace_id="+testWorkspaceID, nil),
		"id",
		created.ID,
	)
	w = httptest.NewRecorder()
	testHandler.InstantiateIssueTemplate(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("InstantiateIssueTemplate status = %d, body = %s", w.Code, w.Body.String())
	}
	var issue IssueResponse
	if err := json.Unmarshal(w.Body.Bytes(), &issue); err != nil {
		t.Fatalf("decode issue response: %v", err)
	}
	if issue.Title != "Weekly triage" {
		t.Fatalf("issue title = %q, want Weekly triage", issue.Title)
	}

	row, err := testHandler.Queries.GetIssue(context.Background(), parseUUID(issue.ID))
	if err != nil {
		t.Fatalf("GetIssue: %v", err)
	}
	if row.OriginType.Valid {
		t.Fatalf("manual template instantiate set origin_type = %#v", row.OriginType)
	}
	if !row.IssueTemplateID.Valid || uuidToString(row.IssueTemplateID) != created.ID {
		t.Fatalf("issue_template_id = %s, want %s", uuidToString(row.IssueTemplateID), created.ID)
	}
	if len(row.IssueTemplateSnapshot) == 0 {
		t.Fatal("issue_template_snapshot is empty")
	}
}

func TestDeleteIssueTemplateRefusesIssueReferences(t *testing.T) {
	if testHandler == nil || testPool == nil {
		t.Skip("handler test database not configured")
	}
	agentID := seedIssueTemplateTestAgent(t)
	templateID := seedIssueTemplate(t, agentID, "Referenced template")
	t.Cleanup(func() {
		_, _ = testPool.Exec(context.Background(), `DELETE FROM issue WHERE issue_template_id = $1`, templateID)
		_, _ = testPool.Exec(context.Background(), `DELETE FROM issue_template WHERE id = $1`, templateID)
	})

	req := withURLParam(
		newRequest(http.MethodPost, "/api/issue-templates/"+templateID+"/instantiate?workspace_id="+testWorkspaceID, nil),
		"id",
		templateID,
	)
	w := httptest.NewRecorder()
	testHandler.InstantiateIssueTemplate(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("InstantiateIssueTemplate status = %d, body = %s", w.Code, w.Body.String())
	}

	deleteReq := withURLParam(
		newRequest(http.MethodDelete, "/api/issue-templates/"+templateID+"?workspace_id="+testWorkspaceID, nil),
		"id",
		templateID,
	)
	w = httptest.NewRecorder()
	testHandler.DeleteIssueTemplate(w, deleteReq)
	if w.Code != http.StatusConflict {
		t.Fatalf("DeleteIssueTemplate status = %d, want 409, body = %s", w.Code, w.Body.String())
	}
}

func TestListIssueTemplateIssuesReturnsOnlyCurrentTemplateReferences(t *testing.T) {
	if testHandler == nil || testPool == nil {
		t.Skip("handler test database not configured")
	}
	agentID := seedIssueTemplateTestAgent(t)
	templateAID := seedIssueTemplate(t, agentID, "Referenced template A")
	templateBID := seedIssueTemplate(t, agentID, "Referenced template B")
	t.Cleanup(func() {
		_, _ = testPool.Exec(context.Background(), `DELETE FROM issue WHERE issue_template_id IN ($1, $2)`, templateAID, templateBID)
		_, _ = testPool.Exec(context.Background(), `DELETE FROM issue_template WHERE id IN ($1, $2)`, templateAID, templateBID)
	})

	for _, templateID := range []string{templateAID, templateBID} {
		req := withURLParam(
			newRequest(http.MethodPost, "/api/issue-templates/"+templateID+"/instantiate?workspace_id="+testWorkspaceID, nil),
			"id",
			templateID,
		)
		w := httptest.NewRecorder()
		testHandler.InstantiateIssueTemplate(w, req)
		if w.Code != http.StatusCreated {
			t.Fatalf("InstantiateIssueTemplate(%s) status = %d, body = %s", templateID, w.Code, w.Body.String())
		}
	}

	req := withURLParam(
		newRequest(http.MethodGet, "/api/issue-templates/"+templateAID+"/issues?workspace_id="+testWorkspaceID, nil),
		"id",
		templateAID,
	)
	w := httptest.NewRecorder()
	testHandler.ListIssueTemplateIssues(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("ListIssueTemplateIssues status = %d, body = %s", w.Code, w.Body.String())
	}

	var resp struct {
		Issues []IssueResponse `json:"issues"`
		Total  int64           `json:"total"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode issue reference response: %v", err)
	}
	if resp.Total != 1 {
		t.Fatalf("total = %d, want 1", resp.Total)
	}
	if len(resp.Issues) != 1 {
		t.Fatalf("len(issues) = %d, want 1", len(resp.Issues))
	}
	if resp.Issues[0].Title != "Referenced template A issue" {
		t.Fatalf("issue title = %q, want Referenced template A issue", resp.Issues[0].Title)
	}
}

func seedIssueTemplateTestAgent(t *testing.T) string {
	t.Helper()
	var id string
	if err := testPool.QueryRow(context.Background(), `
		INSERT INTO agent (workspace_id, name, description, runtime_mode, runtime_config, runtime_id, owner_id)
		VALUES ($1, $2, 'issue template test agent', 'local', '{}'::jsonb, $3, $4)
		RETURNING id
	`, testWorkspaceID, "Issue Template Test Agent "+t.Name(), handlerTestRuntimeID(t), testUserID).Scan(&id); err != nil {
		t.Fatalf("seed agent: %v", err)
	}
	t.Cleanup(func() { _, _ = testPool.Exec(context.Background(), `DELETE FROM agent WHERE id = $1`, id) })
	return id
}

func seedIssueTemplate(t *testing.T, agentID, title string) string {
	t.Helper()
	template, err := testHandler.Queries.CreateIssueTemplate(context.Background(), db.CreateIssueTemplateParams{
		WorkspaceID:        parseUUID(testWorkspaceID),
		Title:              title,
		IssueTitleTemplate: title + " issue",
		IssueBodyTemplate:  strToText("Body from template"),
		AssigneeType:       "agent",
		AssigneeID:         parseUUID(agentID),
		Priority:           "medium",
		CreatedByType:      "member",
		CreatedByID:        parseUUID(testUserID),
	})
	if err != nil {
		t.Fatalf("seed issue template: %v", err)
	}
	return uuidToString(template.ID)
}
