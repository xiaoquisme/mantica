package handler

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/robfig/cron/v3"

	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

// --- Request / Response types ---

type ScheduledTaskResponse struct {
	ID          string  `json:"id"`
	WorkspaceID string  `json:"workspace_id"`
	Name        string  `json:"name"`
	AgentID     string  `json:"agent_id"`
	Schedule    string  `json:"schedule"`
	Prompt      string  `json:"prompt"`
	Enabled     bool    `json:"enabled"`
	LastRunAt   *string `json:"last_run_at"`
	NextRunAt   *string `json:"next_run_at"`
	CreatedBy   string  `json:"created_by"`
	CreatedAt   string  `json:"created_at"`
	UpdatedAt   string  `json:"updated_at"`
}

func scheduledTaskToResponse(st db.ScheduledTask) ScheduledTaskResponse {
	return ScheduledTaskResponse{
		ID:          uuidToString(st.ID),
		WorkspaceID: uuidToString(st.WorkspaceID),
		Name:        st.Name,
		AgentID:     uuidToString(st.AgentID),
		Schedule:    st.Schedule,
		Prompt:      st.Prompt,
		Enabled:     st.Enabled,
		LastRunAt:   timestampToPtr(st.LastRunAt),
		NextRunAt:   timestampToPtr(st.NextRunAt),
		CreatedBy:   uuidToString(st.CreatedBy),
		CreatedAt:   timestampToString(st.CreatedAt),
		UpdatedAt:   timestampToString(st.UpdatedAt),
	}
}

// --- Handlers ---

// ListScheduledTasks handles GET /api/scheduled-tasks.
func (h *Handler) ListScheduledTasks(w http.ResponseWriter, r *http.Request) {
	workspaceID := resolveWorkspaceID(r)
	tasks, err := h.Queries.ListScheduledTasks(r.Context(), parseUUID(workspaceID))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list scheduled tasks")
		return
	}
	resp := make([]ScheduledTaskResponse, len(tasks))
	for i, t := range tasks {
		resp[i] = scheduledTaskToResponse(t)
	}
	writeJSON(w, http.StatusOK, resp)
}

