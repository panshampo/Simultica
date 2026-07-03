package handler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/multica-ai/multica/server/internal/middleware"
	"github.com/multica-ai/multica/server/internal/service"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

const (
	automationExecutionCreateIssue = "create_issue"
	automationExecutionRunOnly     = "run_only"
)

type InlineIssueConfig struct {
	IssueTitleTemplate string  `json:"issue_title_template"`
	IssueBodyTemplate  *string `json:"issue_body_template"`
	AssigneeType       string  `json:"assignee_type"`
	AssigneeID         string  `json:"assignee_id"`
	ProjectID          *string `json:"project_id"`
	Priority           string  `json:"priority"`
	ExecutionMode      string  `json:"execution_mode,omitempty"`
}

type AutomationResponse struct {
	ID                string             `json:"id"`
	WorkspaceID       string             `json:"workspace_id"`
	Title             string             `json:"title"`
	SourceMode        string             `json:"source_mode"`
	TemplateID        *string            `json:"template_id"`
	InlineIssueConfig *InlineIssueConfig `json:"inline_issue_config,omitempty"`
	Status            string             `json:"status"`
	ConcurrencyPolicy string             `json:"concurrency_policy"`
	CreatedByType     string             `json:"created_by_type"`
	CreatedByID       string             `json:"created_by_id"`
	LastRunAt         *string            `json:"last_run_at"`
	CreatedAt         string             `json:"created_at"`
	UpdatedAt         string             `json:"updated_at"`
}

type AutomationTriggerResponse struct {
	ID                string               `json:"id"`
	AutomationID      string               `json:"automation_id"`
	Kind              string               `json:"kind"`
	Enabled           bool                 `json:"enabled"`
	CronExpression    *string              `json:"cron_expression"`
	Timezone          *string              `json:"timezone"`
	NextRunAt         *string              `json:"next_run_at"`
	WebhookToken      *string              `json:"webhook_token"`
	WebhookPath       *string              `json:"webhook_path"`
	WebhookURL        *string              `json:"webhook_url"`
	Provider          *string              `json:"provider"`
	HasSigningSecret  bool                 `json:"has_signing_secret"`
	SigningSecretHint *string              `json:"signing_secret_hint"`
	Label             *string              `json:"label"`
	LastFiredAt       *string              `json:"last_fired_at"`
	CreatedAt         string               `json:"created_at"`
	UpdatedAt         string               `json:"updated_at"`
	EventFilters      []WebhookEventFilter `json:"event_filters,omitempty"`
}

type AutomationRunResponse struct {
	ID                   string  `json:"id"`
	AutomationID         string  `json:"automation_id"`
	TriggerID            *string `json:"trigger_id"`
	Source               string  `json:"source"`
	SourceModeSnapshot   string  `json:"source_mode_snapshot"`
	Status               string  `json:"status"`
	IssueID              *string `json:"issue_id"`
	TaskID               *string `json:"task_id"`
	TriggeredAt          string  `json:"triggered_at"`
	CompletedAt          *string `json:"completed_at"`
	FailureReason        *string `json:"failure_reason"`
	TriggerPayload       any     `json:"trigger_payload"`
	ResolvedIssuePayload any     `json:"resolved_issue_payload"`
	TemplateSnapshot     any     `json:"template_snapshot"`
	Result               any     `json:"result"`
	CreatedAt            string  `json:"created_at"`
}

type CreateAutomationRequest struct {
	Title             string             `json:"title"`
	SourceMode        string             `json:"source_mode"`
	TemplateID        *string            `json:"template_id"`
	InlineIssueConfig *InlineIssueConfig `json:"inline_issue_config"`
	Status            *string            `json:"status"`
	ConcurrencyPolicy *string            `json:"concurrency_policy"`
	ExecutionMode     *string            `json:"execution_mode"`
}

type UpdateAutomationRequest struct {
	Title             *string            `json:"title"`
	SourceMode        *string            `json:"source_mode"`
	TemplateID        *string            `json:"template_id"`
	InlineIssueConfig *InlineIssueConfig `json:"inline_issue_config"`
	Status            *string            `json:"status"`
	ConcurrencyPolicy *string            `json:"concurrency_policy"`
	ExecutionMode     *string            `json:"execution_mode"`
}

func automationToResponse(a db.Automation) AutomationResponse {
	var inline *InlineIssueConfig
	if len(a.InlineIssueConfig) > 0 {
		var cfg InlineIssueConfig
		if err := json.Unmarshal(a.InlineIssueConfig, &cfg); err == nil {
			inline = &cfg
		}
	}
	return AutomationResponse{
		ID:                uuidToString(a.ID),
		WorkspaceID:       uuidToString(a.WorkspaceID),
		Title:             a.Title,
		SourceMode:        a.SourceMode,
		TemplateID:        uuidToPtr(a.TemplateID),
		InlineIssueConfig: inline,
		Status:            a.Status,
		ConcurrencyPolicy: a.ConcurrencyPolicy,
		CreatedByType:     a.CreatedByType,
		CreatedByID:       uuidToString(a.CreatedByID),
		LastRunAt:         timestampToPtr(a.LastRunAt),
		CreatedAt:         timestampToString(a.CreatedAt),
		UpdatedAt:         timestampToString(a.UpdatedAt),
	}
}

