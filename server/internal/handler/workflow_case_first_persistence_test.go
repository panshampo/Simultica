package handler

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

func TestWorkflowCaseFirstPersistence(t *testing.T) {
	if testPool == nil {
		t.Skip("test database not available")
	}

	ctx := context.Background()
	queries := db.New(testPool)
	workspaceID := parseUUID(testWorkspaceID)

	workflowCase, err := queries.CreateWorkflowCase(ctx, db.CreateWorkflowCaseParams{
		WorkspaceID: workspaceID,
		Title:       "Case-first persistence",
		Description: "DB persistence smoke test",
		Status:      "draft",
		CreatedBy:   pgtype.Text{String: "test", Valid: true},
		UpdatedBy:   pgtype.Text{String: "test", Valid: true},
	})
	if err != nil {
		t.Fatalf("create workflow_case: %v", err)
	}
	t.Cleanup(func() {
		_, _ = testPool.Exec(ctx, `DELETE FROM workflow_case WHERE id = $1`, workflowCase.ID)
	})

	definition, err := queries.UpsertWorkflowDefinitionDraft(ctx, db.UpsertWorkflowDefinitionDraftParams{
		WorkspaceID:     workspaceID,
		CaseID:          workflowCase.ID,
		DraftJson:       []byte(`{"meta":{"name":"case-first"}}`),
		SourceTemplates: []byte(`[]`),
		UpdatedBy:       pgtype.Text{String: "test", Valid: true},
	})
	if err != nil {
		t.Fatalf("upsert workflow_definition: %v", err)
	}

	version, err := queries.CreateWorkflowDefinitionVersion(ctx, db.CreateWorkflowDefinitionVersionParams{
		WorkspaceID:      workspaceID,
		CaseID:           workflowCase.ID,
		DefinitionID:     definition.ID,
		SnapshotJson:     []byte(`{"nodes":[]}`),
		SourceSkills:     []byte(`[]`),
		ValidationReport: []byte(`{"valid":true}`),
		ConfirmedBy:      pgtype.Text{String: "test", Valid: true},
	})
	if err != nil {
		t.Fatalf("create workflow_definition_version: %v", err)
	}

	run, err := queries.CreateWorkflowRun(ctx, db.CreateWorkflowRunParams{
		WorkspaceID:         workspaceID,
		Status:              "running",
		CurrentNode:         "start",
		NodesState:          []byte(`{}`),
		DefinitionSnapshot:  []byte(`{"nodes":[]}`),
		SourceSkills:        []byte(`[]`),
		CaseID:              workflowCase.ID,
		DefinitionVersionID: version.ID,
	})
	if err != nil {
		t.Fatalf("create workflow_run: %v", err)
	}

	if _, err := queries.CreateWorkflowRunNode(ctx, db.CreateWorkflowRunNodeParams{
		WorkspaceID: workspaceID,
		CaseID:      workflowCase.ID,
		RunID:       run.ID,
		NodeID:      "start",
		NodeType:    "llm",
		Dispatch:    "inline",
		CarrierKind: "inline",
		Status:      "pending",
	}); err != nil {
		t.Fatalf("create workflow_run_node: %v", err)
	}

	if _, err := queries.CreateWorkflowRunNodeEvent(ctx, db.CreateWorkflowRunNodeEventParams{
		WorkspaceID: workspaceID,
		CaseID:      workflowCase.ID,
		RunID:       run.ID,
		NodeID:      "start",
		EventType:   "node_started",
		Attempt:     1,
		Logs:        []byte(`[]`),
	}); err != nil {
		t.Fatalf("create workflow_run_node_event: %v", err)
	}

	runs, err := queries.ListWorkflowRunsByCase(ctx, workflowCase.ID)
	if err != nil {
		t.Fatalf("list workflow_runs by case: %v", err)
	}
	if len(runs) != 1 || runs[0].ID != run.ID {
		t.Fatalf("ListWorkflowRunsByCase returned %+v, want run %s", runs, run.ID.String())
	}

	events, err := queries.ListWorkflowRunNodeEvents(ctx, db.ListWorkflowRunNodeEventsParams{
		RunID:  run.ID,
		NodeID: "start",
	})
	if err != nil {
		t.Fatalf("list workflow_run_node_events: %v", err)
	}
	if len(events) != 1 || events[0].EventType != "node_started" {
		t.Fatalf("ListWorkflowRunNodeEvents returned %+v, want node_started event", events)
	}
}
