---
name: Two-level memory adopted in TES-184
description: multica adopted two-level memory (workspace + repo) in TES-184 to prevent cross-project pollution
type: project
---

Memory spec (docs/agent-memory-format.md) was revised in TES-184 to support two levels of memory storage:
- **Repo-level**: `memory/` inside each git repo, git-tracked, naturally isolated per project
- **Workspace-level**: `workdir/memory/` symlink, shared across all repos in the same workspace session

**Why:** The workspace owner has multiple unrelated (or weakly-related) projects in one workspace. The original single-pool workspace-level design caused memory pollution — agents working on one project were contaminated by context from unrelated projects.

**How to apply:** When writing memory for multica-specific knowledge (architecture decisions, conventions, known bugs), write to this repo's `memory/` directory and commit with `chore(N/A): update memory`. Cross-repo or team-wide knowledge goes to `workdir/memory/` (workspace level).