func (h *Handler) automationTriggerToResponse(t db.AutomationTrigger) AutomationTriggerResponse {
	resp := AutomationTriggerResponse{
		ID:             uuidToString(t.ID),
		AutomationID:   uuidToString(t.AutomationID),
		Kind:           t.Kind,
		Enabled:        t.Enabled,
		CronExpression: textToPtr(t.CronExpression),
		Timezone:       textToPtr(t.Timezone),
		NextRunAt:      timestampToPtr(t.NextRunAt),
		WebhookToken:   textToPtr(t.WebhookToken),
		Label:          textToPtr(t.Label),
		LastFiredAt:    timestampToPtr(t.LastFiredAt),
		CreatedAt:      timestampToString(t.CreatedAt),
		UpdatedAt:      timestampToString(t.UpdatedAt),
	}
	if t.Kind == "webhook" && t.WebhookToken.Valid && t.WebhookToken.String != "" {
		path := automationWebhookPathForToken(t.WebhookToken.String)
		resp.WebhookPath = &path
		if h.cfg.PublicURL != "" {
			full := h.cfg.PublicURL + path
			resp.WebhookURL = &full
		}
		provider := t.Provider
		if provider == "" {
			provider = "generic"
		}
		resp.Provider = &provider
		if t.SigningSecret.Valid && t.SigningSecret.String != "" {
			resp.HasSigningSecret = true
			hint := signingSecretHint(t.SigningSecret.String)
			resp.SigningSecretHint = &hint
		}
		if len(t.EventFilters) > 0 {
			var filters []WebhookEventFilter
			if err := json.Unmarshal(t.EventFilters, &filters); err == nil {
				resp.EventFilters = filters
			}
		}
	}
	return resp
}

func automationWebhookPathForToken(token string) string {
	return "/api/webhooks/automations/" + token
}

func automationRunToResponse(r db.AutomationRun) AutomationRunResponse {
	return automationRunToResponseWithPayload(r, true)
}

func automationRunToResponseSlim(r db.AutomationRun) AutomationRunResponse {
	resp := automationRunToResponseWithPayload(r, false)
	resp.TriggerPayload = nil
	return resp
}

func automationRunToResponseWithPayload(r db.AutomationRun, includePayload bool) AutomationRunResponse {
	var payload any
	if includePayload && len(r.TriggerPayload) > 0 {
		json.Unmarshal(r.TriggerPayload, &payload)
	}
	var resolved any
	if len(r.ResolvedIssuePayload) > 0 {
		json.Unmarshal(r.ResolvedIssuePayload, &resolved)
	}
	var template any
	if len(r.TemplateSnapshot) > 0 {
		json.Unmarshal(r.TemplateSnapshot, &template)
	}
	var result any
	if len(r.Result) > 0 {
		json.Unmarshal(r.Result, &result)
	}
	return AutomationRunResponse{
		ID:                   uuidToString(r.ID),
		AutomationID:         uuidToString(r.AutomationID),
		TriggerID:            uuidToPtr(r.TriggerID),
		Source:               r.Source,
		SourceModeSnapshot:   r.SourceModeSnapshot,
		Status:               r.Status,
		IssueID:              uuidToPtr(r.IssueID),
		TaskID:               uuidToPtr(r.TaskID),
		TriggeredAt:          timestampToString(r.TriggeredAt),
		CompletedAt:          timestampToPtr(r.CompletedAt),
		FailureReason:        textToPtr(r.FailureReason),
		TriggerPayload:       payload,
		ResolvedIssuePayload: resolved,
		TemplateSnapshot:     template,
		Result:               result,
		CreatedAt:            timestampToString(r.CreatedAt),
	}
}

func (h *Handler) ListAutomations(w http.ResponseWriter, r *http.Request) {
	workspaceID := h.resolveWorkspaceID(r)
	var statusFilter pgtype.Text
	if s := r.URL.Query().Get("status"); s != "" {
		statusFilter = pgtype.Text{String: s, Valid: true}
	}
	var modeFilter pgtype.Text
	if s := r.URL.Query().Get("source_mode"); s != "" {
		modeFilter = pgtype.Text{String: s, Valid: true}
	}
	automations, err := h.Queries.ListAutomations(r.Context(), db.ListAutomationsParams{
		WorkspaceID: parseUUID(workspaceID),
		Status:      statusFilter,
		SourceMode:  modeFilter,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list automations")
		return
	}
	resp := make([]AutomationResponse, len(automations))
	for i, automation := range automations {
		resp[i] = automationToResponse(automation)
	}
	writeJSON(w, http.StatusOK, map[string]any{"automations": resp, "total": len(resp)})
}

func (h *Handler) GetAutomation(w http.ResponseWriter, r *http.Request) {
	automation, ok := h.loadAutomationInWorkspace(w, r, chi.URLParam(r, "id"), h.resolveWorkspaceID(r))
	if !ok {
		return
	}
	triggers, err := h.Queries.ListAutomationTriggers(r.Context(), automation.ID)
	if err != nil {
		triggers = nil
	}
	triggerResp := make([]AutomationTriggerResponse, len(triggers))
	for i, trigger := range triggers {
		triggerResp[i] = h.automationTriggerToResponse(trigger)
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"automation": automationToResponse(automation),
		"triggers":   triggerResp,
	})
}

