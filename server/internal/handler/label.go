package handler

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

type LabelResponse struct {
	ID          string `json:"id"`
	WorkspaceID string `json:"workspace_id"`
	Name        string `json:"name"`
	Color       string `json:"color"`
}

func labelToResponse(l db.IssueLabel) LabelResponse {
	return LabelResponse{
		ID:          uuidToString(l.ID),
		WorkspaceID: uuidToString(l.WorkspaceID),
		Name:        l.Name,
		Color:       l.Color,
	}
}

func (h *Handler) ListWorkspaceLabels(w http.ResponseWriter, r *http.Request) {
	wsID := chi.URLParam(r, "id")
	if wsID == "" {
		wsID = resolveWorkspaceID(r)
	}
	labels, err := h.Queries.GetWorkspaceLabels(r.Context(), parseUUID(wsID))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list labels")
		return
	}
	resp := make([]LabelResponse, len(labels))
	for i, l := range labels {
		resp[i] = labelToResponse(l)
	}
	writeJSON(w, http.StatusOK, resp)
}

type UpdateIssueLabelsRequest struct {
	LabelIDs []string `json:"label_ids"`
}

func (h *Handler) UpdateIssueLabels(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	issue, ok := h.loadIssueForUser(w, r, id)
	if !ok {
		return
	}

	var req UpdateIssueLabelsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	// Delete all existing labels for the issue then re-insert.
	if err := h.Queries.DeleteIssueLabels(r.Context(), issue.ID); err != nil {
		slog.Warn("delete issue labels failed", "issue_id", id, "error", err)
		writeError(w, http.StatusInternalServerError, "failed to update labels")
		return
	}

	for _, labelID := range req.LabelIDs {
		if err := h.Queries.AddIssueLabel(r.Context(), db.AddIssueLabelParams{
			IssueID: issue.ID,
			LabelID: parseUUID(labelID),
		}); err != nil {
			slog.Warn("add issue label failed", "issue_id", id, "label_id", labelID, "error", err)
		}
	}

	labels, err := h.Queries.GetIssueLabels(r.Context(), issue.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to fetch updated labels")
		return
	}
	resp := make([]LabelResponse, len(labels))
	for i, l := range labels {
		resp[i] = labelToResponse(l)
	}

	workspaceID := uuidToString(issue.WorkspaceID)
	userID := requestUserID(r)
	actorType, actorID := h.resolveActor(r, userID, workspaceID)
	h.publish(protocol.EventIssueUpdated, workspaceID, actorType, actorID, map[string]any{
		"issue_id":       id,
		"labels_changed": true,
		"labels":         resp,
	})

	writeJSON(w, http.StatusOK, resp)
}
