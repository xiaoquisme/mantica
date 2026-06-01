package daemon

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/xiaoquisme/mantica/server/pkg/db/generated"
)

// TaskType represents the type of task
type TaskType string

const (
	TaskTypeBugFix      TaskType = "bug_fix"
	TaskTypeFeature     TaskType = "feature"
	TaskTypeRefactoring TaskType = "refactoring"
	TaskTypeUnknown     TaskType = "unknown"
)

// SubTaskTemplate defines a template for creating sub-tasks
type SubTaskTemplate struct {
	TaskDesc  string
	DependsOn []int // indices of tasks this depends on
}

// DecompositionRule defines how to decompose a task
type DecompositionRule struct {
	Name     string
	SubTasks []SubTaskTemplate
}

// SubTaskResult represents the result of a sub-task
type SubTaskResult struct {
	Success bool
	Output  string
	Error   string
}

// SubTask represents a sub-task created by Kanban Agent
type SubTask struct {
	ID          string
	IssueID     string
	Description string
	DependsOn   []int
	Status      string
	Result      *SubTaskResult
}

// KanbanAgent orchestrates task decomposition and execution
type KanbanAgent struct {
	DecompositionRules map[TaskType]DecompositionRule
	queries            *db.Queries
	logger             *slog.Logger
}

// NewKanbanAgent creates a new Kanban Agent
func NewKanbanAgent(queries *db.Queries, logger *slog.Logger) *KanbanAgent {
	k := &KanbanAgent{
		DecompositionRules: make(map[TaskType]DecompositionRule),
		queries:            queries,
		logger:             logger,
	}

	// Initialize decomposition rules
	k.initDecompositionRules()

	return k
}

// initDecompositionRules sets up the default decomposition rules
func (k *KanbanAgent) initDecompositionRules() {
	// Bug Fix rule
	k.DecompositionRules[TaskTypeBugFix] = DecompositionRule{
		Name: "Bug Fix",
		SubTasks: []SubTaskTemplate{
			{TaskDesc: "Analyze root cause"},
			{TaskDesc: "Implement fix", DependsOn: []int{0}},
			{TaskDesc: "Test fix", DependsOn: []int{1}},
			{TaskDesc: "Review code", DependsOn: []int{1}},
		},
	}

	// Feature Implementation rule
	k.DecompositionRules[TaskTypeFeature] = DecompositionRule{
		Name: "Feature Implementation",
		SubTasks: []SubTaskTemplate{
			{TaskDesc: "Analyze requirements"},
			{TaskDesc: "Design architecture", DependsOn: []int{0}},
			{TaskDesc: "Implement feature", DependsOn: []int{1}},
			{TaskDesc: "Write and run tests", DependsOn: []int{2}},
			{TaskDesc: "Review implementation", DependsOn: []int{2}},
		},
	}

	// Refactoring rule
	k.DecompositionRules[TaskTypeRefactoring] = DecompositionRule{
		Name: "Refactoring",
		SubTasks: []SubTaskTemplate{
			{TaskDesc: "Plan refactoring"},
			{TaskDesc: "Execute refactoring", DependsOn: []int{0}},
			{TaskDesc: "Review changes", DependsOn: []int{1}},
		},
	}
}

// ClassifyTask determines the task type based on issue description
func (k *KanbanAgent) ClassifyTask(title, description string) TaskType {
	// Simple keyword-based classification
	titleLower := title + " " + description

	// Bug fix indicators
	bugKeywords := []string{"bug", "fix", "error", "crash", "issue", "broken", "not working"}
	for _, kw := range bugKeywords {
		if contains(titleLower, kw) {
			return TaskTypeBugFix
		}
	}

	// Refactoring indicators
	refactorKeywords := []string{"refactor", "cleanup", "restructure", "reorganize", "improve"}
	for _, kw := range refactorKeywords {
		if contains(titleLower, kw) {
			return TaskTypeRefactoring
		}
	}

	// Default to feature
	return TaskTypeFeature
}

