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
	for _, node := range def.Nodes {
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
		dispatch := strings.TrimSpace(node.Dispatch)
		if dispatch == "" {
			dispatch = inferRuntimeWorkflowDispatch(node.Type)
		}
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

	run, err := h.Queries.CreateWorkflowRun(r.Context(), db.CreateWorkflowRunParams{
		WorkspaceID:        workspaceID,
		RootIssueID:        issueID,
		PlannerTaskID:      plannerTaskID,
		Status:             "running",
		CurrentNode:        "START",
		NodesState:         []byte(`{}`),
		DefinitionSnapshot: []byte(req.Definition),
		SourceSkills:       sourceSkills,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "create workflow run: "+err.Error())
		return
	}

	if err := h.startSidecarRun(r.Context(), workspaceID, issueID, run.ID, req.Definition, req.InitialState); err != nil {
		msg := "workflow sidecar unavailable: " + err.Error()
		_, _ = h.Queries.UpdateWorkflowRunProgress(r.Context(), db.UpdateWorkflowRunProgressParams{
			ID:     run.ID,
			Status: pgtype.Text{String: "failed", Valid: true},
			Error:  pgtype.Text{String: msg, Valid: true},
		})
		writeError(w, http.StatusBadGateway, msg)
		return
	}

	h.publishWorkflowRunUpdated(workspaceID, run)
	writeJSON(w, http.StatusAccepted, workflowRunToResponse(run))
}

func (h *Handler) startSidecarRun(ctx context.Context, workspaceID, issueID, runID pgtype.UUID, definition json.RawMessage, initial map[string]any) error {
	if initial == nil {
		initial = map[string]any{}
	}
	payload := map[string]any{
		"run_id":        uuidToString(runID),
		"workspace_id":  uuidToString(workspaceID),
		"root_issue_id": uuidToString(issueID),
		"definition":    json.RawMessage(definition),
		"initial_state": initial,
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
