package main

// cmd_memory.go — multica memory subcommand group.
//
// Commands operate on the workspace-level SQLite memory database at
//   {WorkspacesRoot}/{WorkspaceID}/memory.db
//
// WorkspacesRoot is resolved as:
//   MANTICA_WORKSPACES_ROOT env var > ~/mantica_workspaces_<profile> > ~/mantica_workspaces
//
// WorkspaceID is resolved from MANTICA_WORKSPACE_ID env var or --workspace-id flag
// (same as other CLI commands).
//
// When running inside a task workdir, agents can always reach the correct DB
// because WorkspaceID is injected by the daemon via MANTICA_WORKSPACE_ID.

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/xiaoquisme/mantica/server/internal/cli"
	"github.com/xiaoquisme/mantica/server/internal/daemon/execenv"
)

var memoryCmd = &cobra.Command{
	Use:   "memory",
	Short: "Manage workspace agent memory",
}

var memoryListCmd = &cobra.Command{
	Use:   "list",
	Short: "List memory entries",
	RunE:  runMemoryList,
}

var memorySearchCmd = &cobra.Command{
	Use:   "search <query>",
	Short: "Full-text search across memory entries",
	Args:  exactArgs(1),
	RunE:  runMemorySearch,
}

var memoryAddCmd = &cobra.Command{
	Use:   "add",
	Short: "Add or update a memory entry",
	RunE:  runMemoryAdd,
}

var memoryGCCmd = &cobra.Command{
	Use:   "gc",
	Short: "Delete expired and optionally old memory entries",
	RunE:  runMemoryGC,
}

var memoryMigrateCmd = &cobra.Command{
	Use:   "migrate",
	Short: "Import legacy memory/*.md files into the SQLite database",
	RunE:  runMemoryMigrate,
}

