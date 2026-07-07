package handler

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

type WorkflowCaseResponse struct {
	ID              string    `json:"id"`
	WorkspaceID     string    `json:"workspace_id"`
	Title           string    `json:"title"`
	Description     string    `json:"description"`
	EntryIssueID    *string   `json:"entry_issue_id,omitempty"`
	SourceIssueID   *string   `json:"source_issue_id,omitempty"`
	OwnerAgentID    *string   `json:"owner_agent_id,omitempty"`
	Status          string    `json:"status"`
	OnlineVersionID *string   `json:"online_version_id,omitempty"`
	CurrentRunID    *string   `json:"current_run_id,omitempty"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

type WorkflowDefinitionResponse struct {
	ID              string          `json:"id"`
	CaseID          string          `json:"case_id"`
	DraftJSON       json.RawMessage `json:"draft_json"`
	SourceTemplates json.RawMessage `json:"source_templates"`
	Status          string          `json:"status"`
	CreatedAt       time.Time       `json:"created_at"`
	UpdatedAt       time.Time       `json:"updated_at"`
}

type WorkflowDefinitionVersionResponse struct {
	ID               string          `json:"id"`
	WorkspaceID      string          `json:"workspace_id"`
	CaseID           string          `json:"case_id"`
	DefinitionID     string          `json:"definition_id"`
	Version          int32           `json:"version"`
	SnapshotJSON     json.RawMessage `json:"snapshot_json"`
	SourceSkills     json.RawMessage `json:"source_skills"`
	ValidationReport json.RawMessage `json:"validation_report"`
	ConfirmedBy      *string         `json:"confirmed_by,omitempty"`
	ConfirmedAt      time.Time       `json:"confirmed_at"`
	CreatedAt        time.Time       `json:"created_at"`
}

type workflowCaseRequest struct {
	Title         *string `json:"title"`
	Description   *string `json:"description"`
	EntryIssueID  *string `json:"entry_issue_id"`
	SourceIssueID *string `json:"source_issue_id"`
	OwnerAgentID  *string `json:"owner_agent_id"`
	Status        *string `json:"status"`
}

type workflowDefinitionDraftRequest struct {
	DraftJSON       json.RawMessage `json:"draft_json"`
	SourceTemplates json.RawMessage `json:"source_templates"`
}

type publishWorkflowDefinitionRequest struct {
	Note string `json:"note"`
}

type startWorkflowCaseRunRequest struct {
	RunKind      string         `json:"run_kind"`
	Label        string         `json:"label"`
	InitialState map[string]any `json:"initial_state"`
}

type publishWorkflowDefinitionResponse struct {
	Version WorkflowDefinitionVersionResponse `json:"version"`
}

type workflowDefinitionValidationIssue struct {
	Code    string `json:"code"`
	Path    string `json:"path"`
	Message string `json:"message"`
}

type workflowDefinitionValidationResponse struct {
	Valid    bool                                `json:"valid"`
	Errors   []workflowDefinitionValidationIssue `json:"errors"`
	Warnings []workflowDefinitionValidationIssue `json:"warnings"`
}

var publicWorkflowCasePatchStatuses = map[string]bool{
	"draft":    true,
	"archived": true,
}

func workflowCaseToResponse(c db.WorkflowCase) WorkflowCaseResponse {
	entryIssueID := uuidToPtr(c.SourceIssueID)
	return WorkflowCaseResponse{
		ID:              uuidToString(c.ID),
		WorkspaceID:     uuidToString(c.WorkspaceID),
		Title:           c.Title,
		Description:     c.Description,
		EntryIssueID:    entryIssueID,
		SourceIssueID:   entryIssueID,
		OwnerAgentID:    uuidToPtr(c.OwnerAgentID),
		Status:          c.Status,
		OnlineVersionID: uuidToPtr(c.OnlineVersionID),
		CurrentRunID:    uuidToPtr(c.CurrentRunID),
		CreatedAt:       c.CreatedAt.Time,
		UpdatedAt:       c.UpdatedAt.Time,
	}
}

func workflowDefinitionToResponse(d db.WorkflowDefinition) WorkflowDefinitionResponse {
	return WorkflowDefinitionResponse{
		ID:              uuidToString(d.ID),
		CaseID:          uuidToString(d.CaseID),
		DraftJSON:       json.RawMessage(d.DraftJson),
		SourceTemplates: json.RawMessage(d.SourceTemplates),
		Status:          d.Status,
		CreatedAt:       d.CreatedAt.Time,
		UpdatedAt:       d.UpdatedAt.Time,
	}
}

func workflowDefinitionVersionToResponse(v db.WorkflowDefinitionVersion) WorkflowDefinitionVersionResponse {
	return WorkflowDefinitionVersionResponse{
		ID:               uuidToString(v.ID),
		WorkspaceID:      uuidToString(v.WorkspaceID),
		CaseID:           uuidToString(v.CaseID),
		DefinitionID:     uuidToString(v.DefinitionID),
		Version:          v.Version,
		SnapshotJSON:     json.RawMessage(v.SnapshotJson),
		SourceSkills:     json.RawMessage(v.SourceSkills),
		ValidationReport: json.RawMessage(v.ValidationReport),
		ConfirmedBy:      textToPtr(v.ConfirmedBy),
		ConfirmedAt:      v.ConfirmedAt.Time,
		CreatedAt:        v.CreatedAt.Time,
	}
}

func (h *Handler) ListWorkflowCases(w http.ResponseWriter, r *http.Request) {
	workspaceID, ok := h.workflowWorkspaceID(w, r)
	if !ok {
		return
	}
	limit := int32(50)
	if raw := r.URL.Query().Get("limit"); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed < 1 || parsed > 200 {
			writeError(w, http.StatusBadRequest, "invalid limit")
			return
		}
		limit = int32(parsed)
	}
	offset := int32(0)
	if raw := r.URL.Query().Get("offset"); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed < 0 {
			writeError(w, http.StatusBadRequest, "invalid offset")
			return
		}
		offset = int32(parsed)
	}

	cases, err := h.Queries.ListWorkflowCases(r.Context(), db.ListWorkflowCasesParams{
		WorkspaceID: workspaceID,
		Offset:      offset,
		Limit:       limit,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "list workflow cases: "+err.Error())
		return
	}
	resp := make([]WorkflowCaseResponse, len(cases))
	for i, c := range cases {
		resp[i] = workflowCaseToResponse(c)
	}
	writeJSON(w, http.StatusOK, map[string]any{"workflow_cases": resp})
}

func (h *Handler) CreateWorkflowCase(w http.ResponseWriter, r *http.Request) {
	workspaceID, ok := h.workflowWorkspaceID(w, r)
	if !ok {
		return
	}
	var req workflowCaseRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	createdBy, ok := requireUserID(w, r)
	if !ok {
		return
	}
	params, ok := h.workflowCaseCreateParams(w, r, workspaceID, req, "", "")
	if !ok {
		return
	}
	params.CreatedBy = pgtype.Text{String: createdBy, Valid: true}
	params.UpdatedBy = pgtype.Text{String: createdBy, Valid: true}
	if !h.ensureWorkflowCaseEntryIssueAvailable(w, r, params.SourceIssueID) {
		return
	}

	created, err := h.Queries.CreateWorkflowCase(r.Context(), params)
	if err != nil {
		if isUniqueViolation(err) && params.SourceIssueID.Valid {
			writeError(w, http.StatusConflict, "entry issue already has an active workflow case")
			return
		}
		writeError(w, http.StatusInternalServerError, "create workflow case: "+err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, workflowCaseToResponse(created))
}

func (h *Handler) GetWorkflowCase(w http.ResponseWriter, r *http.Request) {
	c, ok := h.workflowCaseFromRequest(w, r)
	if !ok {
		return
	}
	writeJSON(w, http.StatusOK, workflowCaseToResponse(c))
}

func (h *Handler) UpdateWorkflowCase(w http.ResponseWriter, r *http.Request) {
	existing, ok := h.workflowCaseFromRequest(w, r)
	if !ok {
		return
	}
	var req workflowCaseRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if _, ok := h.workflowCaseEntryIssueIDFromRequest(w, req); !ok {
		return
	}
	updatedBy, ok := requireUserID(w, r)
	if !ok {
		return
	}
	if req.OwnerAgentID != nil && strings.TrimSpace(*req.OwnerAgentID) != "" {
		ownerID, ok := parseUUIDOrBadRequest(w, strings.TrimSpace(*req.OwnerAgentID), "owner_agent_id")
		if !ok {
			return
		}
		if _, ok := h.requireWorkflowAgentInWorkspace(w, r, ownerID, existing.WorkspaceID); !ok {
			return
		}
	}

	params := db.UpdateWorkflowCaseParams{
		ID:        existing.ID,
		UpdatedBy: pgtype.Text{String: updatedBy, Valid: true},
	}
	if req.Title != nil {
		title := strings.TrimSpace(*req.Title)
		if title == "" {
			writeError(w, http.StatusBadRequest, "title is required")
			return
		}
		params.Title = pgtype.Text{String: title, Valid: true}
	}
	if req.Description != nil {
		params.Description = pgtype.Text{String: *req.Description, Valid: true}
	}
	if req.OwnerAgentID != nil && strings.TrimSpace(*req.OwnerAgentID) != "" {
		params.OwnerAgentID = parseUUID(strings.TrimSpace(*req.OwnerAgentID))
	}
	if req.Status != nil {
		status := strings.TrimSpace(*req.Status)
		if status == "" {
			writeError(w, http.StatusBadRequest, "status is required")
			return
		}
		if !publicWorkflowCasePatchStatuses[status] {
			writeError(w, http.StatusBadRequest, "status may only be draft or archived")
			return
		}
		params.Status = pgtype.Text{String: status, Valid: true}
	}

	updated, err := h.Queries.UpdateWorkflowCase(r.Context(), params)
	if err != nil {
		if isCheckViolation(err) {
			writeError(w, http.StatusBadRequest, "invalid workflow case status")
			return
		}
		writeError(w, http.StatusInternalServerError, "update workflow case: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, workflowCaseToResponse(updated))
}

// DeleteWorkflowCase hard-deletes a WorkflowCase and cascades its definitions,
// versions, runs, run nodes, and node events at the DB layer. It refuses when
// the case still has an active run, and it detaches carrier sub-issues (which
// have no FK to runs) from their workflow binding first so no issue is left
// pointing at a deleted run. Entry issues are not touched.
func (h *Handler) DeleteWorkflowCase(w http.ResponseWriter, r *http.Request) {
	c, ok := h.workflowCaseFromRequest(w, r)
	if !ok {
		return
	}

	activeRuns, err := h.Queries.CountActiveWorkflowRunsByCase(r.Context(), c.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "count active workflow runs: "+err.Error())
		return
	}
	if activeRuns > 0 {
		writeError(w, http.StatusConflict, "workflow case has active runs; cancel them before deleting")
		return
	}

	runs, err := h.Queries.ListWorkflowRunsByCase(r.Context(), c.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "list workflow runs: "+err.Error())
		return
	}

	tx, err := h.TxStarter.Begin(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "begin delete workflow case transaction: "+err.Error())
		return
	}
	defer tx.Rollback(r.Context())
	qtx := h.Queries.WithTx(tx)

	if len(runs) > 0 {
		runIDs := make([]pgtype.UUID, 0, len(runs))
		for _, run := range runs {
			runIDs = append(runIDs, run.ID)
		}
		if err := qtx.ClearWorkflowNodeCarrierIssueBindings(r.Context(), db.ClearWorkflowNodeCarrierIssueBindingsParams{
			WorkspaceID: c.WorkspaceID,
			RunIds:      runIDs,
		}); err != nil {
			writeError(w, http.StatusInternalServerError, "detach workflow carrier issues: "+err.Error())
			return
		}
	}

	if _, err := qtx.DeleteWorkflowCase(r.Context(), db.DeleteWorkflowCaseParams{
		ID:          c.ID,
		WorkspaceID: c.WorkspaceID,
	}); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeError(w, http.StatusNotFound, "workflow case not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "delete workflow case: "+err.Error())
		return
	}

	if err := tx.Commit(r.Context()); err != nil {
		writeError(w, http.StatusInternalServerError, "commit delete workflow case: "+err.Error())
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) CreateWorkflowCaseFromIssue(w http.ResponseWriter, r *http.Request) {
	workspaceID, ok := h.workflowWorkspaceID(w, r)
	if !ok {
		return
	}
	issueID, ok := parseUUIDOrBadRequest(w, chi.URLParam(r, "id"), "issue id")
	if !ok {
		return
	}
	issue, ok := h.requireWorkflowIssueInWorkspace(w, r, issueID, workspaceID)
	if !ok {
		return
	}

	var req workflowCaseRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	createdBy, ok := requireUserID(w, r)
	if !ok {
		return
	}
	params, ok := h.workflowCaseCreateParams(w, r, workspaceID, req, issue.Title, textValue(issue.Description))
	if !ok {
		return
	}
	params.SourceIssueID = issueID
	params.CreatedBy = pgtype.Text{String: createdBy, Valid: true}
	params.UpdatedBy = pgtype.Text{String: createdBy, Valid: true}

	if existing, err := h.Queries.GetActiveWorkflowCaseByEntryIssue(r.Context(), issueID); err == nil {
		writeError(w, http.StatusConflict, "entry issue already has an active workflow case: "+uuidToString(existing.ID))
		return
	} else if !errors.Is(err, pgx.ErrNoRows) {
		writeError(w, http.StatusInternalServerError, "check entry issue workflow case: "+err.Error())
		return
	}

	created, err := h.Queries.CreateWorkflowCase(r.Context(), params)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "create workflow case: "+err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, workflowCaseToResponse(created))
}

func (h *Handler) GetWorkflowCaseDefinition(w http.ResponseWriter, r *http.Request) {
	c, ok := h.workflowCaseFromRequest(w, r)
	if !ok {
		return
	}
	definition, err := h.Queries.GetWorkflowDefinitionByCase(r.Context(), c.ID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeError(w, http.StatusNotFound, "workflow definition not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "load workflow definition: "+err.Error())
		return
	}
	if uuidToString(definition.WorkspaceID) != uuidToString(c.WorkspaceID) {
		writeError(w, http.StatusForbidden, "workflow definition is outside workspace")
		return
	}
	writeJSON(w, http.StatusOK, workflowDefinitionToResponse(definition))
}

func (h *Handler) UpsertWorkflowCaseDefinitionDraft(w http.ResponseWriter, r *http.Request) {
	c, ok := h.workflowCaseFromRequest(w, r)
	if !ok {
		return
	}
	var req workflowDefinitionDraftRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if len(req.DraftJSON) == 0 {
		req.DraftJSON = json.RawMessage(`{}`)
	}
	if !json.Valid(req.DraftJSON) {
		writeError(w, http.StatusBadRequest, "draft_json must be valid JSON")
		return
	}
	if len(req.SourceTemplates) == 0 {
		req.SourceTemplates = json.RawMessage(`[]`)
	}
	if !json.Valid(req.SourceTemplates) {
		writeError(w, http.StatusBadRequest, "source_templates must be valid JSON")
		return
	}
	updatedBy, ok := requireUserID(w, r)
	if !ok {
		return
	}

	definition, err := h.Queries.UpsertWorkflowDefinitionDraft(r.Context(), db.UpsertWorkflowDefinitionDraftParams{
		WorkspaceID:     c.WorkspaceID,
		CaseID:          c.ID,
		DraftJson:       []byte(req.DraftJSON),
		SourceTemplates: []byte(req.SourceTemplates),
		UpdatedBy:       pgtype.Text{String: updatedBy, Valid: true},
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "upsert workflow definition draft: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, workflowDefinitionToResponse(definition))
}

func (h *Handler) ValidateWorkflowCaseDefinitionDraft(w http.ResponseWriter, r *http.Request) {
	c, ok := h.workflowCaseFromRequest(w, r)
	if !ok {
		return
	}
	definition, err := h.Queries.GetWorkflowDefinitionByCase(r.Context(), c.ID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeError(w, http.StatusNotFound, "workflow definition not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "load workflow definition: "+err.Error())
		return
	}
	resp := h.validateWorkflowDefinitionDraft(r, c, definition)
	writeJSON(w, http.StatusOK, resp)
}

func (h *Handler) ListWorkflowCaseDefinitionVersions(w http.ResponseWriter, r *http.Request) {
	c, ok := h.workflowCaseFromRequest(w, r)
	if !ok {
		return
	}
	versions, err := h.Queries.ListWorkflowDefinitionVersions(r.Context(), c.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "list workflow definition versions: "+err.Error())
		return
	}
	resp := make([]WorkflowDefinitionVersionResponse, len(versions))
	for i, v := range versions {
		if uuidToString(v.WorkspaceID) != uuidToString(c.WorkspaceID) {
			writeError(w, http.StatusForbidden, "workflow definition version is outside workspace")
			return
		}
		resp[i] = workflowDefinitionVersionToResponse(v)
	}
	writeJSON(w, http.StatusOK, map[string]any{"versions": resp})
}

func (h *Handler) ListWorkflowCaseRuns(w http.ResponseWriter, r *http.Request) {
	c, ok := h.workflowCaseFromRequest(w, r)
	if !ok {
		return
	}
	runs, err := h.Queries.ListWorkflowRunsByCase(r.Context(), c.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "list workflow case runs: "+err.Error())
		return
	}
	resp := make([]WorkflowRunResponse, 0, len(runs))
	for _, run := range runs {
		if uuidToString(run.WorkspaceID) != uuidToString(c.WorkspaceID) {
			writeError(w, http.StatusForbidden, "workflow run is outside workspace")
			return
		}
		runResp, err := h.workflowRunToResponseWithSubIssueStatus(r.Context(), run)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "attach workflow run nodes: "+err.Error())
			return
		}
		resp = append(resp, runResp)
	}
	writeJSON(w, http.StatusOK, map[string]any{"runs": resp})
}

// StartWorkflowCaseRun creates a new WorkflowRun from the case's online
// version only. It never accepts an arbitrary historical definition_version_id,
// allows multiple parallel active runs, and persists run_kind/label. It does
// not project run status onto the case.
func (h *Handler) StartWorkflowCaseRun(w http.ResponseWriter, r *http.Request) {
	c, ok := h.workflowCaseFromRequest(w, r)
	if !ok {
		return
	}
	var raw map[string]json.RawMessage
	if err := json.NewDecoder(r.Body).Decode(&raw); err != nil && !errors.Is(err, io.EOF) {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if _, present := raw["definition_version_id"]; present {
		writeError(w, http.StatusBadRequest, "definition_version_id is not accepted; runs always use the online version")
		return
	}
	var req startWorkflowCaseRunRequest
	if len(raw) > 0 {
		merged, _ := json.Marshal(raw)
		if err := json.Unmarshal(merged, &req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request body")
			return
		}
	}
	runKind := normalizeWorkflowRunKind(req.RunKind)
	if !workflowRunKinds[runKind] {
		writeError(w, http.StatusBadRequest, "unsupported run_kind")
		return
	}
	if !c.OnlineVersionID.Valid {
		writeError(w, http.StatusBadRequest, "workflow case has no online version; publish a version before starting a run")
		return
	}
	version, err := h.Queries.GetWorkflowDefinitionVersion(r.Context(), c.OnlineVersionID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeError(w, http.StatusNotFound, "online workflow definition version not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "load online workflow definition version: "+err.Error())
		return
	}
	if uuidToString(version.CaseID) != uuidToString(c.ID) || uuidToString(version.WorkspaceID) != uuidToString(c.WorkspaceID) {
		writeError(w, http.StatusInternalServerError, "online version does not belong to workflow case")
		return
	}

	run, err := h.createAndStartWorkflowRun(r.Context(), startRuntimeWorkflowParams{
		WorkspaceID:         c.WorkspaceID,
		CaseID:              c.ID,
		RootIssueID:         c.SourceIssueID,
		DefinitionVersionID: version.ID,
		Definition:          json.RawMessage(version.SnapshotJson),
		InitialState:        req.InitialState,
		SourceSkills:        version.SourceSkills,
		RunKind:             runKind,
		Label:               req.Label,
	})
	if err != nil {
		if run.ID.Valid && errors.Is(err, errWorkflowSidecarUnavailable) {
			writeError(w, http.StatusBadGateway, err.Error())
			return
		}
		writeError(w, http.StatusInternalServerError, "start workflow case run: "+err.Error())
		return
	}
	writeJSON(w, http.StatusAccepted, workflowRunToResponse(run))
}

func (h *Handler) GetWorkflowCaseCurrentRun(w http.ResponseWriter, r *http.Request) {
	c, ok := h.workflowCaseFromRequest(w, r)
	if !ok {
		return
	}
	if !c.CurrentRunID.Valid {
		writeError(w, http.StatusNotFound, "workflow case has no current run")
		return
	}
	run, err := h.Queries.GetWorkflowRun(r.Context(), c.CurrentRunID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeError(w, http.StatusNotFound, "workflow run not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "load workflow run: "+err.Error())
		return
	}
	if uuidToString(run.CaseID) != uuidToString(c.ID) || uuidToString(run.WorkspaceID) != uuidToString(c.WorkspaceID) {
		writeError(w, http.StatusForbidden, "workflow run is outside workflow case")
		return
	}
	resp, err := h.workflowRunToResponseWithSubIssueStatus(r.Context(), run)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "load workflow run nodes: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

func (h *Handler) CancelWorkflowCaseRun(w http.ResponseWriter, r *http.Request) {
	c, ok := h.workflowCaseFromRequest(w, r)
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
	if uuidToString(run.CaseID) != uuidToString(c.ID) || uuidToString(run.WorkspaceID) != uuidToString(c.WorkspaceID) {
		writeError(w, http.StatusForbidden, "workflow run is outside workflow case")
		return
	}

	var req cancelWorkflowRunRequest
	_ = json.NewDecoder(r.Body).Decode(&req)
	reason := strings.TrimSpace(req.Reason)
	if reason == "" {
		reason = "workflow stopped by user"
	}

	cancelledRun, err := h.cancelWorkflowRun(r.Context(), run, reason)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "cancel workflow run: "+err.Error())
		return
	}

	h.publishWorkflowRunUpdated(r.Context(), c.WorkspaceID, cancelledRun)
	writeJSON(w, http.StatusOK, workflowRunToResponse(cancelledRun))
}

// PublishWorkflowCaseDefinition freezes the current draft into an immutable
// version and makes it the case's sole online version. It never starts a run
// and never projects run/lifecycle status onto the case.
func (h *Handler) PublishWorkflowCaseDefinition(w http.ResponseWriter, r *http.Request) {
	c, ok := h.workflowCaseFromRequest(w, r)
	if !ok {
		return
	}
	var req publishWorkflowDefinitionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil && !errors.Is(err, io.EOF) {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	publishedBy, ok := requireUserID(w, r)
	if !ok {
		return
	}

	definition, err := h.Queries.GetWorkflowDefinitionByCase(r.Context(), c.ID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeError(w, http.StatusNotFound, "workflow definition not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "load workflow definition: "+err.Error())
		return
	}
	if uuidToString(definition.WorkspaceID) != uuidToString(c.WorkspaceID) {
		writeError(w, http.StatusForbidden, "workflow definition is outside workspace")
		return
	}

	validation := h.validateWorkflowDefinitionDraft(r, c, definition)
	if !validation.Valid {
		writeJSON(w, http.StatusBadRequest, validation)
		return
	}
	runtimeDef, err := validateRuntimeWorkflowDefinition(json.RawMessage(definition.DraftJson))
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	sourceSkills, _ := json.Marshal(runtimeDef.SourceSkills)
	validationReport, _ := json.Marshal(validation)
	version, err := h.Queries.CreateWorkflowDefinitionVersion(r.Context(), db.CreateWorkflowDefinitionVersionParams{
		WorkspaceID:      c.WorkspaceID,
		CaseID:           c.ID,
		DefinitionID:     definition.ID,
		SnapshotJson:     definition.DraftJson,
		SourceSkills:     sourceSkills,
		ValidationReport: validationReport,
		ConfirmedBy:      pgtype.Text{String: publishedBy, Valid: true},
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "create workflow definition version: "+err.Error())
		return
	}

	if _, err := h.Queries.SetWorkflowCaseOnlineVersion(r.Context(), db.SetWorkflowCaseOnlineVersionParams{
		ID:              c.ID,
		OnlineVersionID: version.ID,
		UpdatedBy:       pgtype.Text{String: publishedBy, Valid: true},
	}); err != nil {
		writeError(w, http.StatusInternalServerError, "set online version: "+err.Error())
		return
	}

	writeJSON(w, http.StatusOK, publishWorkflowDefinitionResponse{
		Version: workflowDefinitionVersionToResponse(version),
	})
}

func (h *Handler) workflowCaseFromRequest(w http.ResponseWriter, r *http.Request) (db.WorkflowCase, bool) {
	workspaceID, ok := h.workflowWorkspaceID(w, r)
	if !ok {
		return db.WorkflowCase{}, false
	}
	caseID, ok := parseUUIDOrBadRequest(w, chi.URLParam(r, "caseId"), "case id")
	if !ok {
		return db.WorkflowCase{}, false
	}
	c, err := h.Queries.GetWorkflowCase(r.Context(), caseID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeError(w, http.StatusNotFound, "workflow case not found")
			return db.WorkflowCase{}, false
		}
		writeError(w, http.StatusInternalServerError, "load workflow case: "+err.Error())
		return db.WorkflowCase{}, false
	}
	if uuidToString(c.WorkspaceID) != uuidToString(workspaceID) {
		writeError(w, http.StatusForbidden, "workflow case is outside workspace")
		return db.WorkflowCase{}, false
	}
	return c, true
}

func (h *Handler) workflowCaseCreateParams(w http.ResponseWriter, r *http.Request, workspaceID pgtype.UUID, req workflowCaseRequest, fallbackTitle, fallbackDescription string) (db.CreateWorkflowCaseParams, bool) {
	title := fallbackTitle
	if req.Title != nil {
		title = *req.Title
	}
	title = strings.TrimSpace(title)
	if title == "" {
		writeError(w, http.StatusBadRequest, "title is required")
		return db.CreateWorkflowCaseParams{}, false
	}
	description := fallbackDescription
	if req.Description != nil {
		description = *req.Description
	}

	params := db.CreateWorkflowCaseParams{
		WorkspaceID:   workspaceID,
		Title:         title,
		Description:   description,
		Status:        "draft",
		SourceIssueID: pgtype.UUID{},
		OwnerAgentID:  pgtype.UUID{},
	}
	if req.Status != nil {
		writeError(w, http.StatusBadRequest, "status cannot be set when creating a workflow case")
		return db.CreateWorkflowCaseParams{}, false
	}
	entryIssueID, ok := h.workflowCaseEntryIssueIDFromRequest(w, req)
	if !ok {
		return db.CreateWorkflowCaseParams{}, false
	}
	if entryIssueID != nil && strings.TrimSpace(*entryIssueID) != "" {
		sourceIssueID, ok := parseUUIDOrBadRequest(w, strings.TrimSpace(*entryIssueID), "entry_issue_id")
		if !ok {
			return db.CreateWorkflowCaseParams{}, false
		}
		if _, ok := h.requireWorkflowIssueInWorkspace(w, r, sourceIssueID, workspaceID); !ok {
			return db.CreateWorkflowCaseParams{}, false
		}
		params.SourceIssueID = sourceIssueID
	}
	if req.OwnerAgentID != nil && strings.TrimSpace(*req.OwnerAgentID) != "" {
		ownerAgentID, ok := parseUUIDOrBadRequest(w, strings.TrimSpace(*req.OwnerAgentID), "owner_agent_id")
		if !ok {
			return db.CreateWorkflowCaseParams{}, false
		}
		if _, ok := h.requireWorkflowAgentInWorkspace(w, r, ownerAgentID, workspaceID); !ok {
			return db.CreateWorkflowCaseParams{}, false
		}
		params.OwnerAgentID = ownerAgentID
	}
	return params, true
}

func (h *Handler) workflowCaseEntryIssueIDFromRequest(w http.ResponseWriter, req workflowCaseRequest) (*string, bool) {
	entryIssueID := ""
	if req.EntryIssueID != nil {
		entryIssueID = strings.TrimSpace(*req.EntryIssueID)
	}
	sourceIssueID := ""
	if req.SourceIssueID != nil {
		sourceIssueID = strings.TrimSpace(*req.SourceIssueID)
	}
	if entryIssueID != "" && sourceIssueID != "" && entryIssueID != sourceIssueID {
		writeError(w, http.StatusBadRequest, "entry_issue_id and source_issue_id must match when both are provided")
		return nil, false
	}
	if entryIssueID != "" {
		return &entryIssueID, true
	}
	if sourceIssueID != "" {
		return &sourceIssueID, true
	}
	return nil, true
}

func (h *Handler) ensureWorkflowCaseEntryIssueAvailable(w http.ResponseWriter, r *http.Request, entryIssueID pgtype.UUID) bool {
	if !entryIssueID.Valid {
		return true
	}
	if existing, err := h.Queries.GetActiveWorkflowCaseByEntryIssue(r.Context(), entryIssueID); err == nil {
		writeError(w, http.StatusConflict, "entry issue already has an active workflow case: "+uuidToString(existing.ID))
		return false
	} else if !errors.Is(err, pgx.ErrNoRows) {
		writeError(w, http.StatusInternalServerError, "check entry issue workflow case: "+err.Error())
		return false
	}
	return true
}

func (h *Handler) workflowCaseAllowedSourceSkills(r *http.Request, c db.WorkflowCase) (map[string]bool, *workflowDefinitionValidationIssue) {
	agentID := pgtype.UUID{}
	if c.OwnerAgentID.Valid {
		agentID = c.OwnerAgentID
	} else if c.SourceIssueID.Valid {
		issue, err := h.Queries.GetIssue(r.Context(), c.SourceIssueID)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return nil, &workflowDefinitionValidationIssue{
					Code:    "SOURCE_ISSUE_NOT_FOUND",
					Path:    "source_issue_id",
					Message: "source issue not found",
				}
			}
			return nil, &workflowDefinitionValidationIssue{
				Code:    "WORKFLOW_VALIDATION_ERROR",
				Path:    "",
				Message: "load source issue: " + err.Error(),
			}
		}
		if uuidToString(issue.WorkspaceID) != uuidToString(c.WorkspaceID) {
			return nil, &workflowDefinitionValidationIssue{
				Code:    "SOURCE_ISSUE_WORKSPACE_MISMATCH",
				Path:    "source_issue_id",
				Message: "source issue is outside workspace",
			}
		}
		if issue.AssigneeID.Valid && issue.AssigneeType.Valid && issue.AssigneeType.String == "agent" {
			agentID = issue.AssigneeID
		}
	}

	allowedSkills := map[string]bool{}
	if !agentID.Valid {
		return allowedSkills, nil
	}
	skills, err := h.Queries.ListAgentSkills(r.Context(), agentID)
	if err != nil {
		return nil, &workflowDefinitionValidationIssue{
			Code:    "WORKFLOW_VALIDATION_ERROR",
			Path:    "",
			Message: "load agent skills: " + err.Error(),
		}
	}
	for _, s := range skills {
		allowedSkills[uuidToString(s.ID)] = true
	}
	return allowedSkills, nil
}

func (h *Handler) requireWorkflowIssueInWorkspace(w http.ResponseWriter, r *http.Request, issueID, workspaceID pgtype.UUID) (db.Issue, bool) {
	issue, err := h.Queries.GetIssue(r.Context(), issueID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeError(w, http.StatusNotFound, "issue not found")
			return db.Issue{}, false
		}
		writeError(w, http.StatusInternalServerError, "load issue: "+err.Error())
		return db.Issue{}, false
	}
	if uuidToString(issue.WorkspaceID) != uuidToString(workspaceID) {
		writeError(w, http.StatusForbidden, "issue is outside workspace")
		return db.Issue{}, false
	}
	return issue, true
}

func (h *Handler) requireWorkflowAgentInWorkspace(w http.ResponseWriter, r *http.Request, agentID, workspaceID pgtype.UUID) (db.Agent, bool) {
	agent, err := h.Queries.GetAgent(r.Context(), agentID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeError(w, http.StatusNotFound, "owner agent not found")
			return db.Agent{}, false
		}
		writeError(w, http.StatusInternalServerError, "load owner agent: "+err.Error())
		return db.Agent{}, false
	}
	if uuidToString(agent.WorkspaceID) != uuidToString(workspaceID) {
		writeError(w, http.StatusForbidden, "owner agent is outside workspace")
		return db.Agent{}, false
	}
	return agent, true
}

func (h *Handler) validateWorkflowDefinitionDraft(r *http.Request, c db.WorkflowCase, definition db.WorkflowDefinition) workflowDefinitionValidationResponse {
	def, err := validateRuntimeWorkflowDefinition(json.RawMessage(definition.DraftJson))
	if err != nil {
		return workflowDefinitionValidationResponse{
			Valid: false,
			Errors: []workflowDefinitionValidationIssue{{
				Code:    "WORKFLOW_VALIDATION_ERROR",
				Path:    "",
				Message: err.Error(),
			}},
			Warnings: []workflowDefinitionValidationIssue{},
		}
	}

	allowedSkills, validationIssue := h.workflowCaseAllowedSourceSkills(r, c)
	if validationIssue != nil {
		return workflowDefinitionValidationResponse{
			Valid:    false,
			Errors:   []workflowDefinitionValidationIssue{*validationIssue},
			Warnings: []workflowDefinitionValidationIssue{},
		}
	}
	if err := validateRuntimeWorkflowSourceSkills(def, allowedSkills); err != nil {
		return workflowDefinitionValidationResponse{
			Valid: false,
			Errors: []workflowDefinitionValidationIssue{{
				Code:    "WORKFLOW_VALIDATION_ERROR",
				Path:    "",
				Message: err.Error(),
			}},
			Warnings: []workflowDefinitionValidationIssue{},
		}
	}

	return workflowDefinitionValidationResponse{
		Valid:    true,
		Errors:   []workflowDefinitionValidationIssue{},
		Warnings: []workflowDefinitionValidationIssue{},
	}
}

func textValue(t pgtype.Text) string {
	if !t.Valid {
		return ""
	}
	return t.String
}
