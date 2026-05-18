package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

// Analyzer performs post-task analysis on completed/failed tasks.
// It reads task_message records and produces structured insights in task_analysis.
type Analyzer struct {
	Queries *db.Queries
	Logger  *slog.Logger
}

func NewAnalyzer(q *db.Queries, logger *slog.Logger) *Analyzer {
	return &Analyzer{Queries: q, Logger: logger}
}

// AnalysisResult holds the computed metrics for a task.
type AnalysisResult struct {
	ToolCount        int32
	ErrorCount       int32
	UniqueTools      int32
	TotalDurationMs  int64
	MessageCount     int32
	FailureClass     string
	FailureDetail    string
	ToolUsage        map[string]ToolStats
	HasRetryPattern  bool
	HasErrorRecovery bool
	LongestToolMs    int64
}

// ToolStats tracks per-tool metrics.
type ToolStats struct {
	Count  int `json:"count"`
	Errors int `json:"errors"`
}

// AnalyzeTask examines a completed task's messages and produces an AnalysisResult.
func (a *Analyzer) AnalyzeTask(ctx context.Context, taskID pgtype.UUID) (*AnalysisResult, error) {
	// Fetch task to get status and error
	task, err := a.Queries.GetAgentTask(ctx, taskID)
	if err != nil {
		return nil, err
	}

	// Fetch all task messages
	messages, err := a.Queries.ListTaskMessages(ctx, taskID)
	if err != nil {
		return nil, err
	}

	result := &AnalysisResult{
		MessageCount: int32(len(messages)),
		ToolUsage:    make(map[string]ToolStats),
	}

	// Track tool calls for retry detection
	type toolCall struct {
		tool string
		ts   time.Time
	}
	var recentCalls []toolCall
	var hadError bool
	var lastSuccessAfterError bool

	for _, msg := range messages {
		switch msg.Type {
		case "tool_use":
			result.ToolCount++
			toolName := msg.Tool.String
			if toolName == "" {
				toolName = "unknown"
			}
			stats := result.ToolUsage[toolName]
			stats.Count++
			result.ToolUsage[toolName] = stats

			// Track for retry pattern
			recentCalls = append(recentCalls, toolCall{tool: toolName})

		case "tool_result":
			output := msg.Output.String
			isError := isErrorMessage(output)
			toolName := msg.Tool.String

			if isError {
				result.ErrorCount++
				stats := result.ToolUsage[toolName]
				stats.Errors++
				result.ToolUsage[toolName] = stats
				hadError = true
			} else if hadError {
				lastSuccessAfterError = true
			}
		}
	}

	// Count unique tools
	result.UniqueTools = int32(len(result.ToolUsage))

	// Detect retry pattern: same tool called 3+ times consecutively
	if len(recentCalls) >= 3 {
		for i := 2; i < len(recentCalls); i++ {
			if recentCalls[i].tool == recentCalls[i-1].tool &&
				recentCalls[i-1].tool == recentCalls[i-2].tool {
				result.HasRetryPattern = true
				break
			}
		}
	}

	result.HasErrorRecovery = hadError && lastSuccessAfterError

	// Determine failure class for failed tasks
	if task.Status == "failed" {
		errMsg := task.Error.String
		result.FailureClass = classifyFailure(errMsg, result)
		result.FailureDetail = errMsg
	}

	return result, nil
}

// SaveAnalysis persists the analysis result to the task_analysis table.
func (a *Analyzer) SaveAnalysis(ctx context.Context, taskID pgtype.UUID, r *AnalysisResult) error {
	toolUsageJSON, _ := json.Marshal(r.ToolUsage)

	_, err := a.Queries.CreateTaskAnalysis(ctx, db.CreateTaskAnalysisParams{
		TaskID:           taskID,
		ToolCount:        r.ToolCount,
		ErrorCount:       r.ErrorCount,
		UniqueTools:      r.UniqueTools,
		TotalDurationMs:  r.TotalDurationMs,
		MessageCount:     r.MessageCount,
		FailureClass:     pgtype.Text{String: r.FailureClass, Valid: r.FailureClass != ""},
		FailureDetail:    pgtype.Text{String: r.FailureDetail, Valid: r.FailureDetail != ""},
		ToolUsage:        toolUsageJSON,
		HasRetryPattern:  pgtype.Bool{Bool: r.HasRetryPattern, Valid: true},
		HasErrorRecovery: pgtype.Bool{Bool: r.HasErrorRecovery, Valid: true},
		LongestToolMs:    pgtype.Int8{Int64: r.LongestToolMs, Valid: r.LongestToolMs > 0},
		Summary:          pgtype.Text{},
		ImprovementHint:  pgtype.Text{},
	})
	return err
}

// AnalyzeAndSave is a convenience method that analyzes and saves in one call.
func (a *Analyzer) AnalyzeAndSave(ctx context.Context, taskID pgtype.UUID) (*AnalysisResult, error) {
	result, err := a.AnalyzeTask(ctx, taskID)
	if err != nil {
		return nil, err
	}

	if err := a.SaveAnalysis(ctx, taskID, result); err != nil {
		return nil, err
	}

	a.Logger.Info("task analysis saved",
		"task_id", uuidToStr(taskID),
		"tools", result.ToolCount,
		"errors", result.ErrorCount,
		"failure_class", result.FailureClass,
		"retry_pattern", result.HasRetryPattern,
	)

	return result, nil
}

// classifyFailure categorizes a failure based on the error message and metrics.
func classifyFailure(errMsg string, r *AnalysisResult) string {
	err := strings.ToLower(errMsg)

	switch {
	case strings.Contains(err, "timeout") || strings.Contains(err, "timed out"):
		return "timeout"
	case strings.Contains(err, "empty output"):
		return "empty_output"
	case strings.Contains(err, "exit status 1") || strings.Contains(err, "exited with error"):
		return "runtime_error"
	case strings.Contains(err, "build") || strings.Contains(err, "compile"):
		return "build_error"
	case strings.Contains(err, "test") && strings.Contains(err, "fail"):
		return "test_fail"
	case r.ErrorCount > 3:
		return "cascading_errors"
	default:
		return "unknown"
	}
}

// isErrorMessage checks if a tool output looks like an error.
func isErrorMessage(output string) bool {
	lower := strings.ToLower(output)
	indicators := []string{
		"error:", "traceback", "exception:", "failed:",
		"permission denied", "not found", "exit status 1",
		"fatal:", "panic:",
	}
	for _, ind := range indicators {
		if strings.Contains(lower, ind) {
			return true
		}
	}
	return false
}

func uuidToStr(u pgtype.UUID) string {
	if !u.Valid {
		return ""
	}
	// Convert UUID bytes to hex string
	b := u.Bytes
	return fmt.Sprintf("%x-%x-%x-%x-%x",
		b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}
