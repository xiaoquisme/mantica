package execenv

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestMemoryDBOpenAndMigrate(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "memory.db")
	memDir := filepath.Join(dir, "memory")
	if err := os.MkdirAll(memDir, 0o755); err != nil {
		t.Fatal(err)
	}

	db, err := OpenMemoryDB(dbPath, "ws-test", memDir)
	if err != nil {
		t.Fatalf("OpenMemoryDB failed: %v", err)
	}
	defer db.Close()

	// Verify the DB file was created.
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		t.Fatal("memory.db not created")
	}
}

func TestMemoryDBAddAndList(t *testing.T) {
	t.Parallel()
	db := openTestMemoryDB(t)

	entry := MemoryEntry{
		Type:        "feedback",
		Name:        "Test entry",
		Description: "A test memory entry",
		Body:        "Body content here.\n\n**Why:** testing.\n**How to apply:** always.",
	}

	saved, err := db.Add(entry)
	if err != nil {
		t.Fatalf("Add failed: %v", err)
	}
	if saved.ID == "" {
		t.Error("expected non-empty ID")
	}
	if saved.Name != entry.Name {
		t.Errorf("name = %q, want %q", saved.Name, entry.Name)
	}

	entries, err := db.List("", 10)
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	if entries[0].Name != entry.Name {
		t.Errorf("listed entry name = %q, want %q", entries[0].Name, entry.Name)
	}
}

func TestMemoryDBListFilterByType(t *testing.T) {
	t.Parallel()
	db := openTestMemoryDB(t)

	for _, e := range []MemoryEntry{
		{Type: "feedback", Name: "fb1", Description: "d1", Body: "b1"},
		{Type: "project", Name: "proj1", Description: "d2", Body: "b2"},
		{Type: "user", Name: "user1", Description: "d3", Body: "b3"},
	} {
		if _, err := db.Add(e); err != nil {
			t.Fatalf("Add failed: %v", err)
		}
	}

	feedbacks, err := db.List("feedback", 10)
	if err != nil {
		t.Fatalf("List feedback failed: %v", err)
	}
	if len(feedbacks) != 1 || feedbacks[0].Name != "fb1" {
		t.Errorf("expected 1 feedback entry 'fb1', got %v", feedbacks)
	}

	all, err := db.List("", 10)
	if err != nil {
		t.Fatalf("List all failed: %v", err)
	}
	if len(all) != 3 {
		t.Errorf("expected 3 entries, got %d", len(all))
	}
}

func TestMemoryDBUpsertByName(t *testing.T) {
	t.Parallel()
	db := openTestMemoryDB(t)

	first, err := db.Add(MemoryEntry{
		Type: "feedback", Name: "same-name", Description: "original", Body: "original body",
	})
	if err != nil {
		t.Fatalf("first Add failed: %v", err)
	}

	// Add with same name — should update, not duplicate.
	second, err := db.Add(MemoryEntry{
		Type: "feedback", Name: "same-name", Description: "updated", Body: "updated body",
	})
	if err != nil {
		t.Fatalf("second Add failed: %v", err)
	}
	if second.ID != first.ID {
		t.Errorf("expected same ID after upsert, first=%s second=%s", first.ID, second.ID)
	}

	all, err := db.List("", 10)
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if len(all) != 1 {
		t.Fatalf("expected 1 entry after upsert, got %d", len(all))
	}
	if all[0].Description != "updated" {
		t.Errorf("description = %q, want %q", all[0].Description, "updated")
	}
}

