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
	AgentName string
	TaskDesc  string
	DependsOn []string
}

// DecompositionRule defines how to decompose a task
type DecompositionRule struct {
	Name         string
	SubTasks     []SubTaskTemplate
	Dependencies map[string][]string // task name -> dependency names
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
	AgentName   string
	Description string
	DependsOn   []string
	Status      string
	Result      *SubTaskResult
}

// KanbanAgent orchestrates task decomposition and execution
type KanbanAgent struct {
	DecompositionRules map[TaskType]DecompositionRule
	SubAgentPool       *SubAgentPool
	queries            *db.Queries
	logger             *slog.Logger
}

// NewKanbanAgent creates a new Kanban Agent
func NewKanbanAgent(pool *SubAgentPool, queries *db.Queries, logger *slog.Logger) *KanbanAgent {
	k := &KanbanAgent{
		DecompositionRules: make(map[TaskType]DecompositionRule),
		SubAgentPool:       pool,
		queries:            queries,
		logger:             logger,
	}

	// Initialize decomposition rules
	k.initDecompositionRules()

	return k
}

// findAgentByName finds an agent by name in a specific workspace
func (k *KanbanAgent) findAgentByName(ctx context.Context, workspaceID pgtype.UUID, name string) (*db.Agent, error) {
	agents, err := k.queries.ListAgents(ctx, workspaceID)
	if err != nil {
		return nil, fmt.Errorf("agent lookup failed: %v", err)
	}

	for _, agent := range agents {
		if agent.Name == name {
			return &agent, nil
		}
	}

	return nil, fmt.Errorf("agent not found: %s", name)
}

