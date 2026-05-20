package daemon

import (
	"context"
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
		fmt.Fprintf(&b, "You are %s, an AI agent working in a Mantica workspace.\n\n", task.Agent.Name)
	} else {
		b.WriteString("You are an AI agent working in a Mantica workspace.\n\n")
	}
	fmt.Fprintf(&b, "Your assigned issue ID is: %s\n\n", task.IssueID)
	fmt.Fprintf(&b, "Run `mantica issue get %s --output json` to understand your task and complete it.\n", task.IssueID)
	return b.String()
}

// BuildPromptWithHints constructs a task prompt and appends improvement hints
// from the agent's recent failure history. This closes the feedback loop:
// failures → analysis → hints → better prompts → better outcomes.
func BuildPromptWithHints(task Task, client *Client) string {
	prompt := BuildPrompt(task)

	if task.Agent == nil || task.AgentID == "" {
		return prompt
	}

	hints, err := client.GetAgentHints(context.Background(), task.AgentID)
	if err != nil || len(hints) == 0 {
		return prompt
	}

	var b strings.Builder
	b.WriteString(prompt)
	b.WriteString("\n---\n")
	b.WriteString("## Lessons from Recent Tasks\n\n")
	b.WriteString("Based on your recent task history, here are patterns to avoid:\n\n")

	for i, h := range hints {
		fmt.Fprintf(&b, "%d. **%s** (seen %d times): %s\n",
			i+1, h.FailureClass, h.OccurrenceCount, h.ImprovementHint)
	}

	b.WriteString("\nApply these lessons to avoid repeating the same mistakes.\n")

	return b.String()
}

// buildChatPrompt constructs a prompt for interactive chat tasks.
func buildChatPrompt(task Task) string {
	var b strings.Builder
	b.WriteString("You are running as a chat assistant for a Mantica workspace.\n")
	b.WriteString("A user is chatting with you directly. Respond to their message.\n\n")
	fmt.Fprintf(&b, "User message:\n%s\n", task.ChatMessage)
	return b.String()
}

// buildScheduledPrompt constructs a prompt for scheduled (cron) tasks.
func buildScheduledPrompt(task Task) string {
	var b strings.Builder
	if task.Agent != nil && task.Agent.Name != "" {
		fmt.Fprintf(&b, "You are %s, an AI agent running a scheduled task in a Mantica workspace.\n\n", task.Agent.Name)
	} else {
		b.WriteString("You are an AI agent running a scheduled task in a Mantica workspace.\n\n")
	}
	b.WriteString("Run the task described in your instructions.\n")
	return b.String()
}
