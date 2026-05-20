package daemon

import (
	"net/http"
	"os"
	"strings"
	"testing"
)

func TestNormalizeServerBaseURL(t *testing.T) {
	t.Parallel()

	got, err := NormalizeServerBaseURL("ws://localhost:8080/ws")
	if err != nil {
		t.Fatalf("NormalizeServerBaseURL returned error: %v", err)
	}
	if got != "http://localhost:8080" {
		t.Fatalf("expected http://localhost:8080, got %s", got)
	}
}

func TestBuildPromptContainsIssueID(t *testing.T) {
	t.Parallel()

	issueID := "a1b2c3d4-e5f6-7890-abcd-ef1234567890"
	prompt := BuildPrompt(Task{
		IssueID: issueID,
		Agent: &AgentData{
			Name: "Local Codex",
			Skills: []SkillData{
				{Name: "Concise", Content: "Be concise."},
			},
		},
	})

	// Prompt should contain the issue ID and CLI hint.
	for _, want := range []string{
		issueID,
		"mantica issue get",
	} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("prompt missing %q", want)
		}
	}

	// Skills should NOT be inlined in the prompt (they're in runtime config).
	for _, absent := range []string{"## Agent Skills", "Be concise."} {
		if strings.Contains(prompt, absent) {
			t.Fatalf("prompt should NOT contain %q (skills are in runtime config)", absent)
		}
	}
}

func TestBuildPromptNoStatusTransition(t *testing.T) {
	t.Parallel()

	// Pipeline handles status transitions now — prompt should never contain
	// status transition commands.
	prompt := BuildPrompt(Task{
		IssueID: "test-issue-123",
	})
	if strings.Contains(prompt, "mantica issue status") {
		t.Fatalf("prompt should NOT contain status command, got:\n%s", prompt)
	}
	if strings.Contains(prompt, "**FIRST**") {
		t.Fatalf("prompt should NOT contain FIRST step, got:\n%s", prompt)
	}
}

func TestBuildPromptNoIssueDetails(t *testing.T) {
	t.Parallel()

	prompt := BuildPrompt(Task{
		IssueID: "test-id",
		Agent:   &AgentData{Name: "Test"},
	})

	// Prompt should not contain issue title/description (agent fetches via CLI).
	for _, absent := range []string{"**Issue:**", "**Summary:**"} {
		if strings.Contains(prompt, absent) {
			t.Fatalf("prompt should NOT contain %q — agent fetches details via CLI", absent)
		}
	}
}

func TestIsWorkspaceNotFoundError(t *testing.T) {
	t.Parallel()

	err := &requestError{
		Method:     http.MethodPost,
		Path:       "/api/daemon/register",
		StatusCode: http.StatusNotFound,
		Body:       `{"error":"workspace not found"}`,
	}
	if !isWorkspaceNotFoundError(err) {
		t.Fatal("expected workspace not found error to be recognized")
	}

	if isWorkspaceNotFoundError(&requestError{StatusCode: http.StatusInternalServerError, Body: `{"error":"workspace not found"}`}) {
		t.Fatal("did not expect 500 to be treated as workspace not found")
	}
}

// TestLoadConfigClaudeNamedEnvVars verifies that MANTICA_CLAUDE_API_KEY and
// MANTICA_CLAUDE_BASE_URL are forwarded to the claude AgentEntry.Env as
// ANTHROPIC_API_KEY and ANTHROPIC_BASE_URL respectively.
func TestLoadConfigClaudeNamedEnvVars(t *testing.T) {
	t.Setenv("MANTICA_CLAUDE_API_KEY", "  sk-test-key  ")
	t.Setenv("MANTICA_CLAUDE_BASE_URL", "  https://custom.endpoint.example.com  ")
	t.Setenv("MANTICA_SERVER_URL", "http://localhost:8080")
	// Ensure claude is found on PATH (the test binary itself will do).
	t.Setenv("MANTICA_CLAUDE_PATH", findSelfBinary(t))

	cfg, err := LoadConfig(Overrides{})
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}

	entry, ok := cfg.Agents["claude"]
	if !ok {
		t.Fatal("expected claude agent entry")
	}
	if entry.Env["ANTHROPIC_API_KEY"] != "sk-test-key" {
		t.Errorf("ANTHROPIC_API_KEY: got %q, want %q", entry.Env["ANTHROPIC_API_KEY"], "sk-test-key")
	}
	if entry.Env["ANTHROPIC_BASE_URL"] != "https://custom.endpoint.example.com" {
		t.Errorf("ANTHROPIC_BASE_URL: got %q, want %q", entry.Env["ANTHROPIC_BASE_URL"], "https://custom.endpoint.example.com")
	}
}

