package handler

import (
	"bytes"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/logger"
	"github.com/multica-ai/multica/server/internal/middleware"
	"github.com/multica-ai/multica/server/internal/service"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

type IssueTemplateResponse struct {
	ID                 string          `json:"id"`
	WorkspaceID        string          `json:"workspace_id"`
	ProjectID          *string         `json:"project_id"`
	Title              string          `json:"title"`
	IssueTitleTemplate string          `json:"issue_title_template"`
	IssueBodyTemplate  *string         `json:"issue_body_template"`
	AssigneeType       string          `json:"assignee_type"`
	AssigneeID         string          `json:"assignee_id"`
	Priority           string          `json:"priority"`
	Labels             json.RawMessage `json:"labels,omitempty"`
	DefaultMetadata    json.RawMessage `json:"default_metadata,omitempty"`
	ExecutionSpec      json.RawMessage `json:"execution_spec,omitempty"`
	CreatedByType      string          `json:"created_by_type"`
	CreatedByID        string          `json:"created_by_id"`
	CreatedAt          string          `json:"created_at"`
	UpdatedAt          string          `json:"updated_at"`
	AutomationRefCount *int64          `json:"automation_ref_count,omitempty"`
	IssueRefCount      *int64          `json:"issue_ref_count,omitempty"`
}

type issueTemplateRequest struct {
	ProjectID          *string          `json:"project_id"`
	Title              *string          `json:"title"`
	IssueTitleTemplate *string          `json:"issue_title_template"`
	IssueBodyTemplate  *string          `json:"issue_body_template"`
	AssigneeType       *string          `json:"assignee_type"`
	AssigneeID         *string          `json:"assignee_id"`
	Priority           *string          `json:"priority"`
	Labels             *json.RawMessage `json:"labels"`
	DefaultMetadata    *json.RawMessage `json:"default_metadata"`
	ExecutionSpec      *json.RawMessage `json:"execution_spec"`
}

type issueTemplatePatchRequest struct {
	issueTemplateRequest
	raw map[string]json.RawMessage
}

func issueTemplateToResponse(t db.IssueTemplate) IssueTemplateResponse {
	return issueTemplateToResponseWithRefCount(t, nil)
}

func issueTemplateRowToResponse(t db.ListIssueTemplatesRow) IssueTemplateResponse {
	count := t.AutomationRefCount
	return IssueTemplateResponse{
		ID:                 uuidToString(t.ID),
		WorkspaceID:        uuidToString(t.WorkspaceID),
		ProjectID:          uuidToPtr(t.ProjectID),
		Title:              t.Title,
		IssueTitleTemplate: t.IssueTitleTemplate,
		IssueBodyTemplate:  textToPtr(t.IssueBodyTemplate),
		AssigneeType:       t.AssigneeType,
		AssigneeID:         uuidToString(t.AssigneeID),
		Priority:           t.Priority,
		Labels:             json.RawMessage(t.Labels),
		DefaultMetadata:    json.RawMessage(t.DefaultMetadata),
		ExecutionSpec:      json.RawMessage(t.ExecutionSpec),
		CreatedByType:      t.CreatedByType,
		CreatedByID:        uuidToString(t.CreatedByID),
		CreatedAt:          timestampToString(t.CreatedAt),
		UpdatedAt:          timestampToString(t.UpdatedAt),
		AutomationRefCount: &count,
	}
}

func issueTemplateToResponseWithRefCount(t db.IssueTemplate, count *int64) IssueTemplateResponse {
	return IssueTemplateResponse{
		ID:                 uuidToString(t.ID),
		WorkspaceID:        uuidToString(t.WorkspaceID),
		ProjectID:          uuidToPtr(t.ProjectID),
		Title:              t.Title,
		IssueTitleTemplate: t.IssueTitleTemplate,
		IssueBodyTemplate:  textToPtr(t.IssueBodyTemplate),
		AssigneeType:       t.AssigneeType,
		AssigneeID:         uuidToString(t.AssigneeID),
		Priority:           t.Priority,
		Labels:             json.RawMessage(t.Labels),
		DefaultMetadata:    json.RawMessage(t.DefaultMetadata),
		ExecutionSpec:      json.RawMessage(t.ExecutionSpec),
		CreatedByType:      t.CreatedByType,
		CreatedByID:        uuidToString(t.CreatedByID),
		CreatedAt:          timestampToString(t.CreatedAt),
		UpdatedAt:          timestampToString(t.UpdatedAt),
		AutomationRefCount: count,
	}
}

func (h *Handler) ListIssueTemplates(w http.ResponseWriter, r *http.Request) {
	workspaceID := h.resolveWorkspaceID(r)
	wsUUID, ok := parseUUIDOrBadRequest(w, workspaceID, "workspace_id")
	if !ok {
		return
	}

	var projectID pgtype.UUID
	if raw := strings.TrimSpace(r.URL.Query().Get("project_id")); raw != "" {
		projectID, ok = parseUUIDOrBadRequest(w, raw, "project_id")
		if !ok {
			return
		}
	}

	templates, err := h.Queries.ListIssueTemplates(r.Context(), db.ListIssueTemplatesParams{
		WorkspaceID: wsUUID,
		ProjectID:   projectID,
	})
	if err != nil {
		slog.Warn("list issue templates failed", append(logger.RequestAttrs(r), "error", err, "workspace_id", workspaceID)...)
		writeError(w, http.StatusInternalServerError, "failed to list issue templates")
		return
	}

	resp := make([]IssueTemplateResponse, 0, len(templates))
	for _, template := range templates {
		resp = append(resp, issueTemplateRowToResponse(template))
	}
	writeJSON(w, http.StatusOK, resp)
}

func (h *Handler) GetIssueTemplate(w http.ResponseWriter, r *http.Request) {
	template, ok := h.loadIssueTemplateInWorkspace(w, r)
	if !ok {
		return
	}
	automationCount, issueCount, err := h.issueTemplateReferenceCounts(r, template.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load issue template")
		return
	}
	resp := issueTemplateToResponseWithRefCount(template, &automationCount)
	resp.IssueRefCount = &issueCount
	writeJSON(w, http.StatusOK, resp)
}

func (h *Handler) CreateIssueTemplate(w http.ResponseWriter, r *http.Request) {
	var req issueTemplateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	workspaceID := h.resolveWorkspaceID(r)
	wsUUID, ok := parseUUIDOrBadRequest(w, workspaceID, "workspace_id")
	if !ok {
		return
	}
	creatorID, ok := requireUserID(w, r)
	if !ok {
		return
	}

	params, ok := h.issueTemplateCreateParamsFromRequest(w, r, wsUUID, workspaceID, creatorID, req)
	if !ok {
		return
	}
	template, err := h.Queries.CreateIssueTemplate(r.Context(), params)
	if err != nil {
		if isCheckViolation(err) {
			writeError(w, http.StatusBadRequest, "invalid issue template")
			return
		}
		slog.Warn("create issue template failed", append(logger.RequestAttrs(r), "error", err, "workspace_id", workspaceID)...)
		writeError(w, http.StatusInternalServerError, "failed to create issue template")
		return
	}
	writeJSON(w, http.StatusCreated, issueTemplateToResponse(template))
}

func (h *Handler) UpdateIssueTemplate(w http.ResponseWriter, r *http.Request) {
	var req issueTemplatePatchRequest
	if err := json.NewDecoder(r.Body).Decode(&req.raw); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	raw, err := json.Marshal(req.raw)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if err := json.Unmarshal(raw, &req.issueTemplateRequest); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	existing, ok := h.loadIssueTemplateInWorkspace(w, r)
	if !ok {
		return
	}
	workspaceID := uuidToString(existing.WorkspaceID)

	params, ok := h.issueTemplateUpdateParamsFromRequest(w, r, existing, workspaceID, req)
	if !ok {
		return
	}
	template, err := h.Queries.UpdateIssueTemplate(r.Context(), params)
	if err != nil {
		if isCheckViolation(err) {
			writeError(w, http.StatusBadRequest, "invalid issue template")
			return
		}
		slog.Warn("update issue template failed", append(logger.RequestAttrs(r), "error", err, "template_id", uuidToString(existing.ID), "workspace_id", workspaceID)...)
		writeError(w, http.StatusInternalServerError, "failed to update issue template")
		return
	}
	writeJSON(w, http.StatusOK, issueTemplateToResponse(template))
}

func (h *Handler) DeleteIssueTemplate(w http.ResponseWriter, r *http.Request) {
	template, ok := h.loadIssueTemplateInWorkspace(w, r)
	if !ok {
		return
	}
	automationCount, issueCount, err := h.issueTemplateReferenceCounts(r, template.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to delete issue template")
		return
	}
	if automationCount > 0 || issueCount > 0 {
		writeJSON(w, http.StatusConflict, map[string]any{
			"error":                "issue template is referenced and cannot be deleted",
			"automation_ref_count": automationCount,
			"issue_ref_count":      issueCount,
		})
		return
	}
	if err := h.Queries.DeleteIssueTemplate(r.Context(), db.DeleteIssueTemplateParams{
		ID:          template.ID,
		WorkspaceID: template.WorkspaceID,
	}); err != nil {
		slog.Warn("delete issue template failed", append(logger.RequestAttrs(r), "error", err, "template_id", uuidToString(template.ID))...)
		writeError(w, http.StatusInternalServerError, "failed to delete issue template")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) InstantiateIssueTemplate(w http.ResponseWriter, r *http.Request) {
	template, ok := h.loadIssueTemplateInWorkspace(w, r)
	if !ok {
		return
	}
	creatorID, ok := requireUserID(w, r)
	if !ok {
		return
	}

	instantiator := service.NewIssueInstantiator()
	params, err := instantiator.IssueCreateParamsFromTemplate(template, "member", parseUUID(creatorID))
	if errors.Is(err, service.ErrTemplateIssueTitleRequired) {
		writeError(w, http.StatusBadRequest, "template issue title is required")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to instantiate issue template")
		return
	}

	prefix := h.getIssuePrefix(r.Context(), template.WorkspaceID)
	res, err := h.IssueService.Create(r.Context(), params, service.IssueCreateOpts{
		ActorID:          creatorID,
		AnalyticsAgentID: uuidToString(template.AssigneeID),
		Platform:         func() string { p, _, _ := middleware.ClientMetadataFromContext(r.Context()); return p }(),
		BroadcastPayload: func(issue db.Issue, _ []db.Attachment) map[string]any {
			return map[string]any{"issue": issueToResponse(issue, prefix)}
		},
	})
	if errors.Is(err, service.ErrActiveDuplicate) {
		dup := *res.DuplicateIssue
		existing := issueToResponse(dup, h.getIssuePrefix(r.Context(), dup.WorkspaceID))
		writeJSON(w, http.StatusConflict, map[string]any{
			"code":  "active_duplicate_issue",
			"error": duplicateIssueMessage(existing),
			"issue": existing,
		})
		return
	}
	if errors.Is(err, service.ErrProjectNotFound) {
		writeError(w, http.StatusBadRequest, "project not found in this workspace")
		return
	}
	if err != nil {
		slog.Warn("instantiate issue template failed", append(logger.RequestAttrs(r), "error", err, "template_id", uuidToString(template.ID))...)
		writeError(w, http.StatusInternalServerError, "failed to instantiate issue template")
		return
	}
	writeJSON(w, http.StatusCreated, issueToResponse(res.Issue, prefix))
}

func (h *Handler) loadIssueTemplateInWorkspace(w http.ResponseWriter, r *http.Request) (db.IssueTemplate, bool) {
	templateID, ok := parseUUIDOrBadRequest(w, chi.URLParam(r, "id"), "issue template id")
	if !ok {
		return db.IssueTemplate{}, false
	}
	workspaceID := h.resolveWorkspaceID(r)
	wsUUID, ok := parseUUIDOrBadRequest(w, workspaceID, "workspace_id")
	if !ok {
		return db.IssueTemplate{}, false
	}
	template, err := h.Queries.GetIssueTemplateInWorkspace(r.Context(), db.GetIssueTemplateInWorkspaceParams{
		ID:          templateID,
		WorkspaceID: wsUUID,
	})
	if isNotFound(err) {
		writeError(w, http.StatusNotFound, "issue template not found")
		return db.IssueTemplate{}, false
	}
	if err != nil {
		slog.Warn("load issue template failed", append(logger.RequestAttrs(r), "error", err, "template_id", chi.URLParam(r, "id"), "workspace_id", workspaceID)...)
		writeError(w, http.StatusInternalServerError, "failed to load issue template")
		return db.IssueTemplate{}, false
	}
	return template, true
}

func (h *Handler) issueTemplateCreateParamsFromRequest(w http.ResponseWriter, r *http.Request, workspaceID pgtype.UUID, workspaceIDStr, creatorID string, req issueTemplateRequest) (db.CreateIssueTemplateParams, bool) {
	if req.Title == nil || strings.TrimSpace(*req.Title) == "" {
		writeError(w, http.StatusBadRequest, "title is required")
		return db.CreateIssueTemplateParams{}, false
	}
	if req.IssueTitleTemplate == nil || strings.TrimSpace(*req.IssueTitleTemplate) == "" {
		writeError(w, http.StatusBadRequest, "issue_title_template is required")
		return db.CreateIssueTemplateParams{}, false
	}
	assigneeType, assigneeID, ok := h.parseTemplateAssignee(w, r, workspaceIDStr, req.AssigneeType, req.AssigneeID)
	if !ok {
		return db.CreateIssueTemplateParams{}, false
	}
	projectID, ok := h.parseAndValidateTemplateProject(w, r, workspaceID, req.ProjectID)
	if !ok {
		return db.CreateIssueTemplateParams{}, false
	}
	priority := "medium"
	if req.Priority != nil && strings.TrimSpace(*req.Priority) != "" {
		priority = strings.TrimSpace(*req.Priority)
	}
	labels, ok := normalizeOptionalJSON(w, req.Labels, "labels")
	if !ok {
		return db.CreateIssueTemplateParams{}, false
	}
	defaultMetadata, ok := normalizeOptionalJSON(w, req.DefaultMetadata, "default_metadata")
	if !ok {
		return db.CreateIssueTemplateParams{}, false
	}
	executionSpec, ok := normalizeOptionalJSON(w, req.ExecutionSpec, "execution_spec")
	if !ok {
		return db.CreateIssueTemplateParams{}, false
	}

	return db.CreateIssueTemplateParams{
		WorkspaceID:        workspaceID,
		ProjectID:          projectID,
		Title:              strings.TrimSpace(*req.Title),
		IssueTitleTemplate: strings.TrimSpace(*req.IssueTitleTemplate),
		IssueBodyTemplate:  ptrToText(req.IssueBodyTemplate),
		AssigneeType:       assigneeType.String,
		AssigneeID:         assigneeID,
		Priority:           priority,
		Labels:             labels,
		DefaultMetadata:    defaultMetadata,
		ExecutionSpec:      executionSpec,
		CreatedByType:      "member",
		CreatedByID:        parseUUID(creatorID),
	}, true
}

func (h *Handler) issueTemplateUpdateParamsFromRequest(w http.ResponseWriter, r *http.Request, existing db.IssueTemplate, workspaceID string, req issueTemplatePatchRequest) (db.UpdateIssueTemplateParams, bool) {
	projectID := existing.ProjectID
	if _, ok := req.raw["project_id"]; ok {
		parsed, ok := h.parseAndValidateTemplateProject(w, r, existing.WorkspaceID, req.ProjectID)
		if !ok {
			return db.UpdateIssueTemplateParams{}, false
		}
		projectID = parsed
	}

	title := existing.Title
	if req.Title != nil {
		title = strings.TrimSpace(*req.Title)
	}
	if title == "" {
		writeError(w, http.StatusBadRequest, "title is required")
		return db.UpdateIssueTemplateParams{}, false
	}

	issueTitle := existing.IssueTitleTemplate
	if req.IssueTitleTemplate != nil {
		issueTitle = strings.TrimSpace(*req.IssueTitleTemplate)
	}
	if issueTitle == "" {
		writeError(w, http.StatusBadRequest, "issue_title_template is required")
		return db.UpdateIssueTemplateParams{}, false
	}

	body := existing.IssueBodyTemplate
	if _, ok := req.raw["issue_body_template"]; ok {
		body = ptrToText(req.IssueBodyTemplate)
	}

	assigneeType := pgtype.Text{String: existing.AssigneeType, Valid: true}
	assigneeID := existing.AssigneeID
	if req.AssigneeType != nil {
		assigneeType = pgtype.Text{String: strings.TrimSpace(*req.AssigneeType), Valid: true}
	}
	if req.AssigneeID != nil {
		parsed, ok := parseUUIDOrBadRequest(w, *req.AssigneeID, "assignee_id")
		if !ok {
			return db.UpdateIssueTemplateParams{}, false
		}
		assigneeID = parsed
	}
	if status, msg := h.validateAssigneePair(r.Context(), r, workspaceID, assigneeType, assigneeID); status != 0 {
		writeError(w, status, msg)
		return db.UpdateIssueTemplateParams{}, false
	}

	priority := existing.Priority
	if req.Priority != nil && strings.TrimSpace(*req.Priority) != "" {
		priority = strings.TrimSpace(*req.Priority)
	}

	labels := existing.Labels
	if _, ok := req.raw["labels"]; ok {
		parsed, ok := normalizeOptionalJSON(w, req.Labels, "labels")
		if !ok {
			return db.UpdateIssueTemplateParams{}, false
		}
		labels = parsed
	}
	defaultMetadata := existing.DefaultMetadata
	if _, ok := req.raw["default_metadata"]; ok {
		parsed, ok := normalizeOptionalJSON(w, req.DefaultMetadata, "default_metadata")
		if !ok {
			return db.UpdateIssueTemplateParams{}, false
		}
		defaultMetadata = parsed
	}
	executionSpec := existing.ExecutionSpec
	if _, ok := req.raw["execution_spec"]; ok {
		parsed, ok := normalizeOptionalJSON(w, req.ExecutionSpec, "execution_spec")
		if !ok {
			return db.UpdateIssueTemplateParams{}, false
		}
		executionSpec = parsed
	}

	return db.UpdateIssueTemplateParams{
		ID:                 existing.ID,
		WorkspaceID:        existing.WorkspaceID,
		ProjectID:          projectID,
		Title:              pgtype.Text{String: title, Valid: true},
		IssueTitleTemplate: pgtype.Text{String: issueTitle, Valid: true},
		IssueBodyTemplate:  body,
		AssigneeType:       assigneeType,
		AssigneeID:         assigneeID,
		Priority:           pgtype.Text{String: priority, Valid: true},
		Labels:             labels,
		DefaultMetadata:    defaultMetadata,
		ExecutionSpec:      executionSpec,
	}, true
}

func (h *Handler) parseTemplateAssignee(w http.ResponseWriter, r *http.Request, workspaceID string, assigneeTypeRaw, assigneeIDRaw *string) (pgtype.Text, pgtype.UUID, bool) {
	if assigneeTypeRaw == nil || assigneeIDRaw == nil {
		writeError(w, http.StatusBadRequest, "assignee_type and assignee_id are required")
		return pgtype.Text{}, pgtype.UUID{}, false
	}
	assigneeID, ok := parseUUIDOrBadRequest(w, *assigneeIDRaw, "assignee_id")
	if !ok {
		return pgtype.Text{}, pgtype.UUID{}, false
	}
	assigneeType := pgtype.Text{String: strings.TrimSpace(*assigneeTypeRaw), Valid: true}
	if status, msg := h.validateAssigneePair(r.Context(), r, workspaceID, assigneeType, assigneeID); status != 0 {
		writeError(w, status, msg)
		return pgtype.Text{}, pgtype.UUID{}, false
	}
	return assigneeType, assigneeID, true
}

func (h *Handler) parseAndValidateTemplateProject(w http.ResponseWriter, r *http.Request, workspaceID pgtype.UUID, raw *string) (pgtype.UUID, bool) {
	if raw == nil || strings.TrimSpace(*raw) == "" {
		return pgtype.UUID{}, true
	}
	projectID, ok := parseUUIDOrBadRequest(w, *raw, "project_id")
	if !ok {
		return pgtype.UUID{}, false
	}
	if _, err := h.Queries.GetProjectInWorkspace(r.Context(), db.GetProjectInWorkspaceParams{
		ID:          projectID,
		WorkspaceID: workspaceID,
	}); err != nil {
		writeError(w, http.StatusBadRequest, "project not found in this workspace")
		return pgtype.UUID{}, false
	}
	return projectID, true
}

func (h *Handler) issueTemplateReferenceCounts(r *http.Request, templateID pgtype.UUID) (int64, int64, error) {
	automationCount, err := h.Queries.CountIssueTemplateAutomationReferences(r.Context(), templateID)
	if err != nil {
		slog.Warn("count issue template automation references failed", append(logger.RequestAttrs(r), "error", err, "template_id", uuidToString(templateID))...)
		return 0, 0, err
	}
	issueCount, err := h.Queries.CountIssueTemplateIssueReferences(r.Context(), templateID)
	if err != nil {
		slog.Warn("count issue template issue references failed", append(logger.RequestAttrs(r), "error", err, "template_id", uuidToString(templateID))...)
		return 0, 0, err
	}
	return automationCount, issueCount, nil
}

func normalizeOptionalJSON(w http.ResponseWriter, raw *json.RawMessage, field string) ([]byte, bool) {
	if raw == nil {
		return nil, true
	}
	trimmed := bytes.TrimSpace(*raw)
	if len(trimmed) == 0 || bytes.Equal(trimmed, []byte("null")) {
		return nil, true
	}
	if !json.Valid(trimmed) {
		writeError(w, http.StatusBadRequest, "invalid "+field)
		return nil, false
	}
	return append([]byte(nil), trimmed...), true
}
