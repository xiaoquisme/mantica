package daemon

import (
	"bufio"
	"context"
	"encoding/json"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// extractSessionToolMessages reads a Hermes session JSONL file and returns
// structured tool_use/tool_result messages suitable for ReportTaskMessages.
//
// This bridges the gap between Hermes's quiet mode (which only outputs final
// text) and Multica's task_message table (which needs per-tool-call data).
// The session JSONL contains full assistant(tool_calls) + tool(result) pairs.
func extractSessionToolMessages(sessionFilePath string) []TaskMessageData {
	f, err := os.Open(sessionFilePath)
	if err != nil {
		return nil
	}
	defer f.Close()

	var messages []TaskMessageData
	pendingToolUses := map[string]string{} // call_id -> tool_name
	seq := 1000 // start high to avoid collision with daemon's live messages

	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 2*1024*1024), 20*1024*1024)

	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}

		var entry struct {
			Role             string `json:"role"`
			Timestamp        string `json:"timestamp"`
			ToolCalls        []struct {
				CallID   string `json:"call_id"`
				ID       string `json:"id"`
				Function struct {
					Name      string `json:"name"`
					Arguments any    `json:"arguments"`
				} `json:"function"`
			} `json:"tool_calls"`
			ReasoningContent string `json:"reasoning_content"`
			Content          string `json:"content"`
			ToolCallID       string `json:"tool_call_id"`
			Name             string `json:"name"`
		}

		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			continue
		}

		switch entry.Role {
		case "assistant":
			for _, tc := range entry.ToolCalls {
				callID := tc.CallID
				if callID == "" {
					callID = tc.ID
				}
				toolName := tc.Function.Name
				if toolName == "" {
					toolName = "unknown"
				}

				// Parse arguments
				var inputJSON map[string]any
				switch args := tc.Function.Arguments.(type) {
				case string:
					json.Unmarshal([]byte(args), &inputJSON)
				case map[string]any:
					inputJSON = args
				}

				if callID != "" {
					pendingToolUses[callID] = toolName
				}

				s := seq
				seq++
				messages = append(messages, TaskMessageData{
					Seq:   int(s),
					Type:  "tool_use",
					Tool:  toolName,
					Input: inputJSON,
				})
			}

		case "tool":
			callID := entry.ToolCallID
			toolName := entry.Name
			output := entry.Content

			// Truncate very long output
			if len(output) > 8192 {
				output = output[:8192] + "...[truncated]"
			}

			// Resolve tool name from pending tool_uses if not set
			if toolName == "" && callID != "" {
				if name, ok := pendingToolUses[callID]; ok {
					toolName = name
				}
			}

			s := seq
			seq++
			messages = append(messages, TaskMessageData{
				Seq:    int(s),
				Type:   "tool_result",
				Tool:   toolName,
				Output: output,
			})
		}
	}

	return messages
}

// findSessionFile locates the session JSONL file for a given session_id.
// It searches in ~/.hermes/sessions/ for files matching the session_id pattern.
func findSessionFile(sessionID string) string {
	if sessionID == "" {
		return ""
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}

	sessionsDir := filepath.Join(home, ".hermes", "sessions")

	// Direct match: {session_id}.jsonl
	direct := filepath.Join(sessionsDir, sessionID+".jsonl")
	if _, err := os.Stat(direct); err == nil {
		return direct
	}

	// Fuzzy match: find files containing the session_id
	entries, err := os.ReadDir(sessionsDir)
	if err != nil {
		return ""
	}

	var candidates []string
	for _, e := range entries {
		if strings.Contains(e.Name(), sessionID) && strings.HasSuffix(e.Name(), ".jsonl") {
			candidates = append(candidates, filepath.Join(sessionsDir, e.Name()))
		}
	}

	if len(candidates) == 0 {
		return ""
	}

	// Return the most recently modified
	sort.Slice(candidates, func(i, j int) bool {
		si, _ := os.Stat(candidates[i])
		sj, _ := os.Stat(candidates[j])
		return si.ModTime().After(sj.ModTime())
	})

	return candidates[0]
}

// ExtractAndSendSessionMessages finds the session file for a completed task
// and sends the tool messages to the server via ReportTaskMessages.
// This is called after CompleteTask to backfill tool_use/tool_result data
// that Hermes's quiet mode doesn't stream in real-time.
func (d *Daemon) ExtractAndSendSessionMessages(taskID, sessionID string, taskLog *slog.Logger) {
	if sessionID == "" {
		return
	}

	sessionFile := findSessionFile(sessionID)
	if sessionFile == "" {
		taskLog.Debug("session file not found for backfill", "session_id", sessionID)
		return
	}

	messages := extractSessionToolMessages(sessionFile)
	if len(messages) == 0 {
		return
	}

	taskLog.Info("backfilling session tool messages",
		"session_id", sessionID,
		"message_count", len(messages),
		"session_file", sessionFile,
	)

	// Send in batches of 50
	batchSize := 50
	for i := 0; i < len(messages); i += batchSize {
		end := i + batchSize
		if end > len(messages) {
			end = len(messages)
		}
		batch := messages[i:end]

		if err := d.client.ReportTaskMessages(
			context.Background(),
			taskID,
			batch,
		); err != nil {
			taskLog.Warn("failed to backfill session messages", "error", err)
			return
		}
	}

	taskLog.Info("session tool messages backfilled", "count", len(messages))
}
