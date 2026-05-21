package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"

	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/xiaoquisme/mantica/server/pkg/db/generated"
	"github.com/xiaoquisme/mantica/server/internal/util"
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

	// ── Quality Gates ──

	// Gate 1: Error rate must be reasonable
	if analysis.ErrorCount > analysis.ToolCount/2 {
		return result
	}
	// Gate 2: Must have enough tool calls to be interesting
	if analysis.ToolCount < 2 {
		return result
	}
	// Gate 3: Output must be substantial (not empty or trivial)
	if analysis.OutputLength.Valid && analysis.OutputLength.Int32 < 20 {
		return result
	}
	// Gate 4: Communication quality check
	if analysis.CommunicationQuality.Valid && analysis.CommunicationQuality.Float64 < 0.4 {
		return result
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

	// ── Dedup: Check if a similar skill already exists ──

	// Check agent-level skills
	existingSkills, _ := e.Queries.ListAgentSkills(ctx, agentID)
	for _, s := range existingSkills {
		if skillCoversPattern(s.Content, toolSeq) {
			// Existing skill already covers this pattern — update quality instead
			e.Queries.RecordSkillSuccess(ctx, s.ID)
			e.Logger.Info("skill reinforced (existing pattern)", "skill_id", util.UUIDToString(s.ID), "task_id", util.UUIDToString(task.ID))
			return result
		}
	}

	// Check workspace-level similar skills
	skillName := generateSkillName(task, toolSeq)
	nameParts := strings.Split(skillName, "/")
	if len(nameParts) > 1 {
		coreName := nameParts[1] // e.g. "terminal-execute_code" from "auto/terminal-execute_code-workflow"
		similar, _ := e.Queries.ListSimilarSkills(ctx, db.ListSimilarSkillsParams{
			WorkspaceID: workspaceID,
			ID:          pgtype.UUID{}, // exclude self
			Column3:     pgtype.Text{String: coreName, Valid: true},
		})
		if len(similar) > 0 {
			// Merge: update the existing similar skill if ours is better
			best := similar[0]
			for _, s := range similar[1:] {
				if s.QualityScore.Float64 > best.QualityScore.Float64 {
					best = s
				}
			}
			e.Logger.Info("skill merged with existing (workspace-level)", "existing_skill", best.Name, "new_pattern", skillName)
			return result
		}
	}

	// ── Calculate quality_score ──

	qualityScore := 50.0 // base

	// Communication quality bonus
	if analysis.CommunicationQuality.Valid {
		qualityScore = analysis.CommunicationQuality.Float64 * 100
	}

	// First attempt success bonus
	if analysis.FirstAttemptSuccess.Valid && analysis.FirstAttemptSuccess.Bool {
		qualityScore += 10
	}

	// Tool efficiency bonus (diverse tool usage = more interesting skill)
	if analysis.ToolEfficiency.Valid && analysis.ToolEfficiency.Float64 > 0.5 {
		qualityScore += 5
	}

	// Low error rate bonus
	if analysis.ErrorCount == 0 {
		qualityScore += 10
	} else if float64(analysis.ErrorCount) < float64(analysis.ToolCount)*0.1 {
		qualityScore += 5
	}

	// Cap at 100
	if qualityScore > 100 {
		qualityScore = 100
	}

	// Gate 5: Minimum quality threshold
	if qualityScore < 40 {
		return result
	}

	// ── Create Skill ──

	skillContent := generateSkillContent(task, messages, toolSeq)

	skill, err := e.Queries.CreateSkill(ctx, db.CreateSkillParams{
		WorkspaceID: workspaceID,
		Name:        skillName,
		Description: fmt.Sprintf("Auto-extracted from task %s (quality: %.0f)", util.UUIDToString(task.ID)[:8], qualityScore),
		Content:     skillContent,
		Config:      []byte("{}"),
		CreatedBy:   pgtype.UUID{},
	})
	if err != nil {
		e.Logger.Warn("failed to auto-create skill", "error", err)
		return result
	}

	// Set quality_score and source_task_id
	e.Queries.UpdateSkillQuality(ctx, db.UpdateSkillQualityParams{
		ID:           skill.ID,
		QualityScore: pgtype.Float8{Float64: qualityScore, Valid: true},
	})

	// Link skill to agent
	e.Queries.AddAgentSkill(ctx, db.AddAgentSkillParams{
		AgentID: agentID,
		SkillID: skill.ID,
	})

	result.SkillExtracted = true
	result.SkillName = skillName

	e.Logger.Info("auto-extracted skill",
		"task_id", util.UUIDToString(task.ID),
		"skill_name", skillName,
		"quality_score", qualityScore,
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

	// Summary of what was accomplished
	b.WriteString("## Summary\n\n")
	textMsgs := filterMessages(messages, "text")
	if len(textMsgs) > 0 {
		// Use the first text message as summary (agent's final output)
		content := textMsgs[0].Content.String
		if len(content) > 500 {
			content = content[:500] + "..."
		}
		b.WriteString(content + "\n\n")
	}

	// Tool sequence with context
	b.WriteString("## Workflow Steps\n\n")
	step := 1
	for _, m := range messages {
		if m.Type != "tool_use" || m.Tool.String == "" {
			continue
		}
		toolName := m.Tool.String
		var input map[string]any
		if m.Input != nil {
			json.Unmarshal(m.Input, &input)
		}

		// Describe the step
		switch toolName {
		case "terminal":
			if cmd, ok := input["command"]; ok {
				b.WriteString(fmt.Sprintf("%d. **Run command**: `%v`\n", step, cmd))
			}
		case "read_file":
			if path, ok := input["path"]; ok {
				b.WriteString(fmt.Sprintf("%d. **Read file**: `%v`\n", step, path))
			}
		case "write_file":
			if path, ok := input["path"]; ok {
				b.WriteString(fmt.Sprintf("%d. **Write file**: `%v`\n", step, path))
			}
		case "patch":
			if path, ok := input["path"]; ok {
				b.WriteString(fmt.Sprintf("%d. **Edit file**: `%v`\n", step, path))
			}
		case "search_files":
			if pattern, ok := input["pattern"]; ok {
				b.WriteString(fmt.Sprintf("%d. **Search**: `%v`\n", step, pattern))
			}
		case "execute_code":
			b.WriteString(fmt.Sprintf("%d. **Execute Python script**\n", step))
		default:
			b.WriteString(fmt.Sprintf("%d. **%s**\n", step, toolName))
		}
		step++
	}
	b.WriteString("\n")

	// Pitfalls section: errors encountered and how they were resolved
	errorMsgs := filterMessages(messages, "tool_result")
	var pitfalls []string
	for _, m := range errorMsgs {
		if isErrorMessage(m.Output.String) {
			output := m.Output.String
			if len(output) > 200 {
				output = output[:200] + "..."
			}
			pitfalls = append(pitfalls, output)
		}
	}
	if len(pitfalls) > 0 {
		b.WriteString("## Pitfalls Encountered\n\n")
		for i, p := range pitfalls {
			if i >= 3 {
				break // limit to 3
			}
			b.WriteString(fmt.Sprintf("- %s\n", p))
		}
		b.WriteString("\n")
	}

	return b.String()
}

func filterMessages(messages []db.TaskMessage, msgType string) []db.TaskMessage {
	var result []db.TaskMessage
	for _, m := range messages {
		if m.Type == msgType {
			result = append(result, m)
		}
	}
	return result
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