func init() {
	memoryCmd.AddCommand(memoryListCmd)
	memoryCmd.AddCommand(memorySearchCmd)
	memoryCmd.AddCommand(memoryAddCmd)
	memoryCmd.AddCommand(memoryGCCmd)
	memoryCmd.AddCommand(memoryMigrateCmd)

	// memory list
	memoryListCmd.Flags().String("type", "", "Filter by type: user, feedback, project, reference")
	memoryListCmd.Flags().Int("limit", 50, "Maximum number of entries to return")
	memoryListCmd.Flags().String("output", "table", "Output format: table or json")

	// memory search
	memorySearchCmd.Flags().Int("limit", 20, "Maximum number of results")
	memorySearchCmd.Flags().String("output", "table", "Output format: table or json")

	// memory add
	memoryAddCmd.Flags().String("type", "", "Entry type: user, feedback, project, reference (required)")
	memoryAddCmd.Flags().String("name", "", "Entry name — short human-friendly title (required)")
	memoryAddCmd.Flags().String("description", "", "One-line relevance hook (required)")
	memoryAddCmd.Flags().String("body", "", "Entry body (markdown)")
	memoryAddCmd.Flags().String("expires-after", "", "TTL duration, e.g. 30d, 7d, 90d")
	memoryAddCmd.Flags().String("output", "json", "Output format: table or json")

	// memory gc
	memoryGCCmd.Flags().String("older-than", "", "Also delete project/feedback entries older than this duration, e.g. 30d")
	memoryGCCmd.Flags().String("output", "table", "Output format: table or json")

	// memory migrate — no extra flags needed
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// resolveWorkspacesRoot returns the workspaces root directory using the same
// logic as the daemon config loader so paths stay consistent.
func resolveWorkspacesRoot(cmd *cobra.Command) (string, error) {
	if v := strings.TrimSpace(os.Getenv("MANTICA_WORKSPACES_ROOT")); v != "" {
		return v, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve home directory: %w", err)
	}
	profile := resolveProfile(cmd)
	if profile != "" {
		return filepath.Join(home, "mantica_workspaces_"+profile), nil
	}
	return filepath.Join(home, "mantica_workspaces"), nil
}

// openMemoryDB resolves the workspace and opens the memory database.
func openMemoryDB(cmd *cobra.Command) (*execenv.MemoryDB, error) {
	workspaceID := resolveWorkspaceID(cmd)
	if workspaceID == "" {
		return nil, fmt.Errorf("workspace ID is required: pass --workspace-id or set MANTICA_WORKSPACE_ID")
	}
	root, err := resolveWorkspacesRoot(cmd)
	if err != nil {
		return nil, err
	}
	return execenv.WorkspaceMemoryDB(root, workspaceID)
}

// parseDuration parses durations like "30d", "7d", "24h".
func parseDuration(s string) (time.Duration, error) {
	if s == "" {
		return 0, nil
	}
	// Handle day suffix.
	if strings.HasSuffix(s, "d") {
		days := strings.TrimSuffix(s, "d")
		var n int
		if _, err := fmt.Sscanf(days, "%d", &n); err != nil || n <= 0 {
			return 0, fmt.Errorf("invalid duration %q (expected e.g. 30d)", s)
		}
		return time.Duration(n) * 24 * time.Hour, nil
	}
	return time.ParseDuration(s)
}

// ---------------------------------------------------------------------------
// Command implementations
// ---------------------------------------------------------------------------

func runMemoryList(cmd *cobra.Command, _ []string) error {
	db, err := openMemoryDB(cmd)
	if err != nil {
		return err
	}
	defer db.Close()

	entryType, _ := cmd.Flags().GetString("type")
	limit, _ := cmd.Flags().GetInt("limit")

	entries, err := db.List(entryType, limit)
	if err != nil {
		return fmt.Errorf("list memory: %w", err)
	}

	output, _ := cmd.Flags().GetString("output")
	if output == "json" {
		return cli.PrintJSON(os.Stdout, entries)
	}

	if len(entries) == 0 {
		fmt.Fprintln(os.Stderr, "No memory entries found.")
		return nil
	}

	headers := []string{"ID", "TYPE", "NAME", "DESCRIPTION", "UPDATED_AT"}
	rows := make([][]string, 0, len(entries))
	for _, e := range entries {
		rows = append(rows, []string{
			e.ID,
			e.Type,
			truncate(e.Name, 40),
			truncate(e.Description, 60),
			e.UpdatedAt.Format("2006-01-02 15:04"),
		})
	}
	cli.PrintTable(os.Stdout, headers, rows)
	return nil
}

func runMemorySearch(cmd *cobra.Command, args []string) error {
	db, err := openMemoryDB(cmd)
	if err != nil {
		return err
	}
	defer db.Close()

	limit, _ := cmd.Flags().GetInt("limit")
	entries, err := db.Search(args[0], limit)
	if err != nil {
		return fmt.Errorf("search memory: %w", err)
	}

	output, _ := cmd.Flags().GetString("output")
	if output == "json" {
		return cli.PrintJSON(os.Stdout, entries)
	}

	if len(entries) == 0 {
		fmt.Fprintln(os.Stderr, "No results found.")
		return nil
	}

	headers := []string{"ID", "TYPE", "NAME", "DESCRIPTION"}
	rows := make([][]string, 0, len(entries))
	for _, e := range entries {
		rows = append(rows, []string{
			e.ID,
			e.Type,
			truncate(e.Name, 40),
			truncate(e.Description, 70),
		})
	}
	cli.PrintTable(os.Stdout, headers, rows)
	return nil
}

func runMemoryAdd(cmd *cobra.Command, _ []string) error {
	db, err := openMemoryDB(cmd)
	if err != nil {
		return err
	}
	defer db.Close()

	entryType, _ := cmd.Flags().GetString("type")
	name, _ := cmd.Flags().GetString("name")
	description, _ := cmd.Flags().GetString("description")
	body, _ := cmd.Flags().GetString("body")
	expiresAfterStr, _ := cmd.Flags().GetString("expires-after")

	if entryType == "" {
		return fmt.Errorf("--type is required (user, feedback, project, reference)")
	}
	if name == "" {
		return fmt.Errorf("--name is required")
	}
	if description == "" {
		return fmt.Errorf("--description is required")
	}

	entry := execenv.MemoryEntry{
		Type:        entryType,
		Name:        name,
		Description: description,
		Body:        body,
	}

	if expiresAfterStr != "" {
		d, err := parseDuration(expiresAfterStr)
		if err != nil {
			return err
		}
		t := time.Now().UTC().Add(d)
		entry.ExpiresAt = &t
	}

	saved, err := db.Add(entry)
	if err != nil {
		return fmt.Errorf("add memory entry: %w", err)
	}

	output, _ := cmd.Flags().GetString("output")
	if output == "json" {
		return cli.PrintJSON(os.Stdout, saved)
	}
	fmt.Printf("Memory entry saved: %s (%s)\n", saved.Name, saved.ID)
	return nil
}

func runMemoryGC(cmd *cobra.Command, _ []string) error {
	db, err := openMemoryDB(cmd)
	if err != nil {
		return err
	}
	defer db.Close()

	olderThanStr, _ := cmd.Flags().GetString("older-than")
	olderThan, err := parseDuration(olderThanStr)
	if err != nil {
		return err
	}

	n, err := db.GC(olderThan)
	if err != nil {
		return fmt.Errorf("memory gc: %w", err)
	}

	fmt.Fprintf(os.Stdout, "Deleted %d memory entries.\n", n)
	return nil
}

func runMemoryMigrate(cmd *cobra.Command, _ []string) error {
	db, err := openMemoryDB(cmd)
	if err != nil {
		return err
	}
	defer db.Close()

	n, err := db.MigrateFromFiles()
	if err != nil {
		return fmt.Errorf("memory migrate: %w", err)
	}
	fmt.Fprintf(os.Stdout, "Migrated %d memory entries from markdown files.\n", n)
	return nil
}

// truncate shortens a string to at most n runes and appends "…" if truncated.
func truncate(s string, n int) string {
	runes := []rune(s)
	if len(runes) <= n {
		return s
	}
	return string(runes[:n-1]) + "…"
}
