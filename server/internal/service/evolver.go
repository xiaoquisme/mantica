package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"

	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"github.com/multica-ai/multica/server/internal/util"
)

// Evolver handles automatic skill extraction from successful tasks
// and improvement hint generation for failed tasks.
type Evolver struct {
	Queries *db.Queries
	Logger  *slog.Logger
}

func NewEvolver(q *db.Queries, logger *slog.Logger) *Evolver {
	return &Evolver{Queries: q, Logger: logger}
}

// EvolutionResult holds the outcome of an evolution analysis.
type EvolutionResult struct {
	SkillExtracted    bool
	SkillName         string
	ImprovementHint   string
	FailurePattern    string
	ConsecutiveFails  int
}

// AnalyzeAndEvolve examines a completed task and decides whether to:
// 1. Extract a reusable skill (for successful tasks with good metrics)
// 2. Generate improvement hints (for repeated failures)
func (e *Evolver) AnalyzeAndEvolve(ctx context.Context, taskID, agentID pgtype.UUID, workspaceID pgtype.UUID) (*EvolutionResult, error) {
	result := &EvolutionResult{}

	// Get task info
	task, err := e.Queries.GetAgentTask(ctx, taskID)
	if err != nil {
		return nil, err
	}

	// Get analysis
	analysis, err := e.Queries.GetTaskAnalysis(ctx, taskID)
	if err != nil {
		// No analysis available, skip
		return result, nil
	}

	if task.Status == "completed" {
		result = e.handleSuccess(ctx, task, analysis, agentID, workspaceID)
	} else if task.Status == "failed" {
		result = e.handleFailure(ctx, task, analysis, agentID)
	}

	return result, nil
}

// handleSuccess extracts a skill from a successful task if it's worth saving.
func (e *Evolver) handleSuccess(ctx context.Context, task db.AgentTaskQueue, analysis db.TaskAnalysis, agentID, workspaceID pgtype.UUID) *EvolutionResult {
	result := &EvolutionResult{}

	// Only extract skills from tasks with good metrics
	if analysis.ErrorCount > analysis.ToolCount/2 {
		return result // too many errors, not a clean success
	}
	if analysis.ToolCount < 2 {
		return result // too simple, not worth a skill
	}

	// Get task messages to understand what was done
	messages, err := e.Queries.ListTaskMessages(ctx, task.ID)
	if err != nil || len(messages) == 0 {
		return result
	}

	// Build tool sequence summary
	toolSeq := extractToolSequence(messages)
	if len(toolSeq) < 2 {
		return result
	}

	// Check if a similar skill already exists for this agent
	existingSkills, _ := e.Queries.ListAgentSkills(ctx, agentID)
	for _, s := range existingSkills {
		if skillCoversPattern(s.Content, toolSeq) {
			return result // already have a skill for this pattern
		}
	}

	// Generate skill from execution pattern
	skillName := generateSkillName(task, toolSeq)
	skillContent := generateSkillContent(task, messages, toolSeq)

	// Save as a workspace skill
	skill, err := e.Queries.CreateSkill(ctx, db.CreateSkillParams{
		WorkspaceID: workspaceID,
		Name:        skillName,
		Description: fmt.Sprintf("Auto-extracted from task %s", util.UUIDToString(task.ID)[:8]),
		Content:     skillContent,
		Config:      []byte("{}"),
		CreatedBy:   pgtype.UUID{},
	})
	if err != nil {
		e.Logger.Warn("failed to auto-create skill", "error", err)
		return result
	}

	// Link skill to agent
	e.Queries.AddAgentSkill(ctx, db.AddAgentSkillParams{
		AgentID: agentID,
		SkillID: skill.ID,
	})

	result.SkillExtracted = true
	result.SkillName = skillName

	e.Logger.Info("auto-extracted skill from successful task",
		"task_id", util.UUIDToString(task.ID),
		"skill_name", skillName,
		"agent_id", util.UUIDToString(agentID),
	)

	return result
}