func TestMemoryDBSearch(t *testing.T) {
	t.Parallel()
	db := openTestMemoryDB(t)

	if _, err := db.Add(MemoryEntry{
		Type: "feedback", Name: "SQLite best practices",
		Description: "Use WAL mode for concurrent access",
		Body:        "Always use WAL mode when multiple processes access the same SQLite database.",
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Add(MemoryEntry{
		Type: "project", Name: "Q2 planning",
		Description: "Q2 roadmap milestones",
		Body:        "Focus on performance improvements and new API endpoints.",
	}); err != nil {
		t.Fatal(err)
	}

	// Search should find the SQLite entry.
	results, err := db.Search("WAL mode", 10)
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}
	if len(results) == 0 {
		t.Fatal("expected at least 1 search result for 'WAL mode'")
	}
	if results[0].Name != "SQLite best practices" {
		t.Errorf("expected 'SQLite best practices', got %q", results[0].Name)
	}
}

func TestMemoryDBDelete(t *testing.T) {
	t.Parallel()
	db := openTestMemoryDB(t)

	saved, err := db.Add(MemoryEntry{
		Type: "reference", Name: "to-delete", Description: "d", Body: "b",
	})
	if err != nil {
		t.Fatal(err)
	}

	if err := db.Delete(saved.ID); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	all, err := db.List("", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 0 {
		t.Errorf("expected 0 entries after delete, got %d", len(all))
	}
}

func TestMemoryDBGC(t *testing.T) {
	t.Parallel()
	db := openTestMemoryDB(t)

	// Add entry with expiry in the past.
	past := time.Now().UTC().Add(-time.Hour)
	if _, err := db.Add(MemoryEntry{
		Type: "project", Name: "expired-entry", Description: "d", Body: "b",
		ExpiresAt: &past,
	}); err != nil {
		t.Fatal(err)
	}
	// Add non-expired entry.
	if _, err := db.Add(MemoryEntry{
		Type: "user", Name: "live-entry", Description: "d", Body: "b",
	}); err != nil {
		t.Fatal(err)
	}

	n, err := db.GC(0)
	if err != nil {
		t.Fatalf("GC failed: %v", err)
	}
	if n != 1 {
		t.Errorf("expected 1 deleted by GC, got %d", n)
	}

	remaining, err := db.List("", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(remaining) != 1 || remaining[0].Name != "live-entry" {
		t.Errorf("unexpected remaining entries: %v", remaining)
	}
}

func TestMemoryDBMarkdownExport(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "memory.db")
	memDir := filepath.Join(dir, "memory")
	os.MkdirAll(memDir, 0o755)

	db, err := OpenMemoryDB(dbPath, "ws-export", memDir)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	saved, err := db.Add(MemoryEntry{
		Type:        "feedback",
		Name:        "Export test entry",
		Description: "Tests markdown export",
		Body:        "This should appear in the markdown file.",
	})
	if err != nil {
		t.Fatal(err)
	}

	// Verify entry markdown file was created.
	mdPath := filepath.Join(memDir, "entries", saved.ID+".md")
	data, err := os.ReadFile(mdPath)
	if err != nil {
		t.Fatalf("markdown file not created at %s: %v", mdPath, err)
	}
	content := string(data)
	for _, want := range []string{
		"---",
		"name: Export test entry",
		"description: Tests markdown export",
		"type: feedback",
		"This should appear in the markdown file.",
	} {
		if !strings.Contains(content, want) {
			t.Errorf("markdown missing %q", want)
		}
	}

	// Verify MEMORY.md index was created.
	indexPath := filepath.Join(memDir, "MEMORY.md")
	indexData, err := os.ReadFile(indexPath)
	if err != nil {
		t.Fatalf("MEMORY.md not created: %v", err)
	}
	indexContent := string(indexData)
	if !strings.Contains(indexContent, "Export test entry") {
		t.Errorf("MEMORY.md missing entry name: %s", indexContent)
	}
	if !strings.Contains(indexContent, "Tests markdown export") {
		t.Errorf("MEMORY.md missing entry description: %s", indexContent)
	}
}

func TestMemoryDBMigrateFromFiles(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "memory.db")
	memDir := filepath.Join(dir, "memory")
	os.MkdirAll(memDir, 0o755)

	// Write legacy markdown files.
	legacyFeedback := `---
name: Use pgx/v5 driver
description: All DB access must use pgx/v5 directly
type: feedback
---

Use pgx/v5 for all database access.

**Why:** performance and type safety.
**How to apply:** any new DB file.
`
	legacyProject := `---
name: Q2 freeze
description: Merge freeze starts 2026-05-07
type: project
---

No non-critical merges during Q2 mobile cut.

**Why:** mobile release stability.
**How to apply:** check before merging.
`
	os.WriteFile(filepath.Join(memDir, "feedback_db.md"), []byte(legacyFeedback), 0o644)
	os.WriteFile(filepath.Join(memDir, "project_q2.md"), []byte(legacyProject), 0o644)
	// MEMORY.md should be ignored.
	os.WriteFile(filepath.Join(memDir, "MEMORY.md"), []byte("- [old](old.md) — old\n"), 0o644)

	db, err := OpenMemoryDB(dbPath, "ws-migrate", memDir)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	n, err := db.MigrateFromFiles()
	if err != nil {
		t.Fatalf("MigrateFromFiles failed: %v", err)
	}
	if n != 2 {
		t.Errorf("expected 2 migrated entries, got %d", n)
	}

	entries, err := db.List("", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 {
		t.Fatalf("expected 2 DB entries after migration, got %d", len(entries))
	}

	names := map[string]bool{}
	for _, e := range entries {
		names[e.Name] = true
	}
	if !names["Use pgx/v5 driver"] {
		t.Error("expected 'Use pgx/v5 driver' in migrated entries")
	}
	if !names["Q2 freeze"] {
		t.Error("expected 'Q2 freeze' in migrated entries")
	}
}

func TestMemoryDBWorkspaceIsolation(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	// Open two DBs for different workspaces using same file (simulates multi-tenant).
	dbPath := filepath.Join(dir, "memory.db")

	dbA, err := OpenMemoryDB(dbPath, "ws-A", filepath.Join(dir, "memA"))
	if err != nil {
		t.Fatal(err)
	}
	defer dbA.Close()
	os.MkdirAll(filepath.Join(dir, "memA"), 0o755)

	dbB, err := OpenMemoryDB(dbPath, "ws-B", filepath.Join(dir, "memB"))
	if err != nil {
		t.Fatal(err)
	}
	defer dbB.Close()
	os.MkdirAll(filepath.Join(dir, "memB"), 0o755)

	// Add entry to workspace A.
	if _, err := dbA.Add(MemoryEntry{Type: "user", Name: "ws-a-entry", Description: "d", Body: "b"}); err != nil {
		t.Fatal(err)
	}

	// Workspace B should not see workspace A's entries.
	bEntries, err := dbB.List("", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(bEntries) != 0 {
		t.Errorf("workspace B should see 0 entries from ws-A, got %d", len(bEntries))
	}
}

func TestMemoryDBRebuildIndexContainsAllTypes(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "memory.db")
	memDir := filepath.Join(dir, "memory")
	os.MkdirAll(memDir, 0o755)

	db, err := OpenMemoryDB(dbPath, "ws-index", memDir)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	for _, e := range []MemoryEntry{
		{Type: "user", Name: "User pref", Description: "user desc", Body: "user body"},
		{Type: "feedback", Name: "Feedback note", Description: "fb desc", Body: "fb body"},
		{Type: "project", Name: "Project milestone", Description: "proj desc", Body: "proj body"},
		{Type: "reference", Name: "API dashboard", Description: "ref desc", Body: "ref body"},
	} {
		if _, err := db.Add(e); err != nil {
			t.Fatalf("Add %s failed: %v", e.Name, err)
		}
	}

	indexData, err := os.ReadFile(filepath.Join(memDir, "MEMORY.md"))
	if err != nil {
		t.Fatalf("MEMORY.md not found: %v", err)
	}
	index := string(indexData)

	for _, want := range []string{
		"## user", "User pref", "user desc",
		"## feedback", "Feedback note", "fb desc",
		"## project", "Project milestone", "proj desc",
		"## reference", "API dashboard", "ref desc",
	} {
		if !strings.Contains(index, want) {
			t.Errorf("MEMORY.md missing %q", want)
		}
	}
}

// openTestMemoryDB is a helper that opens a MemoryDB in a temp dir.
func openTestMemoryDB(t *testing.T) *MemoryDB {
	t.Helper()
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "memory.db")
	memDir := filepath.Join(dir, "memory")
	os.MkdirAll(memDir, 0o755)
	db, err := OpenMemoryDB(dbPath, "ws-test", memDir)
	if err != nil {
		t.Fatalf("openTestMemoryDB: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}
