---
name: SQLite memory backend (TES-28)
description: memory.db at {WorkspacesRoot}/{WorkspaceID}/memory.db; use `multica memory` CLI, not direct file writes
type: project
---

TES-28 migrated workspace memory from flat markdown files to a SQLite WAL-mode database.

The database lives at `{WorkspacesRoot}/{WorkspaceID}/memory.db`. The `memory/` directory remains as a human-readable export target (markdown files under `memory/entries/<id>.md` and `memory/MEMORY.md` index).

**Why:** flat files had no search, no deduplication, no TTL, and no concurrent-write safety.

**How to apply:**

- Agents should use `multica memory add/search/list/gc` for all memory operations.
- Direct writes to `memory/*.md` still work but bypass deduplication and TTL.
- Run `multica memory migrate` once on legacy workspaces to import existing `.md` files.
- The implementation is in `server/internal/daemon/execenv/memory_db.go` (pure-Go, modernc/sqlite, no CGO).
- Vector search (AC-5) is not yet implemented; FTS5 full-text search is the primary retrieval path.
