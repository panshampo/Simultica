package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

type reviewWorkflowRunStepRequest struct {
	Decision string `json:"decision"`
	Comment  string `json:"comment"`
}

type reviewWorkflowRunStepResponse struct {
	RunID      string    `json:"run_id"`
	StepID     string    `json:"step_id"`
	Decision   string    `json:"decision"`
	ReviewedBy string    `json:"reviewed_by"`
	ReviewedAt time.Time `json:"reviewed_at"`
}

// ReviewWorkflowRunStep records a human approve/reject decision on a Review Step
// that is currently waiting (node status = pending_review). It only emits a
// binary decision as step output; workflow routing decides the next step. It
// never lets the caller pick the next step or mutate the run snapshot.
func (h *Handler) ReviewWorkflowRunStep(w http.ResponseWriter, r *http.Request) {
	c, ok := h.workflowCaseFromRequest(w, r)
	if !ok {
		return
	}
	runID, ok := parseUUIDOrBadRequest(w, chi.URLParam(r, "runId"), "workflow run id")
	if !ok {
		return
	}
	stepID := strings.TrimSpace(chi.URLParam(r, "stepId"))
	if stepID == "" {
		writeError(w, http.StatusBadRequest, "step id is required")
		return
	}
	reviewedBy, ok := requireUserID(w, r)
	if !ok {
		return
	}

	var req reviewWorkflowRunStepRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	decision := strings.TrimSpace(req.Decision)
	if decision != "approved" && decision != "rejected" {
		writeError(w, http.StatusBadRequest, "decision must be approved or rejected")
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

	// The step must be a human_review node currently pending review.
	node, err := h.Queries.GetWorkflowRunNode(r.Context(), db.GetWorkflowRunNodeParams{
		RunID:  runID,
		NodeID: stepID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeError(w, http.StatusNotFound, "review step not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "load review step: "+err.Error())
		return
	}
	if node.NodeType != "human_review" {
		writeError(w, http.StatusConflict, "step is not a review step")
		return
	}
	if node.Status != "pending_review" {
		writeError(w, http.StatusConflict, "review step is not pending review")
		return
	}

	reviewedAt := time.Now().UTC()
	output := map[string]any{
		"review_decision": decision,
		"review_comment":  strings.TrimSpace(req.Comment),
		"reviewed_by":     reviewedBy,
		"reviewed_at":     reviewedAt.Format(time.RFC3339),
	}
	outputRaw, err := json.Marshal(output)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "encode review output: "+err.Error())
		return
	}

	tx, err := h.TxStarter.Begin(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "begin review transaction: "+err.Error())
		return
	}
	defer tx.Rollback(r.Context())
	qtx := h.Queries.WithTx(tx)

	// Guarded update: only transitions a pending_review node. A racing review
	// loses here and gets a 409 instead of double-writing the decision.
	if _, err := qtx.MarkWorkflowRunNodeReviewed(r.Context(), db.MarkWorkflowRunNodeReviewedParams{
		RunID:          runID,
		NodeID:         stepID,
		OutputSnapshot: outputRaw,
	}); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeError(w, http.StatusConflict, "review step is not pending review")
			return
		}
		writeError(w, http.StatusInternalServerError, "record review decision: "+err.Error())
		return
	}

	eventType := "node_review_approved"
	if decision == "rejected" {
		eventType = "node_review_rejected"
	}
	if _, err := qtx.CreateWorkflowRunNodeEvent(r.Context(), db.CreateWorkflowRunNodeEventParams{
		WorkspaceID:    run.WorkspaceID,
		CaseID:         run.CaseID,
		RunID:          runID,
		NodeID:         stepID,
		EventType:      eventType,
		Attempt:        1,
		OutputSnapshot: outputRaw,
	}); err != nil {
		writeError(w, http.StatusInternalServerError, "record review event: "+err.Error())
		return
	}

	nextNodesState, err := applyReviewDecisionToNodesState(run.NodesState, stepID, output)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "project review decision: "+err.Error())
		return
	}
	// The run leaves the review gate and resumes; routing decides the next step.
	updatedRun, err := qtx.UpdateWorkflowRunProgress(r.Context(), db.UpdateWorkflowRunProgressParams{
		ID:         runID,
		Status:     pgtype.Text{String: "running", Valid: true},
		NodesState: nextNodesState,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "resume workflow run: "+err.Error())
		return
	}
	if err := tx.Commit(r.Context()); err != nil {
		writeError(w, http.StatusInternalServerError, "commit review transaction: "+err.Error())
		return
	}

	h.publishWorkflowRunUpdated(r.Context(), run.WorkspaceID, updatedRun)
	writeJSON(w, http.StatusOK, reviewWorkflowRunStepResponse{
		RunID:      uuidToString(runID),
		StepID:     stepID,
		Decision:   decision,
		ReviewedBy: reviewedBy,
		ReviewedAt: reviewedAt,
	})
}

// GetWorkflowRunStepReview reports the current review state of a step so the
// sidecar (or any poller) can wait for a decision. It returns the node status
// and, once decided, the review decision recorded in output_snapshot.
func (h *Handler) GetWorkflowRunStepReview(w http.ResponseWriter, r *http.Request) {
	c, ok := h.workflowCaseFromRequest(w, r)
	if !ok {
		return
	}
	runID, ok := parseUUIDOrBadRequest(w, chi.URLParam(r, "runId"), "workflow run id")
	if !ok {
		return
	}
	stepID := strings.TrimSpace(chi.URLParam(r, "stepId"))
	if stepID == "" {
		writeError(w, http.StatusBadRequest, "step id is required")
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

	node, err := h.Queries.GetWorkflowRunNode(r.Context(), db.GetWorkflowRunNodeParams{
		RunID:  runID,
		NodeID: stepID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeError(w, http.StatusNotFound, "review step not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "load review step: "+err.Error())
		return
	}

	resp := map[string]any{
		"run_id":  uuidToString(runID),
		"step_id": stepID,
		"status":  node.Status,
	}
	if len(node.OutputSnapshot) > 0 && string(node.OutputSnapshot) != "null" {
		var output map[string]any
		if err := json.Unmarshal(node.OutputSnapshot, &output); err == nil {
			if decision, ok := output["review_decision"].(string); ok {
				resp["decision"] = decision
			}
			if comment, ok := output["review_comment"].(string); ok {
				resp["comment"] = comment
			}
		}
	}
	writeJSON(w, http.StatusOK, resp)
}

// applyReviewDecisionToNodesState marks the review node done and stores the
// decision output so the run detail view and the sidecar can read it.
func applyReviewDecisionToNodesState(raw []byte, nodeID string, output map[string]any) ([]byte, error) {
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
	node["status"] = "done"
	node["output"] = output
	nodesState[nodeID] = node
	return json.Marshal(nodesState)
}
