# Simplified Kanban Architecture - Implementation Progress

## Overview

Mantica now uses a simplified agent architecture with a single Kanban agent orchestrating all work via direct LLM execution.

## Completed Tasks

### Phase 1: Status Simplification ✅
- [x] Simplified issue statuses: 14 → 6 (backlog, todo, doing, done, blocked, cancelled)
- [x] Updated frontend type definitions and status config
- [x] Updated backend validation and pipeline configuration
- [x] Created database migration (056_simplify_issue_status)
- [x] Updated swimlanes: 6 → 4 (Todo, Doing, Done, Blocked)

### Phase 2: Kanban Agent Simplification ✅
- [x] Removed role-based agents (BA, TL, DEV, QA, Code Review)
- [x] Kanban agent directly executes subtasks via runTask()
- [x] Only kanban agent polls for tasks
- [x] Simplified decomposition rules (no dependencies)
- [x] Simplified agent_config.yaml

### Phase 3: Database & Pipeline ✅
- [x] Updated subagent_role CHECK constraint
- [x] Updated pending_task_index to allow parallel subtasks
- [x] Simplified pipeline stages (only "todo" → "doing")
- [x] Removed Classifier stage logic

## Architecture

### Before (Old)
```
Kanban Agent → Role Agents (BA, TL, DEV, QA, Code Review) → LLM
                ↓
         Database Queue (agent_task_queue)
```

### After (New)
```
Kanban Agent → LLM (Claude/Codex/Hermes)
                ↓
         Direct Execution (no queue)
```

## Status Values

| Status | Description |
|--------|-------------|
| backlog | Not started |
| todo | Ready to work on |
| doing | Currently being worked on |
| done | Completed |
| blocked | Cannot proceed |
| cancelled | No longer needed |

## Key Files

- `server/internal/daemon/kanban.go` - Kanban agent
- `server/internal/daemon/daemon.go` - Task handling
- `server/internal/pipeline/pipeline.go` - Pipeline config
- `packages/core/types/issue.ts` - IssueStatus type
- `packages/core/issues/config/status.ts` - Status config
- `agent_config.yaml` - Agent configuration

## Migration Notes

The old architecture used 14 statuses and 6 role agents. The new architecture uses 6 statuses and 1 orchestrator agent. Database migration 056 maps old statuses to new ones.