// initDecompositionRules sets up the default decomposition rules
func (k *KanbanAgent) initDecompositionRules() {
	// Bug Fix rule
	k.DecompositionRules[TaskTypeBugFix] = DecompositionRule{
		Name: "Bug Fix",
		SubTasks: []SubTaskTemplate{
			{AgentName: "TL", TaskDesc: "Analyze root cause"},
			{AgentName: "DEV", TaskDesc: "Implement fix", DependsOn: []string{"TL"}},
			{AgentName: "QA", TaskDesc: "Test fix", DependsOn: []string{"DEV"}},
			{AgentName: "Code Review", TaskDesc: "Review code", DependsOn: []string{"DEV"}},
		},
		Dependencies: map[string][]string{
			"DEV":         {"TL"},
			"QA":          {"DEV"},
			"Code Review": {"DEV"},
		},
	}

	// Feature Implementation rule
	k.DecompositionRules[TaskTypeFeature] = DecompositionRule{
		Name: "Feature Implementation",
		SubTasks: []SubTaskTemplate{
			{AgentName: "BA", TaskDesc: "Analyze requirements"},
			{AgentName: "TL", TaskDesc: "Design architecture", DependsOn: []string{"BA"}},
			{AgentName: "DEV", TaskDesc: "Implement feature", DependsOn: []string{"TL"}},
			{AgentName: "QA", TaskDesc: "Write and run tests", DependsOn: []string{"DEV"}},
			{AgentName: "Code Review", TaskDesc: "Review implementation", DependsOn: []string{"DEV"}},
		},
		Dependencies: map[string][]string{
			"TL":          {"BA"},
			"DEV":         {"TL"},
			"QA":          {"DEV"},
			"Code Review": {"DEV"},
		},
	}

	// Refactoring rule
	k.DecompositionRules[TaskTypeRefactoring] = DecompositionRule{
		Name: "Refactoring",
		SubTasks: []SubTaskTemplate{
			{AgentName: "TL", TaskDesc: "Plan refactoring"},
			{AgentName: "DEV", TaskDesc: "Execute refactoring", DependsOn: []string{"TL"}},
			{AgentName: "Code Review", TaskDesc: "Review changes", DependsOn: []string{"DEV"}},
		},
		Dependencies: map[string][]string{
			"DEV":         {"TL"},
			"Code Review": {"DEV"},
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
			ID:          fmt.Sprintf("%s-%s-%d", issueID, template.AgentName, i),
			IssueID:     issueID,
			AgentName:   template.AgentName,
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
// Each task is executed via the cyclic Main→Terminal→Summary pattern.
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

	for _, task := range subTasks {
		if completed[task.ID] {
			continue
		}

		// Check if all dependencies are completed
		allDepsCompleted := true
		for _, dep := range task.DependsOn {
			// Find the task with this agent name
			found := false
			for _, t := range subTasks {
				if t.AgentName == dep && completed[t.ID] {
					found = true
					break
				}
			}
			if !found {
				allDepsCompleted = false
				break
			}
		}

		if allDepsCompleted {
			ready = append(ready, task)
		}
	}

	return ready
}

// executeSubTask executes a single sub-task by creating a database task and waiting for completion
func (k *KanbanAgent) executeSubTask(ctx context.Context, parentTaskID string, workspaceID pgtype.UUID, task SubTask) *SubTaskResult {
	k.logger.Info("executing sub-task", "task_id", task.ID, "agent", task.AgentName, "description", task.Description)

	// Parse parent task ID
	parentUUID, err := parseUUID(parentTaskID)
	if err != nil {
		return &SubTaskResult{
			Success: false,
			Error:   fmt.Sprintf("invalid parent task ID: %v", err),
		}
	}

	// Find agent by name in the workspace
	agent, err := k.findAgentByName(ctx, workspaceID, task.AgentName)
	if err != nil {
		return &SubTaskResult{
			Success: false,
			Error:   fmt.Sprintf("agent not found: %s, error: %v", task.AgentName, err),
		}
	}

	// Find a runtime for this agent
	runtimeID, err := k.findRuntimeForAgent(ctx, agent.ID)
	if err != nil {
		return &SubTaskResult{
			Success: false,
			Error:   fmt.Sprintf("no runtime found for agent: %s, error: %v", task.AgentName, err),
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
	subTaskDepth := 1 // Could be calculated based on parent depth

	createdTask, err := k.queries.CreateSubTask(ctx, db.CreateSubTaskParams{
		AgentID:      agent.ID,
		RuntimeID:    runtimeID,
		IssueID:      issueID,
		Priority:     5, // Default priority
		ParentTaskID: parentUUID,
		SubagentRole: pgtype.Text{String: task.AgentName, Valid: true},
		TaskDepth:    int32(subTaskDepth),
	})
	if err != nil {
		return &SubTaskResult{
			Success: false,
			Error:   fmt.Sprintf("failed to create sub-task: %v", err),
		}
	}

	k.logger.Info("sub-task created", "subtask_id", createdTask.ID, "agent", task.AgentName)

	// Wait for sub-task to complete (poll database)
	result := k.waitForSubTaskCompletion(ctx, createdTask.ID, 5*time.Minute)
	return result
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

// findRuntimeForAgent finds a runtime for an agent
func (k *KanbanAgent) findRuntimeForAgent(ctx context.Context, agentID pgtype.UUID) (pgtype.UUID, error) {
	// Get agent details to find associated runtime
	agent, err := k.queries.GetAgent(ctx, agentID)
	if err != nil {
		return pgtype.UUID{}, fmt.Errorf("failed to get agent: %v", err)
	}

	// If agent has a runtime_id, use it
	if agent.RuntimeID.Valid {
		return agent.RuntimeID, nil
	}

	// Otherwise, find a default runtime
	// This needs to be implemented based on your runtime selection logic
	return pgtype.UUID{}, fmt.Errorf("no runtime found for agent")
}

// parseUUID parses a string into a pgtype.UUID
func parseUUID(s string) (pgtype.UUID, error) {
	var uuid pgtype.UUID
	err := uuid.Scan(s)
	return uuid, err
}

// WaitForSubTasks waits for all sub-tasks to complete
func (k *KanbanAgent) WaitForSubTasks(ctx context.Context, timeout time.Duration) error {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	timeoutCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	for {
		select {
		case <-timeoutCtx.Done():
			return fmt.Errorf("timeout waiting for sub-tasks")
		case <-ticker.C:
			// Check sub-task status from database
			k.logger.Debug("checking sub-task status")
		}
	}
}

// AggregateResults combines results from all sub-tasks
func (k *KanbanAgent) AggregateResults(results map[string]*SubTaskResult) string {
	var output string
	successCount := 0
	failureCount := 0

	for taskID, result := range results {
		if result.Success {
			successCount++
			output += fmt.Sprintf("✓ %s: %s\n", taskID, result.Output)
		} else {
			failureCount++
			output += fmt.Sprintf("✗ %s: %s\n", taskID, result.Error)
		}
	}

	summary := fmt.Sprintf("\n=== Task Summary ===\n")
	summary += fmt.Sprintf("Total: %d, Success: %d, Failed: %d\n", len(results), successCount, failureCount)
	summary += output

	return summary
}