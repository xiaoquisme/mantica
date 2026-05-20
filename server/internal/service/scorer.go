package service

import (
	"context"
	"encoding/json"
	"log/slog"
	"math"

	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/xiaoquisme/mantica/server/pkg/db/generated"
)

// Scorer manages agent scoring — ELO-like rating system for agents.
type Scorer struct {
	Queries *db.Queries
	Logger  *slog.Logger
}

func NewScorer(q *db.Queries, logger *slog.Logger) *Scorer {
	return &Scorer{Queries: q, Logger: logger}
}

const (
	defaultScore   = 1000.0
	kFactorBase    = 32.0
	kFactorMax     = 64.0
	toolCountDiv   = 10.0
	trendWindow    = 10
	trendThreshold = 20.0
)

// ScoreTask updates the agent's score after a task completes or fails.
func (s *Scorer) ScoreTask(ctx context.Context, taskID, agentID pgtype.UUID, workspaceID pgtype.UUID) error {
	// Get task info
	task, err := s.Queries.GetAgentTask(ctx, taskID)
	if err != nil {
		return err
	}

	// Get analysis if available (may not exist for old tasks)
	analysis, analysisErr := s.Queries.GetTaskAnalysis(ctx, taskID)

	// Get or create agent score
	score, scoreErr := s.Queries.GetAgentScore(ctx, db.GetAgentScoreParams{
		AgentID:     agentID,
		WorkspaceID: workspaceID,
	})
	if scoreErr != nil {
		score = db.AgentScore{
			OverallScore:   defaultScore,
			TaskTypeScores: []byte("{}"),
		}
	}

	// Determine task outcome
	success := task.Status == "completed"
	toolCount := int32(0)
	errorCount := int32(0)
	errorRate := 0.0
	if analysisErr == nil {
		toolCount = analysis.ToolCount
		errorCount = analysis.ErrorCount
		if toolCount > 0 {
			errorRate = float64(errorCount) / float64(toolCount)
		}
	}

	// Calculate score delta
	delta := calculateDelta(success, toolCount, errorRate)

	// Update overall score
	oldScore := score.OverallScore
	newScore := oldScore + delta

	// Update task type scores
	typeScores := make(map[string]float64)
	if score.TaskTypeScores != nil {
		json.Unmarshal(score.TaskTypeScores, &typeScores)
	}
	typeScores["general"] += delta
	typeScoresJSON, _ := json.Marshal(typeScores)

	// Update aggregate stats
	totalTasks := score.TotalTasks + 1
	successfulTasks := score.SuccessfulTasks
	failedTasks := score.FailedTasks
	if success {
		successfulTasks++
	} else {
		failedTasks++
	}
	successRate := float64(successfulTasks) / float64(totalTasks)

	// Update rolling averages
	avgTools := rollingAvg(score.AvgToolCount, float64(toolCount), totalTasks)
	avgErrors := rollingAvg(score.AvgErrorCount, float64(errorCount), totalTasks)
	avgErrRate := rollingAvg(score.AvgErrorRate, errorRate, totalTasks)

	// Determine trend
	trend := "stable"
	history, histErr := s.Queries.ListScoreHistory(ctx, db.ListScoreHistoryParams{
		AgentID: agentID,
		Limit:   trendWindow,
	})
	if histErr == nil && len(history) >= 3 {
		totalDelta := 0.0
		for _, h := range history {
			totalDelta += h.ScoreDelta
		}
		avgDelta := totalDelta / float64(len(history))
		if avgDelta > trendThreshold/float64(trendWindow) {
			trend = "improving"
		} else if avgDelta < -trendThreshold/float64(trendWindow) {
			trend = "declining"
		}
	}

	// Save score
	_, err = s.Queries.UpsertAgentScore(ctx, db.UpsertAgentScoreParams{
		AgentID:         agentID,
		WorkspaceID:     workspaceID,
		OverallScore:    newScore,
		TaskTypeScores:  typeScoresJSON,
		TotalTasks:      totalTasks,
		SuccessfulTasks: successfulTasks,
		FailedTasks:     failedTasks,
		SuccessRate:     successRate,
		AvgToolCount:    avgTools,
		AvgErrorCount:   avgErrors,
		AvgErrorRate:    avgErrRate,
		ScoreTrend:      trend,
		TrendSamples:    score.TrendSamples + 1,
	})
	if err != nil {
		return err
	}

	// Save history
	_, err = s.Queries.CreateScoreHistory(ctx, db.CreateScoreHistoryParams{
		AgentID:     agentID,
		WorkspaceID: workspaceID,
		TaskID:      taskID,
		TaskType:    pgtype.Text{String: "general", Valid: true},
		ScoreBefore: oldScore,
		ScoreAfter:  newScore,
		ScoreDelta:  delta,
		Success:     success,
		ToolCount:   toolCount,
		ErrorCount:  errorCount,
	})
	if err != nil {
		return err
	}

	s.Logger.Info("agent score updated",
		"agent_id", uuidToStr(agentID),
		"success", success,
		"delta", delta,
		"old_score", oldScore,
		"new_score", newScore,
		"trend", trend,
	)

	return nil
}

func calculateDelta(success bool, toolCount int32, errorRate float64) float64 {
	k := kFactorBase
	if toolCount > 10 {
		k = math.Min(kFactorMax, kFactorBase+float64(toolCount)/toolCountDiv)
	}

	mult := 1.0
	if success {
		if errorRate < 0.1 {
			mult = 1.2
		} else if errorRate > 0.5 {
			mult = 0.5
		}
	} else {
		mult = -1.0
		if errorRate > 0.8 {
			mult = -1.3
		}
	}

	return k * mult * 0.1
}

func rollingAvg(current, newValue float64, count int32) float64 {
	if count <= 1 {
		return newValue
	}
	return (current*float64(count-1) + newValue) / float64(count)
}
