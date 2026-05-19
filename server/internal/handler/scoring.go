package handler

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

// ── Agent Score Endpoints ──

type AgentScoreResponse struct {
	AgentID         string         `json:"agent_id"`
	AgentName       string         `json:"agent_name"`
	OverallScore    float64        `json:"overall_score"`
	TaskTypeScores  map[string]any `json:"task_type_scores"`
	TotalTasks      int32          `json:"total_tasks"`
	SuccessfulTasks int32          `json:"successful_tasks"`
	FailedTasks     int32          `json:"failed_tasks"`
	SuccessRate     float64        `json:"success_rate"`
	AvgToolCount    float64        `json:"avg_tool_count"`
	AvgErrorCount   float64        `json:"avg_error_count"`
	AvgErrorRate    float64        `json:"avg_error_rate"`
	ScoreTrend      string         `json:"score_trend"`
	UpdatedAt       string         `json:"updated_at"`
}

func scoreRowToResponse(id, agentID, workspaceID pgtype.UUID, score float64, typeScores []byte,
	total, successful, failed int32, successRate float64,
	avgTools, avgErrors, avgErrRate float64, trend string,
	updatedAt pgtype.Timestamptz, agentName string) AgentScoreResponse {

	var ts map[string]any
	if typeScores != nil {
		json.Unmarshal(typeScores, &ts)
	}
	if ts == nil {
		ts = map[string]any{}
	}
	return AgentScoreResponse{
		AgentID:         uuidToString(agentID),
		AgentName:       agentName,
		OverallScore:    score,
		TaskTypeScores:  ts,
		TotalTasks:      total,
		SuccessfulTasks: successful,
		FailedTasks:     failed,
		SuccessRate:     successRate,
		AvgToolCount:    avgTools,
		AvgErrorCount:   avgErrors,
		AvgErrorRate:    avgErrRate,
		ScoreTrend:      trend,
		UpdatedAt:       formatTS(updatedAt),
	}
}

// GetAgentScore returns the score for a single agent.
func (h *Handler) GetAgentScore(w http.ResponseWriter, r *http.Request) {
	agentID := chi.URLParam(r, "id")
	workspaceID := r.Header.Get("X-Workspace-ID")

	score, err := h.Queries.GetAgentScore(r.Context(), db.GetAgentScoreParams{
		AgentID:     parseUUID(agentID),
		WorkspaceID: parseUUID(workspaceID),
	})
	if err != nil {
		writeError(w, http.StatusNotFound, "score not found")
		return
	}

	agent, _ := h.Queries.GetAgent(r.Context(), parseUUID(agentID))
	writeJSON(w, http.StatusOK, scoreRowToResponse(
		score.ID, score.AgentID, score.WorkspaceID, score.OverallScore,
		score.TaskTypeScores, score.TotalTasks, score.SuccessfulTasks,
		score.FailedTasks, score.SuccessRate, score.AvgToolCount,
		score.AvgErrorCount, score.AvgErrorRate, score.ScoreTrend,
		score.UpdatedAt, agent.Name,
	))
}

// ListAgentScores returns scores for all agents in a workspace.
func (h *Handler) ListAgentScores(w http.ResponseWriter, r *http.Request) {
	workspaceID := r.Header.Get("X-Workspace-ID")

	scores, err := h.Queries.ListAgentScoresByWorkspace(r.Context(), parseUUID(workspaceID))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list scores")
		return
	}

	var resp []AgentScoreResponse
	for _, s := range scores {
		resp = append(resp, scoreRowToResponse(
			s.ID, s.AgentID, s.WorkspaceID, s.OverallScore,
			s.TaskTypeScores, s.TotalTasks, s.SuccessfulTasks,
			s.FailedTasks, s.SuccessRate, s.AvgToolCount,
			s.AvgErrorCount, s.AvgErrorRate, s.ScoreTrend,
			s.UpdatedAt, s.AgentName,
		))
	}
	if resp == nil {
		resp = []AgentScoreResponse{}
	}
	writeJSON(w, http.StatusOK, resp)
}

