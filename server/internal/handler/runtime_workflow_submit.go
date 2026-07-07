package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

type submitRuntimeWorkflowRequest struct {
	PlannerTaskID string          `json:"planner_task_id"`
	Definition    json.RawMessage `json:"definition"`
	InitialState  map[string]any  `json:"initial_state"`
}

type startRuntimeWorkflowParams struct {
	WorkspaceID         pgtype.UUID
	CaseID              pgtype.UUID
	RootIssueID         pgtype.UUID
	PlannerTaskID       pgtype.UUID
	DefinitionVersionID pgtype.UUID
	Definition          json.RawMessage
	InitialState        map[string]any
	SourceSkills        []byte
	RunKind             string
	Label               string
	// ProjectCaseStatus keeps the legacy behavior of mirroring run status onto
	// the case. Only the debug/compatibility submit path sets it. The atomic
	// WorkflowCase control plane leaves it false so case status stays
	// active/archived and never projects a run.
	ProjectCaseStatus bool
}

type runtimeWorkflowDefinition struct {
	Meta         map[string]any        `json:"meta"`
	SourceSkills []map[string]any      `json:"source_skills"`
	State        map[string]any        `json:"state"`
	Nodes        []runtimeWorkflowNode `json:"nodes"`
	Routing      []map[string]any      `json:"routing"`
}

type runtimeWorkflowNode struct {
	ID                   string         `json:"id"`
	Type                 string         `json:"type"`
	Dispatch             string         `json:"dispatch"`
	Carrier              string         `json:"carrier"`
	CarrierKind          string         `json:"carrier_kind"`
	Agent                string         `json:"agent"`
	SourceSkillID        string         `json:"source_skill_id"`
	SourceSkillName      string         `json:"source_skill_name"`
	SourceNodeID         string         `json:"source_node_id"`
	ModifiedFromTemplate bool           `json:"modified_from_template"`
	Config               map[string]any `json:"config"`
	Outputs              []string       `json:"outputs"`
}

var runtimeWorkflowAllowedNodeTypes = map[string]bool{
	"agent":          true,
	"main_agent":     true,
	"subissue":       true,
	"condition":      true,
	"merge":          true,
	"final_response": true,
}

var runtimeWorkflowAllowedDispatches = map[string]bool{
	"subissue":        true,
	"direct_subagent": true,
	"inline":          true,
	"main_issue_task": true,
}

var runtimeWorkflowAllowedCarrierKinds = map[string]bool{
	"issue":         true,
	"issue_task":    true,
	"agent_runtime": true,
	"inline":        true,
}

var errWorkflowSidecarUnavailable = errors.New("workflow sidecar unavailable")

var workflowRunKinds = map[string]bool{
	"primary":    true,
	"experiment": true,
	"shadow":     true,
	"replay":     true,
	"debug":      true,
}

func normalizeWorkflowRunKind(kind string) string {
	kind = strings.TrimSpace(kind)
	if kind == "" {
		return "primary"
	}
	return kind
}

func validateRuntimeWorkflowDefinition(raw json.RawMessage) (runtimeWorkflowDefinition, error) {
	var def runtimeWorkflowDefinition
	if len(raw) == 0 {
		return def, errors.New("definition required")
	}
	if err := json.Unmarshal(raw, &def); err != nil {
		return def, fmt.Errorf("definition must be valid JSON: %w", err)
	}
	if len(def.Nodes) == 0 {
		return def, errors.New("definition.nodes must contain at least one node")
	}
	ids := map[string]bool{}
	for i := range def.Nodes {
		node := &def.Nodes[i]
		if strings.TrimSpace(node.ID) == "" {
			return def, errors.New("node.id required")
		}
		if ids[node.ID] {
			return def, fmt.Errorf("duplicate node id %q", node.ID)
		}
		ids[node.ID] = true
		if !runtimeWorkflowAllowedNodeTypes[node.Type] {
			return def, fmt.Errorf("unsupported node type %q", node.Type)
		}
		dispatch, carrierKind, err := normalizeRuntimeWorkflowCarrier(*node)
		if err != nil {
			return def, err
		}
		node.Dispatch = dispatch
		node.CarrierKind = carrierKind
		if !runtimeWorkflowAllowedDispatches[dispatch] {
			return def, fmt.Errorf("unsupported dispatch %q", dispatch)
		}
		if dispatch == "direct_subagent" {
			return def, errors.New("dispatch direct_subagent is reserved for TraeX integration and is not implemented yet")
		}
		if dispatch == "subissue" && strings.TrimSpace(node.Agent) == "" {
			return def, fmt.Errorf("subissue node %q requires agent", node.ID)
		}
		if dispatch == "main_issue_task" && node.Type != "main_agent" && node.Type != "final_response" {
			return def, fmt.Errorf("main_issue_task node %q must use type main_agent or final_response", node.ID)
		}
	}
	if len(def.Routing) == 0 {
		return def, errors.New("definition.routing required")
	}
	return def, nil
}