func (h *Handler) loadAutomationInWorkspace(w http.ResponseWriter, r *http.Request, automationID, workspaceID string) (db.Automation, bool) {
	automationUUID, ok := parseUUIDOrBadRequest(w, automationID, "automation id")
	if !ok {
		return db.Automation{}, false
	}
	wsUUID, ok := parseUUIDOrBadRequest(w, workspaceID, "workspace id")
	if !ok {
		return db.Automation{}, false
	}
	automation, err := h.Queries.GetAutomationInWorkspace(r.Context(), db.GetAutomationInWorkspaceParams{
		ID:          automationUUID,
		WorkspaceID: wsUUID,
	})
	if err != nil {
		writeError(w, http.StatusNotFound, "automation not found")
		return db.Automation{}, false
	}
	return automation, true
}

func (h *Handler) CreateAutomation(w http.ResponseWriter, r *http.Request) {
	var req CreateAutomationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	workspaceID := h.resolveWorkspaceID(r)
	wsUUID, ok := parseUUIDOrBadRequest(w, workspaceID, "workspace id")
	if !ok {
		return
	}
	userID, ok := requireUserID(w, r)
	if !ok {
		return
	}
	params, ok := h.createAutomationParamsFromRequest(w, r, req, wsUUID, parseUUID(userID))
	if !ok {
		return
	}
	automation, err := h.Queries.CreateAutomation(r.Context(), params)
	if err != nil {
		if isCheckViolation(err) {
			writeError(w, http.StatusBadRequest, "invalid automation")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to create automation")
		return
	}
	writeJSON(w, http.StatusCreated, automationToResponse(automation))
}

func (h *Handler) createAutomationParamsFromRequest(w http.ResponseWriter, r *http.Request, req CreateAutomationRequest, workspaceID, creatorID pgtype.UUID) (db.CreateAutomationParams, bool) {
	if strings.TrimSpace(req.Title) == "" {
		writeError(w, http.StatusBadRequest, "title is required")
		return db.CreateAutomationParams{}, false
	}
	sourceMode := strings.TrimSpace(req.SourceMode)
	if sourceMode == "" {
		sourceMode = "inline"
	}
	status := "active"
	if req.Status != nil && strings.TrimSpace(*req.Status) != "" {
		status = strings.TrimSpace(*req.Status)
	}
	concurrencyPolicy := "skip"
	if req.ConcurrencyPolicy != nil && strings.TrimSpace(*req.ConcurrencyPolicy) != "" {
		concurrencyPolicy = strings.TrimSpace(*req.ConcurrencyPolicy)
	}
	executionMode := ""
	if req.ExecutionMode != nil {
		executionMode = strings.TrimSpace(*req.ExecutionMode)
	}

	params := db.CreateAutomationParams{
		WorkspaceID:       workspaceID,
		Title:             strings.TrimSpace(req.Title),
		SourceMode:        sourceMode,
		Status:            status,
		ConcurrencyPolicy: concurrencyPolicy,
		CreatedByType:     "member",
		CreatedByID:       creatorID,
	}
	switch sourceMode {
	case "inline":
		if req.InlineIssueConfig == nil || req.TemplateID != nil {
			writeError(w, http.StatusBadRequest, "inline automation requires inline_issue_config and forbids template_id")
			return db.CreateAutomationParams{}, false
		}
		config, ok := h.validateInlineIssueConfig(w, r, workspaceID, req.InlineIssueConfig, executionMode)
		if !ok {
			return db.CreateAutomationParams{}, false
		}
		raw, err := json.Marshal(config)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to encode inline_issue_config")
			return db.CreateAutomationParams{}, false
		}
		params.InlineIssueConfig = raw
	case "template":
		if req.TemplateID == nil || req.InlineIssueConfig != nil {
			writeError(w, http.StatusBadRequest, "template automation requires template_id and forbids inline_issue_config")
			return db.CreateAutomationParams{}, false
		}
		if executionMode == automationExecutionRunOnly {
			writeError(w, http.StatusBadRequest, "template automation does not support run_only")
			return db.CreateAutomationParams{}, false
		}
		templateID, ok := parseUUIDOrBadRequest(w, *req.TemplateID, "template_id")
		if !ok {
			return db.CreateAutomationParams{}, false
		}
		if _, err := h.Queries.GetIssueTemplateInWorkspace(r.Context(), db.GetIssueTemplateInWorkspaceParams{
			ID:          templateID,
			WorkspaceID: workspaceID,
		}); err != nil {
			writeError(w, http.StatusBadRequest, "template_id must reference an issue template in this workspace")
			return db.CreateAutomationParams{}, false
		}
		params.TemplateID = templateID
	default:
		writeError(w, http.StatusBadRequest, "invalid source_mode")
		return db.CreateAutomationParams{}, false
	}
	return params, true
}

