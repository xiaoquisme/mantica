package agent

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestNewReturnsHermesBackend(t *testing.T) {
	t.Parallel()
	b, err := New("hermes", Config{ExecutablePath: "/nonexistent/hermes"})
	if err != nil {
		t.Fatalf("New(hermes) error: %v", err)
	}
	if _, ok := b.(*hermesBackend); !ok {
		t.Fatalf("expected *hermesBackend, got %T", b)
	}
}

// ── readOutput tests ──

func TestHermesReadOutputHappyPath(t *testing.T) {
	t.Parallel()

	b := &hermesBackend{cfg: Config{Logger: slog.Default()}}

	input := strings.Join([]string{
		"Hello, I've analyzed the code.",
		"Here are my findings.",
		"session_id: ses_abc123",
	}, "\n")

	output, sessionID := b.readOutput(strings.NewReader(input))

	if sessionID != "ses_abc123" {
		t.Errorf("sessionID: got %q, want %q", sessionID, "ses_abc123")
	}
	wantOutput := "Hello, I've analyzed the code.\nHere are my findings."
	if output != wantOutput {
		t.Errorf("output: got %q, want %q", output, wantOutput)
	}
}

func TestHermesReadOutputNoSessionID(t *testing.T) {
	t.Parallel()

	b := &hermesBackend{cfg: Config{Logger: slog.Default()}}

	input := "Just some plain text output.\nNo session line here."

	output, sessionID := b.readOutput(strings.NewReader(input))

	if sessionID != "" {
		t.Errorf("sessionID: got %q, want empty", sessionID)
	}
	if output != "Just some plain text output.\nNo session line here." {
		t.Errorf("output: got %q", output)
	}
}

func TestHermesReadOutputEmptyInput(t *testing.T) {
	t.Parallel()

	b := &hermesBackend{cfg: Config{Logger: slog.Default()}}

	output, sessionID := b.readOutput(strings.NewReader(""))

	if sessionID != "" {
		t.Errorf("sessionID: got %q, want empty", sessionID)
	}
	if output != "" {
		t.Errorf("output: got %q, want empty", output)
	}
}

func TestHermesReadOutputOnlySessionID(t *testing.T) {
	t.Parallel()

	b := &hermesBackend{cfg: Config{Logger: slog.Default()}}

	input := "session_id: ses_only"

	output, sessionID := b.readOutput(strings.NewReader(input))

	if sessionID != "ses_only" {
		t.Errorf("sessionID: got %q, want %q", sessionID, "ses_only")
	}
	if output != "" {
		t.Errorf("output: got %q, want empty", output)
	}
}

func TestHermesReadOutputSessionIDWithSpaces(t *testing.T) {
	t.Parallel()

	b := &hermesBackend{cfg: Config{Logger: slog.Default()}}

	input := "result text\nsession_id:   ses_spaces   "

	output, sessionID := b.readOutput(strings.NewReader(input))

	if sessionID != "ses_spaces" {
		t.Errorf("sessionID: got %q, want %q", sessionID, "ses_spaces")
	}
	if output != "result text" {
		t.Errorf("output: got %q, want %q", output, "result text")
	}
}

func TestHermesReadOutputMultilineWithEmptyLines(t *testing.T) {
	t.Parallel()

	b := &hermesBackend{cfg: Config{Logger: slog.Default()}}

	input := strings.Join([]string{
		"Line 1",
		"",
		"Line 3",
		"session_id: ses_multi",
	}, "\n")

	output, sessionID := b.readOutput(strings.NewReader(input))

	if sessionID != "ses_multi" {
		t.Errorf("sessionID: got %q, want %q", sessionID, "ses_multi")
	}
	wantOutput := "Line 1\n\nLine 3"
	if output != wantOutput {
		t.Errorf("output: got %q, want %q", output, wantOutput)
	}
}

func TestHermesReadOutputScannerError(t *testing.T) {
	t.Parallel()

	b := &hermesBackend{cfg: Config{Logger: slog.Default()}}

	output, sessionID := b.readOutput(&ioErrReader{
		data: "before error\nsession_id: ses_err\n",
	})

	// The reader delivers data then errors; we should get what was read.
	if sessionID != "ses_err" {
		t.Errorf("sessionID: got %q, want %q", sessionID, "ses_err")
	}
	if output != "before error" {
		t.Errorf("output: got %q, want %q", output, "before error")
	}
}

// ── ExtraArgs test ──

func TestHermesExtraArgsPassedToCommand(t *testing.T) {
	t.Parallel()

	// Create a fake hermes script that prints all args, one per line.
	dir := t.TempDir()
	fakeBin := filepath.Join(dir, "hermes")
	if err := os.WriteFile(fakeBin, []byte("#!/bin/sh\nfor a in \"$@\"; do echo \"$a\"; done\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	b := &hermesBackend{cfg: Config{
		ExecutablePath: fakeBin,
		ExtraArgs:      []string{"--profile", "cli"},
		Logger:         slog.Default(),
	}}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	sess, err := b.Execute(ctx, "hello", ExecOptions{})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}

	// Drain messages.
	for range sess.Messages {
	}
	res := <-sess.Result

	if res.Status != "completed" {
		t.Fatalf("status: got %q, want completed (err: %s)", res.Status, res.Error)
	}

	// The fake script echoes each arg on its own line.
	args := strings.Split(strings.TrimSpace(res.Output), "\n")

	// Expect: --profile cli chat -q hello -Q
	if len(args) < 6 {
		t.Fatalf("expected at least 6 args, got %d: %v", len(args), args)
	}
	if args[0] != "--profile" || args[1] != "cli" {
		t.Errorf("first two args should be --profile cli, got %v", args[:2])
	}
	if args[2] != "chat" {
		t.Errorf("args[2] should be chat, got %q", args[2])
	}
}

// ── sessionIDRe tests ──

func TestSessionIDRegex(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		line    string
		wantID  string
		wantOK  bool
	}{
		{"standard", "session_id: abc123", "abc123", true},
		{"with spaces", "session_id:   xyz  ", "xyz", true},
		{"uuid style", "session_id: 550e8400-e29b-41d4-a716-446655440000", "550e8400-e29b-41d4-a716-446655440000", true},
		{"no match", "some other line", "", false},
		{"partial match", "my session_id: nope", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			m := sessionIDRe.FindStringSubmatch(tt.line)
			if tt.wantOK {
				if m == nil {
					t.Fatalf("expected match for %q", tt.line)
				}
				got := strings.TrimSpace(m[1])
				if got != tt.wantID {
					t.Errorf("got %q, want %q", got, tt.wantID)
				}
			} else {
				if m != nil {
					t.Errorf("expected no match for %q, got %v", tt.line, m)
				}
			}
		})
	}
}