func normalizeRuntimeWorkflowCarrier(node runtimeWorkflowNode) (string, string, error) {
	dispatch := strings.TrimSpace(node.Dispatch)
	carrier := strings.TrimSpace(node.Carrier)
	carrierKind := strings.TrimSpace(node.CarrierKind)
	if carrier != "" {
		if !runtimeWorkflowAllowedCarrierKinds[carrier] {
			return "", "", fmt.Errorf("unsupported carrier %q", carrier)
		}
	}
	if carrierKind != "" {
		if !runtimeWorkflowAllowedCarrierKinds[carrierKind] {
			return "", "", fmt.Errorf("unsupported carrier_kind %q", carrierKind)
		}
	}
	if carrier != "" && carrierKind != "" && carrier != carrierKind {
		return "", "", fmt.Errorf("conflicting carrier %q and carrier_kind %q for node %q", carrier, carrierKind, node.ID)
	}
	if carrierKind == "" {
		carrierKind = carrier
	}
	if carrierKind != "" {
		carrierDispatch := dispatchFromCarrierKind(carrierKind)
		if dispatch != "" && dispatch != carrierDispatch {
			return "", "", fmt.Errorf("conflicting carrier_kind %q and dispatch %q for node %q", carrierKind, dispatch, node.ID)
		}
		dispatch = carrierDispatch
	}
	if dispatch == "" {
		dispatch = inferRuntimeWorkflowDispatch(node.Type)
	}
	if carrierKind == "" {
		carrierKind = carrierKindFromDispatch(dispatch)
	}
	return dispatch, carrierKind, nil
}

func inferRuntimeWorkflowDispatch(nodeType string) string {
	switch nodeType {
	case "agent", "subissue", "llm":
		return "subissue"
	case "main_agent", "final_response":
		return "main_issue_task"
	default:
		return "inline"
	}
}

func dispatchFromCarrierKind(carrierKind string) string {
	switch carrierKind {
	case "issue":
		return "subissue"
	case "issue_task":
		return "main_issue_task"
	case "agent_runtime":
		return "direct_subagent"
	default:
		return "inline"
	}
}

