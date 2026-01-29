# Super Multica

A multi-component architecture for distributed agent systems.

## Project Structure

```
src/
├── agent/          # Agent module
│   ├── profile/    # Agent profile management
│   ├── session/    # Session persistence
│   └── tools/      # Agent tools (exec, process)
├── gateway/        # Gateway module
├── client/         # Client module
└── shared/         # Shared types and utilities
```

## Getting Started

```bash
pnpm install
pnpm dev
```

## Agent CLI

Use the agent module directly from the CLI for isolated testing.

```bash
# New sessions get a UUIDv7 ID (shown on start)
pnpm agent:cli "hello"
# [session: 019c0b0a-b111-765c-8bbd-f4149beac9c4]

# Continue a session
pnpm agent:cli --session 019c0b0a-b111-765c-8bbd-f4149beac9c4 "what did I say?"

# Or use a custom session name
pnpm agent:cli --session demo "remember my name is Alice"
pnpm agent:cli --session demo "what's my name?"

# Override provider/model
pnpm agent:cli --provider openai --model gpt-4o-mini "hi"

# Use an agent profile
pnpm agent:cli --profile my-agent "hello"

# Set thinking level
pnpm agent:cli --thinking high "solve this complex problem"
```

## Sessions

Sessions persist conversation history to `~/.super-multica/sessions/<id>/`. Each session includes:

- `session.jsonl` - Message history in JSONL format
- `meta.json` - Session metadata (provider, model, thinking level)

Sessions use UUIDv7 for IDs by default, providing time-ordered unique identifiers. The agent automatically handles context compaction when conversations grow too long.

## Agent Profiles

Agent profiles define identity, personality, tools, and memory for an agent. Profiles are stored as markdown files in `~/.super-multica/agent-profiles/<id>/`.

### Profile CLI

```bash
# Create a new profile with default templates
pnpm agent:profile new my-agent

# List all profiles
pnpm agent:profile list

# Show profile contents
pnpm agent:profile show my-agent

# Open profile directory in file manager
pnpm agent:profile edit my-agent
```

### Profile Structure

Each profile contains:

- `identity.md` - Agent name and role
- `soul.md` - Personality and behavioral constraints
- `tools.md` - Tool usage instructions
- `memory.md` - Persistent knowledge
- `bootstrap.md` - Initial conversation context

## Agent Tools

### exec

Execute short-lived shell commands and return output.

```
exec({ command: "ls -la", cwd: "/path/to/dir", timeoutMs: 30000 })
```

### process

Manage long-running background processes (servers, watchers, daemons). Output is buffered (up to 64KB) and terminated processes are automatically cleaned up after 1 hour.

```
# Start a background process (returns immediately with process ID)
process({ action: "start", command: "npm run dev" })

# Check process status
process({ action: "status", id: "<process-id>" })

# Read process output
process({ action: "output", id: "<process-id>" })

# Stop a process
process({ action: "stop", id: "<process-id>" })

# Clean up terminated processes
process({ action: "cleanup" })
```

## Scripts

- `pnpm dev` - Run in development mode
- `pnpm agent:cli` - Run the agent CLI for module-level testing
- `pnpm agent:profile` - Manage agent profiles
- `pnpm build` - Build for production
- `pnpm start` - Run production build
- `pnpm typecheck` - Type check without emitting