func (h *Handler) validateInlineIssueConfig(w http.ResponseWriter, r *http.Request, workspaceID pgtype.UUID, raw *InlineIssueConfig, executionMode string) (InlineIssueConfig, bool) {
	config := *raw
	if strings.TrimSpace(config.IssueTitleTemplate) == "" {
		writeError(w, http.StatusBadRequest, "inline_issue_config.issue_title_template is required")
		return InlineIssueConfig{}, false
	}
	if err := service.ValidateIssueTitleTemplate(config.IssueTitleTemplate); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return InlineIssueConfig{}, false
	}
	if strings.TrimSpace(config.AssigneeID) == "" {
		writeError(w, http.StatusBadRequest, "inline_issue_config.assignee_id is required")
		return InlineIssueConfig{}, false
	}
	assigneeID, ok := parseUUIDOrBadRequest(w, config.AssigneeID, "inline_issue_config.assignee_id")
	if !ok {
		return InlineIssueConfig{}, false
	}
	if config.AssigneeType == "" {
		config.AssigneeType = "agent"
	}
	if !isValidAutopilotAssigneeType(config.AssigneeType) {
		writeError(w, http.StatusBadRequest, "inline_issue_config.assignee_type must be agent or squad")
		return InlineIssueConfig{}, false
	}
	if !h.validateAutopilotAssignee(w, r, config.AssigneeType, assigneeID, workspaceID) {
		return InlineIssueConfig{}, false
	}
	if _, ok := h.parseAutopilotProjectID(w, r, config.ProjectID, workspaceID); !ok {
		return InlineIssueConfig{}, false
	}
	if config.Priority == "" {
		config.Priority = "none"
	}
	if executionMode != "" {
		config.ExecutionMode = executionMode
	}
	if config.ExecutionMode == "" {
		config.ExecutionMode = automationExecutionCreateIssue
	}
	if config.ExecutionMode != automationExecutionCreateIssue && config.ExecutionMode != automationExecutionRunOnly {
		writeError(w, http.StatusBadRequest, "execution_mode must be create_issue or run_only")
		return InlineIssueConfig{}, false
	}
	return config, true
}

func (h *Handler) UpdateAutomation(w http.ResponseWriter, r *http.Request) {
	automation, ok := h.loadAutomationInWorkspace(w, r, chi.URLParam(r, "id"), h.resolveWorkspaceID(r))
	if !ok {
		return
	}
	bodyBytes, err := io.ReadAll(r.Body)
	if err != nil {
		writeError(w, http.StatusBadRequest, "failed to read request body")
		return
	}
	var req UpdateAutomationRequest
	if err := json.Unmarshal(bodyBytes, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	var rawFields map[string]json.RawMessage
	json.Unmarshal(bodyBytes, &rawFields)

	if req.Title != nil || req.Status != nil || req.ConcurrencyPolicy != nil {
		params := db.UpdateAutomationParams{ID: automation.ID}
		if req.Title != nil {
			params.Title = pgtype.Text{String: strings.TrimSpace(*req.Title), Valid: true}
		}
		if req.Status != nil {
			params.Status = pgtype.Text{String: strings.TrimSpace(*req.Status), Valid: true}
		}
		if req.ConcurrencyPolicy != nil {
			params.ConcurrencyPolicy = pgtype.Text{String: strings.TrimSpace(*req.ConcurrencyPolicy), Valid: true}
		}
		updated, err := h.Queries.UpdateAutomation(r.Context(), params)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to update automation")
			return
		}
		automation = updated
	}
	sourceSent := req.SourceMode != nil || req.TemplateID != nil || req.InlineIssueConfig != nil || rawFields["execution_mode"] != nil
	if sourceSent {
		sourceMode := automation.SourceMode
		if req.SourceMode != nil && strings.TrimSpace(*req.SourceMode) != "" {
			sourceMode = strings.TrimSpace(*req.SourceMode)
		}
		switch sourceMode {
		case "inline":
			if req.InlineIssueConfig == nil {
				writeError(w, http.StatusBadRequest, "inline automation requires inline_issue_config")
				return
			}
			executionMode := ""
			if req.ExecutionMode != nil {
				executionMode = strings.TrimSpace(*req.ExecutionMode)
			}
			config, ok := h.validateInlineIssueConfig(w, r, automation.WorkspaceID, req.InlineIssueConfig, executionMode)
			if !ok {
				return
			}
			raw, _ := json.Marshal(config)
			updated, err := h.Queries.ReplaceAutomationInlineSource(r.Context(), db.ReplaceAutomationInlineSourceParams{
				ID:                automation.ID,
				InlineIssueConfig: raw,
			})
			if err != nil {
				writeError(w, http.StatusInternalServerError, "failed to update automation")
				return
			}
			automation = updated
		case "template":
			if req.InlineIssueConfig != nil {
				writeError(w, http.StatusBadRequest, "template automation forbids inline_issue_config")
				return
			}
			if req.ExecutionMode != nil && strings.TrimSpace(*req.ExecutionMode) == automationExecutionRunOnly {
				writeError(w, http.StatusBadRequest, "template automation does not support run_only")
				return
			}
			if req.TemplateID == nil {
				writeError(w, http.StatusBadRequest, "template automation requires template_id")
				return
			}
			templateID, ok := parseUUIDOrBadRequest(w, *req.TemplateID, "template_id")
			if !ok {
				return
			}
			if _, err := h.Queries.GetIssueTemplateInWorkspace(r.Context(), db.GetIssueTemplateInWorkspaceParams{
				ID:          templateID,
				WorkspaceID: automation.WorkspaceID,
			}); err != nil {
				writeError(w, http.StatusBadRequest, "template_id must reference an issue template in this workspace")
				return
			}
			updated, err := h.Queries.ReplaceAutomationTemplateSource(r.Context(), db.ReplaceAutomationTemplateSourceParams{
				ID:         automation.ID,
				TemplateID: templateID,
			})
			if err != nil {
				writeError(w, http.StatusInternalServerError, "failed to update automation")
				return
			}
			automation = updated
		default:
			writeError(w, http.StatusBadRequest, "invalid source_mode")
			return
		}
	}
	writeJSON(w, http.StatusOK, automationToResponse(automation))
}

