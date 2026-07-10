package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"github.com/multica-ai/multica/server/pkg/protocol"
	"gopkg.in/yaml.v3"
)

type WorkflowRunResponse struct {
	ID                  string                    `json:"id"`
	RootIssueID         *string                   `json:"root_issue_id,omitempty"`
	CaseID              *string                   `json:"case_id,omitempty"`
	DefinitionVersionID *string                   `json:"definition_version_id,omitempty"`
	SkillID             *string                   `json:"skill_id"`
	PlannerTaskID       *string                   `json:"planner_task_id,omitempty"`
	Status              string                    `json:"status"`
	Label               string                    `json:"label"`
	CurrentNode         string                    `json:"current_node"`
	NodesState          json.RawMessage           `json:"nodes_state"`
	DefinitionSnapshot  json.RawMessage           `json:"definition_snapshot"`
	SourceSkills        json.RawMessage           `json:"source_skills"`
	Nodes               []WorkflowRunNodeResponse `json:"nodes,omitempty"`
	Error               *string                   `json:"error"`
	CancelReason        *string                   `json:"cancel_reason,omitempty"`
	CancelledAt         *time.Time                `json:"cancelled_at,omitempty"`
	StartedAt           *time.Time                `json:"started_at,omitempty"`
	CompletedAt         *time.Time                `json:"completed_at,omitempty"`
	CreatedAt           time.Time                 `json:"created_at"`
	UpdatedAt           time.Time                 `json:"updated_at"`
}

type WorkflowRunNodeResponse struct {
	ID             string          `json:"id"`
	RunID          string          `json:"run_id"`
	NodeID         string          `json:"node_id"`
	NodeType       string          `json:"node_type"`
	Dispatch       string          `json:"dispatch"`
	CarrierKind    string          `json:"carrier_kind"`
	Status         string          `json:"status"`
	Attempt        int32           `json:"attempt"`
	InputSnapshot  json.RawMessage `json:"input_snapshot,omitempty"`
	OutputSnapshot json.RawMessage `json:"output_snapshot,omitempty"`
	Error          json.RawMessage `json:"error,omitempty"`
	Logs           json.RawMessage `json:"logs,omitempty"`
	CarrierRef     json.RawMessage `json:"carrier_ref,omitempty"`
	StartedAt      *time.Time      `json:"started_at,omitempty"`
	CompletedAt    *time.Time      `json:"completed_at,omitempty"`
	CreatedAt      time.Time       `json:"created_at"`
	UpdatedAt      time.Time       `json:"updated_at"`
}

type IssueWorkflowContextResponse struct {
	Role           string  `json:"role"`
	WorkflowCaseID *string `json:"workflow_case_id,omitempty"`
	WorkflowRunID  *string `json:"workflow_run_id,omitempty"`
	WorkflowNodeID *string `json:"workflow_node_id,omitempty"`
	CarrierKind    *string `json:"carrier_kind,omitempty"`
}

func workflowRunNodeToResponse(node db.WorkflowRunNode) WorkflowRunNodeResponse {
	resp := WorkflowRunNodeResponse{
		ID:             uuidToString(node.ID),
		RunID:          uuidToString(node.RunID),
		NodeID:         node.NodeID,
		NodeType:       node.NodeType,
		Dispatch:       node.Dispatch,
		CarrierKind:    workflowCarrierKindFromNode(node),
		Status:         node.Status,
		Attempt:        node.Attempt,
		InputSnapshot:  json.RawMessage(node.InputSnapshot),
		OutputSnapshot: json.RawMessage(node.OutputSnapshot),
		Error:          json.RawMessage(node.Error),
		Logs:           json.RawMessage(node.Logs),
		CarrierRef:     json.RawMessage(node.CarrierRef),
		CreatedAt:      node.CreatedAt.Time,
		UpdatedAt:      node.UpdatedAt.Time,
	}
	if node.StartedAt.Valid {
		t := node.StartedAt.Time
		resp.StartedAt = &t
	}
	if node.CompletedAt.Valid {
		t := node.CompletedAt.Time
		resp.CompletedAt = &t
	}
	return resp
}

func workflowRunToResponse(run db.WorkflowRun) WorkflowRunResponse {
	resp := WorkflowRunResponse{
		ID:                 uuidToString(run.ID),
		Status:             run.Status,
		Label:              run.Label,
		CurrentNode:        run.CurrentNode,
		NodesState:         json.RawMessage(run.NodesState),
		DefinitionSnapshot: json.RawMessage(run.DefinitionSnapshot),
		SourceSkills:       json.RawMessage(run.SourceSkills),
		CreatedAt:          run.CreatedAt.Time,
		UpdatedAt:          run.UpdatedAt.Time,
	}
	if run.RootIssueID.Valid {
		s := uuidToString(run.RootIssueID)
		resp.RootIssueID = &s
	}
	if run.CaseID.Valid {
		s := uuidToString(run.CaseID)
		resp.CaseID = &s
	}
	if run.DefinitionVersionID.Valid {
		s := uuidToString(run.DefinitionVersionID)
		resp.DefinitionVersionID = &s
	}
	if run.SkillID.Valid {
		s := uuidToString(run.SkillID)
		resp.SkillID = &s
	}
	if run.PlannerTaskID.Valid {
		s := uuidToString(run.PlannerTaskID)
		resp.PlannerTaskID = &s
	}
	if run.Error.Valid {
		e := run.Error.String
		resp.Error = &e
	}
	if run.CancelReason.Valid {
		s := run.CancelReason.String
		resp.CancelReason = &s
	}
	if run.CancelledAt.Valid {
		t := run.CancelledAt.Time
		resp.CancelledAt = &t
	}
	if run.StartedAt.Valid {
		t := run.StartedAt.Time
		resp.StartedAt = &t
	}
	if run.CompletedAt.Valid {
		t := run.CompletedAt.Time
		resp.CompletedAt = &t
	}
	return resp
}

