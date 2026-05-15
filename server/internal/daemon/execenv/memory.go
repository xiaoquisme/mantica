package execenv

// Workspace memory lives at {WorkspacesRoot}/{WorkspaceID}/memory/ — one shared
// dir per workspace, mirroring the repocache layout. Per-task workdirs symlink
// it as `./memory` so every task (any provider) reads and writes the same
// accumulated context across sessions.
//
// AC-1: memory.db is co-located with the memory/ directory at
//   {WorkspacesRoot}/{WorkspaceID}/memory.db
// and opened in WAL mode for concurrent-safe multi-process access.

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
)

// ensureWorkspaceMemoryMounted creates the workspace-scoped memory directory
// and SQLite database (if missing) and links the directory into the per-task
// workdir as `./memory`.
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

	// AC-1: initialise memory.db alongside the memory/ directory.
	dbPath := filepath.Join(workspacesRoot, workspaceID, "memory.db")
	if db, err := OpenMemoryDB(dbPath, workspaceID, shared); err != nil {
		logger.Warn("execenv: workspace memory.db init failed", "path", dbPath, "error", err)
	} else {
		db.Close()
	}

	link := filepath.Join(workDir, "memory")
	if err := ensureSymlink(shared, link); err != nil {
		logger.Warn("execenv: workspace memory symlink failed", "link", link, "target", shared, "error", err)
	}
	return nil
}

// WorkspaceMemoryDB opens the memory database for a given workspace.
// Returns the MemoryDB ready for use; caller must Close() it.
func WorkspaceMemoryDB(workspacesRoot, workspaceID string) (*MemoryDB, error) {
	shared := filepath.Join(workspacesRoot, workspaceID, "memory")
	dbPath := filepath.Join(workspacesRoot, workspaceID, "memory.db")
	return OpenMemoryDB(dbPath, workspaceID, shared)
}