// TestLoadConfigClaudeGenericEnvPassthrough verifies that any env var prefixed
// with MANTICA_CLAUDE_ENV_ is forwarded verbatim to the claude AgentEntry.Env.
// This supports Vertex AI mode and any future auth scheme without code changes.
func TestLoadConfigClaudeGenericEnvPassthrough(t *testing.T) {
	t.Setenv("MANTICA_CLAUDE_ENV_ANTHROPIC_AUTH_TOKEN", "sk-vertex-token")
	t.Setenv("MANTICA_CLAUDE_ENV_ANTHROPIC_VERTEX_BASE_URL", "https://gateway.example.ai/api")
	t.Setenv("MANTICA_CLAUDE_ENV_ANTHROPIC_VERTEX_PROJECT_ID", "my-project")
	t.Setenv("MANTICA_CLAUDE_ENV_CLAUDE_CODE_USE_VERTEX", "1")
	t.Setenv("MANTICA_CLAUDE_ENV_CLAUDE_CODE_SKIP_VERTEX_AUTH", "1")
	t.Setenv("MANTICA_SERVER_URL", "http://localhost:8080")
	t.Setenv("MANTICA_CLAUDE_PATH", findSelfBinary(t))

	cfg, err := LoadConfig(Overrides{})
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}

	entry, ok := cfg.Agents["claude"]
	if !ok {
		t.Fatal("expected claude agent entry")
	}

	wantEnv := map[string]string{
		"ANTHROPIC_AUTH_TOKEN":          "sk-vertex-token",
		"ANTHROPIC_VERTEX_BASE_URL":     "https://gateway.example.ai/api",
		"ANTHROPIC_VERTEX_PROJECT_ID":   "my-project",
		"CLAUDE_CODE_USE_VERTEX":        "1",
		"CLAUDE_CODE_SKIP_VERTEX_AUTH":  "1",
	}
	for k, want := range wantEnv {
		if got := entry.Env[k]; got != want {
			t.Errorf("env[%s]: got %q, want %q", k, got, want)
		}
	}
}

// TestLoadConfigClaudeEnvPrefixEmptyKey ensures a MANTICA_CLAUDE_ENV_ var with
// no suffix (i.e. target key is empty) is silently ignored.
func TestLoadConfigClaudeEnvPrefixEmptyKey(t *testing.T) {
	t.Setenv("MANTICA_CLAUDE_ENV_", "should-be-ignored")
	t.Setenv("MANTICA_SERVER_URL", "http://localhost:8080")
	t.Setenv("MANTICA_CLAUDE_PATH", findSelfBinary(t))

	cfg, err := LoadConfig(Overrides{})
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}

	entry, ok := cfg.Agents["claude"]
	if !ok {
		t.Fatal("expected claude agent entry")
	}
	if _, found := entry.Env[""]; found {
		t.Error("empty-key env var should have been ignored")
	}
}

// findSelfBinary returns the path to the running test binary, which is used as
// a stand-in for the claude CLI so exec.LookPath succeeds in tests.
func findSelfBinary(t *testing.T) string {
	t.Helper()
	self, err := os.Executable()
	if err != nil {
		t.Fatalf("os.Executable: %v", err)
	}
	return self
}