func (h *Handler) workflowRunToResponseWithSubIssueStatus(ctx context.Context, run db.WorkflowRun) (WorkflowRunResponse, error) {
	resp := workflowRunToResponse(run)

	nodes, err := h.Queries.ListWorkflowRunNodes(ctx, run.ID)
	if err != nil {
		return resp, fmt.Errorf("list workflow run nodes: %w", err)
	}
	if len(nodes) > 0 {
		resp.Nodes = make([]WorkflowRunNodeResponse, 0, len(nodes))
		for _, node := range nodes {
			resp.Nodes = append(resp.Nodes, workflowRunNodeToResponse(node))
		}
	}

	subIssues, err := h.Queries.ListIssuesByOrigin(ctx, db.ListIssuesByOriginParams{
		WorkspaceID: run.WorkspaceID,
		OriginType:  pgtype.Text{String: "workflow_node", Valid: true},
		OriginID:    run.ID,
	})
	if err != nil {
		return resp, fmt.Errorf("list sub-issues: %w", err)
	}
	if len(subIssues) == 0 {
		return resp, nil
	}

	statusByIssueID := make(map[string]string, len(subIssues))
	for _, iss := range subIssues {
		statusByIssueID[uuidToString(iss.ID)] = iss.Status
	}

	var nodesState map[string]json.RawMessage
	if err := json.Unmarshal(resp.NodesState, &nodesState); err != nil {
		return resp, fmt.Errorf("unmarshal nodes_state: %w", err)
	}

	changed := false
	for nodeID, rawNode := range nodesState {
		var node map[string]any
		if err := json.Unmarshal(rawNode, &node); err != nil {
			continue
		}
		subIssueID, _ := node["sub_issue_id"].(string)
		if subIssueID == "" {
			subIssueID, _ = node["subIssueId"].(string)
		}
		if subIssueID == "" {
			continue
		}
		issueStatus, ok := statusByIssueID[subIssueID]
		if !ok {
			continue
		}
		mapped := issueStatusToNodeStatus(issueStatus)
		if current, _ := node["status"].(string); current == mapped {
			continue
		}
		node["status"] = mapped
		updated, err := json.Marshal(node)
		if err != nil {
			continue
		}
		nodesState[nodeID] = updated
		changed = true
	}

	if changed {
		merged, err := json.Marshal(nodesState)
		if err == nil {
			resp.NodesState = merged
		}
	}

	return resp, nil
}

func issueStatusToNodeStatus(issueStatus string) string {
	switch issueStatus {
	case "backlog", "todo":
		return "pending"
	case "in_progress":
		return "running"
	case "in_review", "done":
		return "done"
	case "blocked":
		return "blocked"
	case "cancelled":
		return "cancelled"
	default:
		return "pending"
	}
}

func (h *Handler) GetIssueWorkflowContext(w http.ResponseWriter, r *http.Request) {
	workspaceID, ok := h.workflowWorkspaceID(w, r)
	if !ok {
		return
	}
	issueID, ok := parseUUIDOrBadRequest(w, chi.URLParam(r, "id"), "issue id")
	if !ok {
		return
	}

	issue, err := h.Queries.GetIssue(r.Context(), issueID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeError(w, http.StatusNotFound, "issue not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "load issue: "+err.Error())
		return
	}
	if uuidToString(issue.WorkspaceID) != uuidToString(workspaceID) {
		writeError(w, http.StatusForbidden, "issue is outside workspace")
		return
	}

	if c, err := h.Queries.GetActiveWorkflowCaseByEntryIssue(r.Context(), issue.ID); err == nil {
		resp := IssueWorkflowContextResponse{Role: "entry_issue"}
		caseID := uuidToString(c.ID)
		resp.WorkflowCaseID = &caseID
		if c.CurrentRunID.Valid {
			runID := uuidToString(c.CurrentRunID)
			resp.WorkflowRunID = &runID
		}
		carrier := "issue"
		resp.CarrierKind = &carrier
		writeJSON(w, http.StatusOK, resp)
		return
	} else if !errors.Is(err, pgx.ErrNoRows) {
		writeError(w, http.StatusInternalServerError, "resolve entry workflow case: "+err.Error())
		return
	}

	if binding, ok := workflowNodeBindingFromIssue(issue); ok {
		resp, ok, err := h.resolveWorkflowNodeIssueContext(r.Context(), workspaceID, issue.ID, binding)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "resolve workflow node issue: "+err.Error())
			return
		}
		if ok {
			writeJSON(w, http.StatusOK, resp)
			return
		}
	}

	writeJSON(w, http.StatusOK, IssueWorkflowContextResponse{Role: "none"})
}

type workflowNodeIssueBinding struct {
	RunID       pgtype.UUID
	CaseID      pgtype.UUID
	NodeID      string
	CarrierKind string
}

func workflowNodeBindingFromIssue(issue db.Issue) (workflowNodeIssueBinding, bool) {
	if issue.OriginType.Valid && issue.OriginType.String == "workflow_node" && issue.OriginID.Valid {
		binding := workflowNodeIssueBinding{RunID: issue.OriginID}
		if metadataBinding, ok := workflowNodeBindingFromMetadataRaw(issue.Metadata); ok {
			if metadataBinding.RunID.Valid {
				binding.RunID = metadataBinding.RunID
			}
			binding.CaseID = metadataBinding.CaseID
			binding.NodeID = metadataBinding.NodeID
			binding.CarrierKind = metadataBinding.CarrierKind
		}
		return binding, true
	}
	return workflowNodeBindingFromMetadataRaw(issue.Metadata)
}

func workflowNodeBindingFromMetadataRaw(raw []byte) (workflowNodeIssueBinding, bool) {
	var binding workflowNodeIssueBinding
	if len(raw) == 0 || string(raw) == "null" {
		return binding, false
	}
	var metadata map[string]any
	if err := json.Unmarshal(raw, &metadata); err != nil {
		return binding, false
	}
	return workflowNodeBindingFromMetadata(metadata)
}

func workflowNodeBindingFromMetadata(metadata map[string]any) (workflowNodeIssueBinding, bool) {
	var binding workflowNodeIssueBinding
	if runID, ok := workflowRunIDFromMetadata(metadata); ok {
		binding.RunID = runID
	}
	if caseID, ok := workflowCaseIDFromMetadata(metadata); ok {
		binding.CaseID = caseID
	}
	if nodeID, ok := stringFromAny(metadata["workflow_node_id"]); ok {
		binding.NodeID = nodeID
	} else if nodeID, ok := stringFromAny(metadata["workflowNodeId"]); ok {
		binding.NodeID = nodeID
	} else if nodeID, ok := stringFromAny(metadata["node_id"]); ok {
		binding.NodeID = nodeID
	} else if nodeID, ok := stringFromAny(metadata["nodeId"]); ok {
		binding.NodeID = nodeID
	}
	if carrier, ok := stringFromAny(metadata["carrier_kind"]); ok {
		binding.CarrierKind = carrier
	} else if carrier, ok := stringFromAny(metadata["carrierKind"]); ok {
		binding.CarrierKind = carrier
	}
	for _, key := range []string{"workflow", "workflow_context", "workflowContext", "workflow_node", "workflowNode"} {
		if nested, ok := metadata[key].(map[string]any); ok {
			if nestedBinding, ok := workflowNodeBindingFromMetadata(nested); ok {
				if !binding.RunID.Valid {
					binding.RunID = nestedBinding.RunID
				}
				if !binding.CaseID.Valid {
					binding.CaseID = nestedBinding.CaseID
				}
				if binding.NodeID == "" {
					binding.NodeID = nestedBinding.NodeID
				}
				if binding.CarrierKind == "" {
					binding.CarrierKind = nestedBinding.CarrierKind
				}
			}
		}
	}
	return binding, binding.RunID.Valid
}

