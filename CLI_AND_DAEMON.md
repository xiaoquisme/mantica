# CLI and Agent Daemon Guide

The `mantica` CLI connects your local machine to Mantica. It handles authentication, workspace management, issue tracking, and runs the agent daemon that executes AI tasks locally.

## Installation

### Homebrew (macOS/Linux)

```bash
brew tap mantica-ai/tap
brew install mantica
```

### Build from Source

```bash
git clone https://github.com/mantica-ai/mantica.git
cd mantica
make build
cp server/bin/mantica /usr/local/bin/mantica
```

### Update

```bash
mantica update
```

This auto-detects your installation method (Homebrew or manual) and upgrades accordingly.

## Quick Start

```bash
# 1. Authenticate (opens browser for login)
mantica login

# 2. Start the agent daemon
mantica daemon start

# 3. Done — agents in your watched workspaces can now execute tasks on your machine
```

`mantica login` automatically discovers all workspaces you belong to and adds them to the daemon watch list.

## Authentication

### Browser Login

```bash
mantica login
```

Opens your browser for OAuth authentication, creates a 90-day personal access token, and auto-configures your workspaces.

### Token Login

```bash
mantica login --token
```

Authenticate by pasting a personal access token directly. Useful for headless environments.

### Check Status

```bash
mantica auth status
```

Shows your current server, user, and token validity.

### Logout

```bash
mantica auth logout
```

Removes the stored authentication token.

## Agent Daemon

The daemon is the local agent runtime. It detects available AI CLIs on your machine, registers them with the Multica server, and executes tasks when agents are assigned work.

### Start

```bash
mantica daemon start
```

By default, the daemon runs in the background and logs to `~/.mantica/daemon.log`.

To run in the foreground (useful for debugging):

```bash
mantica daemon start --foreground
```

### Stop

```bash
mantica daemon stop
```

### Status

```bash
mantica daemon status
mantica daemon status --output json
```

Shows PID, uptime, detected agents, and watched workspaces.

### Logs

```bash
mantica daemon logs              # Last 50 lines
mantica daemon logs -f           # Follow (tail -f)
mantica daemon logs -n 100       # Last 100 lines
```

### Supported Agents

The daemon auto-detects these AI CLIs on your PATH:

| CLI | Command | Description |
|-----|---------|-------------|
| [Claude Code](https://docs.anthropic.com/en/docs/claude-code) | `claude` | Anthropic's coding agent |
| [Codex](https://github.com/openai/codex) | `codex` | OpenAI's coding agent |

You need at least one installed. The daemon registers each detected CLI as an available runtime.

### How It Works

1. On start, the daemon detects installed agent CLIs and registers a runtime for each agent in each watched workspace
2. It polls the server at a configurable interval (default: 3s) for claimed tasks
3. When a task arrives, it creates an isolated workspace directory, spawns the agent CLI, and streams results back
4. Heartbeats are sent periodically (default: 15s) so the server knows the daemon is alive
5. On shutdown, all runtimes are deregistered

### Configuration

Daemon behavior is configured via flags or environment variables:

| Setting | Flag | Env Variable | Default |
|---------|------|--------------|---------|
| Poll interval | `--poll-interval` | `MANTICA_DAEMON_POLL_INTERVAL` | `3s` |
| Heartbeat interval | `--heartbeat-interval` | `MANTICA_DAEMON_HEARTBEAT_INTERVAL` | `15s` |
| Agent timeout | `--agent-timeout` | `MANTICA_AGENT_TIMEOUT` | `2h` |
| Max concurrent tasks | `--max-concurrent-tasks` | `MANTICA_DAEMON_MAX_CONCURRENT_TASKS` | `20` |
| Daemon ID | `--daemon-id` | `MANTICA_DAEMON_ID` | hostname |
| Device name | `--device-name` | `MANTICA_DAEMON_DEVICE_NAME` | hostname |
| Runtime name | `--runtime-name` | `MANTICA_AGENT_RUNTIME_NAME` | `Local Agent` |
| Workspaces root | — | `MANTICA_WORKSPACES_ROOT` | `~/mantica_workspaces` |

Agent-specific overrides:

| Variable | Description |
|----------|-------------|
| `MANTICA_CLAUDE_PATH` | Custom path to the `claude` binary |
| `MANTICA_CLAUDE_MODEL` | Override the Claude model used |
| `MANTICA_CODEX_PATH` | Custom path to the `codex` binary |
| `MANTICA_CODEX_MODEL` | Override the Codex model used |

### Self-Hosted Server

When connecting to a self-hosted Multica instance, point the CLI to your server before logging in:

```bash
export MANTICA_APP_URL=https://app.example.com
export MANTICA_SERVER_URL=wss://api.example.com/ws

mantica login
mantica daemon start
```

Or set them persistently:

```bash
mantica config set app_url https://app.example.com
mantica config set server_url wss://api.example.com/ws
```

### Profiles

Profiles let you run multiple daemons on the same machine — for example, one for production and one for a staging server.

```bash
# Start a daemon for the staging server
mantica --profile staging login
mantica --profile staging daemon start

# Default profile runs separately
mantica daemon start
```

Each profile gets its own config directory (`~/.mantica/profiles/<name>/`), daemon state, health port, and workspace root.

## Workspaces

### List Workspaces

```bash
mantica workspace list
```

Watched workspaces are marked with `*`. The daemon only processes tasks for watched workspaces.

### Watch / Unwatch

```bash
mantica workspace watch <workspace-id>
mantica workspace unwatch <workspace-id>
```

### Get Details

```bash
mantica workspace get <workspace-id>
mantica workspace get <workspace-id> --output json
```

### List Members

```bash
mantica workspace members <workspace-id>
```

## Issues

### List Issues

```bash
mantica issue list
mantica issue list --status in_progress
mantica issue list --priority urgent --assignee "Agent Name"
mantica issue list --limit 20 --output json
```

Available filters: `--status`, `--priority`, `--assignee`, `--limit`.

### Get Issue

```bash
mantica issue get <id>
mantica issue get <id> --output json
```

### Create Issue

```bash
mantica issue create --title "Fix login bug" --description "..." --priority high --assignee "Lambda"
```

Flags: `--title` (required), `--description`, `--status`, `--priority`, `--assignee`, `--parent`, `--due-date`.

### Update Issue

```bash
mantica issue update <id> --title "New title" --priority urgent
```

### Assign Issue

```bash
mantica issue assign <id> --to "Lambda"
mantica issue assign <id> --unassign
```

### Change Status

```bash
mantica issue status <id> in_progress
```

Valid statuses: `backlog`, `todo`, `in_progress`, `in_review`, `done`, `blocked`, `cancelled`.

### Comments

```bash
# List comments
mantica issue comment list <issue-id>

# Add a comment
mantica issue comment add <issue-id> --content "Looks good, merging now"

# Reply to a specific comment
mantica issue comment add <issue-id> --parent <comment-id> --content "Thanks!"

# Delete a comment
mantica issue comment delete <comment-id>
```

### Execution History

```bash
# List all execution runs for an issue
mantica issue runs <issue-id>
mantica issue runs <issue-id> --output json

# View messages for a specific execution run
mantica issue run-messages <task-id>
mantica issue run-messages <task-id> --output json

# Incremental fetch (only messages after a given sequence number)
mantica issue run-messages <task-id> --since 42 --output json
```

The `runs` command shows all past and current executions for an issue, including running tasks. The `run-messages` command shows the detailed message log (tool calls, thinking, text, errors) for a single run. Use `--since` for efficient polling of in-progress runs.

## Configuration

### View Config

```bash
mantica config show
```

Shows config file path, server URL, app URL, and default workspace.

### Set Values

```bash
mantica config set server_url wss://api.example.com/ws
mantica config set app_url https://app.example.com
mantica config set workspace_id <workspace-id>
```

## Other Commands

```bash
mantica version              # Show CLI version and commit hash
mantica update               # Update to latest version
mantica agent list           # List agents in the current workspace
```

## Output Formats

Most commands support `--output` with two formats:

- `table` — human-readable table (default for list commands)
- `json` — structured JSON (useful for scripting and automation)

```bash
mantica issue list --output json
mantica daemon status --output json
```
