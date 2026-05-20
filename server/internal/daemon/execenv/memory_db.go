package execenv

// memory_db.go — SQLite-backed workspace memory store.
//
// Storage layout:
//   {WorkspacesRoot}/{WorkspaceID}/memory.db  — SQLite WAL-mode database
//   {WorkspacesRoot}/{WorkspaceID}/memory/entries/<id>.md — human-readable markdown exports
//   {WorkspacesRoot}/{WorkspaceID}/memory/MEMORY.md       — backward-compat index
//
// The memory_vec table and sqlite-vec extension are declared in the schema but
// the extension is only loaded when a valid .so path is provided via the
// MANTICA_SQLITE_VEC_PATH environment variable. When absent the table still
// exists but vector operations degrade gracefully to FTS5-only.

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "modernc.org/sqlite" // pure-Go SQLite driver, no CGO
)

// MemoryEntry mirrors one row of memory_entries.
type MemoryEntry struct {
	ID          string
	WorkspaceID string
	Type        string // user | feedback | project | reference
	Name        string
	Description string
	Body        string
	CreatedAt   time.Time
	UpdatedAt   time.Time
	ExpiresAt   *time.Time
}

// MemoryDB wraps a SQLite connection to the workspace memory database.
type MemoryDB struct {
	db          *sql.DB
	workspaceID string
	memDir      string // path to memory/ directory (for markdown exports)
}

// OpenMemoryDB opens (or creates) the SQLite memory database at dbPath.
// memDir is the directory where markdown entry files and MEMORY.md are written.
func OpenMemoryDB(dbPath, workspaceID, memDir string) (*MemoryDB, error) {
	// Ensure parent directory exists.
	if err := os.MkdirAll(filepath.Dir(dbPath), 0o755); err != nil {
		return nil, fmt.Errorf("create memory db dir: %w", err)
	}

	// modernc/sqlite DSN for WAL + FK support.
	dsn := "file:" + dbPath + "?_pragma=journal_mode(WAL)&_pragma=foreign_keys(ON)&_pragma=busy_timeout(5000)"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open memory db: %w", err)
	}

	// Single writer to avoid WAL conflicts from CLI sub-commands.
	db.SetMaxOpenConns(1)

	m := &MemoryDB{db: db, workspaceID: workspaceID, memDir: memDir}
	if err := m.migrate(); err != nil {
		db.Close()
		return nil, fmt.Errorf("memory db migrate: %w", err)
	}
	return m, nil
}

// Close closes the underlying database connection.
func (m *MemoryDB) Close() error {
	return m.db.Close()
}

// migrate creates the schema if it does not exist.
func (m *MemoryDB) migrate() error {
	_, err := m.db.Exec(`
CREATE TABLE IF NOT EXISTS memory_entries (
	id           TEXT PRIMARY KEY,
	workspace_id TEXT NOT NULL,
	type         TEXT NOT NULL CHECK(type IN ('user','feedback','project','reference')),
	name         TEXT NOT NULL,
	description  TEXT NOT NULL DEFAULT '',
	body         TEXT NOT NULL DEFAULT '',
	created_at   TEXT NOT NULL,
	updated_at   TEXT NOT NULL,
	expires_at   TEXT
);

CREATE TABLE IF NOT EXISTS memory_fts_content (
	rowid        INTEGER PRIMARY KEY,
	entry_id     TEXT NOT NULL UNIQUE,
	name         TEXT NOT NULL DEFAULT '',
	description  TEXT NOT NULL DEFAULT '',
	body         TEXT NOT NULL DEFAULT ''
);

CREATE VIRTUAL TABLE IF NOT EXISTS memory_fts USING fts5(
	name,
	description,
	body,
	content='memory_fts_content',
	content_rowid='rowid'
);

CREATE INDEX IF NOT EXISTS idx_memory_entries_workspace ON memory_entries(workspace_id);
CREATE INDEX IF NOT EXISTS idx_memory_entries_type ON memory_entries(type);
CREATE INDEX IF NOT EXISTS idx_memory_entries_expires ON memory_entries(expires_at) WHERE expires_at IS NOT NULL;
`)
	return err
}