func workflowCaseIDFromMetadata(metadata map[string]any) (pgtype.UUID, bool) {
	for _, key := range []string{"workflow_case_id", "workflowCaseId", "case_id", "caseId"} {
		if id, ok := parseUUIDStringFromAny(metadata[key]); ok {
			return id, true
		}
	}
	for _, key := range []string{"workflow", "workflow_context", "workflowContext", "workflow_node", "workflowNode"} {
		if nested, ok := metadata[key].(map[string]any); ok {
			if id, ok := workflowCaseIDFromMetadata(nested); ok {
				return id, true
			}
		}
	}
	return pgtype.UUID{}, false
}

func workflowRunIDFromIssueContext(issue db.Issue) (pgtype.UUID, bool) {
	if binding, ok := workflowNodeBindingFromIssue(issue); ok {
		return binding.RunID, true
	}
	if len(issue.Metadata) == 0 || string(issue.Metadata) == "null" {
		return pgtype.UUID{}, false
	}

	var metadata map[string]any
	if err := json.Unmarshal(issue.Metadata, &metadata); err != nil {
		return pgtype.UUID{}, false
	}
	return workflowRunIDFromMetadata(metadata)
}

func workflowRunIDFromMetadata(metadata map[string]any) (pgtype.UUID, bool) {
	for _, key := range []string{"workflow_run_id", "workflowRunId", "run_id", "runId", "origin_id", "originId"} {
		if id, ok := parseUUIDStringFromAny(metadata[key]); ok {
			return id, true
		}
	}
	for _, key := range []string{"workflow", "workflow_context", "workflowContext", "workflow_node", "workflowNode"} {
		if nested, ok := metadata[key].(map[string]any); ok {
			if id, ok := workflowRunIDFromMetadata(nested); ok {
				return id, true
			}
		}
	}
	return pgtype.UUID{}, false
}

func stringFromAny(value any) (string, bool) {
	raw, ok := value.(string)
	if !ok {
		return "", false
	}
	raw = strings.TrimSpace(raw)
	return raw, raw != ""
}

func parseUUIDStringFromAny(value any) (pgtype.UUID, bool) {
	raw, ok := value.(string)
	if !ok {
		return pgtype.UUID{}, false
	}
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return pgtype.UUID{}, false
	}
	var id pgtype.UUID
	if err := id.Scan(raw); err != nil || !id.Valid {
		return pgtype.UUID{}, false
	}
	return id, true
}

func (h *Handler) resolveWorkflowNodeIssueContext(ctx context.Context, workspaceID, issueID pgtype.UUID, binding workflowNodeIssueBinding) (IssueWorkflowContextResponse, bool, error) {
	run, err := h.Queries.GetWorkflowRun(ctx, binding.RunID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return IssueWorkflowContextResponse{}, false, nil
		}
		return IssueWorkflowContextResponse{}, false, fmt.Errorf("load workflow run: %w", err)
	}
	if uuidToString(run.WorkspaceID) != uuidToString(workspaceID) || !run.CaseID.Valid {
		return IssueWorkflowContextResponse{}, false, nil
	}

	workflowCase, err := h.Queries.GetWorkflowCase(ctx, run.CaseID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return IssueWorkflowContextResponse{}, false, nil
		}
		return IssueWorkflowContextResponse{}, false, fmt.Errorf("load workflow case: %w", err)
	}
	if uuidToString(workflowCase.WorkspaceID) != uuidToString(workspaceID) {
		return IssueWorkflowContextResponse{}, false, nil
	}

	if binding.CaseID.Valid && uuidToString(binding.CaseID) != uuidToString(workflowCase.ID) {
		return IssueWorkflowContextResponse{}, false, nil
	}
	if binding.NodeID != "" {
		caseID := uuidToString(workflowCase.ID)
		runID := uuidToString(run.ID)
		carrier := binding.CarrierKind
		if carrier == "" {
			carrier = "issue"
		}
		return IssueWorkflowContextResponse{
			Role:           "node_issue",
			WorkflowCaseID: &caseID,
			WorkflowRunID:  &runID,
			WorkflowNodeID: &binding.NodeID,
			CarrierKind:    &carrier,
		}, true, nil
	}

	node, err := h.Queries.GetWorkflowRunNodeByIssueCarrier(ctx, db.GetWorkflowRunNodeByIssueCarrierParams{
		WorkspaceID: workspaceID,
		RunID:       run.ID,
		IssueID:     uuidToString(issueID),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return IssueWorkflowContextResponse{}, false, nil
		}
		return IssueWorkflowContextResponse{}, false, fmt.Errorf("find workflow run node carrier: %w", err)
	}

	caseID := uuidToString(workflowCase.ID)
	runID := uuidToString(run.ID)
	nodeID := node.NodeID
	carrier := workflowCarrierKindFromNode(node)
	return IssueWorkflowContextResponse{
		Role:           "node_issue",
		WorkflowCaseID: &caseID,
		WorkflowRunID:  &runID,
		WorkflowNodeID: &nodeID,
		CarrierKind:    &carrier,
	}, true, nil
}

func workflowCarrierKindFromNode(node db.WorkflowRunNode) string {
	if node.CarrierKind != "" {
		return node.CarrierKind
	}
	if len(node.CarrierRef) > 0 {
		var ref map[string]any
		if err := json.Unmarshal(node.CarrierRef, &ref); err == nil {
			if kind, _ := ref["kind"].(string); kind != "" {
				return kind
			}
			if kind, _ := ref["carrier_kind"].(string); kind != "" {
				return kind
			}
			if refType, _ := ref["type"].(string); refType == "issue" || refType == "subissue" {
				return "issue"
			}
		}
	}
	switch node.Dispatch {
	case "subissue":
		return "issue"
	case "main_issue_task":
		return "issue_task"
	case "direct_subagent":
		return "agent_runtime"
	default:
		return "inline"
	}
}

