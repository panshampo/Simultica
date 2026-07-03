package service

import (
	"encoding/json"
	"testing"

	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

func TestIssueInstantiator_ResolveTemplateAndBuildCreateParams(t *testing.T) {
	templateID := pgtype.UUID{Bytes: [16]byte{1}, Valid: true}
	workspaceID := pgtype.UUID{Bytes: [16]byte{2}, Valid: true}
	agentID := pgtype.UUID{Bytes: [16]byte{3}, Valid: true}
	creatorID := pgtype.UUID{Bytes: [16]byte{4}, Valid: true}
	template := db.IssueTemplate{
		ID:                 templateID,
		WorkspaceID:        workspaceID,
		Title:              "Weekly triage template",
		IssueTitleTemplate: "Weekly triage",
		IssueBodyTemplate:  pgtype.Text{String: "Review inbox and backlog", Valid: true},
		AssigneeType:       "agent",
		AssigneeID:         agentID,
		Priority:           "medium",
		Labels:             []byte(`["triage"]`),
	}

	params, err := NewIssueInstantiator().IssueCreateParamsFromTemplate(template, "member", creatorID)
	if err != nil {
		t.Fatalf("IssueCreateParamsFromTemplate returned error: %v", err)
	}
	if params.Title != "Weekly triage" {
		t.Fatalf("Title = %q, want Weekly triage", params.Title)
	}
	if params.Description.String != "Review inbox and backlog" || !params.Description.Valid {
		t.Fatalf("Description = %#v, want valid body", params.Description)
	}
	if !params.IssueTemplateID.Valid || params.IssueTemplateID != templateID {
		t.Fatalf("IssueTemplateID = %#v, want %#v", params.IssueTemplateID, templateID)
	}
	if len(params.IssueTemplateSnapshot) == 0 {
		t.Fatal("IssueTemplateSnapshot is empty")
	}

	var snapshot map[string]any
	if err := json.Unmarshal(params.IssueTemplateSnapshot, &snapshot); err != nil {
		t.Fatalf("snapshot is not valid json: %v", err)
	}
	if snapshot["issue_title_template"] != "Weekly triage" {
		t.Fatalf("snapshot issue_title_template = %v, want Weekly triage", snapshot["issue_title_template"])
	}
	labels, ok := snapshot["labels"].([]any)
	if !ok {
		t.Fatalf("snapshot labels = %#v, want JSON array", snapshot["labels"])
	}
	if len(labels) != 1 || labels[0] != "triage" {
		t.Fatalf("snapshot labels = %#v, want [triage]", labels)
	}
	if params.OriginType.Valid {
		t.Fatalf("OriginType should be unset for manual template instantiate, got %#v", params.OriginType)
	}
}

func TestIssueInstantiator_ResolveTemplateRequiresIssueTitle(t *testing.T) {
	_, err := NewIssueInstantiator().ResolveTemplate(db.IssueTemplate{})
	if err != ErrTemplateIssueTitleRequired {
		t.Fatalf("ResolveTemplate error = %v, want ErrTemplateIssueTitleRequired", err)
	}
}
