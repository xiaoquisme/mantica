package execenv

import (
	"fmt"
	"strings"
)

// RolePromptTemplate defines a condensed prompt template for a specific agent role.
type RolePromptTemplate struct {
	// Role name (e.g. "Classifier", "BA", "TL", "DEV", "QA")
	Name string
	
	// Core responsibilities (1-2 lines)
	Responsibilities string
	
	// Essential commands for this role
	EssentialCommands []string
	
	// Output format expected from this role
	OutputFormat string
	
	// Skills typically used by this role
	TypicalSkills []string
}

// roleTemplates maps agent role names to their condensed prompt templates.
var roleTemplates = map[string]RolePromptTemplate{
	"classifier": {
		Name:           "Classifier",
		Responsibilities: "Classify issues and route to the correct next stage. You do NOT implement or analyze.",
		EssentialCommands: []string{
			"mantica issue get <id> --output json",
			"mantica issue status <id> <status>",
			"mantica issue comment add <id> --content \"...\"",
		},
		OutputFormat: "Post a comment with the classification result and route to the next stage.",
		TypicalSkills: []string{},
	},
	"ba": {
		Name:           "BA (Business Analyst)",
		Responsibilities: "Analyze requirements, create specifications, and define acceptance criteria.",
		EssentialCommands: []string{
			"mantica issue get <id> --output json",
			"mantica issue comment list <id> --output json",
			"mantica issue status <id> in_progress",
			"mantica issue status <id> in_review",
			"mantica issue comment add <id> --content \"...\"",
		},
		OutputFormat: "Post detailed analysis, requirements, and acceptance criteria as comments.",
		TypicalSkills: []string{"research", "documentation"},
	},
	"tl": {
		Name:           "TL (Tech Lead)",
		Responsibilities: "Design technical approach, review architecture, and guide implementation.",
		EssentialCommands: []string{
			"mantica issue get <id> --output json",
			"mantica issue comment list <id> --output json",
			"mantica issue status <id> in_progress",
			"mantica issue status <id> in_review",
			"mantica repo checkout <url>",
			"mantica issue comment add <id> --content \"...\"",
		},
		OutputFormat: "Post technical design, architecture decisions, and implementation plan.",
		TypicalSkills: []string{"architecture", "code-review"},
	},
	"dev": {
		Name:           "DEV (Developer)",
		Responsibilities: "Implement code changes, fix bugs, and deliver working software.",
		EssentialCommands: []string{
			"mantica issue get <id> --output json",
			"mantica issue status <id> in_progress",
			"mantica issue status <id> in_review",
			"mantica repo checkout <url>",
			"mantica issue comment add <id> --content \"...\"",
		},
		OutputFormat: "Post implementation summary with commit hash and any notes.",
		TypicalSkills: []string{"development", "testing"},
	},
	"code-review": {
		Name:           "Code Review",
		Responsibilities: "Review code changes for quality, security, and adherence to standards.",
		EssentialCommands: []string{
			"mantica issue get <id> --output json",
			"mantica issue comment list <id> --output json",
			"mantica issue status <id> in_progress",
			"mantica repo checkout <url>",
			"mantica issue comment add <id> --content \"...\"",
		},
		OutputFormat: "Post review findings, approve or request changes.",
		TypicalSkills: []string{"code-review", "security"},
	},
	"qa": {
		Name:           "QA (Quality Assurance)",
		Responsibilities: "Verify implementation meets requirements and create test cases.",
		EssentialCommands: []string{
			"mantica issue get <id> --output json",
			"mantica issue comment list <id> --output json",
			"mantica issue status <id> in_progress",
			"mantica issue status <id> done",
			"mantica repo checkout <url>",
			"mantica issue comment add <id> --content \"...\"",
		},
		OutputFormat: "Post test results, pass/fail status, and any defects found.",
		TypicalSkills: []string{"testing", "test-case-writing"},
	},
	"patrol": {
		Name:           "Patrol",
		Responsibilities: "Monitor and report on workspace health, stale tasks, and issues.",
		EssentialCommands: []string{
			"mantica issue list --status in_progress --output json",
			"mantica issue get <id> --output json",
			"mantica issue comment add <id> --content \"...\"",
		},
		OutputFormat: "Post summary of findings and any issues that need attention.",
		TypicalSkills: []string{"monitoring", "reporting"},
	},
}

