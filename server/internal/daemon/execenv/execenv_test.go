package execenv

import (
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func testLogger() *slog.Logger {
	return slog.Default()
}

func TestShortID(t *testing.T) {
	t.Parallel()
	tests := []struct {
		input, want string
	}{
		{"a1b2c3d4-e5f6-7890-abcd-ef1234567890", "a1b2c3d4"},
		{"abcdef12", "abcdef12"},
		{"ab", "ab"},
		{"a1b2c3d4e5f67890", "a1b2c3d4"},
	}
	for _, tt := range tests {
		if got := shortID(tt.input); got != tt.want {
			t.Errorf("shortID(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestSanitizeName(t *testing.T) {
	t.Parallel()
	tests := []struct {
		input, want string
	}{
		{"Code Reviewer", "code-reviewer"},
		{"my_agent!@#v2", "my-agent-v2"},
		{"  spaces  ", "spaces"},
		{"UPPERCASE", "uppercase"},
		{"a-very-long-name-that-exceeds-thirty-characters-total", "a-very-long-name-that-exceeds"},
		{"", "agent"},
		{"---", "agent"},
		{"日本語テスト", "agent"},
	}
	for _, tt := range tests {
		if got := sanitizeName(tt.input); got != tt.want {
			t.Errorf("sanitizeName(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestRepoNameFromURL(t *testing.T) {
	t.Parallel()
	tests := []struct {
		input, want string
	}{
		{"https://github.com/org/my-repo.git", "my-repo"},
		{"https://github.com/org/my-repo", "my-repo"},
		{"git@github.com:org/my-repo.git", "my-repo"},
		{"https://github.com/org/repo/", "repo"},
		{"my-repo", "my-repo"},
		{"", "repo"},
	}
	for _, tt := range tests {
		if got := repoNameFromURL(tt.input); got != tt.want {
			t.Errorf("repoNameFromURL(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestPrepareDirectoryMode(t *testing.T) {
	t.Parallel()
	workspacesRoot := t.TempDir()

	env, err := Prepare(PrepareParams{
		WorkspacesRoot: workspacesRoot,
		WorkspaceID:    "ws-test-001",
		TaskID:         "a1b2c3d4-e5f6-7890-abcd-ef1234567890",
		AgentName:      "Test Agent",
		Task: TaskContextForEnv{
			IssueID: "a1b2c3d4-e5f6-7890-abcd-ef1234567890",
			AgentSkills: []SkillContextForEnv{
				{Name: "Code Review", Content: "Be concise."},
			},
		},
	}, testLogger())
	if err != nil {
		t.Fatalf("Prepare failed: %v", err)
	}
	defer env.Cleanup(true)

	// Verify directory structure.
	for _, sub := range []string{"workdir", "output", "logs"} {
		path := filepath.Join(env.RootDir, sub)
		if _, err := os.Stat(path); os.IsNotExist(err) {
			t.Fatalf("expected %s to exist", path)
		}
	}

	// Verify context file contains issue ID and CLI hints.
	content, err := os.ReadFile(filepath.Join(env.WorkDir, ".agent_context", "issue_context.md"))
	if err != nil {
		t.Fatalf("failed to read issue_context.md: %v", err)
	}
	for _, want := range []string{"a1b2c3d4-e5f6-7890-abcd-ef1234567890", "Code Review"} {
		if !strings.Contains(string(content), want) {
			t.Fatalf("issue_context.md missing %q", want)
		}
	}

	// Verify skill files.
	skillContent, err := os.ReadFile(filepath.Join(env.WorkDir, ".agent_context", "skills", "code-review", "SKILL.md"))
	if err != nil {
		t.Fatalf("failed to read SKILL.md: %v", err)
	}
	if !strings.Contains(string(skillContent), "Be concise.") {
		t.Fatal("SKILL.md missing content")
	}
}

func TestPrepareWithRepoContext(t *testing.T) {
	t.Parallel()
	workspacesRoot := t.TempDir()

	taskCtx := TaskContextForEnv{
		IssueID: "b2c3d4e5-f6a7-8901-bcde-f12345678901",
		Repos: []RepoContextForEnv{
			{URL: "https://github.com/org/backend", Description: "Go backend"},
			{URL: "https://github.com/org/frontend", Description: "React frontend"},
		},
	}
	env, err := Prepare(PrepareParams{
		WorkspacesRoot: workspacesRoot,
		WorkspaceID:    "ws-test-002",
		TaskID:         "b2c3d4e5-f6a7-8901-bcde-f12345678901",
		AgentName:      "Code Reviewer",
		Provider:       "claude",
		Task:           taskCtx,
	}, testLogger())
	if err != nil {
		t.Fatalf("Prepare failed: %v", err)
	}
	defer env.Cleanup(true)

	// Inject runtime config (done separately in daemon, replicate here).
	if err := InjectRuntimeConfig(env.WorkDir, "claude", taskCtx); err != nil {
		t.Fatalf("InjectRuntimeConfig failed: %v", err)
	}

	// Workdir should be empty (no pre-created repo dirs).
	entries, err := os.ReadDir(env.WorkDir)
	if err != nil {
		t.Fatalf("failed to read workdir: %v", err)
	}
	for _, e := range entries {
		name := e.Name()
		if name != ".agent_context" && name != "CLAUDE.md" && name != ".claude" && name != "memory" {
			t.Errorf("unexpected entry in workdir: %s", name)
		}
	}

	// CLAUDE.md should contain repo info.
	content, err := os.ReadFile(filepath.Join(env.WorkDir, "CLAUDE.md"))
	if err != nil {
		t.Fatalf("failed to read CLAUDE.md: %v", err)
	}
	s := string(content)
	for _, want := range []string{
		"mantica repo checkout",
		"https://github.com/org/backend",
		"Go backend",
		"https://github.com/org/frontend",
		"React frontend",
	} {
		if !strings.Contains(s, want) {
			t.Errorf("CLAUDE.md missing %q", want)
		}
	}
}

func TestWriteContextFiles(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	ctx := TaskContextForEnv{
		IssueID: "test-issue-id-1234",
		AgentSkills: []SkillContextForEnv{
			{
				Name:    "Go Conventions",
				Content: "Follow Go conventions.",
				Files: []SkillFileContextForEnv{
					{Path: "templates/example.go", Content: "package main"},
				},
			},
		},
	}

	if err := writeContextFiles(dir, "", ctx); err != nil {
		t.Fatalf("writeContextFiles failed: %v", err)
	}

	content, err := os.ReadFile(filepath.Join(dir, ".agent_context", "issue_context.md"))
	if err != nil {
		t.Fatalf("failed to read: %v", err)
	}

	s := string(content)
	for _, want := range []string{
		"test-issue-id-1234",
		"## Agent Skills",
		"Go Conventions",
	} {
		if !strings.Contains(s, want) {
			t.Errorf("content missing %q", want)
		}
	}

	// Issue details should NOT be in the context file (agent fetches via CLI).
	for _, absent := range []string{"## Description", "## Workspace Context"} {
		if strings.Contains(s, absent) {
			t.Errorf("content should NOT contain %q — agent fetches details via CLI", absent)
		}
	}

	// Verify skill directory and files.
	skillMd, err := os.ReadFile(filepath.Join(dir, ".agent_context", "skills", "go-conventions", "SKILL.md"))
	if err != nil {
		t.Fatalf("failed to read SKILL.md: %v", err)
	}
	if !strings.Contains(string(skillMd), "Follow Go conventions.") {
		t.Error("SKILL.md missing content")
	}

	supportFile, err := os.ReadFile(filepath.Join(dir, ".agent_context", "skills", "go-conventions", "templates", "example.go"))
	if err != nil {
		t.Fatalf("failed to read supporting file: %v", err)
	}
	if string(supportFile) != "package main" {
		t.Errorf("supporting file content = %q, want %q", string(supportFile), "package main")
	}
}

func TestWriteContextFilesOmitsSkillsWhenEmpty(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	ctx := TaskContextForEnv{
		IssueID: "minimal-issue-id",
	}

	if err := writeContextFiles(dir, "", ctx); err != nil {
		t.Fatalf("writeContextFiles failed: %v", err)
	}

	content, err := os.ReadFile(filepath.Join(dir, ".agent_context", "issue_context.md"))
	if err != nil {
		t.Fatalf("failed to read: %v", err)
	}

	s := string(content)
	if !strings.Contains(s, "minimal-issue-id") {
		t.Error("expected issue ID to be present")
	}
	if strings.Contains(s, "## Agent Skills") {
		t.Error("expected skills section to be omitted when no skills")
	}
}

func TestWriteContextFilesClaudeNativeSkills(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	ctx := TaskContextForEnv{
		IssueID: "claude-skill-test",
		AgentSkills: []SkillContextForEnv{
			{
				Name:    "Go Conventions",
				Content: "Follow Go conventions.",
				Files: []SkillFileContextForEnv{
					{Path: "templates/example.go", Content: "package main"},
				},
			},
		},
	}

	if err := writeContextFiles(dir, "claude", ctx); err != nil {
		t.Fatalf("writeContextFiles failed: %v", err)
	}

	// Skills should be in .claude/skills/ (native discovery), NOT .agent_context/skills/.
	skillMd, err := os.ReadFile(filepath.Join(dir, ".claude", "skills", "go-conventions", "SKILL.md"))
	if err != nil {
		t.Fatalf("failed to read .claude/skills/go-conventions/SKILL.md: %v", err)
	}
	if !strings.Contains(string(skillMd), "Follow Go conventions.") {
		t.Error("SKILL.md missing content")
	}

	// Supporting files should also be under .claude/skills/.
	supportFile, err := os.ReadFile(filepath.Join(dir, ".claude", "skills", "go-conventions", "templates", "example.go"))
	if err != nil {
		t.Fatalf("failed to read supporting file: %v", err)
	}
	if string(supportFile) != "package main" {
		t.Errorf("supporting file content = %q, want %q", string(supportFile), "package main")
	}

	// .agent_context/skills/ should NOT exist for Claude.
	if _, err := os.Stat(filepath.Join(dir, ".agent_context", "skills")); !os.IsNotExist(err) {
		t.Error("expected .agent_context/skills/ to NOT exist for Claude provider")
	}

	// issue_context.md should still be in .agent_context/.
	if _, err := os.Stat(filepath.Join(dir, ".agent_context", "issue_context.md")); os.IsNotExist(err) {
		t.Error("expected .agent_context/issue_context.md to exist")
	}
}

func TestCleanupPreservesLogs(t *testing.T) {
	t.Parallel()
	workspacesRoot := t.TempDir()

	env, err := Prepare(PrepareParams{
		WorkspacesRoot: workspacesRoot,
		WorkspaceID:    "ws-test-003",
		TaskID:         "d4e5f6a7-b8c9-0123-defa-234567890123",
		AgentName:      "Preserve Test",
		Task:           TaskContextForEnv{IssueID: "preserve-test-id"},
	}, testLogger())
	if err != nil {
		t.Fatalf("Prepare failed: %v", err)
	}

	// Write something to logs/.
	os.WriteFile(filepath.Join(env.RootDir, "logs", "test.log"), []byte("log data"), 0o644)

	// Cleanup with removeAll=false.
	if err := env.Cleanup(false); err != nil {
		t.Fatalf("Cleanup failed: %v", err)
	}

	// workdir should be gone.
	if _, err := os.Stat(env.WorkDir); !os.IsNotExist(err) {
		t.Fatal("expected workdir to be removed")
	}

	// logs should still exist.
	logFile := filepath.Join(env.RootDir, "logs", "test.log")
	if _, err := os.Stat(logFile); os.IsNotExist(err) {
		t.Fatal("expected logs/test.log to be preserved")
	}
}

func TestInjectRuntimeConfigClaude(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	ctx := TaskContextForEnv{
		IssueID: "test-issue-id",
		AgentSkills: []SkillContextForEnv{
			{Name: "Go Conventions", Content: "Follow Go conventions.", Files: []SkillFileContextForEnv{
				{Path: "example.go", Content: "package main"},
			}},
			{Name: "PR Review", Content: "Review PRs carefully."},
		},
	}

	if err := InjectRuntimeConfig(dir, "claude", ctx); err != nil {
		t.Fatalf("InjectRuntimeConfig failed: %v", err)
	}

	content, err := os.ReadFile(filepath.Join(dir, "CLAUDE.md"))
	if err != nil {
		t.Fatalf("failed to read CLAUDE.md: %v", err)
	}

	s := string(content)
	for _, want := range []string{
		"Multica Agent Runtime",
		"mantica issue get",
		"mantica issue comment list",
		"Go Conventions",
		"PR Review",
		"discovered automatically",
		"## Workspace Memory",
		"memory/MEMORY.md",
	} {
		if !strings.Contains(s, want) {
			t.Errorf("CLAUDE.md missing %q", want)
		}
	}
	// Workspace Memory section must appear before Workflow so the agent reads memory first.
	memIdx := strings.Index(s, "## Workspace Memory")
	flowIdx := strings.Index(s, "### Workflow")
	if memIdx < 0 || flowIdx < 0 || memIdx > flowIdx {
		t.Errorf("expected ## Workspace Memory to appear before ### Workflow (mem=%d, flow=%d)", memIdx, flowIdx)
	}
}

func TestInjectRuntimeConfigCodex(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	ctx := TaskContextForEnv{
		IssueID:     "test-issue-id",
		AgentSkills: []SkillContextForEnv{{Name: "Coding", Content: "Write good code."}},
	}

	if err := InjectRuntimeConfig(dir, "codex", ctx); err != nil {
		t.Fatalf("InjectRuntimeConfig failed: %v", err)
	}

	content, err := os.ReadFile(filepath.Join(dir, "AGENTS.md"))
	if err != nil {
		t.Fatalf("failed to read AGENTS.md: %v", err)
	}

	s := string(content)
	if !strings.Contains(s, "Multica Agent Runtime") {
		t.Error("AGENTS.md missing meta skill header")
	}
	if !strings.Contains(s, "Coding") {
		t.Error("AGENTS.md missing skill name")
	}
	if !strings.Contains(s, "## Workspace Memory") || !strings.Contains(s, "memory/MEMORY.md") {
		t.Error("AGENTS.md missing Workspace Memory section")
	}
}

func TestInjectRuntimeConfigNoSkills(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	ctx := TaskContextForEnv{IssueID: "test-issue-id"}

	if err := InjectRuntimeConfig(dir, "claude", ctx); err != nil {
		t.Fatalf("InjectRuntimeConfig failed: %v", err)
	}

	content, err := os.ReadFile(filepath.Join(dir, "CLAUDE.md"))
	if err != nil {
		t.Fatalf("failed to read CLAUDE.md: %v", err)
	}

	s := string(content)
	if !strings.Contains(s, "mantica issue get") {
		t.Error("should reference mantica CLI even without skills")
	}
	if strings.Contains(s, "## Skills") {
		t.Error("should not have Skills section when there are no skills")
	}
}

func TestWriteContextFilesOpencodeNativeSkills(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	ctx := TaskContextForEnv{
		IssueID: "opencode-skill-test",
		AgentSkills: []SkillContextForEnv{
			{
				Name:    "Go Conventions",
				Content: "Follow Go conventions.",
				Files: []SkillFileContextForEnv{
					{Path: "templates/example.go", Content: "package main"},
				},
			},
		},
	}

	if err := writeContextFiles(dir, "opencode", ctx); err != nil {
		t.Fatalf("writeContextFiles failed: %v", err)
	}

	// Skills should be in .config/opencode/skills/ (native discovery).
	skillMd, err := os.ReadFile(filepath.Join(dir, ".config", "opencode", "skills", "go-conventions", "SKILL.md"))
	if err != nil {
		t.Fatalf("failed to read .config/opencode/skills/go-conventions/SKILL.md: %v", err)
	}
	if !strings.Contains(string(skillMd), "Follow Go conventions.") {
		t.Error("SKILL.md missing content")
	}

	// Supporting files should also be under .config/opencode/skills/.
	supportFile, err := os.ReadFile(filepath.Join(dir, ".config", "opencode", "skills", "go-conventions", "templates", "example.go"))
	if err != nil {
		t.Fatalf("failed to read supporting file: %v", err)
	}
	if string(supportFile) != "package main" {
		t.Errorf("supporting file content = %q, want %q", string(supportFile), "package main")
	}

	// .agent_context/skills/ should NOT exist for OpenCode.
	if _, err := os.Stat(filepath.Join(dir, ".agent_context", "skills")); !os.IsNotExist(err) {
		t.Error("expected .agent_context/skills/ to NOT exist for OpenCode provider")
	}

	// issue_context.md should still be in .agent_context/.
	if _, err := os.Stat(filepath.Join(dir, ".agent_context", "issue_context.md")); os.IsNotExist(err) {
		t.Error("expected .agent_context/issue_context.md to exist")
	}
}

func TestInjectRuntimeConfigOpencode(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	ctx := TaskContextForEnv{
		IssueID:     "test-issue-id",
		AgentSkills: []SkillContextForEnv{{Name: "Coding", Content: "Write good code."}},
	}

	if err := InjectRuntimeConfig(dir, "opencode", ctx); err != nil {
		t.Fatalf("InjectRuntimeConfig failed: %v", err)
	}

	// OpenCode uses AGENTS.md (same as codex).
	content, err := os.ReadFile(filepath.Join(dir, "AGENTS.md"))
	if err != nil {
		t.Fatalf("failed to read AGENTS.md: %v", err)
	}

	s := string(content)
	if !strings.Contains(s, "Multica Agent Runtime") {
		t.Error("AGENTS.md missing meta skill header")
	}
	if !strings.Contains(s, "Coding") {
		t.Error("AGENTS.md missing skill name")
	}
	if !strings.Contains(s, "discovered automatically") {
		t.Error("AGENTS.md missing native skill discovery hint")
	}
	if !strings.Contains(s, "## Workspace Memory") || !strings.Contains(s, "memory/MEMORY.md") {
		t.Error("AGENTS.md missing Workspace Memory section")
	}

	// CLAUDE.md should NOT exist.
	if _, err := os.Stat(filepath.Join(dir, "CLAUDE.md")); !os.IsNotExist(err) {
		t.Error("expected CLAUDE.md to NOT exist for OpenCode provider")
	}
}

func TestPrepareWithRepoContextOpencode(t *testing.T) {
	t.Parallel()
	workspacesRoot := t.TempDir()

	taskCtx := TaskContextForEnv{
		IssueID: "c3d4e5f6-a7b8-9012-cdef-123456789012",
		Repos: []RepoContextForEnv{
			{URL: "https://github.com/org/backend", Description: "Go backend"},
		},
	}
	env, err := Prepare(PrepareParams{
		WorkspacesRoot: workspacesRoot,
		WorkspaceID:    "ws-test-oc",
		TaskID:         "c3d4e5f6-a7b8-9012-cdef-123456789012",
		AgentName:      "OpenCode Agent",
		Provider:       "opencode",
		Task:           taskCtx,
	}, testLogger())
	if err != nil {
		t.Fatalf("Prepare failed: %v", err)
	}
	defer env.Cleanup(true)

	if err := InjectRuntimeConfig(env.WorkDir, "opencode", taskCtx); err != nil {
		t.Fatalf("InjectRuntimeConfig failed: %v", err)
	}

	// Workdir should only contain expected entries.
	entries, err := os.ReadDir(env.WorkDir)
	if err != nil {
		t.Fatalf("failed to read workdir: %v", err)
	}
	for _, e := range entries {
		name := e.Name()
		if name != ".agent_context" && name != "AGENTS.md" && name != "memory" {
			t.Errorf("unexpected entry in workdir: %s", name)
		}
	}

	// AGENTS.md should contain repo info.
	content, err := os.ReadFile(filepath.Join(env.WorkDir, "AGENTS.md"))
	if err != nil {
		t.Fatalf("failed to read AGENTS.md: %v", err)
	}
	s := string(content)
	for _, want := range []string{
		"mantica repo checkout",
		"https://github.com/org/backend",
		"Go backend",
	} {
		if !strings.Contains(s, want) {
			t.Errorf("AGENTS.md missing %q", want)
		}
	}
}

func TestInjectRuntimeConfigUnknownProvider(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	// Unknown provider should be a no-op.
	if err := InjectRuntimeConfig(dir, "unknown", TaskContextForEnv{}); err != nil {
		t.Fatalf("expected no error for unknown provider, got: %v", err)
	}

	// No files should be created.
	entries, _ := os.ReadDir(dir)
	if len(entries) != 0 {
		t.Fatalf("expected empty dir for unknown provider, got %d entries", len(entries))
	}
}

func TestPrepareCodexHomeSeedsFromShared(t *testing.T) {
	// Cannot use t.Parallel() with t.Setenv.

	// Create a fake shared codex home.
	sharedHome := t.TempDir()
	os.WriteFile(filepath.Join(sharedHome, "auth.json"), []byte(`{"token":"secret"}`), 0o644)
	os.WriteFile(filepath.Join(sharedHome, "config.json"), []byte(`{"model":"o3"}`), 0o644)
	os.WriteFile(filepath.Join(sharedHome, "config.toml"), []byte(`model = "o3"`), 0o644)
	os.WriteFile(filepath.Join(sharedHome, "instructions.md"), []byte("Be helpful."), 0o644)

	// Point CODEX_HOME to our fake shared home.
	t.Setenv("CODEX_HOME", sharedHome)

	codexHome := filepath.Join(t.TempDir(), "codex-home")
	if err := prepareCodexHome(codexHome, testLogger()); err != nil {
		t.Fatalf("prepareCodexHome failed: %v", err)
	}

	// auth.json should be a symlink.
	authPath := filepath.Join(codexHome, "auth.json")
	fi, err := os.Lstat(authPath)
	if err != nil {
		t.Fatalf("auth.json not found: %v", err)
	}
	if fi.Mode()&os.ModeSymlink == 0 {
		t.Error("auth.json should be a symlink")
	}
	target, _ := os.Readlink(authPath)
	if target != filepath.Join(sharedHome, "auth.json") {
		t.Errorf("auth.json symlink target = %q, want %q", target, filepath.Join(sharedHome, "auth.json"))
	}
	// Verify content is accessible through symlink.
	data, _ := os.ReadFile(authPath)
	if string(data) != `{"token":"secret"}` {
		t.Errorf("auth.json content = %q", data)
	}

	// config.json should be a copy (not symlink).
	configPath := filepath.Join(codexHome, "config.json")
	fi, err = os.Lstat(configPath)
	if err != nil {
		t.Fatalf("config.json not found: %v", err)
	}
	if fi.Mode()&os.ModeSymlink != 0 {
		t.Error("config.json should be a copy, not a symlink")
	}
	data, _ = os.ReadFile(configPath)
	if string(data) != `{"model":"o3"}` {
		t.Errorf("config.json content = %q", data)
	}

	// config.toml should be copied.
	data, _ = os.ReadFile(filepath.Join(codexHome, "config.toml"))
	if string(data) != `model = "o3"` {
		t.Errorf("config.toml content = %q", data)
	}

	// instructions.md should be copied.
	data, _ = os.ReadFile(filepath.Join(codexHome, "instructions.md"))
	if string(data) != "Be helpful." {
		t.Errorf("instructions.md content = %q", data)
	}
}

func TestPrepareCodexHomeSkipsMissingFiles(t *testing.T) {
	// Cannot use t.Parallel() with t.Setenv.

	// Empty shared home — no files to seed.
	sharedHome := t.TempDir()
	t.Setenv("CODEX_HOME", sharedHome)

	codexHome := filepath.Join(t.TempDir(), "codex-home")
	if err := prepareCodexHome(codexHome, testLogger()); err != nil {
		t.Fatalf("prepareCodexHome failed: %v", err)
	}

	// Directory should exist but be empty (no auth.json, no config.json, etc.).
	entries, err := os.ReadDir(codexHome)
	if err != nil {
		t.Fatalf("failed to read codex-home: %v", err)
	}
	if len(entries) != 0 {
		names := make([]string, len(entries))
		for i, e := range entries {
			names[i] = e.Name()
		}
		t.Errorf("expected empty codex-home, got: %v", names)
	}
}

func TestPrepareCreatesWorkspaceMemorySymlink(t *testing.T) {
	t.Parallel()
	workspacesRoot := t.TempDir()

	env, err := Prepare(PrepareParams{
		WorkspacesRoot: workspacesRoot,
		WorkspaceID:    "ws-mem-001",
		TaskID:         "11111111-2222-3333-4444-555555555555",
		AgentName:      "Memory Test",
		Task:           TaskContextForEnv{IssueID: "memory-test-issue"},
	}, testLogger())
	if err != nil {
		t.Fatalf("Prepare failed: %v", err)
	}
	defer env.Cleanup(true)

	sharedMem := filepath.Join(workspacesRoot, "ws-mem-001", "memory")
	if fi, err := os.Stat(sharedMem); err != nil || !fi.IsDir() {
		t.Fatalf("expected shared memory dir at %s", sharedMem)
	}

	link := filepath.Join(env.WorkDir, "memory")
	fi, err := os.Lstat(link)
	if err != nil {
		t.Fatalf("workdir memory entry missing: %v", err)
	}
	if fi.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("expected %s to be a symlink", link)
	}
	target, err := os.Readlink(link)
	if err != nil {
		t.Fatalf("readlink failed: %v", err)
	}
	if target != sharedMem {
		t.Errorf("symlink target = %q, want %q", target, sharedMem)
	}

	// Writes through the symlink land in the shared dir (proves the mount works).
	if err := os.WriteFile(filepath.Join(link, "MEMORY.md"), []byte("- entry\n"), 0o644); err != nil {
		t.Fatalf("write through symlink failed: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(sharedMem, "MEMORY.md"))
	if err != nil {
		t.Fatalf("read shared memory failed: %v", err)
	}
	if string(data) != "- entry\n" {
		t.Errorf("shared MEMORY.md content = %q", data)
	}
}

func TestReuseReestablishesMemorySymlink(t *testing.T) {
	t.Parallel()
	workspacesRoot := t.TempDir()

	// Simulate a workdir created before the workspace-memory feature: just a bare dir.
	workDir := filepath.Join(workspacesRoot, "ws-mem-002", "abcd1234", "workdir")
	if err := os.MkdirAll(workDir, 0o755); err != nil {
		t.Fatalf("mkdir workdir: %v", err)
	}

	env := Reuse(workspacesRoot, "ws-mem-002", workDir, "claude", TaskContextForEnv{IssueID: "i"}, testLogger())
	if env == nil {
		t.Fatal("Reuse returned nil")
	}

	sharedMem := filepath.Join(workspacesRoot, "ws-mem-002", "memory")
	if _, err := os.Stat(sharedMem); err != nil {
		t.Fatalf("shared memory dir missing after Reuse: %v", err)
	}

	link := filepath.Join(workDir, "memory")
	fi, err := os.Lstat(link)
	if err != nil {
		t.Fatalf("memory symlink missing after Reuse: %v", err)
	}
	if fi.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("expected %s to be a symlink", link)
	}
	target, _ := os.Readlink(link)
	if target != sharedMem {
		t.Errorf("symlink target = %q, want %q", target, sharedMem)
	}
}

func TestWorkspaceMemorySharedAcrossTasks(t *testing.T) {
	t.Parallel()
	workspacesRoot := t.TempDir()

	mkEnv := func(taskID string) *Environment {
		env, err := Prepare(PrepareParams{
			WorkspacesRoot: workspacesRoot,
			WorkspaceID:    "ws-mem-shared",
			TaskID:         taskID,
			AgentName:      "Shared Memory Test",
			Task:           TaskContextForEnv{IssueID: "i"},
		}, testLogger())
		if err != nil {
			t.Fatalf("Prepare failed: %v", err)
		}
		return env
	}

	envA := mkEnv("aaaaaaaa-1111-2222-3333-444444444444")
	defer envA.Cleanup(true)
	envB := mkEnv("bbbbbbbb-1111-2222-3333-444444444444")
	defer envB.Cleanup(true)

	if err := os.WriteFile(filepath.Join(envA.WorkDir, "memory", "MEMORY.md"), []byte("from A"), 0o644); err != nil {
		t.Fatalf("write from task A: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(envB.WorkDir, "memory", "MEMORY.md"))
	if err != nil {
		t.Fatalf("read from task B: %v", err)
	}
	if string(data) != "from A" {
		t.Errorf("task B saw %q, want %q (memory not shared across tasks)", data, "from A")
	}
}

func TestInjectRuntimeConfigContainsWriteMemorySection(t *testing.T) {
	t.Parallel()

	wantPhrases := []string{
		"## Writing to Memory",
		"./memory/",
		"MEMORY.md",
		"feedback",
		"project",
		"reference",
		"What NOT to save",
		"docs/agent-memory-format.md",
	}

	for _, provider := range []string{"claude", "codex", "opencode", "openclaw", "hermes"} {
		provider := provider
		t.Run(provider, func(t *testing.T) {
			t.Parallel()
			dir := t.TempDir()

			ctx := TaskContextForEnv{IssueID: "memory-section-test"}
			if err := InjectRuntimeConfig(dir, provider, ctx); err != nil {
				t.Fatalf("InjectRuntimeConfig failed: %v", err)
			}

			fileName := "AGENTS.md"
			if provider == "claude" {
				fileName = "CLAUDE.md"
			}
			content, err := os.ReadFile(filepath.Join(dir, fileName))
			if err != nil {
				t.Fatalf("failed to read %s: %v", fileName, err)
			}
			s := string(content)
			for _, want := range wantPhrases {
				if !strings.Contains(s, want) {
					t.Errorf("%s missing %q", fileName, want)
				}
			}
		})
	}
}

func TestInjectRuntimeConfigCommentTriggerMentionsFeedbackMemory(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	ctx := TaskContextForEnv{
		IssueID:          "comment-trigger-test",
		TriggerCommentID: "comment-abc",
	}
	if err := InjectRuntimeConfig(dir, "claude", ctx); err != nil {
		t.Fatalf("InjectRuntimeConfig failed: %v", err)
	}
	content, err := os.ReadFile(filepath.Join(dir, "CLAUDE.md"))
	if err != nil {
		t.Fatalf("failed to read CLAUDE.md: %v", err)
	}
	s := string(content)
	if !strings.Contains(s, "save it as a `feedback` memory") {
		t.Error("comment-triggered config should nudge agent to save feedback memory")
	}
}

func TestEnsureSymlinkRepairsBrokenLink(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	src := filepath.Join(dir, "source.json")
	dst := filepath.Join(dir, "link.json")

	os.WriteFile(src, []byte("real"), 0o644)

	// Create a broken symlink pointing to a non-existent file.
	os.Symlink(filepath.Join(dir, "old-source.json"), dst)

	if err := ensureSymlink(src, dst); err != nil {
		t.Fatalf("ensureSymlink failed: %v", err)
	}

	// Should now point to src.
	target, _ := os.Readlink(dst)
	if target != src {
		t.Errorf("symlink target = %q, want %q", target, src)
	}
	data, _ := os.ReadFile(dst)
	if string(data) != "real" {
		t.Errorf("content = %q, want %q", data, "real")
	}
}

func TestInjectRuntimeConfigContainsProjectConventionsStep(t *testing.T) {
	t.Parallel()

	// Step 0 must include instructions to check project_conventions.md (step 5)
	// and apply conventions with override priority (step 6).
	wantPhrases := []string{
		"project_conventions.md",
		"branch_strategy",
		"commit_format",
		"status_transitions",
		"override",
	}

	for _, provider := range []string{"claude", "codex", "opencode", "openclaw", "hermes"} {
		provider := provider
		t.Run(provider, func(t *testing.T) {
			t.Parallel()
			dir := t.TempDir()

			ctx := TaskContextForEnv{IssueID: "conventions-step-test"}
			if err := InjectRuntimeConfig(dir, provider, ctx); err != nil {
				t.Fatalf("InjectRuntimeConfig failed: %v", err)
			}

			fileName := "AGENTS.md"
			if provider == "claude" {
				fileName = "CLAUDE.md"
			}
			content, err := os.ReadFile(filepath.Join(dir, fileName))
			if err != nil {
				t.Fatalf("failed to read %s: %v", fileName, err)
			}
			s := string(content)
			for _, want := range wantPhrases {
				if !strings.Contains(s, want) {
					t.Errorf("%s missing %q", fileName, want)
				}
			}
		})
	}
}

func TestInjectRuntimeConfigConventionsPrecedesWorkflow(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	ctx := TaskContextForEnv{IssueID: "precedence-test"}
	if err := InjectRuntimeConfig(dir, "claude", ctx); err != nil {
		t.Fatalf("InjectRuntimeConfig failed: %v", err)
	}
	content, err := os.ReadFile(filepath.Join(dir, "CLAUDE.md"))
	if err != nil {
		t.Fatalf("failed to read CLAUDE.md: %v", err)
	}
	s := string(content)

	// project_conventions.md loading (step 5) must appear inside the
	// Step 0 block, which in turn must appear before ### Workflow.
	convIdx := strings.Index(s, "project_conventions.md")
	flowIdx := strings.Index(s, "### Workflow")
	if convIdx < 0 {
		t.Fatal("CLAUDE.md missing project_conventions.md reference")
	}
	if flowIdx < 0 {
		t.Fatal("CLAUDE.md missing ### Workflow section")
	}
	if convIdx > flowIdx {
		t.Errorf("project_conventions.md reference (pos %d) must appear before ### Workflow (pos %d)", convIdx, flowIdx)
	}
}
