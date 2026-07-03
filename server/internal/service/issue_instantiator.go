package service

import (
	"encoding/json"
	"errors"
	"strings"

	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

var ErrTemplateIssueTitleRequired = errors.New("template issue title is required")

type ResolvedIssuePayload struct {
	Title        string      `json:"title"`
	Body         string      `json:"body"`
	AssigneeType string      `json:"assignee_type"`
	AssigneeID   pgtype.UUID `json:"assignee_id"`
	ProjectID    pgtype.UUID `json:"project_id"`
	Priority     string      `json:"priority"`
}

type IssueTemplateSnapshot struct {
	ID                 pgtype.UUID     `json:"id"`
	WorkspaceID        pgtype.UUID     `json:"workspace_id"`
	ProjectID          pgtype.UUID     `json:"project_id,omitempty"`
	Title              string          `json:"title"`
	IssueTitleTemplate string          `json:"issue_title_template"`
	IssueBodyTemplate  string          `json:"issue_body_template,omitempty"`
	AssigneeType       string          `json:"assignee_type"`
	AssigneeID         pgtype.UUID     `json:"assignee_id"`
	Priority           string          `json:"priority"`
	Labels             json.RawMessage `json:"labels,omitempty"`
	DefaultMetadata    json.RawMessage `json:"default_metadata,omitempty"`
	ExecutionSpec      json.RawMessage `json:"execution_spec,omitempty"`
}

type IssueInstantiator struct{}

func NewIssueInstantiator() *IssueInstantiator {
	return &IssueInstantiator{}
}

func (s *IssueInstantiator) ResolveTemplate(template db.IssueTemplate) (ResolvedIssuePayload, error) {
	payload := ResolvedIssuePayload{
		Title:        template.IssueTitleTemplate,
		Body:         template.IssueBodyTemplate.String,
		AssigneeType: template.AssigneeType,
		AssigneeID:   template.AssigneeID,
		ProjectID:    template.ProjectID,
		Priority:     template.Priority,
	}
	if strings.TrimSpace(payload.Title) == "" {
		return ResolvedIssuePayload{}, ErrTemplateIssueTitleRequired
	}
	if payload.Priority == "" {
		payload.Priority = "medium"
	}
	return payload, nil
}

func (s *IssueInstantiator) SnapshotTemplate(template db.IssueTemplate) ([]byte, error) {
	body := ""
	if template.IssueBodyTemplate.Valid {
		body = template.IssueBodyTemplate.String
	}
	return json.Marshal(IssueTemplateSnapshot{
		ID:                 template.ID,
		WorkspaceID:        template.WorkspaceID,
		ProjectID:          template.ProjectID,
		Title:              template.Title,
		IssueTitleTemplate: template.IssueTitleTemplate,
		IssueBodyTemplate:  body,
		AssigneeType:       template.AssigneeType,
		AssigneeID:         template.AssigneeID,
		Priority:           template.Priority,
		Labels:             rawMessageOrNil(template.Labels),
		DefaultMetadata:    rawMessageOrNil(template.DefaultMetadata),
		ExecutionSpec:      rawMessageOrNil(template.ExecutionSpec),
	})
}

func rawMessageOrNil(raw []byte) json.RawMessage {
	trimmed := strings.TrimSpace(string(raw))
	if trimmed == "" {
		return nil
	}
	return json.RawMessage(trimmed)
}

func (s *IssueInstantiator) IssueCreateParamsFromTemplate(template db.IssueTemplate, creatorType string, creatorID pgtype.UUID) (IssueCreateParams, error) {
	payload, err := s.ResolveTemplate(template)
	if err != nil {
		return IssueCreateParams{}, err
	}
	snapshot, err := s.SnapshotTemplate(template)
	if err != nil {
		return IssueCreateParams{}, err
	}
	return IssueCreateParams{
		WorkspaceID:           template.WorkspaceID,
		Title:                 payload.Title,
		Description:           pgtype.Text{String: payload.Body, Valid: payload.Body != ""},
		Status:                "todo",
		Priority:              payload.Priority,
		AssigneeType:          pgtype.Text{String: payload.AssigneeType, Valid: payload.AssigneeType != ""},
		AssigneeID:            payload.AssigneeID,
		CreatorType:           creatorType,
		CreatorID:             creatorID,
		ProjectID:             payload.ProjectID,
		IssueTemplateID:       template.ID,
		IssueTemplateSnapshot: snapshot,
	}, nil
}
