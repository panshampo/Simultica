package handler

import (
	"bytes"
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
	ID                 string          `json:"id"`
	RootIssueID        string          `json:"root_issue_id"`
	SkillID            *string         `json:"skill_id"`
	PlannerTaskID      *string         `json:"planner_task_id,omitempty"`
	Status             string          `json:"status"`
	CurrentNode        string          `json:"current_node"`
	NodesState         json.RawMessage `json:"nodes_state"`
	DefinitionSnapshot json.RawMessage `json:"definition_snapshot"`
	SourceSkills       json.RawMessage `json:"source_skills"`
	Error              *string         `json:"error"`
	CancelReason       *string         `json:"cancel_reason,omitempty"`
	CancelledAt        *time.Time      `json:"cancelled_at,omitempty"`
	StartedAt          *time.Time      `json:"started_at,omitempty"`
	CompletedAt        *time.Time      `json:"completed_at,omitempty"`
	CreatedAt          time.Time       `json:"created_at"`
	UpdatedAt          time.Time       `json:"updated_at"`
}

func workflowRunToResponse(run db.WorkflowRun) WorkflowRunResponse {
	resp := WorkflowRunResponse{
		ID:                 uuidToString(run.ID),
		RootIssueID:        uuidToString(run.RootIssueID),
		Status:             run.Status,
		CurrentNode:        run.CurrentNode,
		NodesState:         json.RawMessage(run.NodesState),
		DefinitionSnapshot: json.RawMessage(run.DefinitionSnapshot),
		SourceSkills:       json.RawMessage(run.SourceSkills),
		CreatedAt:          run.CreatedAt.Time,
		UpdatedAt:          run.UpdatedAt.Time,
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
	writeJSON(w, http.StatusOK, workflowRunToResponse(run))
}

type createWorkflowRunRequest struct {
	RootIssueID        string          `json:"root_issue_id"`
	SkillID            string          `json:"skill_id"`
	DefinitionSnapshot json.RawMessage `json:"definition_snapshot"`
	CurrentNode        string          `json:"current_node"`
}

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

	h.publishWorkflowRunUpdated(workspaceID, run)
	writeJSON(w, http.StatusCreated, workflowRunToResponse(run))
}

type updateWorkflowRunRequest struct {
	Status      *string         `json:"status"`
	CurrentNode *string         `json:"current_node"`
	NodesState  json.RawMessage `json:"nodes_state"`
	Error       *string         `json:"error"`
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

	h.publishWorkflowRunUpdated(workspaceID, run)
	writeJSON(w, http.StatusOK, workflowRunToResponse(run))
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

	run, err := h.Queries.CancelWorkflowRun(r.Context(), db.CancelWorkflowRunParams{
		ID:           runID,
		CancelReason: pgtype.Text{String: reason, Valid: true},
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "cancel workflow run: "+err.Error())
		return
	}

	for _, childIssueID := range childIssueIDsFromNodesState(existing.NodesState) {
		if err := h.TaskService.CancelTasksForIssue(r.Context(), childIssueID); err != nil {
			slog.Warn("cancel workflow child tasks failed",
				"workflow_run_id", uuidToString(runID),
				"child_issue_id", uuidToString(childIssueID),
				"error", err)
		}
	}
	originChildren, err := h.Queries.ListIssuesByOrigin(r.Context(), db.ListIssuesByOriginParams{
		WorkspaceID: existing.WorkspaceID,
		OriginType:  pgtype.Text{String: "workflow_node", Valid: true},
		OriginID:    runID,
	})
	if err != nil {
		slog.Warn("list workflow children by origin failed",
			"workflow_run_id", uuidToString(runID),
			"error", err)
	}
	for _, child := range originChildren {
		if err := h.TaskService.CancelTasksForIssue(r.Context(), child.ID); err != nil {
			slog.Warn("cancel workflow origin child tasks failed",
				"workflow_run_id", uuidToString(runID),
				"child_issue_id", uuidToString(child.ID),
				"error", err)
		}
	}

	h.publishWorkflowRunUpdated(workspaceID, run)
	writeJSON(w, http.StatusOK, workflowRunToResponse(run))
}

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

func (h *Handler) publishWorkflowRunUpdated(workspaceID pgtype.UUID, run db.WorkflowRun) {
	h.publish(protocol.EventWorkflowRunUpdated, uuidToString(workspaceID), "system", "", map[string]any{
		"workflow_run": workflowRunToResponse(run),
	})
}

type startIssueWorkflowRunRequest struct {
	SkillID      string         `json:"skill_id"`
	InitialState map[string]any `json:"initial_state"`
}

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
	h.publishWorkflowRunUpdated(workspaceID, run)
	writeJSON(w, http.StatusAccepted, workflowRunToResponse(run))
}