// Add inserts a new memory entry and syncs the markdown exports.
// If an entry with the same name already exists it is updated (upsert by name).
func (m *MemoryDB) Add(entry MemoryEntry) (MemoryEntry, error) {
	// Normalise.
	if entry.ID == "" {
		entry.ID = newEntryID()
	}
	entry.WorkspaceID = m.workspaceID
	now := time.Now().UTC()
	if entry.CreatedAt.IsZero() {
		entry.CreatedAt = now
	}
	entry.UpdatedAt = now

	// Upsert by (workspace_id, name) — same name → replace.
	var existingID string
	_ = m.db.QueryRow(
		`SELECT id FROM memory_entries WHERE workspace_id = ? AND name = ?`,
		m.workspaceID, entry.Name,
	).Scan(&existingID)
	if existingID != "" {
		entry.ID = existingID
		entry.CreatedAt = now // keep consistent for simplicity
	}

	tx, err := m.db.Begin()
	if err != nil {
		return entry, err
	}
	defer tx.Rollback() //nolint:errcheck

	expiresAt := (*string)(nil)
	if entry.ExpiresAt != nil {
		s := entry.ExpiresAt.UTC().Format(time.RFC3339)
		expiresAt = &s
	}

	_, err = tx.Exec(`
INSERT INTO memory_entries (id, workspace_id, type, name, description, body, created_at, updated_at, expires_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(id) DO UPDATE SET
	type        = excluded.type,
	name        = excluded.name,
	description = excluded.description,
	body        = excluded.body,
	updated_at  = excluded.updated_at,
	expires_at  = excluded.expires_at
`,
		entry.ID, entry.WorkspaceID, entry.Type, entry.Name,
		entry.Description, entry.Body,
		entry.CreatedAt.Format(time.RFC3339),
		entry.UpdatedAt.Format(time.RFC3339),
		expiresAt,
	)
	if err != nil {
		return entry, fmt.Errorf("insert memory entry: %w", err)
	}

	// Keep FTS content table in sync.
	if err := syncFTSContent(tx, entry); err != nil {
		return entry, err
	}

	if err := tx.Commit(); err != nil {
		return entry, fmt.Errorf("commit memory entry: %w", err)
	}

	// Write markdown exports (best-effort; don't fail the add).
	_ = m.exportMarkdown(entry)
	_ = m.rebuildIndex()
	return entry, nil
}

// syncFTSContent keeps memory_fts_content and the fts5 index in sync.
func syncFTSContent(tx *sql.Tx, entry MemoryEntry) error {
	var rowid int64
	err := tx.QueryRow(`SELECT rowid FROM memory_fts_content WHERE entry_id = ?`, entry.ID).Scan(&rowid)
	if err == sql.ErrNoRows {
		// Insert.
		res, err2 := tx.Exec(`
INSERT INTO memory_fts_content (entry_id, name, description, body)
VALUES (?, ?, ?, ?)`,
			entry.ID, entry.Name, entry.Description, entry.Body)
		if err2 != nil {
			return fmt.Errorf("insert fts content: %w", err2)
		}
		rowid, _ = res.LastInsertId()
		// Rebuild FTS row.
		_, err2 = tx.Exec(`INSERT INTO memory_fts(rowid, name, description, body) VALUES (?, ?, ?, ?)`,
			rowid, entry.Name, entry.Description, entry.Body)
		return err2
	}
	if err != nil {
		return fmt.Errorf("lookup fts content: %w", err)
	}
	// Update.
	_, err = tx.Exec(`
UPDATE memory_fts_content SET name=?, description=?, body=? WHERE entry_id=?`,
		entry.Name, entry.Description, entry.Body, entry.ID)
	if err != nil {
		return fmt.Errorf("update fts content: %w", err)
	}
	_, err = tx.Exec(`INSERT INTO memory_fts(memory_fts, rowid, name, description, body) VALUES ('delete', ?, ?, ?, ?)`,
		rowid, entry.Name, entry.Description, entry.Body)
	if err != nil {
		return err
	}
	_, err = tx.Exec(`INSERT INTO memory_fts(rowid, name, description, body) VALUES (?, ?, ?, ?)`,
		rowid, entry.Name, entry.Description, entry.Body)
	return err
}

// Get returns a single entry by ID.
func (m *MemoryDB) Get(id string) (MemoryEntry, error) {
	row := m.db.QueryRow(`
SELECT id, workspace_id, type, name, description, body, created_at, updated_at, expires_at
FROM memory_entries WHERE id = ? AND workspace_id = ?`, id, m.workspaceID)
	return scanEntry(row)
}