// GetRoleTemplate returns the prompt template for a given role name.
// Returns nil if no specific template is defined.
func GetRoleTemplate(roleName string) *RolePromptTemplate {
	// Normalize role name
	normalized := strings.ToLower(strings.TrimSpace(roleName))
	
	// Direct match
	if template, ok := roleTemplates[normalized]; ok {
		return &template
	}
	
	// Partial match
	for key, template := range roleTemplates {
		if strings.Contains(normalized, key) || strings.Contains(key, normalized) {
			return &template
		}
	}
	
	return nil
}

// buildRoleSpecificContent generates a condensed prompt for a specific role.
func buildRoleSpecificContent(template *RolePromptTemplate, provider string, ctx TaskContextForEnv) string {
	var b strings.Builder
	
	// Header
	b.WriteString(fmt.Sprintf("# %s Agent\n\n", template.Name))
	b.WriteString("You are working in a Mantica workspace.\n\n")
	
	// Role description
	b.WriteString("## Your Role\n\n")
	b.WriteString(template.Responsibilities)
	b.WriteString("\n\n")
	
	// Issue ID (critical info)
	b.WriteString("## Task\n\n")
	b.WriteString(fmt.Sprintf("Your assigned issue ID is: **%s**\n\n", ctx.IssueID))
	
	// Essential commands
	b.WriteString("## Commands\n\n")
	for _, cmd := range template.EssentialCommands {
		b.WriteString(fmt.Sprintf("- `%s`\n", cmd))
	}
	b.WriteString("\n")
	
	// Output format
	b.WriteString("## Output\n\n")
	b.WriteString(template.OutputFormat)
	b.WriteString("\n\n")
	
	// Skills (if any)
	if len(ctx.AgentSkills) > 0 {
		b.WriteString("## Skills\n\n")
		for _, skill := range ctx.AgentSkills {
			b.WriteString(fmt.Sprintf("- %s\n", skill.Name))
		}
		b.WriteString("\n")
	}
	
	// Memory reminder (brief)
	b.WriteString("## Memory\n\n")
	b.WriteString("Check `memory/MEMORY.md` for project conventions before starting.\n\n")
	
	return b.String()
}

// buildGenericCondensedContent generates a condensed prompt when no role template matches.
func buildGenericCondensedContent(provider string, ctx TaskContextForEnv) string {
	var b strings.Builder
	
	b.WriteString("# Mantica Agent\n\n")
	b.WriteString("You are working in a Mantica workspace.\n\n")
	
	b.WriteString("## Task\n\n")
	b.WriteString(fmt.Sprintf("Your assigned issue ID is: **%s**\n\n", ctx.IssueID))
	b.WriteString("Run `mantica issue get <id> --output json` to understand your task.\n\n")
	
	b.WriteString("## Essential Commands\n\n")
	b.WriteString("- `mantica issue get <id> --output json` — Get issue details\n")
	b.WriteString("- `mantica issue status <id> <status>` — Update status\n")
	b.WriteString("- `mantica issue comment add <id> --content \"...\"` — Add comment\n")
	b.WriteString("- `mantica repo checkout <url>` — Check out repository\n\n")
	
	b.WriteString("## Workflow\n\n")
	b.WriteString("1. Understand task\n")
	b.WriteString("2. Do the work\n")
	b.WriteString("3. Update status\n")
	b.WriteString("4. Add comment\n\n")
	
	return b.String()
}
