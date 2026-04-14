package daemon

import (
	"fmt"
	"strings"
)

// statusTransition maps current issue status to the "in progress" status
// the agent should immediately set when picking up the task.
var statusTransition = map[string]string{
	"backlog":           "classifying",
	"classifying":       "classifying",
	"ready_analyze":     "in_analyze",
	"ready_arch_design": "in_arch_design",
	"ready_dev":         "in_dev",
	"ready_review":      "in_review",
	"ready_test":        "in_test",
}

// BuildPrompt constructs the task prompt for an agent CLI.
// Keep this minimal — detailed instructions live in CLAUDE.md / AGENTS.md
// injected by execenv.InjectRuntimeConfig.
func BuildPrompt(task Task) string {
	if task.ChatSessionID != "" {
		return buildChatPrompt(task)
	}
	var b strings.Builder
	if task.Agent != nil && task.Agent.Name != "" {
		fmt.Fprintf(&b, "You are %s, an AI agent working in a Multica workspace.\n\n", task.Agent.Name)
	} else {
		b.WriteString("You are an AI agent working in a Multica workspace.\n\n")
	}
	fmt.Fprintf(&b, "Your assigned issue ID is: %s\n\n", task.IssueID)

	// If we know the current issue status, give explicit first-step instructions.
	if inProgress, ok := statusTransition[task.IssueStatus]; ok {
		fmt.Fprintf(&b, "**FIRST**: Run `multica issue status %s %s` to mark the issue as in-progress before doing any other work.\n\n", task.IssueID, inProgress)
	}

	fmt.Fprintf(&b, "Then run `multica issue get %s --output json` to understand your task and complete it.\n", task.IssueID)
	return b.String()
}

// buildChatPrompt constructs a prompt for interactive chat tasks.
func buildChatPrompt(task Task) string {
	var b strings.Builder
	b.WriteString("You are running as a chat assistant for a Multica workspace.\n")
	b.WriteString("A user is chatting with you directly. Respond to their message.\n\n")
	fmt.Fprintf(&b, "User message:\n%s\n", task.ChatMessage)
	return b.String()
}