// CreateScheduledTask handles POST /api/scheduled-tasks.
func (h *Handler) CreateScheduledTask(w http.ResponseWriter, r *http.Request) {
	workspaceID := resolveWorkspaceID(r)
	userID, ok := requireUserID(w, r)
	if !ok {
		return
	}

	var req struct {
		Name     string `json:"name"`
		AgentID  string `json:"agent_id"`
		Schedule string `json:"schedule"`
		Prompt   string `json:"prompt"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Name == "" || req.AgentID == "" || req.Schedule == "" || req.Prompt == "" {
		writeError(w, http.StatusBadRequest, "name, agent_id, schedule, and prompt are required")
		return
	}

	// Validate cron expression.
	nextRun, err := computeNextRun(req.Schedule)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid cron expression: "+err.Error())
		return
	}

	st, err := h.Queries.CreateScheduledTask(r.Context(), db.CreateScheduledTaskParams{
		WorkspaceID: parseUUID(workspaceID),
		Name:        req.Name,
		AgentID:     parseUUID(req.AgentID),
		Schedule:    req.Schedule,
		Prompt:      req.Prompt,
		Enabled:     true,
		NextRunAt:   pgtype.Timestamptz{Time: nextRun, Valid: true},
		CreatedBy:   parseUUID(userID),
	})
	if err != nil {
		if isUniqueViolation(err) {
			writeError(w, http.StatusConflict, "scheduled task with this name already exists")
			return
		}
		slog.Error("create scheduled task failed", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to create scheduled task")
		return
	}

	writeJSON(w, http.StatusCreated, scheduledTaskToResponse(st))
}

// UpdateScheduledTask handles PUT /api/scheduled-tasks/{id}.
func (h *Handler) UpdateScheduledTask(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	var req struct {
		Name     *string `json:"name"`
		AgentID  *string `json:"agent_id"`
		Schedule *string `json:"schedule"`
		Prompt   *string `json:"prompt"`
		Enabled  *bool   `json:"enabled"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	// Build update params.
	params := db.UpdateScheduledTaskParams{
		ID: parseUUID(id),
	}
	if req.Name != nil {
		params.Name = pgtype.Text{String: *req.Name, Valid: true}
	}
	if req.AgentID != nil {
		params.AgentID = parseUUID(*req.AgentID)
	}
	if req.Prompt != nil {
		params.Prompt = pgtype.Text{String: *req.Prompt, Valid: true}
	}
	if req.Enabled != nil {
		params.Enabled = pgtype.Bool{Bool: *req.Enabled, Valid: true}
	}

	// If schedule changed, recompute next_run_at.
	if req.Schedule != nil {
		nextRun, err := computeNextRun(*req.Schedule)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid cron expression: "+err.Error())
			return
		}
		params.Schedule = pgtype.Text{String: *req.Schedule, Valid: true}
		params.NextRunAt = pgtype.Timestamptz{Time: nextRun, Valid: true}
	} else {
		// Preserve existing next_run_at: load current value.
		existing, err := h.Queries.GetScheduledTask(r.Context(), parseUUID(id))
		if err != nil {
			if isNotFound(err) {
				writeError(w, http.StatusNotFound, "scheduled task not found")
				return
			}
			writeError(w, http.StatusInternalServerError, "failed to load scheduled task")
			return
		}
		params.NextRunAt = existing.NextRunAt
	}

	st, err := h.Queries.UpdateScheduledTask(r.Context(), params)
	if err != nil {
		if isNotFound(err) {
			writeError(w, http.StatusNotFound, "scheduled task not found")
			return
		}
		if isUniqueViolation(err) {
			writeError(w, http.StatusConflict, "scheduled task with this name already exists")
			return
		}
		slog.Error("update scheduled task failed", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to update scheduled task")
		return
	}

	writeJSON(w, http.StatusOK, scheduledTaskToResponse(st))
}

// DeleteScheduledTask handles DELETE /api/scheduled-tasks/{id}.
func (h *Handler) DeleteScheduledTask(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := h.Queries.DeleteScheduledTask(r.Context(), parseUUID(id)); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to delete scheduled task")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// RunScheduledTaskNow handles POST /api/scheduled-tasks/{id}/run.
// It immediately enqueues a task for the scheduled task's agent.
func (h *Handler) RunScheduledTaskNow(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	st, err := h.Queries.GetScheduledTask(r.Context(), parseUUID(id))
	if err != nil {
		if isNotFound(err) {
			writeError(w, http.StatusNotFound, "scheduled task not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to load scheduled task")
		return
	}

	task, err := h.TaskService.EnqueueScheduledTask(r.Context(), st)
	if err != nil {
		slog.Error("run scheduled task now failed", "error", err, "scheduled_task_id", uuidToString(st.ID))
		writeError(w, http.StatusInternalServerError, "failed to enqueue task: "+err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{
		"task_id":           uuidToString(task.ID),
		"scheduled_task_id": uuidToString(st.ID),
	})
}

// --- Helpers ---

// computeNextRun parses a cron expression and returns the next run time from now.
func computeNextRun(expr string) (time.Time, error) {
	parser := cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)
	sched, err := parser.Parse(expr)
	if err != nil {
		return time.Time{}, err
	}
	return sched.Next(time.Now()), nil
}

// ComputeNextRunFrom parses a cron expression and returns the next run time after the given time.
// Exported for use by the scheduler.
func ComputeNextRunFrom(expr string, from time.Time) (time.Time, error) {
	parser := cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)
	sched, err := parser.Parse(expr)
	if err != nil {
		return time.Time{}, err
	}
	return sched.Next(from), nil
}