// GetAgentScoreHistory returns recent score changes for an agent.
func (h *Handler) GetAgentScoreHistory(w http.ResponseWriter, r *http.Request) {
	agentID := chi.URLParam(r, "id")
	limit := 20
	if l := r.URL.Query().Get("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 && parsed <= 100 {
			limit = parsed
		}
	}

	history, err := h.Queries.ListScoreHistory(r.Context(), db.ListScoreHistoryParams{
		AgentID: parseUUID(agentID),
		Limit:   int32(limit),
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list history")
		return
	}

	type Entry struct {
		TaskID      string  `json:"task_id"`
		TaskType    string  `json:"task_type"`
		ScoreBefore float64 `json:"score_before"`
		ScoreAfter  float64 `json:"score_after"`
		ScoreDelta  float64 `json:"score_delta"`
		Success     bool    `json:"success"`
		ToolCount   int32   `json:"tool_count"`
		ErrorCount  int32   `json:"error_count"`
		CreatedAt   string  `json:"created_at"`
	}

	var resp []Entry
	for _, h := range history {
		resp = append(resp, Entry{
			TaskID:      uuidToString(h.TaskID),
			TaskType:    h.TaskType.String,
			ScoreBefore: h.ScoreBefore,
			ScoreAfter:  h.ScoreAfter,
			ScoreDelta:  h.ScoreDelta,
			Success:     h.Success,
			ToolCount:   h.ToolCount,
			ErrorCount:  h.ErrorCount,
			CreatedAt:   formatTS(h.CreatedAt),
		})
	}
	if resp == nil {
		resp = []Entry{}
	}
	writeJSON(w, http.StatusOK, resp)
}

// ── Task Analysis Endpoints ──

type TaskAnalysisResponse struct {
	TaskID           string         `json:"task_id"`
	ToolCount        int32          `json:"tool_count"`
	ErrorCount       int32          `json:"error_count"`
	UniqueTools      int32          `json:"unique_tools"`
	TotalDurationMs  int64          `json:"total_duration_ms"`
	MessageCount     int32          `json:"message_count"`
	FailureClass     string         `json:"failure_class"`
	FailureDetail    string         `json:"failure_detail"`
	ToolUsage        map[string]any `json:"tool_usage"`
	HasRetryPattern  bool           `json:"has_retry_pattern"`
	HasErrorRecovery bool           `json:"has_error_recovery"`
	Summary          string         `json:"summary"`
	ImprovementHint  string         `json:"improvement_hint"`
	CreatedAt        string         `json:"created_at"`
}

func analysisToResponse(a db.TaskAnalysis) TaskAnalysisResponse {
	var tu map[string]any
	if a.ToolUsage != nil {
		json.Unmarshal(a.ToolUsage, &tu)
	}
	return TaskAnalysisResponse{
		TaskID:           uuidToString(a.TaskID),
		ToolCount:        a.ToolCount,
		ErrorCount:       a.ErrorCount,
		UniqueTools:      a.UniqueTools,
		TotalDurationMs:  a.TotalDurationMs,
		MessageCount:     a.MessageCount,
		FailureClass:     a.FailureClass.String,
		FailureDetail:    a.FailureDetail.String,
		ToolUsage:        tu,
		HasRetryPattern:  a.HasRetryPattern.Bool,
		HasErrorRecovery: a.HasErrorRecovery.Bool,
		Summary:          a.Summary.String,
		ImprovementHint:  a.ImprovementHint.String,
		CreatedAt:        formatTS(a.CreatedAt),
	}
}

// GetTaskAnalysis returns the analysis for a single task.
func (h *Handler) GetTaskAnalysis(w http.ResponseWriter, r *http.Request) {
	taskID := chi.URLParam(r, "id")
	analysis, err := h.Queries.GetTaskAnalysis(r.Context(), parseUUID(taskID))
	if err != nil {
		writeError(w, http.StatusNotFound, "analysis not found")
		return
	}
	writeJSON(w, http.StatusOK, analysisToResponse(analysis))
}