func (h *Handler) DeleteAutomation(w http.ResponseWriter, r *http.Request) {
	automation, ok := h.loadAutomationInWorkspace(w, r, chi.URLParam(r, "id"), h.resolveWorkspaceID(r))
	if !ok {
		return
	}
	if err := h.Queries.DeleteAutomation(r.Context(), automation.ID); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to delete automation")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) CreateAutomationTrigger(w http.ResponseWriter, r *http.Request) {
	automation, ok := h.loadAutomationInWorkspace(w, r, chi.URLParam(r, "id"), h.resolveWorkspaceID(r))
	if !ok {
		return
	}
	var req CreateAutopilotTriggerRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	trigger, ok := h.createAutomationTriggerFromRequest(w, r, automation.ID, req)
	if !ok {
		return
	}
	writeJSON(w, http.StatusCreated, h.automationTriggerToResponse(trigger))
}

func (h *Handler) createAutomationTriggerFromRequest(w http.ResponseWriter, r *http.Request, automationID pgtype.UUID, req CreateAutopilotTriggerRequest) (db.AutomationTrigger, bool) {
	if req.Kind == "" {
		writeError(w, http.StatusBadRequest, "kind is required")
		return db.AutomationTrigger{}, false
	}
	if req.Kind != "schedule" && req.Kind != "webhook" {
		writeError(w, http.StatusBadRequest, "kind must be schedule or webhook")
		return db.AutomationTrigger{}, false
	}
	if req.Kind == "schedule" && (req.CronExpression == nil || *req.CronExpression == "") {
		writeError(w, http.StatusBadRequest, "cron_expression is required for schedule triggers")
		return db.AutomationTrigger{}, false
	}
	if req.Kind == "webhook" && req.Timezone != nil && *req.Timezone != "" {
		writeError(w, http.StatusBadRequest, "timezone is not valid for webhook triggers")
		return db.AutomationTrigger{}, false
	}
	if req.Kind != "webhook" && len(req.EventFilters) > 0 {
		writeError(w, http.StatusBadRequest, "event_filters is only valid for webhook triggers")
		return db.AutomationTrigger{}, false
	}
	if err := validateWebhookEventFilters(req.EventFilters); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return db.AutomationTrigger{}, false
	}
	provider := "generic"
	if req.Provider != nil && *req.Provider != "" {
		if req.Kind != "webhook" {
			writeError(w, http.StatusBadRequest, "provider is only valid for webhook triggers")
			return db.AutomationTrigger{}, false
		}
		if !isAllowedWebhookProvider(*req.Provider) {
			writeError(w, http.StatusBadRequest, "provider must be generic or github")
			return db.AutomationTrigger{}, false
		}
		provider = *req.Provider
	}
	var nextRunAt pgtype.Timestamptz
	var cronText pgtype.Text
	var tzText pgtype.Text
	var webhookToken pgtype.Text
	var eventFilters []byte
	if req.Kind == "schedule" {
		cronText = ptrToText(req.CronExpression)
		tzText = ptrToText(req.Timezone)
		tz := "UTC"
		if req.Timezone != nil && *req.Timezone != "" {
			tz = *req.Timezone
		}
		t, err := computeNextRun(*req.CronExpression, tz)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return db.AutomationTrigger{}, false
		}
		nextRunAt = pgtype.Timestamptz{Time: t, Valid: true}
	} else {
		token, err := generateWebhookToken()
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to generate webhook token")
			return db.AutomationTrigger{}, false
		}
		webhookToken = pgtype.Text{String: token, Valid: true}
		encoded, err := encodeWebhookEventFilters(req.EventFilters)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to encode event_filters")
			return db.AutomationTrigger{}, false
		}
		eventFilters = encoded
	}
	trigger, err := h.Queries.CreateAutomationTrigger(r.Context(), db.CreateAutomationTriggerParams{
		AutomationID:   automationID,
		Kind:           req.Kind,
		Enabled:        true,
		CronExpression: cronText,
		Timezone:       tzText,
		NextRunAt:      nextRunAt,
		WebhookToken:   webhookToken,
		Label:          ptrToText(req.Label),
		Provider:       pgtype.Text{String: provider, Valid: provider != ""},
		EventFilters:   eventFilters,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create trigger")
		return db.AutomationTrigger{}, false
	}
	return trigger, true
}

