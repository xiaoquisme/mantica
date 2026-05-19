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
	// New dimensions
	OutputLanguage       string  // "zh", "en", "mixed", "unknown"
	OutputLength         int32   // total character count of final output
	ToolEfficiency       float64 // unique_tools / tool_count (higher = more diverse, less repetitive)
	FirstAttemptSuccess  bool    // no errors at all
	CommunicationQuality float64 // 0-1 score based on output length and structure
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

	// New dimensions
	result.FirstAttemptSuccess = !hadError && result.ErrorCount == 0

	// Tool efficiency: unique / total (higher = less repetitive)
	if result.ToolCount > 0 {
		result.ToolEfficiency = float64(result.UniqueTools) / float64(result.ToolCount)
	}

	// Output language and length from text messages
	for _, msg := range messages {
		if msg.Type == "text" && msg.Content.Valid {
			content := msg.Content.String
			result.OutputLength += int32(len(content))
			if result.OutputLanguage == "" {
				result.OutputLanguage = detectLanguage(content)
			}
		}
	}

	// Communication quality: based on output length and structure
	result.CommunicationQuality = scoreCommunication(result.OutputLength, task.Status)

	// Determine failure class for failed tasks
	if task.Status == "failed" {
		errMsg := task.Error.String
		result.FailureClass = classifyFailure(errMsg, result)
		result.FailureDetail = errMsg
	} else if task.Status == "cancelled" {
		result.FailureClass = "cancelled"
		result.FailureDetail = "Task was cancelled (timeout or user intervention)"
	}

	return result, nil
}

// SaveAnalysis persists the analysis result to the task_analysis table.
func (a *Analyzer) SaveAnalysis(ctx context.Context, taskID pgtype.UUID, r *AnalysisResult) error {
	toolUsageJSON, _ := json.Marshal(r.ToolUsage)

	_, err := a.Queries.CreateTaskAnalysis(ctx, db.CreateTaskAnalysisParams{
		TaskID:               taskID,
		ToolCount:            r.ToolCount,
		ErrorCount:           r.ErrorCount,
		UniqueTools:          r.UniqueTools,
		TotalDurationMs:      r.TotalDurationMs,
		MessageCount:         r.MessageCount,
		FailureClass:         pgtype.Text{String: r.FailureClass, Valid: r.FailureClass != ""},
		FailureDetail:        pgtype.Text{String: r.FailureDetail, Valid: r.FailureDetail != ""},
		ToolUsage:            toolUsageJSON,
		HasRetryPattern:      pgtype.Bool{Bool: r.HasRetryPattern, Valid: true},
		HasErrorRecovery:     pgtype.Bool{Bool: r.HasErrorRecovery, Valid: true},
		LongestToolMs:        pgtype.Int8{Int64: r.LongestToolMs, Valid: r.LongestToolMs > 0},
		Summary:              pgtype.Text{},
		ImprovementHint:      pgtype.Text{},
		OutputLanguage:       pgtype.Text{String: r.OutputLanguage, Valid: r.OutputLanguage != ""},
		OutputLength:         pgtype.Int4{Int32: r.OutputLength, Valid: true},
		ToolEfficiency:       pgtype.Float8{Float64: r.ToolEfficiency, Valid: true},
		FirstAttemptSuccess:  pgtype.Bool{Bool: r.FirstAttemptSuccess, Valid: true},
		CommunicationQuality: pgtype.Float8{Float64: r.CommunicationQuality, Valid: true},
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

// isErrorMessage checks if a tool output looks like a real error.
// Avoids false positives from test output, grep results, or log lines
// that happen to contain the word "error".
func isErrorMessage(output string) bool {
	lower := strings.ToLower(output)

	// Strong indicators: these almost always mean a real error
	strongIndicators := []string{
		"traceback (most recent call last):",
		"fatal error:",
		"panic:",
		"segmentation fault",
		"oom-killer",
		"killed",
	}
	for _, ind := range strongIndicators {
		if strings.Contains(lower, ind) {
			return true
		}
	}

	// Exit code patterns: "exit status 1", "exited with error code"
	if strings.Contains(lower, "exit status") && !strings.Contains(lower, "exit status 0") {
		// But not if it's just test output showing exit codes
		if !strings.Contains(lower, "--- pass") && !strings.Contains(lower, "--- ok") {
			return true
		}
	}

	// Command failure patterns
	if strings.HasPrefix(strings.TrimSpace(lower), "error:") ||
		strings.HasPrefix(strings.TrimSpace(lower), "error ") {
		return true
	}

	// Permission / not found
	if strings.Contains(lower, "permission denied") && !strings.Contains(lower, "test_") {
		return true
	}

	return false
}

func uuidToStr(u pgtype.UUID) string {
	if !u.Valid {
		return ""
	}
	b := u.Bytes
	return fmt.Sprintf("%x-%x-%x-%x-%x",
		b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

// detectLanguage identifies the primary language of a text.
// Returns "zh" for Chinese, "en" for English, "mixed" for both.
func detectLanguage(text string) string {
	zhCount := 0
	enCount := 0

	for _, r := range text {
		if r >= 0x4E00 && r <= 0x9FFF {
			// CJK Unified Ideographs
			zhCount++
		} else if r >= 0x3400 && r <= 0x4DBF {
			// CJK Extension A
			zhCount++
		} else if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') {
			enCount++
		}
	}

	total := zhCount + enCount
	if total == 0 {
		return "unknown"
	}

	zhRatio := float64(zhCount) / float64(total)
	enRatio := float64(enCount) / float64(total)

	if zhRatio > 0.3 && enRatio > 0.3 {
		return "mixed"
	}
	if zhRatio > enRatio {
		return "zh"
	}
	return "en"
}

// scoreCommunication rates the quality of the agent's output.
// Factors: length (too short = bad, reasonable = good), structure (has markdown = good).
func scoreCommunication(outputLen int32, taskStatus string) float64 {
	if taskStatus != "completed" {
		return 0.0
	}

	// Length scoring
	var lengthScore float64
	switch {
	case outputLen < 20:
		lengthScore = 0.2 // too terse
	case outputLen < 100:
		lengthScore = 0.6 // brief but acceptable
	case outputLen < 500:
		lengthScore = 1.0 // good length
	case outputLen < 2000:
		lengthScore = 0.8 // a bit verbose
	default:
		lengthScore = 0.5 // very long, likely unfocused
	}

	return lengthScore
}