// ListFailedAnalyses returns recent failed task analyses.
func (h *Handler) ListFailedAnalyses(w http.ResponseWriter, r *http.Request) {
	limit := 20
	if l := r.URL.Query().Get("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 && parsed <= 100 {
			limit = parsed
		}
	}

	analyses, err := h.Queries.ListFailedAnalyses(r.Context(), int32(limit))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list analyses")
		return
	}

	type Row struct {
		TaskAnalysisResponse
		AgentID string `json:"agent_id"`
	}
	var resp []Row
	for _, a := range analyses {
		var tu map[string]any
		if a.ToolUsage != nil {
			json.Unmarshal(a.ToolUsage, &tu)
		}
		resp = append(resp, Row{
			TaskAnalysisResponse: TaskAnalysisResponse{
				TaskID:           uuidToString(a.TaskID),
				ToolCount:        a.ToolCount,
				ErrorCount:       a.ErrorCount,
				UniqueTools:      a.UniqueTools,
				TotalDurationMs:  a.TotalDurationMs,
				MessageCount:     a.MessageCount,
				FailureClass:     a.FailureClass.String,
				FailureDetail:    a.FailureDetail.String,
				ToolUsage:        tu,
				HasRetryPattern:  a.HasRetryPattern.Bool,
				HasErrorRecovery: a.HasErrorRecovery.Bool,
				Summary:          a.Summary.String,
				ImprovementHint:  a.ImprovementHint.String,
				CreatedAt:        formatTS(a.CreatedAt),
			},
			AgentID: uuidToString(a.AgentID),
		})
	}
	if resp == nil {
		resp = []Row{}
	}
	writeJSON(w, http.StatusOK, resp)
}

// formatTS converts pgtype.Timestamptz to RFC3339 string.
func formatTS(ts pgtype.Timestamptz) string {
	if !ts.Valid {
		return ""
	}
	return ts.Time.Format(time.RFC3339)
}

// ── Improvement Hints Endpoint ──

type ImprovementHint struct {
	FailureClass    string `json:"failure_class"`
	ImprovementHint string `json:"improvement_hint"`
	Summary         string `json:"summary"`
	OccurrenceCount int64  `json:"occurrence_count"`
	LastSeen        string `json:"last_seen"`
}

type AgentHintsResponse struct {
	AgentID string             `json:"agent_id"`
	Hints   []ImprovementHint  `json:"hints"`
}

// GetAgentHints returns improvement hints for an agent based on recent failures.
func (h *Handler) GetAgentHints(w http.ResponseWriter, r *http.Request) {
	agentID := chi.URLParam(r, "id")
	days := 7
	if d := r.URL.Query().Get("days"); d != "" {
		if parsed, err := strconv.Atoi(d); err == nil && parsed > 0 && parsed <= 30 {
			days = parsed
		}
	}

	since := time.Now().AddDate(0, 0, -days)

	hints, err := h.Queries.GetAgentImprovementHints(r.Context(), db.GetAgentImprovementHintsParams{
		AgentID: parseUUID(agentID),
		CreatedAt: pgtype.Timestamptz{Time: since, Valid: true},
		Limit:     10,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get hints")
		return
	}

	var resp []ImprovementHint
	for _, h := range hints {
		lastSeen := ""
		if t, ok := h.LastSeen.(time.Time); ok {
			lastSeen = t.Format(time.RFC3339)
		}
		resp = append(resp, ImprovementHint{
			FailureClass:    h.FailureClass.String,
			ImprovementHint: h.ImprovementHint.String,
			Summary:         h.Summary.String,
			OccurrenceCount: h.OccurrenceCount,
			LastSeen:        lastSeen,
		})
	}
	if resp == nil {
		resp = []ImprovementHint{}
	}

	writeJSON(w, http.StatusOK, AgentHintsResponse{
		AgentID: agentID,
		Hints:   resp,
	})
}
