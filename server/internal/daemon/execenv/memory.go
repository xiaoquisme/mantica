package execenv

// Workspace memory lives at {WorkspacesRoot}/{WorkspaceID}/memory/ — one shared
// dir per workspace, mirroring the repocache layout. Per-task workdirs symlink
// it as `./memory` so every task (any provider) reads and writes the same
// accumulated context across sessions.

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
)

// ensureWorkspaceMemoryMounted creates the workspace-scoped memory directory
// (if missing) and links it into the per-task workdir as `./memory`.
// Missing-source is impossible because we MkdirAll first; this satisfies the
// "no memory directory" acceptance criterion with no special-case branches.
func ensureWorkspaceMemoryMounted(workspacesRoot, workspaceID, workDir string, logger *slog.Logger) error {
	if workspacesRoot == "" || workspaceID == "" || workDir == "" {
		return nil
	}

	shared := filepath.Join(workspacesRoot, workspaceID, "memory")
	if err := os.MkdirAll(shared, 0o755); err != nil {
		return fmt.Errorf("create workspace memory dir: %w", err)
	}

	link := filepath.Join(workDir, "memory")
	if err := ensureSymlink(shared, link); err != nil {
		logger.Warn("execenv: workspace memory symlink failed", "link", link, "target", shared, "error", err)
	}
	return nil
}
