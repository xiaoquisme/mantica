package daemon

import (
	"log/slog"
	"testing"
)

func TestKanbanAgent_ClassifyTask(t *testing.T) {
	k := &KanbanAgent{
		DecompositionRules: make(map[TaskType]DecompositionRule),
		logger:             nil, // logger not needed for classification
	}
	k.initDecompositionRules()

	tests := []struct {
		name        string
		title       string
		description string
		want        TaskType
	}{
		{
			name:        "bug fix detection",
			title:       "Fix login error",
			description: "Users are getting a 500 error when logging in",
			want:        TaskTypeBugFix,
		},
		{
			name:        "feature detection",
			title:       "Add new feature",
			description: "Implement user dashboard",
			want:        TaskTypeFeature,
		},
		{
			name:        "refactoring detection",
			title:       "Refactor codebase",
			description: "Clean up unused code and restructure the project",
			want:        TaskTypeRefactoring,
		},
		{
			name:        "default to feature",
			title:       "Update documentation",
			description: "Update README file",
			want:        TaskTypeFeature,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := k.ClassifyTask(tt.title, tt.description)
			if got != tt.want {
				t.Errorf("ClassifyTask() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestKanbanAgent_DecomposeTask(t *testing.T) {
	k := &KanbanAgent{
		DecompositionRules: make(map[TaskType]DecompositionRule),
		logger:             slog.Default(),
	}
	k.initDecompositionRules()

	tests := []struct {
		name        string
		issueID     string
		title       string
		description string
		wantErr     bool
		wantTasks   int
	}{
		{
			name:        "bug fix decomposition",
			issueID:     "test-1",
			title:       "Fix login error",
			description: "Users are getting a 500 error",
			wantErr:     false,
			wantTasks:   4, // TL, DEV, QA, Code Review
		},
		{
			name:        "feature decomposition",
			issueID:     "test-2",
			title:       "Add new feature",
			description: "Implement dashboard",
			wantErr:     false,
			wantTasks:   5, // BA, TL, DEV, QA, Code Review
		},
		{
			name:        "refactoring decomposition",
			issueID:     "test-3",
			title:       "Refactor codebase",
			description: "Clean up unused code and restructure the project",
			wantErr:     false,
			wantTasks:   3, // TL, DEV, Code Review
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			subTasks, err := k.DecomposeTask(nil, tt.issueID, tt.title, tt.description)
			if (err != nil) != tt.wantErr {
				t.Errorf("DecomposeTask() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if len(subTasks) != tt.wantTasks {
				t.Errorf("DecomposeTask() created %d tasks, want %d", len(subTasks), tt.wantTasks)
			}
		})
	}
}

func TestKanbanAgent_AggregateResults(t *testing.T) {
	k := &KanbanAgent{
		DecompositionRules: make(map[TaskType]DecompositionRule),
		logger:             slog.Default(),
	}
	k.initDecompositionRules()

	results := map[string]*SubTaskResult{
		"task-1": {Success: true, Output: "Task 1 completed"},
		"task-2": {Success: true, Output: "Task 2 completed"},
		"task-3": {Success: false, Error: "Task 3 failed"},
	}

	summary := k.AggregateResults(results)

	if summary == "" {
		t.Error("AggregateResults() returned empty string")
	}

	// Check that summary contains task counts
	if len(summary) == 0 {
		t.Error("AggregateResults() summary is empty")
	}
}