# Agent Architecture: Simplified Kanban Model

## Overview

Mantica uses a simplified agent architecture where a single Kanban agent orchestrates all work, and LLMs (Claude, Codex, Hermes) execute tasks directly.

## Architecture Diagram

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                           Kanban Agent (Orchestrator)                        │
├─────────────────────────────────────────────────────────────────────────────┤
│  - Receives issues assigned to it                                           │
│  - Analyzes and decomposes into subtasks                                    │
│  - Executes each subtask via LLM (Claude/Codex/Hermes)                     │
│  - Updates issue status based on results                                    │
└─────────────────────────────────────────────────────────────────────────────┘
                                    │
                                    │ runTask()
                                    ▼
                    ┌─────────────────────────────────────┐
                    │         LLM Execution               │
                    │  ┌─────────┐ ┌─────────┐ ┌─────────┐│
                    │  │ Claude  │ │  Codex  │ │ Hermes  ││
                    │  └─────────┘ └─────────┘ └─────────┘│
                    └─────────────────────────────────────┘
```

## Issue Status Flow

```
backlog → todo → doing → done
                   ↓
                blocked
```

| Status | Description |
|--------|-------------|
| backlog | Not started, in queue |
| todo | Ready to work on |
| doing | Currently being worked on |
| done | Completed successfully |
| blocked | Cannot proceed (dependencies, errors) |
| cancelled | No longer needed |

## Kanban Agent Workflow

1. **Receive Issue**: Kanban agent is assigned an issue
2. **Analyze**: Read issue title, description, and comments
3. **Decompose**: Break down into actionable subtasks
4. **Execute**: Run each subtask via LLM
5. **Report**: Update issue status and post results

## Execution Model

- **Single Orchestrator**: Only Kanban agent polls for tasks
- **Direct Execution**: Subtasks are executed via `runTask()` without database queue
- **No Role Agents**: No separate BA, TL, DEV, QA, Code Review agents
- **LLM as Executor**: Claude/Codex/Hermes execute tasks directly

## Configuration

### agent_config.yaml

```yaml
skills: []

agents:
  - name: KANBAN
    description: Orchestrator agent that decomposes issues and coordinates execution
    provider: claude
    instructions: |
      You are the Kanban Agent, the sole orchestrator for this workspace.
      ...
```

### Pipeline Configuration

```go
var Stages = map[string]StageConfig{
    "todo": {AgentName: "KANBAN", InProgressStatus: "doing"},
}
```

## Migration from Old Architecture

The previous architecture used:
- Multiple role agents (BA, TL, DEV, QA, Code Review)
- Complex status workflow (14 statuses)
- Database queue for task dispatch

The new architecture simplifies to:
- Single Kanban agent
- Simple status workflow (6 statuses)
- Direct LLM execution

## Files Changed

- `server/internal/daemon/kanban.go` - Kanban agent implementation
- `server/internal/daemon/daemon.go` - Task handling (only kanban)
- `server/internal/pipeline/pipeline.go` - Simplified pipeline
- `packages/core/types/issue.ts` - IssueStatus type
- `packages/core/issues/config/status.ts` - Status configuration
