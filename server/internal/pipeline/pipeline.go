package pipeline

// StageConfig defines the agent and in-progress status for a ready_* status.
type StageConfig struct {
	AgentName        string // agent name to assign
	InProgressStatus string // status to set when agent starts (in_*)
}

// Stages maps ready_* status values to their pipeline stage config.
// Triggered automatically when an agent sets status to ready_*.
// Note: backlog is NOT here — it is handled separately on assigneeChanged
// so users can freely move issues back to backlog without auto-triggering.
var Stages = map[string]StageConfig{
	"ready_analyze":     {AgentName: "BA", InProgressStatus: "in_analyze"},
	"ready_arch_design": {AgentName: "TL", InProgressStatus: "in_arch_design"},
	"ready_dev":         {AgentName: "DEV", InProgressStatus: "in_dev"},
	"ready_review":      {AgentName: "Code Review", InProgressStatus: "in_review"},
	"ready_test":        {AgentName: "QA", InProgressStatus: "in_test"},
}

// ClassifierStage is the entry stage triggered when Classifier is assigned to a backlog issue.
var ClassifierStage = StageConfig{
	AgentName:        "Classifier",
	InProgressStatus: "classifying",
}

// IsClassifierAgent returns true if the given agent name is the Classifier.
func IsClassifierAgent(name string) bool {
	return name == ClassifierStage.AgentName
}

// IsReadyStatus returns true if the status is a pipeline trigger.
func IsReadyStatus(status string) bool {
	_, ok := Stages[status]
	return ok
}

// AllowedAgentTransitions maps agent name to the statuses they are allowed to set.
// This prevents agents from setting status to backlog, done, or other stages they don't own.
var AllowedAgentTransitions = map[string][]string{
	"Classifier":  {"ready_analyze", "ready_arch_design", "blocked"},
	"BA":          {"ready_arch_design", "blocked"},
	"TL":          {"ready_dev", "blocked"},
	"DEV":         {"ready_review", "blocked"},
	"Code Review": {"ready_test", "ready_dev", "blocked"},
	"QA":          {"done", "ready_dev", "blocked"},
}

// IsAllowedAgentTransition checks if an agent is allowed to set a given status.
// Returns true if the agent name is not in the map (no restriction) or the status is allowed.
func IsAllowedAgentTransition(agentName, newStatus string) bool {
	allowed, ok := AllowedAgentTransitions[agentName]
	if !ok {
		return true
	}
	for _, s := range allowed {
		if s == newStatus {
			return true
		}
	}
	return false
}

// FanInRule describes how a parent issue advances when its parallel children
// all reach a terminal status.
type FanInRule struct {
	NextStatus            string   // status to set on the parent when fan-in fires
	TerminalChildStatuses []string // child statuses that count as "done" for fan-in
}

// IsTerminal returns true if the given child status counts as terminal under this rule.
func (r FanInRule) IsTerminal(status string) bool {
	for _, s := range r.TerminalChildStatuses {
		if s == status {
			return true
		}
	}
	return false
}

// FanInConfig maps a parent's in_* status to the rule that decides when the
// parent advances based on its parallel child issues. When every child reaches
// a terminal status, the parent is moved to NextStatus and the existing
// pipeline machinery handles the rest (agent reassignment, task enqueue, WS
// broadcast).
var FanInConfig = map[string]FanInRule{
	"in_arch_design": {NextStatus: "ready_dev", TerminalChildStatuses: []string{"done"}},
	"in_dev":         {NextStatus: "ready_review", TerminalChildStatuses: []string{"done"}},
	"in_review":      {NextStatus: "ready_test", TerminalChildStatuses: []string{"done"}},
	"in_test":        {NextStatus: "done", TerminalChildStatuses: []string{"done"}},
}

// FanInRuleFor returns the fan-in rule for a parent's current status, if any.
func FanInRuleFor(parentStatus string) (FanInRule, bool) {
	r, ok := FanInConfig[parentStatus]
	return r, ok
}
