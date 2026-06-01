package execenv

import (
	"fmt"
	"strings"
)

// RolePromptConfig defines the prompt configuration for a specific agent role.
type RolePromptConfig struct {
	Name             string
	Responsibilities string
	EssentialCommands []string
	OutputFormat     string
	TypicalSkills    []string
}

// RolePrompts maps agent names to their role-specific prompt configurations.
var RolePrompts = map[string]RolePromptConfig{
	"classifier": {
		Name:           "Classifier",
		Responsibilities: "Classify issues, determine type (bug/feature/refactor), and route to appropriate stage.",
		EssentialCommands: []string{
			"mantica issue get <id> --output json",
			"mantica issue comment list <id> --output json",
			"mantica issue status <id> todo",
			"mantica issue comment add <id> --content \"...\"",
		},
		OutputFormat: "Post a comment with the classification result and route to the next stage.",
		TypicalSkills: []string{},
	},
	"kanban": {
		Name:           "Kanban Agent",
		Responsibilities: "Orchestrate task execution, decompose issues, and coordinate subtasks.",
		EssentialCommands: []string{
			"mantica issue get <id> --output json",
			"mantica issue comment list <id> --output json",
			"mantica issue status <id> in_dev",
			"mantica issue status <id> done",
			"mantica issue comment add <id> --content \"...\"",
		},
		OutputFormat: "Post execution summary with results from all subtasks.",
		TypicalSkills: []string{"orchestration", "task-management"},
	},
	"dev": {
		Name:           "Developer",
		Responsibilities: "Implement code changes, fix bugs, and deliver working software.",
		EssentialCommands: []string{
			"mantica issue get <id> --output json",
			"mantica issue status <id> in_dev",
			"mantica issue status <id> done",
			"mantica repo checkout <url>",
			"mantica issue comment add <id> --content \"...\"",
		},
		OutputFormat: "Post implementation summary with commit hash and any notes.",
		TypicalSkills: []string{"development", "testing"},
	},
	"patrol": {
		Name:           "Patrol",
		Responsibilities: "Monitor and report on workspace health, stale tasks, and issues.",
		EssentialCommands: []string{
			"mantica issue list --status in_dev --output json",
			"mantica issue get <id> --output json",
			"mantica issue comment add <id> --content \"...\"",
		},
		OutputFormat: "Post summary of findings and any issues that need attention.",
		TypicalSkills: []string{"monitoring", "reporting"},
	},
}

// GetRoleTemplate returns the role prompt config for the given agent name.
// Returns nil if no role prompt is defined for the agent.
func GetRoleTemplate(agentName string) *RolePromptConfig {
	if cfg, ok := RolePrompts[agentName]; ok {
		return &cfg
	}
	return nil
}

// IsRoleSupported returns true if the given agent name has a role prompt config.
func IsRoleSupported(agentName string) bool {
	_, ok := RolePrompts[agentName]
	return ok
}

// buildRoleSpecificContent generates role-specific condensed content.
func buildRoleSpecificContent(template *RolePromptConfig, provider string, ctx TaskContextForEnv) string {
	var b strings.Builder
	
	b.WriteString(fmt.Sprintf("# %s - Task Context\n\n", template.Name))
	b.WriteString(fmt.Sprintf("Provider: %s\n", provider))
	if ctx.IssueID != "" {
		b.WriteString(fmt.Sprintf("Issue: %s\n", ctx.IssueID))
	}
	b.WriteString(fmt.Sprintf("\n## Responsibilities\n%s\n", template.Responsibilities))
	b.WriteString(fmt.Sprintf("\n## Output Format\n%s\n", template.OutputFormat))
	
	if len(template.EssentialCommands) > 0 {
		b.WriteString("\n## Essential Commands\n")
		for _, cmd := range template.EssentialCommands {
			b.WriteString(fmt.Sprintf("- %s\n", cmd))
		}
	}
	
	return b.String()
}

// buildGenericCondensedContent generates generic condensed content when no role matches.
func buildGenericCondensedContent(provider string, ctx TaskContextForEnv) string {
	var b strings.Builder
	
	b.WriteString(fmt.Sprintf("# Task Context\n\n"))
	b.WriteString(fmt.Sprintf("Provider: %s\n", provider))
	if ctx.IssueID != "" {
		b.WriteString(fmt.Sprintf("Issue: %s\n", ctx.IssueID))
	}
	if ctx.AgentName != "" {
		b.WriteString(fmt.Sprintf("Agent: %s\n", ctx.AgentName))
	}
	
	return b.String()
}