// handleFailure generates improvement hints for repeated failures.
func (e *Evolver) handleFailure(ctx context.Context, task db.AgentTaskQueue, analysis db.TaskAnalysis, agentID pgtype.UUID) *EvolutionResult {
	result := &EvolutionResult{}

	failureClass := analysis.FailureClass.String
	if failureClass == "" {
		return result
	}

	// Count recent consecutive failures with the same class
	recentTasks, err := e.Queries.ListAgentTasks(ctx, agentID)
	if err != nil {
		return result
	}

	consecutive := 0
	for _, t := range recentTasks {
		ta, err := e.Queries.GetTaskAnalysis(ctx, t.ID)
		if err != nil {
			continue
		}
		if ta.FailureClass.String == failureClass {
			consecutive++
		} else {
			break
		}
	}

	result.ConsecutiveFails = consecutive
	result.FailurePattern = failureClass

	// Generate hint after 2+ consecutive failures
	if consecutive >= 2 {
		hint := generateImprovementHint(failureClass, analysis, consecutive)

		// Save hint to task_analysis
		e.Queries.UpdateTaskAnalysisSummary(ctx, db.UpdateTaskAnalysisSummaryParams{
			TaskID:          task.ID,
			Summary:         pgtype.Text{String: fmt.Sprintf("Consecutive %s failure #%d", failureClass, consecutive), Valid: true},
			ImprovementHint: pgtype.Text{String: hint, Valid: true},
		})

		result.ImprovementHint = hint

		e.Logger.Warn("repeated failure detected",
			"agent_id", util.UUIDToString(agentID),
			"failure_class", failureClass,
			"consecutive", consecutive,
			"hint", hint,
		)
	}

	return result
}

// ── Helper Functions ──

func extractToolSequence(messages []db.TaskMessage) []string {
	var seq []string
	for _, m := range messages {
		if m.Type == "tool_use" && m.Tool.Valid && m.Tool.String != "" {
			seq = append(seq, m.Tool.String)
		}
	}
	return seq
}

func skillCoversPattern(content string, toolSeq []string) bool {
	// Simple heuristic: check if the skill content mentions the tool sequence
	if content == "" {
		return false
	}
	lower := strings.ToLower(content)
	for _, tool := range toolSeq {
		if !strings.Contains(lower, strings.ToLower(tool)) {
			return false
		}
	}
	return true
}

func generateSkillName(task db.AgentTaskQueue, toolSeq []string) string {
	// Use tool sequence as skill name basis
	uniqueTools := unique(toolSeq)
	return fmt.Sprintf("auto/%s-workflow", strings.Join(uniqueTools, "-"))
}

func generateSkillContent(task db.AgentTaskQueue, messages []db.TaskMessage, toolSeq []string) string {
	var b strings.Builder

	b.WriteString("# Auto-Extracted Workflow\n\n")
	b.WriteString(fmt.Sprintf("Extracted from task %s\n\n", util.UUIDToString(task.ID)[:8]))

	b.WriteString("## Tool Sequence\n\n")
	for i, tool := range toolSeq {
		b.WriteString(fmt.Sprintf("%d. `%s`\n", i+1, tool))
	}
	b.WriteString("\n")

	// Include key tool inputs
	b.WriteString("## Key Operations\n\n")
	for _, m := range messages {
		if m.Type == "tool_use" && m.Input != nil {
			var input map[string]any
			json.Unmarshal(m.Input, &input)
			if cmd, ok := input["command"]; ok {
				b.WriteString(fmt.Sprintf("- `%s`: `%v`\n", m.Tool.String, cmd))
			} else if path, ok := input["path"]; ok {
				b.WriteString(fmt.Sprintf("- `%s`: %v\n", m.Tool.String, path))
			}
		}
	}

	return b.String()
}

func generateImprovementHint(failureClass string, analysis db.TaskAnalysis, consecutive int) string {
	switch failureClass {
	case "timeout":
		return fmt.Sprintf("Task has timed out %d times consecutively. Consider: reducing scope, breaking into smaller tasks, or increasing timeout.", consecutive)
	case "empty_output":
		return fmt.Sprintf("Agent returned empty output %d times. Check: is the prompt clear? Is the agent configured correctly?", consecutive)
	case "runtime_error":
		return fmt.Sprintf("Runtime error occurred %d times. Check: test failures, build errors, missing dependencies.", consecutive)
	case "cascading_errors":
		return fmt.Sprintf("High error rate (%d errors in %d tool calls). The agent may be stuck in a loop. Consider: simplifying the task, providing more context.", analysis.ErrorCount, analysis.ToolCount)
	case "build_error":
		return "Build error detected. Check: syntax errors, missing imports, dependency issues."
	case "test_fail":
		return "Test failures detected. Check: test expectations, recent code changes, test environment."
	default:
		return fmt.Sprintf("Unknown failure pattern (%d consecutive). Review task output manually.", consecutive)
	}
}

func unique(items []string) []string {
	seen := make(map[string]bool)
	var result []string
	for _, item := range items {
		if !seen[item] {
			seen[item] = true
			result = append(result, item)
		}
	}
	return result
}
