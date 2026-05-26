package daemon

import (
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

const (
	DefaultServerURL             = "ws://localhost:8080/ws"
	DefaultPollInterval          = 3 * time.Second
	DefaultHeartbeatInterval     = 15 * time.Second
	DefaultAgentTimeout          = 2 * time.Hour
	DefaultRuntimeName           = "Local Agent"
	DefaultConfigReloadInterval  = 5 * time.Second
	DefaultWorkspaceSyncInterval = 30 * time.Second
	DefaultHealthPort            = 19514
	DefaultMaxConcurrentTasks    = 20
)

// Config holds all daemon configuration.
type Config struct {
	ServerBaseURL      string
	DatabaseURL        string // PostgreSQL connection string for context cache
	DaemonID           string
	DeviceName         string
	RuntimeName        string
	CLIVersion         string                // mantica CLI version (e.g. "0.1.13")
	Profile            string                // profile name (empty = default)
	Agents             map[string]AgentEntry // "claude" -> entry, "codex" -> entry, "opencode" -> entry, "openclaw" -> entry, "hermes" -> entry
	WorkspacesRoot     string                // base path for execution envs (default: ~/mantica_workspaces)
	KeepEnvAfterTask   bool                  // preserve env after task for debugging
	HealthPort         int                   // local HTTP port for health checks (default: 19514)
	MaxConcurrentTasks int                   // max tasks running in parallel (default: 20)
	PollInterval       time.Duration
	HeartbeatInterval  time.Duration
	AgentTimeout       time.Duration
}

// Overrides allows CLI flags to override environment variables and defaults.
// Zero values are ignored and the env/default value is used instead.
type Overrides struct {
	ServerURL          string
	WorkspacesRoot     string
	PollInterval       time.Duration
	HeartbeatInterval  time.Duration
	AgentTimeout       time.Duration
	MaxConcurrentTasks int
	DaemonID           string
	DeviceName         string
	RuntimeName        string
	Profile            string // profile name (empty = default)
	HealthPort         int    // health check port (0 = use default)
}

// LoadConfig builds the daemon configuration from environment variables
// and optional CLI flag overrides.
func LoadConfig(overrides Overrides) (Config, error) {
	// Server URL: override > env > default
	rawServerURL := envOrDefault("MANTICA_SERVER_URL", DefaultServerURL)
	if overrides.ServerURL != "" {
		rawServerURL = overrides.ServerURL
	}
	serverBaseURL, err := NormalizeServerBaseURL(rawServerURL)
	if err != nil {
		return Config{}, err
	}

	// Database URL for context cache (optional)
	databaseURL := os.Getenv("DATABASE_URL")

	// Probe available agent CLIs
	agents := map[string]AgentEntry{}
	claudePath := envOrDefault("MANTICA_CLAUDE_PATH", "claude")
	if _, err := exec.LookPath(claudePath); err == nil {
		claudeEntry := AgentEntry{
			Path:  claudePath,
			Model: strings.TrimSpace(os.Getenv("MANTICA_CLAUDE_MODEL")),
			AvailableModels: []string{
				"claude-sonnet-4-6", "claude-opus-4-6", "claude-haiku-3-6",
			},
		}
		// Inject env vars for the claude process in two complementary ways:
		//
		// 1. Named shorthands (kept for backward compat):
		//    MANTICA_CLAUDE_API_KEY  → ANTHROPIC_API_KEY
		//    MANTICA_CLAUDE_BASE_URL → ANTHROPIC_BASE_URL
		//
		// 2. Generic passthrough prefix — any env var of the form
		//    MANTICA_CLAUDE_ENV_<NAME>=value is forwarded to the claude process
		//    as <NAME>=value.  This covers Vertex AI mode and any other auth
		//    scheme without requiring new named shorthands:
		//      MANTICA_CLAUDE_ENV_ANTHROPIC_AUTH_TOKEN=...
		//      MANTICA_CLAUDE_ENV_ANTHROPIC_VERTEX_BASE_URL=...
		//      MANTICA_CLAUDE_ENV_ANTHROPIC_VERTEX_PROJECT_ID=...
		//      MANTICA_CLAUDE_ENV_CLAUDE_CODE_USE_VERTEX=1
		//      MANTICA_CLAUDE_ENV_CLAUDE_CODE_SKIP_VERTEX_AUTH=1
		//    etc.
		//
		// All of these take precedence over whatever the daemon process inherited,
		// so credentials always reach the agent CLI even when it runs as a service.
		if apiKey := strings.TrimSpace(os.Getenv("MANTICA_CLAUDE_API_KEY")); apiKey != "" {
			if claudeEntry.Env == nil {
				claudeEntry.Env = map[string]string{}
			}
			claudeEntry.Env["ANTHROPIC_API_KEY"] = apiKey
		}
		if baseURL := strings.TrimSpace(os.Getenv("MANTICA_CLAUDE_BASE_URL")); baseURL != "" {
			if claudeEntry.Env == nil {
				claudeEntry.Env = map[string]string{}
			}
			claudeEntry.Env["ANTHROPIC_BASE_URL"] = baseURL
		}
		const claudeEnvPrefix = "MANTICA_CLAUDE_ENV_"
		for _, kv := range os.Environ() {
			if !strings.HasPrefix(kv, claudeEnvPrefix) {
				continue
			}
			eq := strings.IndexByte(kv, '=')
			if eq < 0 {
				continue
			}
			targetKey := kv[len(claudeEnvPrefix):eq]
			targetVal := kv[eq+1:]
			if targetKey == "" {
				continue
			}
			if claudeEntry.Env == nil {
				claudeEntry.Env = map[string]string{}
			}
			claudeEntry.Env[targetKey] = targetVal
		}
		agents["claude"] = claudeEntry
	}
	codexPath := envOrDefault("MANTICA_CODEX_PATH", "codex")
	if _, err := exec.LookPath(codexPath); err == nil {
		agents["codex"] = AgentEntry{
			Path:  codexPath,
			Model: strings.TrimSpace(os.Getenv("MANTICA_CODEX_MODEL")),
			AvailableModels: []string{
				"gpt-5.2", "gpt-5.2-mini", "gpt-5.2-codex", "o3", "o3-mini", "o4-mini",
			},
		}
	}
	opencodePath := envOrDefault("MANTICA_OPENCODE_PATH", "opencode")
	if _, err := exec.LookPath(opencodePath); err == nil {
		agents["opencode"] = AgentEntry{
			Path:  opencodePath,
			Model: strings.TrimSpace(os.Getenv("MANTICA_OPENCODE_MODEL")),
			AvailableModels: []string{
				"claude-sonnet-4-6", "claude-opus-4-6", "gpt-5.2", "gpt-5.2-mini", "o3", "o3-mini",
			},
		}
	}
	openclawPath := envOrDefault("MANTICA_OPENCLAW_PATH", "openclaw")
	if _, err := exec.LookPath(openclawPath); err == nil {
		agents["openclaw"] = AgentEntry{
			Path:  openclawPath,
			Model: strings.TrimSpace(os.Getenv("MANTICA_OPENCLAW_MODEL")),
			AvailableModels: []string{
				"claude-sonnet-4-6", "claude-opus-4-6", "gpt-5.2", "gpt-5.2-mini", "o3", "o3-mini",
			},
		}
	}
	hermesPath := envOrDefault("MANTICA_HERMES_PATH", "hermes")
	if _, err := exec.LookPath(hermesPath); err == nil {
		hermesEntry := AgentEntry{
			Path:  hermesPath,
			Model: strings.TrimSpace(os.Getenv("MANTICA_HERMES_MODEL")),
		}
		if hermesProfile := strings.TrimSpace(os.Getenv("MANTICA_HERMES_PROFILE")); hermesProfile != "" {
			hermesEntry.ExtraArgs = []string{"--profile", hermesProfile}
		}
		agents["hermes"] = hermesEntry
	}
	if len(agents) == 0 {
		return Config{}, fmt.Errorf("no agent CLI found: install claude, codex, opencode, openclaw, or hermes and ensure it is on PATH")
	}

	// Host info
	host, err := os.Hostname()
	if err != nil || strings.TrimSpace(host) == "" {
		host = "local-machine"
	}

	// Durations: override > env > default
	pollInterval, err := durationFromEnv("MANTICA_DAEMON_POLL_INTERVAL", DefaultPollInterval)
	if err != nil {
		return Config{}, err
	}
	if overrides.PollInterval > 0 {
		pollInterval = overrides.PollInterval
	}

	heartbeatInterval, err := durationFromEnv("MANTICA_DAEMON_HEARTBEAT_INTERVAL", DefaultHeartbeatInterval)
	if err != nil {
		return Config{}, err
	}
	if overrides.HeartbeatInterval > 0 {
		heartbeatInterval = overrides.HeartbeatInterval
	}

	agentTimeout, err := durationFromEnv("MANTICA_AGENT_TIMEOUT", DefaultAgentTimeout)
	if err != nil {
		return Config{}, err
	}
	if overrides.AgentTimeout > 0 {
		agentTimeout = overrides.AgentTimeout
	}

	maxConcurrentTasks, err := intFromEnv("MANTICA_DAEMON_MAX_CONCURRENT_TASKS", DefaultMaxConcurrentTasks)
	if err != nil {
		return Config{}, err
	}
	if overrides.MaxConcurrentTasks > 0 {
		maxConcurrentTasks = overrides.MaxConcurrentTasks
	}

	// Profile
	profile := overrides.Profile

	// String overrides
	daemonID := envOrDefault("MANTICA_DAEMON_ID", host)
	if overrides.DaemonID != "" {
		daemonID = overrides.DaemonID
	}
	// Suffix daemon ID with profile name to avoid collisions when multiple
	// daemons register against the same server.
	if profile != "" && !strings.HasSuffix(daemonID, "-"+profile) {
		daemonID = daemonID + "-" + profile
	}

	deviceName := envOrDefault("MANTICA_DAEMON_DEVICE_NAME", host)
	if overrides.DeviceName != "" {
		deviceName = overrides.DeviceName
	}

	runtimeName := envOrDefault("MANTICA_AGENT_RUNTIME_NAME", DefaultRuntimeName)
	if overrides.RuntimeName != "" {
		runtimeName = overrides.RuntimeName
	}

	// Workspaces root: override > env > default (~/mantica_workspaces or ~/mantica_workspaces_<profile>)
	workspacesRoot := strings.TrimSpace(os.Getenv("MANTICA_WORKSPACES_ROOT"))
	if overrides.WorkspacesRoot != "" {
		workspacesRoot = overrides.WorkspacesRoot
	}
	if workspacesRoot == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return Config{}, fmt.Errorf("resolve home directory: %w (set MANTICA_WORKSPACES_ROOT to override)", err)
		}
		if profile != "" {
			workspacesRoot = filepath.Join(home, "mantica_workspaces_"+profile)
		} else {
			workspacesRoot = filepath.Join(home, "mantica_workspaces")
		}
	}
	workspacesRoot, err = filepath.Abs(workspacesRoot)
	if err != nil {
		return Config{}, fmt.Errorf("resolve absolute workspaces root: %w", err)
	}

	// Health port: override > default
	healthPort := DefaultHealthPort
	if overrides.HealthPort > 0 {
		healthPort = overrides.HealthPort
	}

	// Keep env after task: env > default (false)
	keepEnv := os.Getenv("MANTICA_KEEP_ENV_AFTER_TASK") == "true" || os.Getenv("MANTICA_KEEP_ENV_AFTER_TASK") == "1"

	return Config{
		ServerBaseURL:      serverBaseURL,
		DatabaseURL:        databaseURL,
		DaemonID:           daemonID,
		DeviceName:         deviceName,
		RuntimeName:        runtimeName,
		Profile:            profile,
		Agents:             agents,
		WorkspacesRoot:     workspacesRoot,
		KeepEnvAfterTask:   keepEnv,
		HealthPort:         healthPort,
		MaxConcurrentTasks: maxConcurrentTasks,
		PollInterval:       pollInterval,
		HeartbeatInterval:  heartbeatInterval,
		AgentTimeout:       agentTimeout,
	}, nil
}

// NormalizeServerBaseURL converts a WebSocket or HTTP URL to a base HTTP URL.
func NormalizeServerBaseURL(raw string) (string, error) {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return "", fmt.Errorf("invalid MANTICA_SERVER_URL: %w", err)
	}
	switch u.Scheme {
	case "ws":
		u.Scheme = "http"
	case "wss":
		u.Scheme = "https"
	case "http", "https":
	default:
		return "", fmt.Errorf("MANTICA_SERVER_URL must use ws, wss, http, or https")
	}
	if u.Path == "/ws" {
		u.Path = ""
	}
	u.RawPath = ""
	u.RawQuery = ""
	u.Fragment = ""
	return strings.TrimRight(u.String(), "/"), nil
}