// contains checks if a string contains a substring (case-insensitive)
func contains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// DecomposeTask breaks down a task into sub-tasks
func (k *KanbanAgent) DecomposeTask(ctx context.Context, issueID, title, description string) ([]SubTask, error) {
	taskType := k.ClassifyTask(title, description)

	rule, ok := k.DecompositionRules[taskType]
	if !ok {
		return nil, fmt.Errorf("no decomposition rule for task type: %s", taskType)
	}

	k.logger.Info("decomposing task", "issue_id", issueID, "task_type", taskType, "rule", rule.Name)

	subTasks := make([]SubTask, 0, len(rule.SubTasks))

	for i, template := range rule.SubTasks {
		subTask := SubTask{
			ID:          fmt.Sprintf("%s-%d", issueID, i),
			IssueID:     issueID,
			Description: fmt.Sprintf("%s: %s", template.TaskDesc, title),
			DependsOn:   template.DependsOn,
			Status:      "pending",
		}
		subTasks = append(subTasks, subTask)
	}

	return subTasks, nil
}

// ExecuteParallel executes ready sub-tasks in parallel, respecting dependency order.
// Tasks without unmet dependencies are executed concurrently.
func (k *KanbanAgent) ExecuteParallel(ctx context.Context, parentTaskID string, workspaceID pgtype.UUID, subTasks []SubTask) (map[string]*SubTaskResult, error) {
	completed := make(map[string]bool)
	results := make(map[string]*SubTaskResult)
	var mu sync.Mutex

	for {
		// Find tasks ready to execute (all deps completed)
		ready := k.findReadyTasks(subTasks, completed)
		if len(ready) == 0 {
			break
		}

		// Execute ready tasks in parallel
		var wg sync.WaitGroup
		for _, task := range ready {
			wg.Add(1)
			go func(t SubTask) {
				defer wg.Done()

				result := k.executeSubTask(ctx, parentTaskID, workspaceID, t)

				mu.Lock()
				results[t.ID] = result
				completed[t.ID] = true
				mu.Unlock()
			}(task)
		}

		wg.Wait()
	}

	// Check if all tasks completed
	if len(completed) != len(subTasks) {
		return results, fmt.Errorf("not all tasks completed: %d/%d", len(completed), len(subTasks))
	}

	return results, nil
}

// findReadyTasks finds tasks whose dependencies are all completed
func (k *KanbanAgent) findReadyTasks(subTasks []SubTask, completed map[string]bool) []SubTask {
	var ready []SubTask

	for idx, task := range subTasks {
		if completed[task.ID] {
			continue
		}

		// Check if all dependencies are completed
		allDepsCompleted := true
		for _, depIdx := range task.DependsOn {
			if depIdx < len(subTasks) && !completed[subTasks[depIdx].ID] {
				allDepsCompleted = false
				break
			}
		}

		if allDepsCompleted {
			ready = append(ready, subTasks[idx])
		}
	}

	return ready
}

// executeSubTask executes a single sub-task by creating a database task and waiting for completion
func (k *KanbanAgent) executeSubTask(ctx context.Context, parentTaskID string, workspaceID pgtype.UUID, task SubTask) *SubTaskResult {
	k.logger.Info("executing sub-task", "task_id", task.ID, "description", task.Description)

	// Parse parent task ID
	parentUUID, err := parseUUID(parentTaskID)
	if err != nil {
		return &SubTaskResult{
			Success: false,
			Error:   fmt.Sprintf("invalid parent task ID: %v", err),
		}
	}

	// Find any available agent with a runtime in the workspace
	agent, runtimeID, err := k.findAvailableAgent(ctx, workspaceID)
	if err != nil {
		return &SubTaskResult{
			Success: false,
			Error:   fmt.Sprintf("no available agent: %v", err),
		}
	}

	// Create sub-task in database
	issueID, err := parseUUID(task.IssueID)
	if err != nil {
		return &SubTaskResult{
			Success: false,
			Error:   fmt.Sprintf("invalid issue ID: %v", err),
		}
	}
	subTaskDepth := 1

	createdTask, err := k.queries.CreateSubTask(ctx, db.CreateSubTaskParams{
		AgentID:      agent.ID,
		RuntimeID:    runtimeID,
		IssueID:      issueID,
		Priority:     5,
		ParentTaskID: parentUUID,
		SubagentRole: pgtype.Text{String: "main", Valid: true},
		TaskDepth:    int32(subTaskDepth),
	})
	if err != nil {
		return &SubTaskResult{
			Success: false,
			Error:   fmt.Sprintf("failed to create sub-task: %v", err),
		}
	}

	k.logger.Info("sub-task created", "subtask_id", createdTask.ID)

	// Wait for sub-task to complete (poll database)
	result := k.waitForSubTaskCompletion(ctx, createdTask.ID, 5*time.Minute)
	return result
}