func (h *Handler) ListAutomationRuns(w http.ResponseWriter, r *http.Request) {
	automation, ok := h.loadAutomationInWorkspace(w, r, chi.URLParam(r, "id"), h.resolveWorkspaceID(r))
	if !ok {
		return
	}
	limit := int32(20)
	offset := int32(0)
	if l := r.URL.Query().Get("limit"); l != "" {
		if v, err := strconv.Atoi(l); err == nil && v > 0 {
			limit = int32(v)
		}
	}
	if limit > 100 {
		limit = 100
	}
	if o := r.URL.Query().Get("offset"); o != "" {
		if v, err := strconv.Atoi(o); err == nil && v >= 0 {
			offset = int32(v)
		}
	}
	runs, err := h.Queries.ListAutomationRuns(r.Context(), db.ListAutomationRunsParams{
		AutomationID: automation.ID,
		Limit:        limit,
		Offset:       offset,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list runs")
		return
	}
	resp := make([]AutomationRunResponse, len(runs))
	for i, run := range runs {
		resp[i] = automationRunToResponseSlim(run)
	}
	writeJSON(w, http.StatusOK, map[string]any{"runs": resp, "total": len(resp)})
}

func (h *Handler) GetAutomationRun(w http.ResponseWriter, r *http.Request) {
	automation, ok := h.loadAutomationInWorkspace(w, r, chi.URLParam(r, "id"), h.resolveWorkspaceID(r))
	if !ok {
		return
	}
	runID, ok := parseUUIDOrBadRequest(w, chi.URLParam(r, "runId"), "run id")
	if !ok {
		return
	}
	run, err := h.Queries.GetAutomationRun(r.Context(), runID)
	if err != nil || uuidToString(run.AutomationID) != uuidToString(automation.ID) {
		writeError(w, http.StatusNotFound, "run not found")
		return
	}
	writeJSON(w, http.StatusOK, automationRunToResponse(run))
}

func (h *Handler) TriggerAutomation(w http.ResponseWriter, r *http.Request) {
	automation, ok := h.loadAutomationInWorkspace(w, r, chi.URLParam(r, "id"), h.resolveWorkspaceID(r))
	if !ok {
		return
	}
	if automation.Status != "active" {
		writeError(w, http.StatusBadRequest, "automation is not active")
		return
	}
	run, err := h.dispatchAutomation(r.Context(), r, automation, pgtype.UUID{}, "manual", nil)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to trigger automation: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, automationRunToResponse(*run))
}

func (h *Handler) dispatchAutomation(ctx context.Context, r *http.Request, automation db.Automation, triggerID pgtype.UUID, source string, payload []byte) (*db.AutomationRun, error) {
	createParams, resolvedPayload, templateSnapshot, runOnly, triggerSummary, err := h.resolveAutomationIssuePayload(ctx, r, automation, triggerID, source, payload)
	if err != nil {
		return nil, err
	}
	return h.AutopilotService.DispatchAutomation(ctx, automation, triggerID, source, payload, createParams, resolvedPayload, templateSnapshot, runOnly, triggerSummary)
}

// DispatchAutomation exposes the automation dispatch path to server-side
// callers such as the scheduler while keeping HTTP handlers on the same
// resolution logic as manual/webhook triggers.
func (h *Handler) DispatchAutomation(ctx context.Context, r *http.Request, automation db.Automation, triggerID pgtype.UUID, source string, payload []byte) (*db.AutomationRun, error) {
	return h.dispatchAutomation(ctx, r, automation, triggerID, source, payload)
}

func (h *Handler) resolveAutomationIssuePayload(ctx context.Context, r *http.Request, automation db.Automation, triggerID pgtype.UUID, source string, payload []byte) (service.IssueCreateParams, []byte, []byte, bool, string, error) {
	switch automation.SourceMode {
	case "inline":
		var cfg InlineIssueConfig
		if err := json.Unmarshal(automation.InlineIssueConfig, &cfg); err != nil {
			return service.IssueCreateParams{}, nil, nil, false, "", fmt.Errorf("invalid inline_issue_config: %w", err)
		}
		params, err := h.issueCreateParamsFromInlineAutomation(ctx, automation, cfg, triggerID, source, payload)
		if err != nil {
			return service.IssueCreateParams{}, nil, nil, false, "", err
		}
		resolved, err := json.Marshal(service.ResolvedIssuePayload{
			Title:        params.Title,
			Body:         params.Description.String,
			AssigneeType: params.AssigneeType.String,
			AssigneeID:   params.AssigneeID,
			ProjectID:    params.ProjectID,
			Priority:     params.Priority,
		})
		return params, resolved, nil, cfg.ExecutionMode == automationExecutionRunOnly, automation.Title, err
	case "template":
		template, err := h.Queries.GetIssueTemplate(ctx, automation.TemplateID)
		if err != nil {
			return service.IssueCreateParams{}, nil, nil, false, "", err
		}
		instantiator := service.NewIssueInstantiator()
		params, err := instantiator.IssueCreateParamsFromTemplate(template, automation.CreatedByType, automation.CreatedByID)
		if err != nil {
			return service.IssueCreateParams{}, nil, nil, false, "", err
		}
		params.AllowDuplicate = true
		resolved, err := instantiator.ResolveTemplate(template)
		if err != nil {
			return service.IssueCreateParams{}, nil, nil, false, "", err
		}
		resolvedBytes, err := json.Marshal(resolved)
		if err != nil {
			return service.IssueCreateParams{}, nil, nil, false, "", err
		}
		snapshot, err := instantiator.SnapshotTemplate(template)
		return params, resolvedBytes, snapshot, false, automation.Title, err
	default:
		return service.IssueCreateParams{}, nil, nil, false, "", fmt.Errorf("unknown automation source_mode %q", automation.SourceMode)
	}
}