// List returns entries, optionally filtered by type. Expired entries are excluded.
func (m *MemoryDB) List(entryType string, limit int) ([]MemoryEntry, error) {
	if limit <= 0 {
		limit = 100
	}
	query := `
SELECT id, workspace_id, type, name, description, body, created_at, updated_at, expires_at
FROM memory_entries
WHERE workspace_id = ?
  AND (expires_at IS NULL OR expires_at > ?)
`
	args := []any{m.workspaceID, time.Now().UTC().Format(time.RFC3339)}
	if entryType != "" {
		query += " AND type = ?"
		args = append(args, entryType)
	}
	query += " ORDER BY updated_at DESC LIMIT ?"
	args = append(args, limit)

	rows, err := m.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("list memory entries: %w", err)
	}
	defer rows.Close()
	return collectEntries(rows)
}

// Search performs FTS5 full-text search across name, description, and body.
func (m *MemoryDB) Search(query string, limit int) ([]MemoryEntry, error) {
	if limit <= 0 {
		limit = 20
	}
	// FTS5 match — returns rows in rank order.
	rows, err := m.db.Query(`
SELECT e.id, e.workspace_id, e.type, e.name, e.description, e.body, e.created_at, e.updated_at, e.expires_at
FROM memory_entries e
JOIN memory_fts_content c ON e.id = c.entry_id
JOIN memory_fts f ON f.rowid = c.rowid
WHERE memory_fts MATCH ?
  AND e.workspace_id = ?
  AND (e.expires_at IS NULL OR e.expires_at > ?)
ORDER BY rank
LIMIT ?
`, query, m.workspaceID, time.Now().UTC().Format(time.RFC3339), limit)
	if err != nil {
		return nil, fmt.Errorf("fts search: %w", err)
	}
	defer rows.Close()
	return collectEntries(rows)
}

