package pipeline

// StageConfig defines the agent and in-progress status for a ready_* status.
type StageConfig struct {
	AgentName        string // agent name to assign
	InProgressStatus string // status to set when agent starts (in_*)
}

// Stages maps ready_* status values to their pipeline stage config.
// With simplified statuses, we only have "todo" as the trigger.
var Stages = map[string]StageConfig{
	"todo": {AgentName: "KANBAN", InProgressStatus: "in_dev"},
}

// KanbanStage is the entry stage for Kanban Agent orchestration.
var KanbanStage = StageConfig{
	AgentName:        "KANBAN",
	InProgressStatus: "in_dev",
}

// ClassifierStage is kept for backwards compatibility but is no longer used.
var ClassifierStage = StageConfig{
	AgentName:        "Classifier",
	InProgressStatus: "classifying",
}

// IsKanbanAgent returns true if the given agent name is the Kanban Agent.
func IsKanbanAgent(name string) bool {
	return name == KanbanStage.AgentName
}

// IsClassifierAgent always returns false since we no longer have a classifier stage.
func IsClassifierAgent(name string) bool {
	return false
}

// IsReadyStatus returns true if the status is a pipeline trigger.
func IsReadyStatus(status string) bool {
	_, ok := Stages[status]
	return ok
}

// IsAllowedAgentTransition checks if an agent is allowed to set a given status.
// In simplified mode, kanban agent can set any status.
func IsAllowedAgentTransition(agentName string, status string) bool {
	// Kanban agent can set any status
	if IsKanbanAgent(agentName) {
		return true
	}
	// Other agents can only set done, blocked, or cancelled
	switch status {
	case "done", "blocked", "cancelled":
		return true
	default:
		return false
	}
}

// RevertStatusFor returns the ready_* status that an in_* status should
// revert to when the agent run for that stage fails.
func RevertStatusFor(inStatus string) (readyStatus string, ok bool) {
	for ready, stage := range Stages {
		if stage.InProgressStatus == inStatus {
			return ready, true
		}
	}
	if inStatus == KanbanStage.InProgressStatus {
		return "backlog", true
	}
	return "", false
}

// FanInRule defines how child issue statuses affect the parent.
type FanInRule struct {
	NextStatus    string   // status to set on parent when all children are terminal
	Terminal      []string // child statuses considered terminal
}

// IsTerminal returns true if the given status is considered terminal for fan-in.
func (r FanInRule) IsTerminal(status string) bool {
	for _, t := range r.Terminal {
		if t == status {
			return true
		}
	}
	return false
}

// FanInRuleFor returns the fan-in rule for a given parent status.
func FanInRuleFor(parentStatus string) (FanInRule, bool) {
	// For any non-terminal parent status, use default fan-in
	switch parentStatus {
	case "done", "cancelled":
		return FanInRule{}, false // already terminal
	default:
		return FanInRule{
			NextStatus: "done",
			Terminal:   []string{"done", "cancelled"},
		}, true
	}
}

// FanInConfig controls how child issue statuses affect the parent.
type FanInConfig struct {
	// ChildTerminalStatuses are statuses considered "finished" for fan-in.
	ChildTerminalStatuses []string
	// ParentDoneStatus is the status to set on the parent when all children are terminal.
	ParentDoneStatus string
	// ParentBlockedStatus is the status to set when any child is blocked.
	ParentBlockedStatus string
}

// DefaultFanIn returns the default fan-in configuration.
func DefaultFanIn() FanInConfig {
	return FanInConfig{
		ChildTerminalStatuses: []string{"done", "cancelled"},
		ParentDoneStatus:      "done",
		ParentBlockedStatus:   "blocked",
	}
}