func (h *Handler) issueCreateParamsFromInlineAutomation(ctx context.Context, automation db.Automation, cfg InlineIssueConfig, triggerID pgtype.UUID, source string, payload []byte) (service.IssueCreateParams, error) {
	if cfg.AssigneeType == "" {
		cfg.AssigneeType = "agent"
	}
	if cfg.Priority == "" {
		cfg.Priority = "none"
	}
	assigneeID, err := automationUUIDFromString(cfg.AssigneeID)
	if err != nil {
		return service.IssueCreateParams{}, err
	}
	var projectID pgtype.UUID
	if cfg.ProjectID != nil && strings.TrimSpace(*cfg.ProjectID) != "" {
		projectID, err = automationUUIDFromString(*cfg.ProjectID)
		if err != nil {
			return service.IssueCreateParams{}, err
		}
	}
	body := ""
	if cfg.IssueBodyTemplate != nil {
		body = *cfg.IssueBodyTemplate
	}
	triggerTimezone := service.DefaultAutopilotTriggerTimezone
	if triggerID.Valid {
		if trigger, err := h.Queries.GetAutomationTrigger(ctx, triggerID); err == nil && trigger.Timezone.Valid && trigger.Timezone.String != "" {
			triggerTimezone = trigger.Timezone.String
		}
	}
	triggeredAt := time.Now().UTC()
	title := renderAutomationIssueTitle(cfg.IssueTitleTemplate, triggeredAt, triggerTimezone)
	body = buildAutomationIssueDescription(body, source, triggeredAt, triggerTimezone, payload)
	return service.IssueCreateParams{
		WorkspaceID:    automation.WorkspaceID,
		Title:          title,
		Description:    pgtype.Text{String: body, Valid: body != ""},
		Status:         "todo",
		Priority:       cfg.Priority,
		AssigneeType:   pgtype.Text{String: cfg.AssigneeType, Valid: cfg.AssigneeType != ""},
		AssigneeID:     assigneeID,
		CreatorType:    automation.CreatedByType,
		CreatorID:      automation.CreatedByID,
		ProjectID:      projectID,
		AllowDuplicate: true,
	}, nil
}

func renderAutomationIssueTitle(tmpl string, triggeredAt time.Time, triggerTimezone string) string {
	if tmpl == "" {
		return tmpl
	}
	triggerDate := formatAutomationTime(triggeredAt, triggerTimezone, "2006-01-02")
	return service.RenderIssueTitleTemplateWithDate(tmpl, triggerDate)
}

func buildAutomationIssueDescription(base string, source string, triggeredAt time.Time, triggerTimezone string, payload []byte) string {
	triggeredAtText := formatAutomationTime(triggeredAt, triggerTimezone, "2006-01-02 15:04 MST")
	var b strings.Builder
	b.WriteString(base)
	b.WriteString("\n\n---\n*Automation run triggered by ")
	if source == "" {
		source = "manual"
	}
	b.WriteString(source)
	b.WriteString(" at ")
	b.WriteString(triggeredAtText)
	b.WriteString(". After starting work, rename this issue to accurately reflect what you are doing.*")

	if source == "webhook" && len(payload) > 0 {
		event := "webhook.received"
		var payloadJSON []byte
		var env struct {
			Event        string          `json:"event"`
			EventPayload json.RawMessage `json:"eventPayload"`
		}
		if err := json.Unmarshal(payload, &env); err == nil {
			if env.Event != "" {
				event = env.Event
			}
			if len(env.EventPayload) > 0 {
				if pretty, err := prettifyAutomationJSON(env.EventPayload); err == nil {
					payloadJSON = pretty
				}
			}
		}
		if len(payloadJSON) == 0 {
			if pretty, err := prettifyAutomationJSON(payload); err == nil {
				payloadJSON = pretty
			} else {
				payloadJSON = payload
			}
		}
		b.WriteString("\n\nWebhook event: ")
		b.WriteString(event)
		b.WriteString("\n\nWebhook payload:\n```json\n")
		b.Write(payloadJSON)
		b.WriteString("\n```")
	}

	return b.String()
}

func prettifyAutomationJSON(raw []byte) ([]byte, error) {
	var v any
	if err := json.Unmarshal(raw, &v); err != nil {
		return nil, err
	}
	return json.MarshalIndent(v, "", "  ")
}

func formatAutomationTime(t time.Time, timezone string, layout string) string {
	label := strings.TrimSpace(timezone)
	if label == "" {
		label = service.DefaultAutopilotTriggerTimezone
	}
	loc, err := time.LoadLocation(label)
	if err != nil {
		loc = time.UTC
		label = service.DefaultAutopilotTriggerTimezone
	}
	return t.In(loc).Format(layout)
}

func automationUUIDFromString(raw string) (pgtype.UUID, error) {
	var id pgtype.UUID
	if err := id.Scan(raw); err != nil {
		return pgtype.UUID{}, err
	}
	return id, nil
}

