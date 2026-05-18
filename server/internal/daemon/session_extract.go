package daemon

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// extractSessionToolMessages reads a Hermes session file (JSONL or JSON)
// and returns structured tool_use/tool_result messages suitable for ReportTaskMessages.
//
// This bridges the gap between Hermes's quiet mode (which only outputs final
// text) and Multica's task_message table (which needs per-tool-call data).
// The session file contains full assistant(tool_calls) + tool(result) pairs.
//
// Supported formats:
//   - JSONL: one JSON object per line (local CLI ~/.hermes/sessions/*.jsonl)
//   - JSON: single object with "messages" array (daemon ~/.hermes/sessions/session_*.json)
func extractSessionToolMessages(sessionFilePath string) []TaskMessageData {
	data, err := os.ReadFile(sessionFilePath)
	if err != nil {
		return nil
	}

	// Detect format: try parsing as single JSON first
	var entries []json.RawMessage
	var singleObj struct {
		Messages []json.RawMessage `json:"messages"`
	}
	if err := json.Unmarshal(data, &singleObj); err == nil && len(singleObj.Messages) > 0 {
		// Single JSON format with messages array
		entries = singleObj.Messages
	} else {
		// JSONL format: one JSON per line
		scanner := bufio.NewScanner(bytes.NewReader(data))
		scanner.Buffer(make([]byte, 0, 2*1024*1024), 20*1024*1024)
		for scanner.Scan() {
			line := scanner.Bytes()
			if len(line) > 0 {
				entries = append(entries, json.RawMessage(line))
			}
		}
	}

	return parseToolMessages(entries)
}

// parseToolMessages extracts tool_use and tool_result pairs from session entries.
func parseToolMessages(entries []json.RawMessage) []TaskMessageData {
	var messages []TaskMessageData
	pendingToolUses := map[string]string{} // call_id -> tool_name
	seq := 1000 // start high to avoid collision with daemon's live messages

	for _, raw := range entries {
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

		if err := json.Unmarshal(raw, &entry); err != nil {
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

				messages = append(messages, TaskMessageData{
					Seq:   seq,
					Type:  "tool_use",
					Tool:  toolName,
					Input: inputJSON,
				})
				seq++
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

			messages = append(messages, TaskMessageData{
				Seq:    seq,
				Type:   "tool_result",
				Tool:   toolName,
				Output: output,
			})
			seq++
		}
	}

	return messages
}

// findSessionFile locates the session JSONL file for a given session_id.
// It searches in ~/.hermes/sessions/ for files matching the session_id pattern.
// Hermes stores sessions as either:
//   - {session_id}.jsonl  (local CLI)
//   - session_{session_id}.json  (daemon/remote)
func findSessionFile(sessionID string) string {
	if sessionID == "" {
		return ""
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}

	sessionsDir := filepath.Join(home, ".hermes", "sessions")

	// Try all known naming patterns
	candidates := []string{
		filepath.Join(sessionsDir, sessionID+".jsonl"),
		filepath.Join(sessionsDir, sessionID+".json"),
		filepath.Join(sessionsDir, "session_"+sessionID+".jsonl"),
		filepath.Join(sessionsDir, "session_"+sessionID+".json"),
	}

	for _, path := range candidates {
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}

	// Fuzzy match: find files containing the session_id
	entries, err := os.ReadDir(sessionsDir)
	if err != nil {
		return ""
	}

	var matches []string
	for _, e := range entries {
		if strings.Contains(e.Name(), sessionID) {
			matches = append(matches, filepath.Join(sessionsDir, e.Name()))
		}
	}

	if len(matches) == 0 {
		return ""
	}

	// Return the most recently modified
	sort.Slice(matches, func(i, j int) bool {
		si, _ := os.Stat(matches[i])
		sj, _ := os.Stat(matches[j])
		return si.ModTime().After(sj.ModTime())
	})

	return matches[0]
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
