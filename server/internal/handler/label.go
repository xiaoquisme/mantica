package handler

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"regexp"

	"github.com/go-chi/chi/v5"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

var hexColorRe = regexp.MustCompile(`^#[0-9a-fA-F]{6}$`)

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

type CreateLabelRequest struct {
	Name  string `json:"name"`
	Color string `json:"color"`
}

func (h *Handler) CreateLabel(w http.ResponseWriter, r *http.Request) {
	wsID := chi.URLParam(r, "id")

	var req CreateLabelRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Name == "" {
		writeError(w, http.StatusBadRequest, "name is required")
		return
	}
	if !hexColorRe.MatchString(req.Color) {
		writeError(w, http.StatusBadRequest, "color must be a hex color (e.g. #ff0000)")
		return
	}

	label, err := h.Queries.CreateLabel(r.Context(), db.CreateLabelParams{
		WorkspaceID: parseUUID(wsID),
		Name:        req.Name,
		Color:       req.Color,
	})
	if err != nil {
		slog.Error("create label failed", "workspace_id", wsID, "error", err)
		writeError(w, http.StatusInternalServerError, "failed to create label")
		return
	}

	resp := labelToResponse(label)
	userID := requestUserID(r)
	actorType, actorID := h.resolveActor(r, userID, wsID)
	h.publish(protocol.EventLabelCreated, wsID, actorType, actorID, map[string]any{
		"label": resp,
	})

	writeJSON(w, http.StatusCreated, resp)
}

func (h *Handler) DeleteLabel(w http.ResponseWriter, r *http.Request) {
	wsID := chi.URLParam(r, "id")
	labelID := chi.URLParam(r, "labelId")

	rows, err := h.Queries.DeleteLabel(r.Context(), db.DeleteLabelParams{
		ID:          parseUUID(labelID),
		WorkspaceID: parseUUID(wsID),
	})
	if err != nil {
		slog.Error("delete label failed", "label_id", labelID, "error", err)
		writeError(w, http.StatusInternalServerError, "failed to delete label")
		return
	}
	if rows == 0 {
		writeError(w, http.StatusNotFound, "label not found")
		return
	}

	userID := requestUserID(r)
	actorType, actorID := h.resolveActor(r, userID, wsID)
	h.publish(protocol.EventLabelDeleted, wsID, actorType, actorID, map[string]any{
		"label_id": labelID,
	})

	w.WriteHeader(http.StatusNoContent)
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

	// Validate that all submitted label IDs belong to this workspace.
	wsLabels, err := h.Queries.GetWorkspaceLabels(r.Context(), issue.WorkspaceID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to validate labels")
		return
	}
	validLabelIDs := make(map[string]bool, len(wsLabels))
	for _, l := range wsLabels {
		validLabelIDs[uuidToString(l.ID)] = true
	}
	for _, labelID := range req.LabelIDs {
		if !validLabelIDs[labelID] {
			writeError(w, http.StatusBadRequest, "label does not belong to this workspace")
			return
		}
	}

	// Delete all existing labels for the issue then re-insert atomically.
	if err := h.Queries.DeleteIssueLabels(r.Context(), issue.ID); err != nil {
		slog.Error("delete issue labels failed", "issue_id", id, "error", err)
		writeError(w, http.StatusInternalServerError, "failed to update labels")
		return
	}

	for _, labelID := range req.LabelIDs {
		if err := h.Queries.AddIssueLabel(r.Context(), db.AddIssueLabelParams{
			IssueID: issue.ID,
			LabelID: parseUUID(labelID),
		}); err != nil {
			slog.Error("add issue label failed", "issue_id", id, "label_id", labelID, "error", err)
			writeError(w, http.StatusInternalServerError, "failed to update labels")
			return
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
		"issue": map[string]any{
			"id":     id,
			"labels": resp,
		},
	})

	writeJSON(w, http.StatusOK, resp)
}