// Deprecated: issue-first workflow control is not canonical. Product UI,
// agent brief, and new clients must use WorkflowCase-scoped APIs. This handler
// remains only for migration/debug compatibility.
func (h *Handler) GetIssueWorkflowRun(w http.ResponseWriter, r *http.Request) {
	workspaceID, ok := h.workflowWorkspaceID(w, r)
	if !ok {
		return
	}
	issueID, ok := parseUUIDOrBadRequest(w, chi.URLParam(r, "id"), "issue id")
	if !ok {
		return
	}

	run, err := h.Queries.GetWorkflowRunByRootIssue(r.Context(), issueID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeError(w, http.StatusNotFound, "no workflow run for this issue")
			return
		}
		writeError(w, http.StatusInternalServerError, "load workflow run: "+err.Error())
		return
	}
	if uuidToString(run.WorkspaceID) != uuidToString(workspaceID) {
		writeError(w, http.StatusForbidden, "workflow run is outside workspace")
		return
	}

	resp, err := h.workflowRunToResponseWithSubIssueStatus(r.Context(), run)
	if err != nil {
		slog.Warn("overlay sub-issue status failed", "run_id", uuidToString(run.ID), "error", err)
		writeJSON(w, http.StatusOK, workflowRunToResponse(run))
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

type createWorkflowRunRequest struct {
	RootIssueID        string          `json:"root_issue_id"`
	SkillID            string          `json:"skill_id"`
	DefinitionSnapshot json.RawMessage `json:"definition_snapshot"`
	CurrentNode        string          `json:"current_node"`
}

// Deprecated: global workflow run creation is not canonical. Product UI,
// agent brief, and new clients must use WorkflowCase-scoped APIs. This handler
// remains only for migration/debug compatibility.
func (h *Handler) CreateWorkflowRun(w http.ResponseWriter, r *http.Request) {
	workspaceID, ok := h.workflowWorkspaceID(w, r)
	if !ok {
		return
	}

	var req createWorkflowRunRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	rootIssueID, ok := parseUUIDOrBadRequest(w, req.RootIssueID, "root_issue_id")
	if !ok {
		return
	}
	root, err := h.Queries.GetIssue(r.Context(), rootIssueID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeError(w, http.StatusNotFound, "root issue not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "load root issue: "+err.Error())
		return
	}
	if uuidToString(root.WorkspaceID) != uuidToString(workspaceID) {
		writeError(w, http.StatusForbidden, "root issue is outside workspace")
		return
	}

	skillID := pgtype.UUID{}
	if req.SkillID != "" {
		parsed, ok := parseUUIDOrBadRequest(w, req.SkillID, "skill_id")
		if !ok {
			return
		}
		if _, err := h.Queries.GetSkillInWorkspace(r.Context(), db.GetSkillInWorkspaceParams{
			ID:          parsed,
			WorkspaceID: workspaceID,
		}); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				writeError(w, http.StatusNotFound, "skill not found")
				return
			}
			writeError(w, http.StatusInternalServerError, "load skill: "+err.Error())
			return
		}
		skillID = parsed
	}

	snapshot := req.DefinitionSnapshot
	if len(snapshot) == 0 {
		snapshot = json.RawMessage(`{}`)
	}
	if req.CurrentNode == "" {
		req.CurrentNode = "START"
	}

	run, err := h.Queries.CreateWorkflowRun(r.Context(), db.CreateWorkflowRunParams{
		WorkspaceID:        workspaceID,
		RootIssueID:        rootIssueID,
		SkillID:            skillID,
		Status:             "running",
		CurrentNode:        req.CurrentNode,
		NodesState:         []byte(`{}`),
		DefinitionSnapshot: []byte(snapshot),
		SourceSkills:       []byte(`[]`),
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "create workflow run: "+err.Error())
		return
	}

	h.publishWorkflowRunUpdated(r.Context(), workspaceID, run)
	writeJSON(w, http.StatusCreated, workflowRunToResponse(run))
}

type updateWorkflowRunRequest struct {
	Status      *string         `json:"status"`
	CurrentNode *string         `json:"current_node"`
	NodesState  json.RawMessage `json:"nodes_state"`
	Error       *string         `json:"error"`
}

type workflowRunNodeEventRequest struct {
	EventType      string          `json:"event_type"`
	Attempt        int32           `json:"attempt"`
	InputSnapshot  json.RawMessage `json:"input_snapshot"`
	OutputSnapshot json.RawMessage `json:"output_snapshot"`
	Error          json.RawMessage `json:"error"`
	Logs           json.RawMessage `json:"logs"`
	CarrierRef     json.RawMessage `json:"carrier_ref"`
	Sequence       *int32          `json:"sequence"`
	OccurredAt     *time.Time      `json:"occurred_at"`
}

func (h *Handler) UpdateWorkflowRun(w http.ResponseWriter, r *http.Request) {
	workspaceID, ok := h.workflowWorkspaceID(w, r)
	if !ok {
		return
	}
	runID, ok := parseUUIDOrBadRequest(w, chi.URLParam(r, "runId"), "workflow run id")
	if !ok {
		return
	}

	existing, err := h.Queries.GetWorkflowRun(r.Context(), runID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeError(w, http.StatusNotFound, "workflow run not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "load workflow run: "+err.Error())
		return
	}
	if uuidToString(existing.WorkspaceID) != uuidToString(workspaceID) {
		writeError(w, http.StatusForbidden, "workflow run is outside workspace")
		return
	}

	var req updateWorkflowRunRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	params := db.UpdateWorkflowRunProgressParams{ID: runID}
	if req.Status != nil {
		params.Status = pgtype.Text{String: *req.Status, Valid: true}
	}
	if req.CurrentNode != nil {
		params.CurrentNode = pgtype.Text{String: *req.CurrentNode, Valid: true}
	}
	if len(req.NodesState) > 0 {
		params.NodesState = []byte(req.NodesState)
	}
	if req.Error != nil {
		params.Error = pgtype.Text{String: *req.Error, Valid: true}
	}

	run, err := h.Queries.UpdateWorkflowRunProgress(r.Context(), params)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "update workflow run: "+err.Error())
		return
	}
	if run.CaseID.Valid && req.Status != nil {
		if err := h.syncWorkflowCaseStatusForRunUpdate(r.Context(), run); err != nil {
			writeError(w, http.StatusInternalServerError, "update workflow case status: "+err.Error())
			return
		}
	}

	h.publishWorkflowRunUpdated(r.Context(), workspaceID, run)
	writeJSON(w, http.StatusOK, workflowRunToResponse(run))
}

