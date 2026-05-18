package agent

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"os/exec"
	"regexp"
	"strings"
	"time"
)

// hermesBackend implements Backend by spawning `hermes chat -q <prompt> -Q`
// and collecting plain text output. Hermes does not support stream-json, so
// a single MessageText is emitted with the full output when done.
type hermesBackend struct {
	cfg Config
}

// sessionIDRe matches the "session_id: <id>" line emitted by hermes at the end of output.
var sessionIDRe = regexp.MustCompile(`(?m)^session_id:\s*(.+)$`)

func (b *hermesBackend) Execute(ctx context.Context, prompt string, opts ExecOptions) (*Session, error) {
	execPath := b.cfg.ExecutablePath
	if execPath == "" {
		execPath = "hermes"
	}
	if _, err := exec.LookPath(execPath); err != nil {
		return nil, fmt.Errorf("hermes executable not found at %q: %w", execPath, err)
	}

	timeout := opts.Timeout
	if timeout == 0 {
		timeout = 20 * time.Minute
	}
	runCtx, cancel := context.WithTimeout(ctx, timeout)

	baseArgs := append([]string{}, b.cfg.ExtraArgs...)
	args := append(baseArgs, "chat", "-q", prompt, "-Q")
	if opts.Model != "" {
		args = append(args, "--model", opts.Model)
	}
	if opts.MaxTurns > 0 {
		args = append(args, "--max-turns", fmt.Sprintf("%d", opts.MaxTurns))
	}
	if opts.ResumeSessionID != "" {
		args = append(args, "--resume", opts.ResumeSessionID)
	}

	cmd := exec.CommandContext(runCtx, execPath, args...)
	if opts.Cwd != "" {
		cmd.Dir = opts.Cwd
	}
	cmd.Env = buildEnv(b.cfg.Env)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		cancel()
		return nil, fmt.Errorf("hermes stdout pipe: %w", err)
	}

	// Capture stderr so we can extract session_id from it.
	// Hermes prints "session_id: <id>" to stderr (not stdout).
	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		cancel()
		return nil, fmt.Errorf("hermes stderr pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		cancel()
		return nil, fmt.Errorf("start hermes: %w", err)
	}

	b.cfg.Logger.Info("hermes started", "pid", cmd.Process.Pid, "cwd", opts.Cwd, "model", opts.Model)

	// Read stderr in background to extract session_id and log errors.
	var stderrBuf bytes.Buffer
	stderrDone := make(chan struct{})
	go func() {
		defer close(stderrDone)
		scanner := bufio.NewScanner(stderrPipe)
		scanner.Buffer(make([]byte, 0, 1024*1024), 10*1024*1024)
		for scanner.Scan() {
			line := scanner.Text()
			stderrBuf.WriteString(line)
			stderrBuf.WriteByte('\n')
			b.cfg.Logger.Debug("[hermes:stderr] " + line)
		}
	}()

	msgCh := make(chan Message, 256)
	resCh := make(chan Result, 1)

	go func() {
		defer cancel()
		defer close(msgCh)
		defer close(resCh)

		startTime := time.Now()
		output, sessionID := b.readOutput(stdout)

		exitErr := cmd.Wait()
		<-stderrDone // wait for stderr reader to finish
		duration := time.Since(startTime)

		status := "completed"
		var errMsg string

		if runCtx.Err() == context.DeadlineExceeded {
			status = "timeout"
			errMsg = fmt.Sprintf("hermes timed out after %s", timeout)
		} else if runCtx.Err() == context.Canceled {
			status = "aborted"
			errMsg = "execution cancelled"
		} else if exitErr != nil {
			status = "failed"
			errMsg = fmt.Sprintf("hermes exited with error: %v", exitErr)
		}

		// If session_id was not found in stdout, try extracting from stderr.
		// Hermes prints "session_id: <id>" to stderr in quiet mode.
		if sessionID == "" {
			sessionID = extractSessionID(stderrBuf.String())
		}

		// Emit the full output as a single text message.
		if output != "" {
			trySend(msgCh, Message{Type: MessageText, Content: output})
		}

		b.cfg.Logger.Info("hermes finished",
			"pid", cmd.Process.Pid,
			"status", status,
			"duration", duration.Round(time.Millisecond).String(),
			"session_id", sessionID,
		)

		resCh <- Result{
			Status:     status,
			Output:     output,
			Error:      errMsg,
			DurationMs: duration.Milliseconds(),
			SessionID:  sessionID,
		}
	}()

	return &Session{Messages: msgCh, Result: resCh}, nil
}

// readOutput reads all stdout lines, extracts the session_id if present,
// and returns the text output (without the session_id line).
func (b *hermesBackend) readOutput(r io.Reader) (output string, sessionID string) {
	var lines []string
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, 1024*1024), 10*1024*1024)

	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	if scanErr := scanner.Err(); scanErr != nil {
		b.cfg.Logger.Warn("hermes stdout scanner error", "error", scanErr)
	}

	// Extract session_id from the last few lines and remove that line from output.
	var outputLines []string
	for _, line := range lines {
		if m := sessionIDRe.FindStringSubmatch(line); m != nil {
			sessionID = strings.TrimSpace(m[1])
		} else {
			outputLines = append(outputLines, line)
		}
	}

	output = strings.Join(outputLines, "\n")
	return output, sessionID
}

// extractSessionID scans text for a "session_id: <id>" line and returns the ID.
func extractSessionID(text string) string {
	for _, line := range strings.Split(text, "\n") {
		if m := sessionIDRe.FindStringSubmatch(line); m != nil {
			return strings.TrimSpace(m[1])
		}
	}
	return ""
}