func carrierKindFromDispatch(dispatch string) string {
	switch dispatch {
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

// validateRuntimeWorkflowSourceSkills rejects any node that declares a
// source_skill_id which is not bound to the main agent (the allowed set).
// Nodes with no source_skill_id (freshly authored, not derived from a
// template) are allowed.
func validateRuntimeWorkflowSourceSkills(def runtimeWorkflowDefinition, allowed map[string]bool) error {
	for _, node := range def.Nodes {
		sid := strings.TrimSpace(node.SourceSkillID)
		if sid == "" {
			continue // 允许无来源的自定义节点
		}
		if !allowed[sid] {
			return fmt.Errorf("node %q references source_skill_id %q not bound to the main agent", node.ID, sid)
		}
	}
	return nil
}

// Deprecated: issue-first workflow control is not canonical. Product UI,
// agent brief, and new clients must use WorkflowCase-scoped APIs. This handler
// remains only for migration/debug compatibility.
func (h *Handler) SubmitRuntimeWorkflow(w http.ResponseWriter, r *http.Request) {
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

	var req submitRuntimeWorkflowRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	def, err := validateRuntimeWorkflowDefinition(req.Definition)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	normalizedDefinition, err := json.Marshal(def)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "normalize workflow definition: "+err.Error())
		return
	}
	req.Definition = normalizedDefinition

	allowedSkills := map[string]bool{}
	if issue.AssigneeID.Valid && issue.AssigneeType.Valid && issue.AssigneeType.String == "agent" {
		skills, err := h.Queries.ListAgentSkills(r.Context(), issue.AssigneeID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "load agent skills: "+err.Error())
			return
		}
		for _, s := range skills {
			allowedSkills[uuidToString(s.ID)] = true
		}
	}
	if err := validateRuntimeWorkflowSourceSkills(def, allowedSkills); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	sourceSkills, _ := json.Marshal(def.SourceSkills)
	plannerTaskID := pgtype.UUID{}
	if req.PlannerTaskID != "" {
		parsed, ok := parseUUIDOrBadRequest(w, req.PlannerTaskID, "planner_task_id")
		if !ok {
			return
		}
		plannerTaskID = parsed
	}

	c, err := h.workflowCaseForRuntimeWorkflowSubmit(r.Context(), workspaceID, issue)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "prepare workflow case: "+err.Error())
		return
	}
	definition, err := h.Queries.UpsertWorkflowDefinitionDraft(r.Context(), db.UpsertWorkflowDefinitionDraftParams{
		WorkspaceID:     workspaceID,
		CaseID:          c.ID,
		DraftJson:       []byte(req.Definition),
		SourceTemplates: []byte(`[]`),
		UpdatedBy:       pgtype.Text{String: "runtime_workflow_submit", Valid: true},
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "upsert workflow definition draft: "+err.Error())
		return
	}
	validationReport, _ := json.Marshal(workflowDefinitionValidationResponse{
		Valid:    true,
		Errors:   []workflowDefinitionValidationIssue{},
		Warnings: []workflowDefinitionValidationIssue{},
	})
	version, err := h.Queries.CreateWorkflowDefinitionVersion(r.Context(), db.CreateWorkflowDefinitionVersionParams{
		WorkspaceID:      workspaceID,
		CaseID:           c.ID,
		DefinitionID:     definition.ID,
		SnapshotJson:     definition.DraftJson,
		SourceSkills:     sourceSkills,
		ValidationReport: validationReport,
		ConfirmedBy:      pgtype.Text{String: "runtime_workflow_submit", Valid: true},
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "create workflow definition version: "+err.Error())
		return
	}

	run, err := h.createAndStartWorkflowRun(r.Context(), startRuntimeWorkflowParams{
		WorkspaceID:         workspaceID,
		CaseID:              c.ID,
		RootIssueID:         issueID,
		PlannerTaskID:       plannerTaskID,
		DefinitionVersionID: version.ID,
		Definition:          req.Definition,
		InitialState:        req.InitialState,
		SourceSkills:        sourceSkills,
		ProjectCaseStatus:   true,
	})
	if err != nil {
		if run.ID.Valid && errors.Is(err, errWorkflowSidecarUnavailable) {
			writeError(w, http.StatusBadGateway, err.Error())
			return
		}
		writeError(w, http.StatusInternalServerError, "create workflow run: "+err.Error())
		return
	}

	writeJSON(w, http.StatusAccepted, workflowRunToResponse(run))
}

func (h *Handler) workflowCaseForRuntimeWorkflowSubmit(ctx context.Context, workspaceID pgtype.UUID, issue db.Issue) (db.WorkflowCase, error) {
	c, err := h.Queries.GetWorkflowCaseBySourceIssue(ctx, issue.ID)
	if err == nil {
		if uuidToString(c.WorkspaceID) != uuidToString(workspaceID) {
			return db.WorkflowCase{}, errors.New("workflow case is outside workspace")
		}
		return c, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return db.WorkflowCase{}, err
	}
	return h.Queries.CreateWorkflowCase(ctx, db.CreateWorkflowCaseParams{
		WorkspaceID:   workspaceID,
		Title:         issue.Title,
		Description:   textValue(issue.Description),
		SourceIssueID: issue.ID,
		OwnerAgentID:  pgtype.UUID{},
		Status:        "draft",
		CreatedBy:     pgtype.Text{String: "runtime_workflow_submit", Valid: true},
		UpdatedBy:     pgtype.Text{String: "runtime_workflow_submit", Valid: true},
	})
}

