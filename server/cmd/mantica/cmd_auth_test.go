package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/cobra"

	"github.com/xiaoquisme/mantica/server/internal/cli"
)

// testCmd returns a minimal cobra.Command with the --profile persistent flag
// registered, matching the rootCmd setup used in production.
func testCmd() *cobra.Command {
	cmd := &cobra.Command{}
	cmd.PersistentFlags().String("profile", "", "")
	return cmd
}

// writeTestConfig saves a CLIConfig JSON file into the given home directory so
// that LoadCLIConfigForProfile picks it up during tests.
func writeTestConfig(t *testing.T, home string, cfg cli.CLIConfig) {
	t.Helper()
	dir := filepath.Join(home, ".mantica")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", dir, err)
	}
	data, err := json.Marshal(cfg)
	if err != nil {
		t.Fatalf("marshal config: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "config.json"), data, 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
}

func TestResolveAppURL(t *testing.T) {
	cmd := testCmd()

	t.Run("prefers MANTICA_APP_URL", func(t *testing.T) {
		t.Setenv("MANTICA_APP_URL", "http://localhost:14000")
		t.Setenv("FRONTEND_ORIGIN", "http://localhost:13000")

		if got := resolveAppURL(cmd); got != "http://localhost:14000" {
			t.Fatalf("resolveAppURL() = %q, want %q", got, "http://localhost:14000")
		}
	})

	t.Run("falls back to FRONTEND_ORIGIN", func(t *testing.T) {
		t.Setenv("MANTICA_APP_URL", "")
		t.Setenv("FRONTEND_ORIGIN", "http://localhost:13026")

		if got := resolveAppURL(cmd); got != "http://localhost:13026" {
			t.Fatalf("resolveAppURL() = %q, want %q", got, "http://localhost:13026")
		}
	})

	t.Run("falls back to server_url from config when no app_url set", func(t *testing.T) {
		t.Setenv("MANTICA_APP_URL", "")
		t.Setenv("FRONTEND_ORIGIN", "")
		home := t.TempDir()
		t.Setenv("HOME", home)
		writeTestConfig(t, home, cli.CLIConfig{ServerURL: "http://localhost:8080"})

		if got := resolveAppURL(cmd); got != "http://localhost:8080" {
			t.Fatalf("resolveAppURL() = %q, want %q", got, "http://localhost:8080")
		}
	})

	t.Run("prefers app_url from config over server_url", func(t *testing.T) {
		t.Setenv("MANTICA_APP_URL", "")
		t.Setenv("FRONTEND_ORIGIN", "")
		home := t.TempDir()
		t.Setenv("HOME", home)
		writeTestConfig(t, home, cli.CLIConfig{
			ServerURL: "http://localhost:8080",
			AppURL:    "http://localhost:3000",
		})

		if got := resolveAppURL(cmd); got != "http://localhost:3000" {
			t.Fatalf("resolveAppURL() = %q, want %q", got, "http://localhost:3000")
		}
	})

	t.Run("defaults to production", func(t *testing.T) {
		t.Setenv("MANTICA_APP_URL", "")
		t.Setenv("FRONTEND_ORIGIN", "")
		t.Setenv("HOME", t.TempDir()) // avoid reading real config

		if got := resolveAppURL(cmd); got != "https://multica.ai" {
			t.Fatalf("resolveAppURL() = %q, want %q", got, "https://multica.ai")
		}
	})
}

func TestNormalizeAPIBaseURL(t *testing.T) {
	t.Run("converts websocket base URL", func(t *testing.T) {
		if got := normalizeAPIBaseURL("ws://localhost:18106/ws"); got != "http://localhost:18106" {
			t.Fatalf("normalizeAPIBaseURL() = %q, want %q", got, "http://localhost:18106")
		}
	})

	t.Run("keeps http base URL", func(t *testing.T) {
		if got := normalizeAPIBaseURL("http://localhost:8080"); got != "http://localhost:8080" {
			t.Fatalf("normalizeAPIBaseURL() = %q, want %q", got, "http://localhost:8080")
		}
	})

	t.Run("falls back to raw value for invalid URL", func(t *testing.T) {
		if got := normalizeAPIBaseURL("://bad-url"); got != "://bad-url" {
			t.Fatalf("normalizeAPIBaseURL() = %q, want %q", got, "://bad-url")
		}
	})
}
