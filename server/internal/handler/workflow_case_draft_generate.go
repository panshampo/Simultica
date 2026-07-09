package handler

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
)

type generateWorkflowDraftRequest struct {
	Instruction   string `json:"instruction"`
	SourceIssueID string `json:"source_issue_id"`
}

type generateWorkflowDraftResponse struct {
	Status  string `json:"status"`
	Message string `json:"message"`
}

// GenerateWorkflowDraft is the entry point for Agent Draft: an agent drafts how
// a Workflow should run, a human inspects and sets it active. This first stage
// only accepts the request and reports it as queued; planner-task wiring lands
// in a later change. Agent Draft is a source of the Draft, not a new entity.
func (h *Handler) GenerateWorkflowDraft(w http.ResponseWriter, r *http.Request) {
	c, ok := h.workflowCaseFromRequest(w, r)
	if !ok {
		return
	}
	_ = c

	var req generateWorkflowDraftRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil && !errors.Is(err, io.EOF) {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	_ = strings.TrimSpace(req.Instruction)

	writeJSON(w, http.StatusAccepted, generateWorkflowDraftResponse{
		Status:  "queued",
		Message: "Agent draft generation is queued.",
	})
}