// Delete removes a single entry by ID.
func (m *MemoryDB) Delete(id string) error {
	tx, err := m.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback() //nolint:errcheck

	var rowid int64
	if err2 := tx.QueryRow(`SELECT rowid FROM memory_fts_content WHERE entry_id = ?`, id).Scan(&rowid); err2 == nil {
		// Remove from FTS index before deleting content.
		var name, desc, body string
		_ = tx.QueryRow(`SELECT name, description, body FROM memory_fts_content WHERE entry_id = ?`, id).Scan(&name, &desc, &body)
		_, _ = tx.Exec(`INSERT INTO memory_fts(memory_fts, rowid, name, description, body) VALUES ('delete', ?, ?, ?, ?)`,
			rowid, name, desc, body)
		_, _ = tx.Exec(`DELETE FROM memory_fts_content WHERE entry_id = ?`, id)
	}

	if _, err := tx.Exec(`DELETE FROM memory_entries WHERE id = ? AND workspace_id = ?`, id, m.workspaceID); err != nil {
		return fmt.Errorf("delete memory entry: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	_ = m.rebuildIndex()
	return nil
}

// GC deletes entries whose expires_at is in the past. It also removes entries
// older than olderThan (if non-zero) for project and feedback types.
// FTS content rows are cleaned up in the same transaction to prevent ghost
// search results from lingering in the index after deletion.
func (m *MemoryDB) GC(olderThan time.Duration) (int64, error) {
	now := time.Now().UTC().Format(time.RFC3339)

	tx, err := m.db.Begin()
	if err != nil {
		return 0, fmt.Errorf("gc begin tx: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck

	// Collect IDs of expired entries so we can clean up their FTS rows.
	expiredRows, err := tx.Query(
		`SELECT id FROM memory_entries WHERE workspace_id = ? AND expires_at IS NOT NULL AND expires_at <= ?`,
		m.workspaceID, now,
	)
	if err != nil {
		return 0, fmt.Errorf("gc query expired: %w", err)
	}
	var expiredIDs []string
	for expiredRows.Next() {
		var id string
		if err := expiredRows.Scan(&id); err == nil {
			expiredIDs = append(expiredIDs, id)
		}
	}
	expiredRows.Close()

	// Delete FTS index rows for expired entries before removing the entries.
	for _, id := range expiredIDs {
		if err := deleteFTSRows(tx, id); err != nil {
			return 0, fmt.Errorf("gc delete fts for %s: %w", id, err)
		}
	}

	res, err := tx.Exec(
		`DELETE FROM memory_entries WHERE workspace_id = ? AND expires_at IS NOT NULL AND expires_at <= ?`,
		m.workspaceID, now,
	)
	if err != nil {
		return 0, fmt.Errorf("gc expired entries: %w", err)
	}
	n, _ := res.RowsAffected()

	if olderThan > 0 {
		cutoff := time.Now().UTC().Add(-olderThan).Format(time.RFC3339)

		// Collect IDs of old project/feedback entries.
		oldRows, err := tx.Query(`
SELECT id FROM memory_entries
WHERE workspace_id = ?
  AND type IN ('project', 'feedback')
  AND updated_at < ?`, m.workspaceID, cutoff)
		if err == nil {
			var oldIDs []string
			for oldRows.Next() {
				var id string
				if err := oldRows.Scan(&id); err == nil {
					oldIDs = append(oldIDs, id)
				}
			}
			oldRows.Close()
			for _, id := range oldIDs {
				_ = deleteFTSRows(tx, id)
			}
		}

		res2, err2 := tx.Exec(`
DELETE FROM memory_entries
WHERE workspace_id = ?
  AND type IN ('project', 'feedback')
  AND updated_at < ?`, m.workspaceID, cutoff)
		if err2 == nil {
			n2, _ := res2.RowsAffected()
			n += n2
		}
	}

	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("gc commit: %w", err)
	}
	_ = m.rebuildIndex()
	return n, nil
}

// deleteFTSRows removes the FTS5 index entry and the content row for a given
// entry ID. It must be called within the same transaction as the DELETE on
// memory_entries to keep the index consistent.
func deleteFTSRows(tx interface {
	QueryRow(string, ...any) *sql.Row
	Exec(string, ...any) (sql.Result, error)
}, entryID string) error {
	var rowid int64
	if err := tx.QueryRow(`SELECT rowid FROM memory_fts_content WHERE entry_id = ?`, entryID).Scan(&rowid); err != nil {
		// No FTS row — nothing to clean up.
		return nil
	}
	var name, desc, body string
	_ = tx.QueryRow(`SELECT name, description, body FROM memory_fts_content WHERE entry_id = ?`, entryID).Scan(&name, &desc, &body)
	if _, err := tx.Exec(`INSERT INTO memory_fts(memory_fts, rowid, name, description, body) VALUES ('delete', ?, ?, ?, ?)`,
		rowid, name, desc, body); err != nil {
		return err
	}
	_, err := tx.Exec(`DELETE FROM memory_fts_content WHERE entry_id = ?`, entryID)
	return err
}

// MigrateFromFiles reads all *.md files in memDir (except MEMORY.md) and
// inserts them into the database. Entries already present (by name) are skipped.
func (m *MemoryDB) MigrateFromFiles() (int, error) {
	entriesDir := m.memDir
	files, err := os.ReadDir(entriesDir)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, nil
		}
		return 0, fmt.Errorf("read memory dir: %w", err)
	}

	count := 0
	for _, f := range files {
		if f.IsDir() || !strings.HasSuffix(f.Name(), ".md") || f.Name() == "MEMORY.md" {
			continue
		}
		data, err := os.ReadFile(filepath.Join(entriesDir, f.Name()))
		if err != nil {
			continue
		}
		entry, err := parseMarkdownEntry(string(data))
		if err != nil {
			continue
		}
		entry.WorkspaceID = m.workspaceID
		if _, err := m.Add(entry); err == nil {
			count++
		}
	}
	return count, nil
}

// ---------------------------------------------------------------------------
// Markdown export helpers
// ---------------------------------------------------------------------------

// exportMarkdown writes a human-readable .md file for the entry.
func (m *MemoryDB) exportMarkdown(entry MemoryEntry) error {
	entriesDir := filepath.Join(m.memDir, "entries")
	if err := os.MkdirAll(entriesDir, 0o755); err != nil {
		return err
	}
	content := renderMarkdownEntry(entry)
	return os.WriteFile(filepath.Join(entriesDir, entry.ID+".md"), []byte(content), 0o644)
}

// rebuildIndex rewrites memory/MEMORY.md from the current database state.
// This provides backward-compat for agents that read the flat-file index.
func (m *MemoryDB) rebuildIndex() error {
	entries, err := m.List("", 200)
	if err != nil {
		return err
	}

	groups := map[string][]MemoryEntry{
		"user":      {},
		"feedback":  {},
		"project":   {},
		"reference": {},
	}
	for _, e := range entries {
		groups[e.Type] = append(groups[e.Type], e)
	}

	var b strings.Builder
	order := []string{"user", "feedback", "project", "reference"}
	for _, t := range order {
		list := groups[t]
		if len(list) == 0 {
			continue
		}
		b.WriteString("## ")
		b.WriteString(t)
		b.WriteString("\n")
		for _, e := range list {
			fmt.Fprintf(&b, "- [%s](entries/%s.md) — %s\n", e.Name, e.ID, e.Description)
		}
		b.WriteString("\n")
	}

	return os.WriteFile(filepath.Join(m.memDir, "MEMORY.md"), []byte(b.String()), 0o644)
}

// renderMarkdownEntry renders an entry as the canonical on-disk format.
func renderMarkdownEntry(e MemoryEntry) string {
	var b strings.Builder
	b.WriteString("---\n")
	fmt.Fprintf(&b, "name: %s\n", e.Name)
	fmt.Fprintf(&b, "description: %s\n", e.Description)
	fmt.Fprintf(&b, "type: %s\n", e.Type)
	b.WriteString("---\n\n")
	b.WriteString(e.Body)
	if !strings.HasSuffix(e.Body, "\n") {
		b.WriteString("\n")
	}
	return b.String()
}

// parseMarkdownEntry parses the canonical on-disk format back into a MemoryEntry.
func parseMarkdownEntry(content string) (MemoryEntry, error) {
	if !strings.HasPrefix(content, "---\n") {
		return MemoryEntry{}, fmt.Errorf("missing frontmatter")
	}
	rest := content[4:]
	end := strings.Index(rest, "\n---\n")
	if end < 0 {
		return MemoryEntry{}, fmt.Errorf("unclosed frontmatter")
	}
	frontmatter := rest[:end]
	body := strings.TrimPrefix(rest[end+5:], "\n")

	entry := MemoryEntry{Body: body}
	for _, line := range strings.Split(frontmatter, "\n") {
		if k, v, ok := strings.Cut(line, ": "); ok {
			switch strings.TrimSpace(k) {
			case "name":
				entry.Name = strings.TrimSpace(v)
			case "description":
				entry.Description = strings.TrimSpace(v)
			case "type":
				entry.Type = strings.TrimSpace(v)
			}
		}
	}
	if entry.Name == "" || entry.Type == "" {
		return MemoryEntry{}, fmt.Errorf("missing required frontmatter fields")
	}
	return entry, nil
}

// ---------------------------------------------------------------------------
// DB helpers
// ---------------------------------------------------------------------------

func scanEntry(row *sql.Row) (MemoryEntry, error) {
	var e MemoryEntry
	var createdStr, updatedStr string
	var expiresStr *string
	err := row.Scan(&e.ID, &e.WorkspaceID, &e.Type, &e.Name, &e.Description, &e.Body,
		&createdStr, &updatedStr, &expiresStr)
	if err != nil {
		return e, err
	}
	e.CreatedAt, _ = time.Parse(time.RFC3339, createdStr)
	e.UpdatedAt, _ = time.Parse(time.RFC3339, updatedStr)
	if expiresStr != nil {
		t, _ := time.Parse(time.RFC3339, *expiresStr)
		e.ExpiresAt = &t
	}
	return e, nil
}

func collectEntries(rows *sql.Rows) ([]MemoryEntry, error) {
	var result []MemoryEntry
	for rows.Next() {
		var e MemoryEntry
		var createdStr, updatedStr string
		var expiresStr *string
		if err := rows.Scan(&e.ID, &e.WorkspaceID, &e.Type, &e.Name, &e.Description, &e.Body,
			&createdStr, &updatedStr, &expiresStr); err != nil {
			return nil, err
		}
		e.CreatedAt, _ = time.Parse(time.RFC3339, createdStr)
		e.UpdatedAt, _ = time.Parse(time.RFC3339, updatedStr)
		if expiresStr != nil {
			t, _ := time.Parse(time.RFC3339, *expiresStr)
			e.ExpiresAt = &t
		}
		result = append(result, e)
	}
	return result, rows.Err()
}

// newEntryID generates a short unique ID for a new entry.
func newEntryID() string {
	// Use timestamp + random suffix for uniqueness.
	return fmt.Sprintf("%x", time.Now().UnixNano())
}
