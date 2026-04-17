package handler

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

type SwimlaneResponse struct {
	ID          string `json:"id"`
	WorkspaceID string `json:"workspace_id"`
	Name        string `json:"name"`
	CreatedAt   string `json:"created_at"`
	UpdatedAt   string `json:"updated_at"`
}

func swimlaneToResponse(s db.Swimlane) SwimlaneResponse {
	return SwimlaneResponse{
		ID:          uuidToString(s.ID),
		WorkspaceID: uuidToString(s.WorkspaceID),
		Name:        s.Name,
		CreatedAt:   timestampToString(s.CreatedAt),
		UpdatedAt:   timestampToString(s.UpdatedAt),
	}
}

type CreateSwimlaneRequest struct {
	Name string `json:"name"`
}

type UpdateSwimlaneRequest struct {
	Name *string `json:"name"`
}

func (h *Handler) ListSwimlanes(w http.ResponseWriter, r *http.Request) {
	workspaceID := resolveWorkspaceID(r)
	swimlanes, err := h.Queries.ListSwimlanes(r.Context(), parseUUID(workspaceID))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list swimlanes")
		return
	}
	resp := make([]SwimlaneResponse, len(swimlanes))
	for i, s := range swimlanes {
		resp[i] = swimlaneToResponse(s)
	}
	writeJSON(w, http.StatusOK, map[string]any{"swimlanes": resp, "total": len(resp)})
}

func (h *Handler) GetSwimlane(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	workspaceID := resolveWorkspaceID(r)
	swimlane, err := h.Queries.GetSwimlaneInWorkspace(r.Context(), db.GetSwimlaneInWorkspaceParams{
		ID: parseUUID(id), WorkspaceID: parseUUID(workspaceID),
	})
	if err != nil {
		writeError(w, http.StatusNotFound, "swimlane not found")
		return
	}
	writeJSON(w, http.StatusOK, swimlaneToResponse(swimlane))
}

func (h *Handler) CreateSwimlane(w http.ResponseWriter, r *http.Request) {
	var req CreateSwimlaneRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" {
		writeError(w, http.StatusBadRequest, "name is required")
		return
	}
	if len(req.Name) > 255 {
		writeError(w, http.StatusBadRequest, "name must be 255 characters or fewer")
		return
	}
	workspaceID := resolveWorkspaceID(r)
	userID, ok := requireUserID(w, r)
	if !ok {
		return
	}
	swimlane, err := h.Queries.CreateSwimlane(r.Context(), db.CreateSwimlaneParams{
		WorkspaceID: parseUUID(workspaceID),
		Name:        req.Name,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create swimlane")
		return
	}
	resp := swimlaneToResponse(swimlane)
	h.publish(protocol.EventSwimlaneCreated, workspaceID, "member", userID, map[string]any{"swimlane": resp})
	writeJSON(w, http.StatusCreated, resp)
}

func (h *Handler) UpdateSwimlane(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	workspaceID := resolveWorkspaceID(r)
	if _, err := h.Queries.GetSwimlaneInWorkspace(r.Context(), db.GetSwimlaneInWorkspaceParams{
		ID: parseUUID(id), WorkspaceID: parseUUID(workspaceID),
	}); err != nil {
		writeError(w, http.StatusNotFound, "swimlane not found")
		return
	}
	userID, ok := requireUserID(w, r)
	if !ok {
		return
	}
	var req UpdateSwimlaneRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	params := db.UpdateSwimlaneParams{ID: parseUUID(id)}
	if req.Name != nil {
		name := strings.TrimSpace(*req.Name)
		if name == "" {
			writeError(w, http.StatusBadRequest, "name is required")
			return
		}
		if len(name) > 255 {
			writeError(w, http.StatusBadRequest, "name must be 255 characters or fewer")
			return
		}
		params.Name = pgtype.Text{String: name, Valid: true}
	}
	swimlane, err := h.Queries.UpdateSwimlane(r.Context(), params)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update swimlane")
		return
	}
	resp := swimlaneToResponse(swimlane)
	h.publish(protocol.EventSwimlaneUpdated, workspaceID, "member", userID, map[string]any{"swimlane": resp})
	writeJSON(w, http.StatusOK, resp)
}

func (h *Handler) DeleteSwimlane(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	workspaceID := resolveWorkspaceID(r)
	if _, err := h.Queries.GetSwimlaneInWorkspace(r.Context(), db.GetSwimlaneInWorkspaceParams{
		ID: parseUUID(id), WorkspaceID: parseUUID(workspaceID),
	}); err != nil {
		writeError(w, http.StatusNotFound, "swimlane not found")
		return
	}
	userID, ok := requireUserID(w, r)
	if !ok {
		return
	}
	if err := h.Queries.DeleteSwimlane(r.Context(), parseUUID(id)); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to delete swimlane")
		return
	}
	h.publish(protocol.EventSwimlaneDeleted, workspaceID, "member", userID, map[string]any{"swimlane_id": id})
	w.WriteHeader(http.StatusNoContent)
}
