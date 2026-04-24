package daemon

import (
	"fmt"
	"strings"
)

// BuildPrompt constructs the task prompt for an agent CLI.
// Keep this minimal — detailed instructions live in agent instructions
// injected by execenv.InjectRuntimeConfig.
func BuildPrompt(task Task) string {
	if task.ScheduledTaskID != "" {
		return buildScheduledPrompt(task)
	}
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
	fmt.Fprintf(&b, "Run `multica issue get %s --output json` to understand your task and complete it.\n", task.IssueID)
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

// buildScheduledPrompt constructs a prompt for scheduled (cron) tasks.
func buildScheduledPrompt(task Task) string {
	var b strings.Builder
	if task.Agent != nil && task.Agent.Name != "" {
		fmt.Fprintf(&b, "You are %s, an AI agent running a scheduled task in a Multica workspace.\n\n", task.Agent.Name)
	} else {
		b.WriteString("You are an AI agent running a scheduled task in a Multica workspace.\n\n")
	}
	b.WriteString(task.ScheduledPrompt)
	b.WriteString("\n")
	return b.String()
}
