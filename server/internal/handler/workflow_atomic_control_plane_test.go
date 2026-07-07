package handler

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

func TestWorkflowCaseOnlineVersion(t *testing.T) {
	ctx := context.Background()
	caseID := createWorkflowCaseForTest(t, "online version schema")
	versionID := createWorkflowDefinitionVersionFixture(t, ctx, caseID, workflowCaseConfirmDefinition())

	updated, err := testHandler.Queries.SetWorkflowCaseOnlineVersion(ctx, db.SetWorkflowCaseOnlineVersionParams{
		ID:              parseUUID(caseID),
		OnlineVersionID: parseUUID(versionID),
		UpdatedBy:       pgtype.Text{String: "test", Valid: true},
	})
	if err != nil {
		t.Fatalf("SetWorkflowCaseOnlineVersion: %v", err)
	}
	if uuidToString(updated.OnlineVersionID) != versionID {
		t.Fatalf("online_version_id = %q, want %s", uuidToString(updated.OnlineVersionID), versionID)
	}

	reloaded, err := testHandler.Queries.GetWorkflowCase(ctx, parseUUID(caseID))
	if err != nil {
		t.Fatalf("GetWorkflowCase: %v", err)
	}
	if uuidToString(reloaded.OnlineVersionID) != versionID {
		t.Fatalf("reloaded online_version_id = %q, want %s", uuidToString(reloaded.OnlineVersionID), versionID)
	}
}

func TestCreateWorkflowRunStoresKindAndLabel(t *testing.T) {
	ctx := context.Background()
	caseID := createWorkflowCaseForTest(t, "run kind label schema")
	versionID := createWorkflowDefinitionVersionFixture(t, ctx, caseID, workflowCaseConfirmDefinition())

	run, err := testHandler.Queries.CreateWorkflowRun(ctx, db.CreateWorkflowRunParams{
		WorkspaceID:         parseUUID(testWorkspaceID),
		Status:              "running",
		CurrentNode:         "START",
		NodesState:          []byte(`{}`),
		DefinitionSnapshot:  []byte(`{"meta":{"name":"kind label"}}`),
		SourceSkills:        []byte(`[]`),
		CaseID:              parseUUID(caseID),
		DefinitionVersionID: parseUUID(versionID),
		RunKind:             "experiment",
		Label:               "Approach A",
	})
	if err != nil {
		t.Fatalf("CreateWorkflowRun: %v", err)
	}
	if run.RunKind != "experiment" {
		t.Fatalf("run_kind = %q, want experiment", run.RunKind)
	}
	if run.Label != "Approach A" {
		t.Fatalf("label = %q, want Approach A", run.Label)
	}

	defaulted, err := testHandler.Queries.CreateWorkflowRun(ctx, db.CreateWorkflowRunParams{
		WorkspaceID:         parseUUID(testWorkspaceID),
		Status:              "running",
		CurrentNode:         "START",
		NodesState:          []byte(`{}`),
		DefinitionSnapshot:  []byte(`{"meta":{"name":"defaults"}}`),
		SourceSkills:        []byte(`[]`),
		CaseID:              parseUUID(caseID),
		DefinitionVersionID: parseUUID(versionID),
	})
	if err != nil {
		t.Fatalf("CreateWorkflowRun defaults: %v", err)
	}
	if defaulted.RunKind != "primary" {
		t.Fatalf("default run_kind = %q, want primary", defaulted.RunKind)
	}
	if defaulted.Label != "" {
		t.Fatalf("default label = %q, want empty", defaulted.Label)
	}
}