// findAvailableAgent finds any agent with a runtime in the workspace
func (k *KanbanAgent) findAvailableAgent(ctx context.Context, workspaceID pgtype.UUID) (*db.Agent, pgtype.UUID, error) {
	agents, err := k.queries.ListAgents(ctx, workspaceID)
	if err != nil {
		return nil, pgtype.UUID{}, fmt.Errorf("failed to list agents: %v", err)
	}

	for _, agent := range agents {
		if agent.RuntimeID.Valid {
			return &agent, agent.RuntimeID, nil
		}
	}

	return nil, pgtype.UUID{}, fmt.Errorf("no agent with runtime found in workspace")
}

// waitForSubTaskCompletion polls the database until the sub-task completes or times out
func (k *KanbanAgent) waitForSubTaskCompletion(ctx context.Context, taskID pgtype.UUID, timeout time.Duration) *SubTaskResult {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	timeoutCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	for {
		select {
		case <-timeoutCtx.Done():
			return &SubTaskResult{
				Success: false,
				Error:   "timeout waiting for sub-task completion",
			}
		case <-ticker.C:
			task, err := k.queries.GetAgentTask(ctx, taskID)
			if err != nil {
				k.logger.Warn("failed to get sub-task status", "task_id", taskID, "error", err)
				continue
			}

			switch task.Status {
			case "completed":
				output := ""
				if len(task.Result) > 0 {
					output = string(task.Result)
				}
				return &SubTaskResult{
					Success: true,
					Output:  output,
					Error:   "",
				}
			case "failed":
				errorMsg := ""
				if task.Error.Valid {
					errorMsg = task.Error.String
				}
				return &SubTaskResult{
					Success: false,
					Output:  "",
					Error:   errorMsg,
				}
			case "cancelled":
				return &SubTaskResult{
					Success: false,
					Output:  "",
					Error:   "sub-task was cancelled",
				}
			// Still running, continue polling
			case "queued", "dispatched", "running":
				continue
			default:
				k.logger.Warn("unknown task status", "task_id", taskID, "status", task.Status)
			}
		}
	}
}

// parseUUID parses a string into a pgtype.UUID
func parseUUID(s string) (pgtype.UUID, error) {
	var uuid pgtype.UUID
	err := uuid.Scan(s)
	return uuid, err
}

// AggregateResults combines results from all sub-tasks
func (k *KanbanAgent) AggregateResults(results []*SubTaskResult) string {
	var output string
	successCount := 0
	failureCount := 0

	for i, result := range results {
		if result.Success {
			successCount++
			output += fmt.Sprintf("✓ Step %d: %s\n", i+1, result.Output)
		} else {
			failureCount++
			output += fmt.Sprintf("✗ Step %d: %s\n", i+1, result.Error)
		}
	}

	summary := fmt.Sprintf("\n=== Task Summary ===\n")
	summary += fmt.Sprintf("Total: %d, Success: %d, Failed: %d\n", len(results), successCount, failureCount)
	summary += output

	return summary
}