func (h *Handler) HandleAutomationWebhook(w http.ResponseWriter, r *http.Request) {
	token := chi.URLParam(r, "token")
	if token == "" {
		writeError(w, http.StatusNotFound, "webhook not found")
		return
	}
	trigRow, err := h.Queries.GetAutomationWebhookTriggerByToken(r.Context(), pgtype.Text{String: token, Valid: true})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeError(w, http.StatusNotFound, "webhook not found")
			return
		}
		slog.Error("automation webhook: token lookup failed", "error", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	middleware.SetWebhookTriggerID(r, uuidToString(trigRow.ID))
	r.Body = http.MaxBytesReader(w, r.Body, maxWebhookBodyBytes)
	body, err := io.ReadAll(r.Body)
	if err != nil {
		var mbe *http.MaxBytesError
		if errors.As(err, &mbe) {
			writeError(w, http.StatusRequestEntityTooLarge, "payload too large")
			return
		}
		writeError(w, http.StatusBadRequest, "failed to read request body")
		return
	}
	automation, err := h.Queries.GetAutomation(r.Context(), trigRow.AutomationID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeError(w, http.StatusNotFound, "webhook not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	if uuidToString(automation.WorkspaceID) != uuidToString(trigRow.AutomationWorkspaceID) {
		writeError(w, http.StatusNotFound, "webhook not found")
		return
	}
	envelope, err := normalizeWebhookPayload(body, r.Header)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	provider := trigRow.Provider
	if provider == "" {
		provider = "generic"
	}
	dedupeKey, dedupeSource := extractDedupeKey(provider, r.Header)
	sigStatus := verifyWebhookSignatureForProvider(provider, trigRow.SigningSecret.String, r.Header, body)
	delivery, dup, err := h.persistInboundDelivery(r, persistDeliveryInput{
		WorkspaceID:         automation.WorkspaceID,
		AutomationID:        automation.ID,
		AutomationTriggerID: trigRow.ID,
		Provider:            provider,
		Event:               envelope.Event,
		DedupeKey:           dedupeKey,
		DedupeSource:        dedupeSource,
		SignatureStatus:     sigStatus,
		ContentType:         envelope.Request.ContentType,
		RawBody:             body,
		SelectedHeaders:     selectedHeadersJSON(r.Header),
	})
	if err != nil {
		slog.Error("automation webhook: persist delivery failed", "error", err, "trigger_id", uuidToString(trigRow.ID))
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	if dup {
		resp := map[string]any{
			"status":      "duplicate",
			"delivery_id": uuidToString(delivery.ID),
		}
		if delivery.AutomationRunID.Valid {
			resp["run_id"] = uuidToString(delivery.AutomationRunID)
		}
		writeJSON(w, http.StatusOK, resp)
		return
	}
	if sigStatus == sigStatusInvalid || sigStatus == sigStatusMissing {
		reason := "invalid_signature"
		if sigStatus == sigStatusMissing {
			reason = "missing_signature"
		}
		respBody := map[string]any{"status": "rejected", "delivery_id": uuidToString(delivery.ID), "reason": reason}
		h.finaliseDeliveryTerminal(r, delivery.ID, deliveryStatusRejected, http.StatusUnauthorized, respBody, reason)
		writeJSON(w, http.StatusUnauthorized, respBody)
		return
	}
	if !trigRow.Enabled {
		respBody := map[string]any{"status": "ignored", "delivery_id": uuidToString(delivery.ID), "reason": "trigger_disabled"}
		h.finaliseDeliveryTerminal(r, delivery.ID, deliveryStatusIgnored, http.StatusOK, respBody, "trigger_disabled")
		writeJSON(w, http.StatusOK, respBody)
		return
	}
	if automation.Status == "archived" {
		respBody := map[string]any{"status": "ignored", "delivery_id": uuidToString(delivery.ID), "reason": "automation_archived"}
		h.finaliseDeliveryTerminal(r, delivery.ID, deliveryStatusIgnored, http.StatusOK, respBody, "automation_archived")
		writeJSON(w, http.StatusOK, respBody)
		return
	}
	if automation.Status != "active" {
		respBody := map[string]any{"status": "ignored", "delivery_id": uuidToString(delivery.ID), "reason": "automation_paused"}
		h.finaliseDeliveryTerminal(r, delivery.ID, deliveryStatusIgnored, http.StatusOK, respBody, "automation_paused")
		writeJSON(w, http.StatusOK, respBody)
		return
	}
	if !webhookEventAllowedByTriggerScope(trigRow.EventFilters, envelope) {
		respBody := map[string]any{"status": "ignored", "delivery_id": uuidToString(delivery.ID), "reason": "event_filtered", "event": envelope.Event}
		h.finaliseDeliveryTerminal(r, delivery.ID, deliveryStatusIgnored, http.StatusOK, respBody, "event_filtered")
		writeJSON(w, http.StatusOK, respBody)
		return
	}
	envelopeBytes, err := json.Marshal(envelope)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to encode envelope")
		return
	}
	run, err := h.dispatchAutomation(r.Context(), r, automation, trigRow.ID, "webhook", envelopeBytes)
	if err != nil {
		respBody := map[string]any{"error": "failed to dispatch automation"}
		if run != nil {
			h.finaliseDeliveryWithAutomationRun(r, delivery.ID, deliveryStatusFailed, run.ID, http.StatusInternalServerError, respBody)
		} else {
			h.finaliseDeliveryTerminal(r, delivery.ID, deliveryStatusFailed, http.StatusInternalServerError, respBody, err.Error())
		}
		writeError(w, http.StatusInternalServerError, "failed to dispatch automation")
		return
	}
	if err := h.Queries.TouchAutomationTriggerFiredAt(r.Context(), trigRow.ID); err != nil {
		slog.Warn("automation webhook: failed to touch last_fired_at", "trigger_id", uuidToString(trigRow.ID), "error", err)
	}
	resp := map[string]any{
		"status":        "accepted",
		"delivery_id":   uuidToString(delivery.ID),
		"run_id":        uuidToString(run.ID),
		"automation_id": uuidToString(automation.ID),
		"trigger_id":    uuidToString(trigRow.ID),
	}
	if run.Status == "skipped" {
		resp["status"] = "skipped"
		if run.FailureReason.Valid {
			resp["reason"] = run.FailureReason.String
		}
	}
	h.finaliseDeliveryWithAutomationRun(r, delivery.ID, deliveryStatusDispatched, run.ID, http.StatusOK, resp)
	writeJSON(w, http.StatusOK, resp)
}
