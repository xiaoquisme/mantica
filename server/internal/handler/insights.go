package handler

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	db "github.com/xiaoquisme/mantica/server/pkg/db/generated"
)

// ── Smart Summary Endpoint ──

type AgentInsight struct {
	Type      string `json:"type"`
	AgentID   string `json:"agent_id"`
	AgentName string `json:"agent_name"`
	Title     string `json:"title"`
	Detail    string `json:"detail"`
	Action    string `json:"action,omitempty"`
	ActionID  string `json:"action_id,omitempty"`
}

type SmartSummaryResponse struct {
	GeneratedAt string         `json:"generated_at"`
	Insights    []AgentInsight `json:"insights"`
	Summary     string         `json:"summary"`
}

func (h *Handler) GetSmartSummary(w http.ResponseWriter, r *http.Request) {
	workspaceID := r.Header.Get("X-Workspace-ID")
	wsUUID := parseUUID(workspaceID)
	now := time.Now()

	var insights []AgentInsight

	scores, err := h.Queries.ListAgentScoresByWorkspace(r.Context(), wsUUID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get scores")
		return
	}

	for _, s := range scores {
		agentID := uuidToString(s.AgentID)
		name := s.AgentName

		if s.ScoreTrend == "declining" {
			insights = append(insights, AgentInsight{
				Type: "warning", AgentID: agentID, AgentName: name,
				Title:   name + " is declining",
				Detail:  "Score trend declining. Recent tasks underperforming.",
				Action:  "View details",
				ActionID: agentID,
			})
		}

		if s.AvgErrorRate > 0.5 && s.TotalTasks > 5 {
			insights = append(insights, AgentInsight{
				Type: "warning", AgentID: agentID, AgentName: name,
				Title:   name + " high error rate",
				Detail:  fmt.Sprintf("%.0f%% error rate across %d tasks.", s.AvgErrorRate*100, s.TotalTasks),
				Action:  "View analysis",
				ActionID: agentID,
			})
		}

		if s.SuccessRate < 0.8 && s.TotalTasks > 10 {
			insights = append(insights, AgentInsight{
				Type: "warning", AgentID: agentID, AgentName: name,
				Title:   name + " low win rate",
				Detail:  fmt.Sprintf("%.0f%% success rate. Consider reviewing instructions.", s.SuccessRate*100),
				Action:  "Review instructions",
				ActionID: agentID,
			})
		}

		if s.ScoreTrend == "improving" && s.TotalTasks > 5 {
			insights = append(insights, AgentInsight{
				Type: "success", AgentID: agentID, AgentName: name,
				Title:   name + " is improving",
				Detail:  fmt.Sprintf("Trending up with %.0f%% win rate.", s.SuccessRate*100),
			})
		}
	}

	// Check recent failure patterns
	failedTasks, _ := h.Queries.ListFailedAnalyses(r.Context(), 20)
	failureCounts := map[string]int{}
	for _, ft := range failedTasks {
		if ft.FailureClass.Valid {
			failureCounts[ft.FailureClass.String]++
		}
	}
	for class, count := range failureCounts {
		if count >= 3 {
			insights = append(insights, AgentInsight{
				Type: "warning",
				Title:  fmt.Sprintf("Repeated %s failures", class),
				Detail: fmt.Sprintf("%d recent tasks failed with '%s'.", count, class),
			})
		}
	}

	summary := genSummary(scores, insights)

	if insights == nil {
		insights = []AgentInsight{}
	}

	writeJSON(w, http.StatusOK, SmartSummaryResponse{
		GeneratedAt: now.Format(time.RFC3339),
		Insights:    insights,
		Summary:     summary,
	})
}

func genSummary(scores []db.ListAgentScoresByWorkspaceRow, insights []AgentInsight) string {
	if len(scores) == 0 {
		return "No agent data yet."
	}
	var totalTasks, totalWins int64
	for _, s := range scores {
		totalTasks += int64(s.TotalTasks)
		totalWins += int64(s.SuccessfulTasks)
	}
	warnings := 0
	for _, i := range insights {
		if i.Type == "warning" {
			warnings++
		}
	}
	if warnings > 0 {
		return fmt.Sprintf("%d tasks completed. %d issues need attention.", totalTasks, warnings)
	}
	return fmt.Sprintf("%d tasks, %d wins. All agents healthy.", totalTasks, totalWins)
}

// formatFloat and formatInt helpers
func fmtFloat(f float64) string {
	return strings.TrimRight(strings.TrimRight(fmt.Sprintf("%.1f", f), "0"), ".")
}