func (h *Handler) syncWorkflowCaseStatusForRunUpdate(ctx context.Context, run db.WorkflowRun) error {
	status := ""
	switch run.Status {
	case "done":
		status = "succeeded"
	case "failed", "cancelled":
		status = run.Status
	case "running":
		status = "running"
	default:
		return nil
	}
	_, err := h.Queries.UpdateWorkflowCase(ctx, db.UpdateWorkflowCaseParams{
		ID:           run.CaseID,
		Status:       pgtype.Text{String: status, Valid: true},
		CurrentRunID: run.ID,
		UpdatedBy:    pgtype.Text{String: "workflow_run", Valid: true},
	})
	return err
}

func (h *Handler) CreateWorkflowRunNodeEvent(w http.ResponseWriter, r *http.Request) {
	workspaceID, ok := h.workflowWorkspaceID(w, r)
	if !ok {
		return
	}
	runID, ok := parseUUIDOrBadRequest(w, chi.URLParam(r, "runId"), "workflow run id")
	if !ok {
		return
	}
	nodeID := strings.TrimSpace(chi.URLParam(r, "nodeId"))
	if nodeID == "" {
		writeError(w, http.StatusBadRequest, "node id is required")
		return
	}

	var req workflowRunNodeEventRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	req.EventType = strings.TrimSpace(req.EventType)
	if req.EventType == "" {
		writeError(w, http.StatusBadRequest, "event_type is required")
		return
	}
	if req.Attempt <= 0 {
		req.Attempt = 1
	}

	tx, err := h.TxStarter.Begin(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "begin workflow node event transaction: "+err.Error())
		return
	}
	defer tx.Rollback(r.Context())
	qtx := h.Queries.WithTx(tx)

	run, err := getWorkflowRunForUpdate(r.Context(), tx, runID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeError(w, http.StatusNotFound, "workflow run not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "load workflow run: "+err.Error())
		return
	}
	if uuidToString(run.WorkspaceID) != uuidToString(workspaceID) {
		writeError(w, http.StatusForbidden, "workflow run is outside workspace")
		return
	}
	if !run.CaseID.Valid {
		writeError(w, http.StatusBadRequest, "workflow run has no case_id")
		return
	}
	workflowCase, err := qtx.GetWorkflowCase(r.Context(), run.CaseID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeError(w, http.StatusBadRequest, "workflow case not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "load workflow case: "+err.Error())
		return
	}
	if uuidToString(workflowCase.WorkspaceID) != uuidToString(workspaceID) {
		writeError(w, http.StatusForbidden, "workflow case is outside workspace")
		return
	}
	if !workflowDefinitionHasNode(run.DefinitionSnapshot, nodeID) {
		writeError(w, http.StatusBadRequest, "node not found in workflow definition")
		return
	}
	nextNodesState, err := applyNodeEventToNodesState(run.NodesState, nodeID, req)
	if err != nil {
		writeError(w, http.StatusBadRequest, "project nodes_state: "+err.Error())
		return
	}

	sequence := pgtype.Int4{}
	if req.Sequence != nil {
		sequence = pgtype.Int4{Int32: *req.Sequence, Valid: true}
	}
	occurredAt := pgtype.Timestamptz{}
	if req.OccurredAt != nil {
		occurredAt = pgtype.Timestamptz{Time: *req.OccurredAt, Valid: true}
	}
	if _, err := qtx.CreateWorkflowRunNodeEvent(r.Context(), db.CreateWorkflowRunNodeEventParams{
		WorkspaceID:    workspaceID,
		CaseID:         run.CaseID,
		RunID:          run.ID,
		NodeID:         nodeID,
		EventType:      req.EventType,
		Attempt:        req.Attempt,
		InputSnapshot:  rawOrNil(req.InputSnapshot),
		OutputSnapshot: rawOrNil(req.OutputSnapshot),
		Error:          rawOrNil(req.Error),
		Logs:           rawOrNil(req.Logs),
		CarrierRef:     rawOrNil(req.CarrierRef),
		Sequence:       sequence,
		OccurredAt:     occurredAt,
	}); err != nil {
		writeError(w, http.StatusInternalServerError, "create workflow run node event: "+err.Error())
		return
	}
	if _, err := qtx.ProjectWorkflowRunNodeEvent(r.Context(), db.ProjectWorkflowRunNodeEventParams{
		RunID:          run.ID,
		NodeID:         nodeID,
		EventType:      req.EventType,
		Attempt:        req.Attempt,
		InputSnapshot:  rawOrNil(req.InputSnapshot),
		OutputSnapshot: rawOrNil(req.OutputSnapshot),
		Error:          rawOrNil(req.Error),
		Logs:           rawOrNil(req.Logs),
		CarrierRef:     rawOrNil(req.CarrierRef),
	}); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeError(w, http.StatusNotFound, "workflow run node not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "project workflow run node event: "+err.Error())
		return
	}
	updatedRun, err := qtx.UpdateWorkflowRunProgress(r.Context(), db.UpdateWorkflowRunProgressParams{
		ID:         run.ID,
		NodesState: nextNodesState,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "update workflow run nodes_state: "+err.Error())
		return
	}
	if err := tx.Commit(r.Context()); err != nil {
		writeError(w, http.StatusInternalServerError, "commit workflow node event transaction: "+err.Error())
		return
	}

	h.publishWorkflowRunUpdated(r.Context(), workspaceID, updatedRun)
	resp, err := h.workflowRunToResponseWithSubIssueStatus(r.Context(), updatedRun)
	if err != nil {
		slog.Warn("workflow node event response: attach nodes failed", "run_id", uuidToString(updatedRun.ID), "error", err)
		writeJSON(w, http.StatusOK, workflowRunToResponse(updatedRun))
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

func getWorkflowRunForUpdate(ctx context.Context, tx pgx.Tx, id pgtype.UUID) (db.WorkflowRun, error) {
	row := tx.QueryRow(ctx, `
		SELECT id, workspace_id, root_issue_id, skill_id, planner_task_id, status,
			current_node, nodes_state, definition_snapshot, source_skills, error,
			cancel_reason, cancelled_at, started_at, completed_at, created_at,
			updated_at, case_id, definition_version_id
		FROM workflow_run
		WHERE id = $1
		FOR UPDATE
	`, id)
	var run db.WorkflowRun
	err := row.Scan(
		&run.ID,
		&run.WorkspaceID,
		&run.RootIssueID,
		&run.SkillID,
		&run.PlannerTaskID,
		&run.Status,
		&run.CurrentNode,
		&run.NodesState,
		&run.DefinitionSnapshot,
		&run.SourceSkills,
		&run.Error,
		&run.CancelReason,
		&run.CancelledAt,
		&run.StartedAt,
		&run.CompletedAt,
		&run.CreatedAt,
		&run.UpdatedAt,
		&run.CaseID,
		&run.DefinitionVersionID,
	)
	return run, err
}

func rawOrNil(raw json.RawMessage) []byte {
	if len(raw) == 0 {
		return nil
	}
	return []byte(raw)
}

func workflowDefinitionHasNode(raw []byte, nodeID string) bool {
	var definition struct {
		Nodes []map[string]any `json:"nodes"`
	}
	if err := json.Unmarshal(raw, &definition); err != nil {
		return false
	}
	for _, node := range definition.Nodes {
		if id, _ := node["id"].(string); id == nodeID {
			return true
		}
	}
	return false
}

func applyNodeEventToNodesState(raw []byte, nodeID string, event workflowRunNodeEventRequest) ([]byte, error) {
	nodesState := map[string]map[string]any{}
	if len(raw) > 0 && string(raw) != "null" {
		if err := json.Unmarshal(raw, &nodesState); err != nil {
			return nil, err
		}
	}
	node := nodesState[nodeID]
	if node == nil {
		node = map[string]any{}
	}

	switch event.EventType {
	case "node_started":
		node["status"] = "running"
	case "node_review_requested":
		node["status"] = "pending_review"
	case "node_blocked":
		node["status"] = "blocked"
	case "node_succeeded":
		node["status"] = "done"
	case "node_failed":
		node["status"] = "failed"
	case "node_cancelled":
		node["status"] = "cancelled"
	case "node_skipped":
		node["status"] = "cancelled"
		node["skipped"] = true
	case "node_carrier_attached":
		applyLegacyCarrierRef(node, event.CarrierRef)
	case "node_log_appended":
		applyLegacyLogs(node, event.Logs)
	default:
		return nil, fmt.Errorf("unsupported event_type %q", event.EventType)
	}

	if len(event.InputSnapshot) > 0 {
		node["input_snapshot"] = json.RawMessage(event.InputSnapshot)
	}
	if len(event.OutputSnapshot) > 0 {
		node["output_snapshot"] = json.RawMessage(event.OutputSnapshot)
	}
	if len(event.Error) > 0 {
		node["error"] = json.RawMessage(event.Error)
	}
	if len(event.CarrierRef) > 0 && event.EventType != "node_carrier_attached" {
		applyLegacyCarrierRef(node, event.CarrierRef)
	}
	if len(event.Logs) > 0 && event.EventType != "node_log_appended" {
		applyLegacyLogs(node, event.Logs)
	}

	nodesState[nodeID] = node
	return json.Marshal(nodesState)
}

func applyLegacyCarrierRef(node map[string]any, raw json.RawMessage) {
	if len(raw) == 0 {
		return
	}
	node["carrier_ref"] = json.RawMessage(raw)

	var ref map[string]any
	if err := json.Unmarshal(raw, &ref); err != nil {
		return
	}
	if refType, _ := ref["type"].(string); refType != "subissue" {
		return
	}
	if subIssueID, _ := ref["sub_issue_id"].(string); subIssueID != "" {
		node["sub_issue_id"] = subIssueID
	} else if subIssueID, _ := ref["subIssueId"].(string); subIssueID != "" {
		node["sub_issue_id"] = subIssueID
	}
	if taskID, _ := ref["task_id"].(string); taskID != "" {
		node["task_id"] = taskID
	} else if taskID, _ := ref["taskId"].(string); taskID != "" {
		node["task_id"] = taskID
	}
}

func applyLegacyLogs(node map[string]any, raw json.RawMessage) {
	if len(raw) == 0 {
		return
	}
	var existing []any
	switch current := node["logs"].(type) {
	case []any:
		existing = append(existing, current...)
	case json.RawMessage:
		_ = json.Unmarshal(current, &existing)
	}

	var incoming []any
	if err := json.Unmarshal(raw, &incoming); err != nil {
		node["logs"] = json.RawMessage(raw)
		return
	}
	node["logs"] = append(existing, incoming...)
}

type cancelWorkflowRunRequest struct {
	Reason string `json:"reason"`
}

type continueWorkflowRunRequest struct {
	Decision string `json:"decision"`
}

type createWorkflowMainNodeTaskRequest struct {
	NodeID   string `json:"node_id"`
	NodeType string `json:"node_type"`
	AgentID  string `json:"agent_id"`
}

func (h *Handler) CreateWorkflowMainNodeTask(w http.ResponseWriter, r *http.Request) {
	workspaceID, ok := h.workflowWorkspaceID(w, r)
	if !ok {
		return
	}
	runID, ok := parseUUIDOrBadRequest(w, chi.URLParam(r, "runId"), "workflow run id")
	if !ok {
		return
	}
	run, err := h.Queries.GetWorkflowRun(r.Context(), runID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeError(w, http.StatusNotFound, "workflow run not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "load workflow run: "+err.Error())
		return
	}
	if uuidToString(run.WorkspaceID) != uuidToString(workspaceID) {
		writeError(w, http.StatusForbidden, "workflow run is outside workspace")
		return
	}
	issue, err := h.Queries.GetIssue(r.Context(), run.RootIssueID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "load root issue: "+err.Error())
		return
	}
	var req createWorkflowMainNodeTaskRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	agentID := issue.AssigneeID
	if strings.TrimSpace(req.AgentID) != "" {
		parsed, ok := parseUUIDOrBadRequest(w, req.AgentID, "agent_id")
		if !ok {
			return
		}
		agentID = parsed
	} else if !issue.AssigneeID.Valid || !issue.AssigneeType.Valid || issue.AssigneeType.String != "agent" {
		writeError(w, http.StatusBadRequest, "root issue must be assigned to an agent or agent_id must be provided")
		return
	}
	nodeID := strings.TrimSpace(req.NodeID)
	if nodeID == "" {
		writeError(w, http.StatusBadRequest, "node_id is required")
		return
	}
	nodeType := strings.TrimSpace(req.NodeType)
	if nodeType == "" {
		nodeType = "main_agent"
	}
	task, err := h.TaskService.EnqueueWorkflowMainNodeTask(r.Context(), issue, agentID, run.ID, nodeID, nodeType)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "create workflow main node task: "+err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, map[string]string{"task_id": uuidToString(task.ID)})
}

// Deprecated: global workflow-run cancellation is not the canonical product
// control path. Product clients must use the WorkflowCase-scoped cancel route.
// This handler remains for internal/debug compatibility and shared side-effect
// behavior.
func (h *Handler) CancelWorkflowRun(w http.ResponseWriter, r *http.Request) {
	workspaceID, ok := h.workflowWorkspaceID(w, r)
	if !ok {
		return
	}
	runID, ok := parseUUIDOrBadRequest(w, chi.URLParam(r, "runId"), "workflow run id")
	if !ok {
		return
	}

	existing, err := h.Queries.GetWorkflowRun(r.Context(), runID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeError(w, http.StatusNotFound, "workflow run not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "load workflow run: "+err.Error())
		return
	}
	if uuidToString(existing.WorkspaceID) != uuidToString(workspaceID) {
		writeError(w, http.StatusForbidden, "workflow run is outside workspace")
		return
	}

	var req cancelWorkflowRunRequest
	_ = json.NewDecoder(r.Body).Decode(&req)
	reason := strings.TrimSpace(req.Reason)
	if reason == "" {
		reason = "workflow stopped by user"
	}

	run, err := h.cancelWorkflowRun(r.Context(), existing, reason)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "cancel workflow run: "+err.Error())
		return
	}

	h.publishWorkflowRunUpdated(r.Context(), workspaceID, run)
	writeJSON(w, http.StatusOK, workflowRunToResponse(run))
}

func (h *Handler) cancelWorkflowRun(ctx context.Context, existing db.WorkflowRun, reason string) (db.WorkflowRun, error) {
	run, err := h.Queries.CancelWorkflowRun(ctx, db.CancelWorkflowRunParams{
		ID:           existing.ID,
		CancelReason: pgtype.Text{String: reason, Valid: true},
	})
	if err != nil {
		return db.WorkflowRun{}, err
	}

	for _, childIssueID := range childIssueIDsFromNodesState(existing.NodesState) {
		if err := h.TaskService.CancelTasksForIssue(ctx, childIssueID); err != nil {
			slog.Warn("cancel workflow child tasks failed",
				"workflow_run_id", uuidToString(existing.ID),
				"child_issue_id", uuidToString(childIssueID),
				"error", err)
		}
	}
	originChildren, err := h.Queries.ListIssuesByOrigin(ctx, db.ListIssuesByOriginParams{
		WorkspaceID: existing.WorkspaceID,
		OriginType:  pgtype.Text{String: "workflow_node", Valid: true},
		OriginID:    existing.ID,
	})
	if err != nil {
		slog.Warn("list workflow children by origin failed",
			"workflow_run_id", uuidToString(existing.ID),
			"error", err)
	}
	for _, child := range originChildren {
		if err := h.TaskService.CancelTasksForIssue(ctx, child.ID); err != nil {
			slog.Warn("cancel workflow origin child tasks failed",
				"workflow_run_id", uuidToString(existing.ID),
				"child_issue_id", uuidToString(child.ID),
				"error", err)
		}
	}

	return run, nil
}

// Deprecated: global workflow-run continuation is not the canonical product
// control path. Product clients must use WorkflowCase-scoped replan/continue
// APIs when those are available. This handler remains for internal/debug
// compatibility only.
func (h *Handler) ContinueWorkflowRun(w http.ResponseWriter, r *http.Request) {
	workspaceID, ok := h.workflowWorkspaceID(w, r)
	if !ok {
		return
	}
	runID, ok := parseUUIDOrBadRequest(w, chi.URLParam(r, "runId"), "workflow run id")
	if !ok {
		return
	}
	previous, err := h.Queries.GetWorkflowRun(r.Context(), runID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeError(w, http.StatusNotFound, "workflow run not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "load workflow run: "+err.Error())
		return
	}
	if uuidToString(previous.WorkspaceID) != uuidToString(workspaceID) {
		writeError(w, http.StatusForbidden, "workflow run is outside workspace")
		return
	}
	if !previous.SkillID.Valid {
		writeError(w, http.StatusBadRequest, "workflow run has no source skill")
		return
	}

	var req continueWorkflowRunRequest
	_ = json.NewDecoder(r.Body).Decode(&req)
	decision := strings.TrimSpace(req.Decision)
	if decision == "" {
		decision = "continue workflow after human approval"
	}
	initial := map[string]any{
		"task":                  "Continue workflow run " + uuidToString(previous.ID),
		"previous_run_id":       uuidToString(previous.ID),
		"continuation_decision": decision,
		"previous_status":       previous.Status,
		"previous_current_node": previous.CurrentNode,
		"previous_nodes_state":  json.RawMessage(previous.NodesState),
	}
	h.startIssueWorkflowRunFromSkill(w, r, workspaceID, previous.RootIssueID, previous.SkillID, initial)
}

func childIssueIDsFromNodesState(raw []byte) []pgtype.UUID {
	var nodes map[string]map[string]any
	if err := json.Unmarshal(raw, &nodes); err != nil {
		return nil
	}
	seen := map[string]bool{}
	var out []pgtype.UUID
	for _, node := range nodes {
		value, _ := node["sub_issue_id"].(string)
		if value == "" {
			value, _ = node["subIssueId"].(string)
		}
		if value == "" || seen[value] {
			continue
		}
		id := parseUUID(value)
		if !id.Valid {
			continue
		}
		seen[value] = true
		out = append(out, id)
	}
	return out
}

func skillWorkflowConfigIsRunnable(raw []byte) (bool, []string) {
	if len(raw) == 0 {
		return true, nil
	}
	var cfg map[string]any
	if err := json.Unmarshal(raw, &cfg); err != nil {
		return false, []string{"invalid skill config"}
	}
	validation, _ := cfg["workflow_validation"].(map[string]any)
	if validation == nil {
		return true, nil
	}
	valid, ok := validation["valid"].(bool)
	if !ok || valid {
		return true, nil
	}
	out := []string{}
	if errorsRaw, ok := validation["errors"].([]any); ok {
		for _, item := range errorsRaw {
			if s, ok := item.(string); ok && strings.TrimSpace(s) != "" {
				out = append(out, s)
			}
		}
	}
	if len(out) == 0 {
		out = append(out, "invalid workflow")
	}
	return false, out
}

func (h *Handler) workflowWorkspaceID(w http.ResponseWriter, r *http.Request) (pgtype.UUID, bool) {
	workspaceID := h.resolveWorkspaceID(r)
	if workspaceID == "" {
		writeError(w, http.StatusBadRequest, "workspace_id required")
		return pgtype.UUID{}, false
	}
	return parseUUIDOrBadRequest(w, workspaceID, "workspace_id")
}

func (h *Handler) publishWorkflowRunUpdated(ctx context.Context, workspaceID pgtype.UUID, run db.WorkflowRun) {
	resp, err := h.workflowRunToResponseWithSubIssueStatus(ctx, run)
	if err != nil {
		slog.Warn("publish workflow run: overlay status failed", "run_id", uuidToString(run.ID), "error", err)
		resp = workflowRunToResponse(run)
	}
	h.publish(protocol.EventWorkflowRunUpdated, uuidToString(workspaceID), "system", "", map[string]any{
		"workflow_run": resp,
	})
}

func (h *Handler) notifyWorkflowRunOfSubIssueStatus(ctx context.Context, workspaceIDStr string, runID pgtype.UUID) {
	workspaceID := parseUUID(workspaceIDStr)
	if !workspaceID.Valid {
		slog.Warn("notify workflow run: invalid workspace id", "workspace_id", workspaceIDStr)
		return
	}
	run, err := h.Queries.GetWorkflowRun(ctx, runID)
	if err != nil {
		slog.Warn("notify workflow run: load run failed", "run_id", uuidToString(runID), "error", err)
		return
	}
	resp, err := h.workflowRunToResponseWithSubIssueStatus(ctx, run)
	if err != nil {
		slog.Warn("notify workflow run: overlay status failed", "run_id", uuidToString(runID), "error", err)
		return
	}
	h.publish(protocol.EventWorkflowRunUpdated, uuidToString(workspaceID), "system", "", map[string]any{
		"workflow_run": resp,
	})
}

type startIssueWorkflowRunRequest struct {
	SkillID      string         `json:"skill_id"`
	InitialState map[string]any `json:"initial_state"`
}

// Deprecated: issue-first workflow control is not canonical. Product UI,
// agent brief, and new clients must use WorkflowCase-scoped APIs. This handler
// remains only for migration/debug compatibility.
func (h *Handler) StartIssueWorkflowRun(w http.ResponseWriter, r *http.Request) {
	workspaceID, ok := h.workflowWorkspaceID(w, r)
	if !ok {
		return
	}
	issueID, ok := parseUUIDOrBadRequest(w, chi.URLParam(r, "id"), "issue id")
	if !ok {
		return
	}
	issue, err := h.Queries.GetIssue(r.Context(), issueID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeError(w, http.StatusNotFound, "issue not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "load issue: "+err.Error())
		return
	}
	if uuidToString(issue.WorkspaceID) != uuidToString(workspaceID) {
		writeError(w, http.StatusForbidden, "issue is outside workspace")
		return
	}

	var req startIssueWorkflowRunRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	skillID, ok := parseUUIDOrBadRequest(w, req.SkillID, "skill_id")
	if !ok {
		return
	}
	h.startIssueWorkflowRunFromSkill(w, r, workspaceID, issueID, skillID, req.InitialState)
}

func (h *Handler) startIssueWorkflowRunFromSkill(w http.ResponseWriter, r *http.Request, workspaceID, issueID, skillID pgtype.UUID, initialState map[string]any) {
	issue, err := h.Queries.GetIssue(r.Context(), issueID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeError(w, http.StatusNotFound, "issue not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "load issue: "+err.Error())
		return
	}
	if uuidToString(issue.WorkspaceID) != uuidToString(workspaceID) {
		writeError(w, http.StatusForbidden, "issue is outside workspace")
		return
	}
	skill, err := h.Queries.GetSkillInWorkspace(r.Context(), db.GetSkillInWorkspaceParams{ID: skillID, WorkspaceID: workspaceID})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeError(w, http.StatusNotFound, "skill not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "load skill: "+err.Error())
		return
	}
	if ok, errors := skillWorkflowConfigIsRunnable(skill.Config); !ok {
		writeError(w, http.StatusBadRequest, "workflow validation failed: "+strings.Join(errors, "; "))
		return
	}

	files, err := h.Queries.ListSkillFiles(r.Context(), skillID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "load skill files: "+err.Error())
		return
	}
	workflowYAML := ""
	for _, f := range files {
		if f.Path == workflowFilePath {
			workflowYAML = f.Content
			break
		}
	}
	if workflowYAML == "" {
		writeError(w, http.StatusBadRequest, "skill has no workflow.yaml")
		return
	}
	if validation := validateSkillWorkflowYAML(workflowYAML); !validation.Valid {
		writeError(w, http.StatusBadRequest, "workflow validation failed: "+strings.Join(validation.Errors, "; "))
		return
	}
	var definition map[string]any
	if err := yaml.Unmarshal([]byte(workflowYAML), &definition); err != nil {
		writeError(w, http.StatusBadRequest, "workflow.yaml must be valid YAML: "+err.Error())
		return
	}
	definitionSnapshot, _ := json.Marshal(definition)
	sourceSkills, _ := json.Marshal(definition["source_skills"])
	if string(sourceSkills) == "null" {
		sourceSkills = []byte(`[]`)
	}

	initial := initialState
	if initial == nil {
		initial = map[string]any{}
	}
	if _, ok := initial["task"]; !ok {
		initial["task"] = issue.Title
	}
	run, err := h.Queries.CreateWorkflowRun(r.Context(), db.CreateWorkflowRunParams{
		WorkspaceID:        workspaceID,
		RootIssueID:        issueID,
		SkillID:            skillID,
		Status:             "running",
		CurrentNode:        "START",
		NodesState:         []byte(`{}`),
		DefinitionSnapshot: definitionSnapshot,
		SourceSkills:       sourceSkills,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "create workflow run: "+err.Error())
		return
	}
	payload := map[string]any{
		"run_id":        uuidToString(run.ID),
		"workspace_id":  uuidToString(workspaceID),
		"root_issue_id": uuidToString(issueID),
		"skill_id":      uuidToString(skillID),
		"workflow_yaml": workflowYAML,
		"initial_state": initial,
	}
	raw, _ := json.Marshal(payload)
	sidecarURL := strings.TrimRight(os.Getenv("MULTICA_WORKFLOW_SIDECAR_URL"), "/")
	if sidecarURL == "" {
		sidecarURL = "http://localhost:18787"
	}
	resp, err := http.Post(sidecarURL+"/runs", "application/json", bytes.NewReader(raw))
	if err != nil {
		writeError(w, http.StatusBadGateway, "workflow sidecar unavailable: "+err.Error())
		return
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		_, _ = h.Queries.UpdateWorkflowRunProgress(r.Context(), db.UpdateWorkflowRunProgressParams{
			ID:     run.ID,
			Status: pgtype.Text{String: "failed", Valid: true},
			Error:  pgtype.Text{String: fmt.Sprintf("workflow sidecar returned %d: %s", resp.StatusCode, string(body)), Valid: true},
		})
		writeError(w, http.StatusBadGateway, fmt.Sprintf("workflow sidecar returned %d: %s", resp.StatusCode, string(body)))
		return
	}
	h.publishWorkflowRunUpdated(r.Context(), workspaceID, run)
	writeJSON(w, http.StatusAccepted, workflowRunToResponse(run))
}
