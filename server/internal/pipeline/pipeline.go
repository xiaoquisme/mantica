package pipeline

// StageConfig defines the agent and in-progress status for a ready_* status.
type StageConfig struct {
	AgentName        string // agent name to assign
	InProgressStatus string // status to set when agent starts (in_*)
}

// Stages maps status values to their pipeline stage config.
// ready_* statuses are triggered automatically when an agent sets them.
// backlog is triggered when a user assigns the Classifier agent.
var Stages = map[string]StageConfig{
	"backlog":            {AgentName: "Classifier", InProgressStatus: "classifying"},
	"ready_analyze":      {AgentName: "BA", InProgressStatus: "in_analyze"},
	"ready_arch_design":  {AgentName: "TL", InProgressStatus: "in_arch_design"},
	"ready_dev":          {AgentName: "DEV", InProgressStatus: "in_dev"},
	"ready_review":       {AgentName: "Code Review", InProgressStatus: "in_review"},
	"ready_test":         {AgentName: "QA", InProgressStatus: "in_test"},
}

// IsReadyStatus returns true if the status is a pipeline trigger.
func IsReadyStatus(status string) bool {
	_, ok := Stages[status]
	return ok
}