func (h *Handler) createAndStartWorkflowRun(ctx context.Context, params startRuntimeWorkflowParams) (db.WorkflowRun, error) {
	if !params.CaseID.Valid {
		return db.WorkflowRun{}, errors.New("case_id required")
	}
	def, err := validateRuntimeWorkflowDefinition(params.Definition)
	if err != nil {
		return db.WorkflowRun{}, err
	}
	if params.SourceSkills == nil {
		params.SourceSkills, _ = json.Marshal(def.SourceSkills)
	}

	run, err := h.Queries.CreateWorkflowRun(ctx, db.CreateWorkflowRunParams{
		WorkspaceID:         params.WorkspaceID,
		RootIssueID:         params.RootIssueID,
		PlannerTaskID:       params.PlannerTaskID,
		Status:              "running",
		CurrentNode:         "START",
		NodesState:          []byte(`{}`),
		DefinitionSnapshot:  []byte(params.Definition),
		SourceSkills:        params.SourceSkills,
		CaseID:              params.CaseID,
		DefinitionVersionID: params.DefinitionVersionID,
		RunKind:             normalizeWorkflowRunKind(params.RunKind),
		Label:               strings.TrimSpace(params.Label),
	})
	if err != nil {
		return db.WorkflowRun{}, err
	}

	for _, node := range def.Nodes {
		if _, err := h.Queries.CreateWorkflowRunNode(ctx, db.CreateWorkflowRunNodeParams{
			WorkspaceID: params.WorkspaceID,
			CaseID:      params.CaseID,
			RunID:       run.ID,
			NodeID:      node.ID,
			NodeType:    node.Type,
			Dispatch:    node.Dispatch,
			CarrierKind: node.CarrierKind,
			Status:      "pending",
		}); err != nil {
			return run, fmt.Errorf("create workflow run node %q: %w", node.ID, err)
		}
	}

	if err := h.startSidecarRun(ctx, sidecarRunParams{
		WorkspaceID:  params.WorkspaceID,
		CaseID:       params.CaseID,
		RootIssueID:  params.RootIssueID,
		RunID:        run.ID,
		Definition:   params.Definition,
		InitialState: params.InitialState,
		RunKind:      run.RunKind,
		Label:        run.Label,
	}); err != nil {
		sidecarErr := fmt.Errorf("%w: %v", errWorkflowSidecarUnavailable, err)
		if _, updateErr := h.Queries.UpdateWorkflowRunProgress(ctx, db.UpdateWorkflowRunProgressParams{
			ID:     run.ID,
			Status: pgtype.Text{String: "failed", Valid: true},
			Error:  pgtype.Text{String: sidecarErr.Error(), Valid: true},
		}); updateErr != nil {
			return run, fmt.Errorf("%w; mark workflow run failed: %v", sidecarErr, updateErr)
		}
		if params.ProjectCaseStatus {
			if _, updateErr := h.Queries.UpdateWorkflowCase(ctx, db.UpdateWorkflowCaseParams{
				ID:           params.CaseID,
				Status:       pgtype.Text{String: "failed", Valid: true},
				CurrentRunID: run.ID,
			}); updateErr != nil {
				return run, fmt.Errorf("%w; mark workflow case failed: %v", sidecarErr, updateErr)
			}
		}
		return run, sidecarErr
	}

	if params.ProjectCaseStatus {
		if _, err := h.Queries.UpdateWorkflowCase(ctx, db.UpdateWorkflowCaseParams{
			ID:           params.CaseID,
			Status:       pgtype.Text{String: "running", Valid: true},
			CurrentRunID: run.ID,
		}); err != nil {
			return run, fmt.Errorf("update workflow case current run: %w", err)
		}
	}

	h.publishWorkflowRunUpdated(ctx, params.WorkspaceID, run)
	return run, nil
}

type sidecarRunParams struct {
	WorkspaceID  pgtype.UUID
	CaseID       pgtype.UUID
	RootIssueID  pgtype.UUID
	RunID        pgtype.UUID
	Definition   json.RawMessage
	InitialState map[string]any
	RunKind      string
	Label        string
}

func (h *Handler) startSidecarRun(ctx context.Context, p sidecarRunParams) error {
	initial := p.InitialState
	if initial == nil {
		initial = map[string]any{}
	}
	payload := map[string]any{
		"run_id":        uuidToString(p.RunID),
		"workspace_id":  uuidToString(p.WorkspaceID),
		"case_id":       uuidToString(p.CaseID),
		"definition":    json.RawMessage(p.Definition),
		"initial_state": initial,
		"run_kind":      p.RunKind,
		"label":         p.Label,
	}
	if p.RootIssueID.Valid {
		payload["root_issue_id"] = uuidToString(p.RootIssueID)
	}
	raw, _ := json.Marshal(payload)
	sidecarURL := strings.TrimRight(os.Getenv("MULTICA_WORKFLOW_SIDECAR_URL"), "/")
	if sidecarURL == "" {
		sidecarURL = "http://localhost:18787"
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, sidecarURL+"/runs", bytes.NewReader(raw))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("sidecar returned %d: %s", resp.StatusCode, string(body))
	}
	return nil
}
