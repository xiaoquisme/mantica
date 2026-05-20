# Agent Memory File Format

This document specifies the on-disk format used by Multica agents to persist
context across sessions. Any agent that reads or writes workspace memory must
conform to this spec; any human admin reviewing or editing memory should be
able to do so with a plain text editor.

The runtime tooling that consumes this format is delivered separately:

- Reading memory at session start
- Writing memory after task completion
- Two-level memory (workspace + repo)

If the spec and the runtime ever disagree, treat it as a spec defect and fix
this document. Do not let the two conventions drift.

---

## 1. Overview

Memory is organised in **two levels** that agents load at session start and
write to at session end. Each level is a small, append-friendly directory of
markdown files. Each file holds one *memory entry* — a single fact, lesson,
or pointer that future agents should know about. A single index file
(`MEMORY.md`) in each level lists every entry in that level.

**Level 1 — Workspace-level** (`workdir/memory/`):
Cross-cutting knowledge that applies to multiple repos or to the workspace as
a whole: team preferences, conventions shared across projects, stakeholder
requirements. Survives workspace/session changes. Always present.

**Level 2 — Repo-level** (`<repo-root>/memory/`):
Knowledge specific to a single checked-out git repository: tech decisions,
known bugs, project-specific references, repo conventions. Persists with the
code via git — survives workspace resets and agent turnover. Only present when
a repo is checked out.

Design constraints:

- **Human-readable.** Admins inspect and edit memory directly. No binary
  formats, no opaque blobs.
- **Machine-parseable.** YAML frontmatter (parseable by the existing
  `gopkg.in/yaml.v3` dependency) carries the structured fields; the body is
  free-form markdown.
- **Cheap to load.** The index is small enough to drop into every prompt; full
  entry bodies are only read when the agent decides they are relevant.
- **Topic-organised, not time-organised.** Entries are named by what they
  describe, not when they were written, so the writer can detect duplicates
  before creating a second file on the same subject.

---

## 2. Directory Layout

### 2.1 Workspace-level memory

Memory lives at the workspace working-directory root in a directory called
`memory/`:

```
<workdir>/
└── memory/
    ├── MEMORY.md            # index — always loaded
    ├── user_role.md         # one memory entry per file
    ├── feedback_testing.md
    ├── project_q2_freeze.md
    └── reference_grafana.md
```

### 2.2 Repo-level memory

When a repository is checked out, repo-level memory lives inside that
repository at its root in a directory also called `memory/`:

```
<workdir>/
└── <repo-name>/
    └── memory/
        ├── MEMORY.md            # index — loaded when repo is checked out
        ├── feedback_testing.md
        └── reference_api_quirks.md
```

Rules:

- The directory is always `memory/` — at `workdir/memory/` for workspace
  level and at `<repo-root>/memory/` for repo level.
- `MEMORY.md` lives **inside** `memory/`, not at the workdir or repo root.
- Every `.md` file in `memory/` other than `MEMORY.md` is a memory entry and
  must conform to the schema in §3.
- Subdirectories under `memory/` are not used in this phase. Keep the layout
  flat.

---

## 3. Two-Level Read / Write Rules

### 3.1 Read phase (session start)

Agents must load memory in this order at the start of every session:

1. Load **workspace-level** `memory/MEMORY.md` and every referenced entry
   file. This is always done, even if the index does not exist yet (treat as
   empty).
2. If a repository is checked out, load **repo-level** `memory/MEMORY.md`
   and every referenced entry file from that repo.
3. When the same topic appears in both levels, **repo-level entries take
   precedence** for decisions that are specific to that repository.

### 3.2 Write phase (session end)

After completing a task, apply this decision rule to each candidate memory:

| Question | Answer | Write to |
|---|---|---|
| Is this knowledge specific to the checked-out repo? | Yes | Repo-level `<repo-root>/memory/`; commit the new/updated file to git |
| Does this apply to multiple repos or the whole workspace? | Yes | Workspace-level `workdir/memory/` |

When writing to repo-level memory, the new or updated file must be committed
to the repository before the session ends. Use a `chore(N/A): update memory`
commit on the repo's main branch so the entry survives for future agents in
any workspace that checks out the same repo.

---

## 4. Memory File Format

A memory entry is a UTF-8 markdown file with YAML frontmatter delimited by
`---` lines. The frontmatter must be the very first content in the file.

### 4.1 Frontmatter schema

| Field         | Required | Type   | Constraint                                                                          |
|---------------|----------|--------|-------------------------------------------------------------------------------------|
| `name`        | yes      | string | Short title. ≤ 60 chars. Human-friendly, not the filename.                          |
| `description` | yes      | string | One-line relevance hook. ≤ 150 chars. **Reused verbatim** in `MEMORY.md` (see §7).  |
| `type`        | yes      | enum   | One of `user`, `feedback`, `project`, `reference`. Closed enum — see §5.            |

No other fields are defined in this phase. Writers must not invent new keys;
readers must ignore unknown keys (and may warn).

### 4.2 Body

Everything after the closing `---` is the entry body. It is markdown. The
required body shape depends on `type` — see §6.

### 4.3 Parse failures

Readers must skip and warn on any file in `memory/` that fails to parse. A
single malformed entry must never abort loading the rest of the index.

---

## 5. Memory Types

The `type` field is a closed enum. Adding a new value requires revising this
spec; agents must not invent ad-hoc types.

| Type        | Purpose                                                                                                  | Decay      |
|-------------|----------------------------------------------------------------------------------------------------------|------------|
| `user`      | Facts about the human (role, expertise, preferences) that tailor agent behaviour to *them specifically*. | Slow       |
| `feedback`  | Corrections or validated approaches the user has confirmed. Save both *don'ts* and *do-keep-doings*.     | Medium     |
| `project`   | Non-derivable workspace facts: deadlines, in-flight initiatives, stakeholder asks.                       | Fast       |
| `reference` | Pointers to external systems (Linear projects, dashboards, channels) and what they are for.              | Slow       |

### 5.1 What NOT to save

Do not create memory entries for any of the following — they are recoverable
from the project itself or are too ephemeral to survive past the session:

- Code patterns, architectural conventions, file paths, project structure —
  read the current code instead.
- Git history or who-changed-what — `git log` / `git blame` are authoritative.
- Bug fixes or debugging recipes — the fix is in the code, the reason is in
  the commit message.
- Anything already documented in `CLAUDE.md` or other in-repo docs.
- In-progress task state, current conversation context, ephemeral todos.

These exclusions apply even when the user explicitly asks the agent to "save
this" — if the request is really about ephemeral state, the agent should
clarify what was *surprising* or *non-obvious* and save only that.

---

## 6. Body Structure by Type

`feedback` and `project` entries decay or need judgement at the edge cases.
For both, the body **must** start with the rule or fact, then include two
labelled lines:

- `**Why:**` — the reason the user gave (often an incident, a stated
  preference, a deadline). This is what lets a future agent judge edge cases
  instead of blindly following the rule.
- `**How to apply:**` — when this guidance kicks in (which files, which kinds
  of task, which decisions).

`user` and `reference` entries are free-form prose. A short paragraph is
typical; multiple paragraphs are allowed when needed.

---

## 7. MEMORY.md Index Format

`MEMORY.md` is a flat list of one-line pointers to every entry in the
directory. It is **not** a memory entry and must not have frontmatter.

### 7.1 Line format

Each entry is exactly one line in the following form:

```
- [<name>](<file>.md) — <description>
```

- `<name>` is the entry's frontmatter `name`.
- `<file>.md` is the entry's filename, relative to `memory/`.
- `<description>` is the entry's frontmatter `description`, **copied
  verbatim**. Writers must not author a separate index hook — the
  frontmatter `description` is the source of truth so admins can scan the
  index and trust it matches the file.
- The em-dash separator is `—` (U+2014), surrounded by single spaces.

### 7.2 Ordering

Group entries by type in this order: `user`, `feedback`, `project`,
`reference`. Within a group, order is not significant; alphabetical by name
is fine. Optional `## <Type>` subheadings are permitted for readability but
not required.

### 7.3 200-line limit

`MEMORY.md` is hard-capped at 200 lines (including any blank lines and
optional subheadings). Lines past 200 are silently truncated by the reader,
so any entry in the tail becomes invisible.

When the index would exceed 200 lines, the writer must consolidate or remove
stale entries before adding a new one. TES-170 is responsible for enforcing
this at write time.

---

## 8. Naming Conventions

Entry filenames use semantic snake_case with a `.md` extension and should
indicate the topic, not the date:

- `user_role.md`
- `feedback_testing.md`
- `project_q2_freeze.md`
- `reference_grafana_latency.md`

Two rules follow from this:

1. **Topic before type when natural.** `feedback_testing.md` is preferred
   over `testing_feedback.md` only when grouping by type aids discovery; the
   important property is that two entries on the same topic collide on
   filename rather than silently coexisting.
2. **Check before writing.** A writer adding a new entry must first look for
   an existing file whose name matches the topic. If one exists, update it
   instead of creating a parallel file. TES-170 implements the check.

Date-based names (`2026-04-23-foo.md`) are not used.

---

## 9. Examples

The following four entries and one index file are valid and copy-paste
runnable against the schema above.

### 9.1 `memory/user_role.md`

```markdown
---
name: User role and stack background
description: User is a senior Go engineer; new to the React side of this repo
type: user
---

The user has been writing Go for ten years and leads the backend team. They
are comfortable reading Go code at any depth and prefer terse explanations
when the topic is in their domain.

This is their first time contributing to the frontend in `apps/web/` and
`packages/views/`. When explaining frontend concepts, prefer analogies to
backend patterns (e.g. "the Query cache is the read model; mutations are
commands") and call out React-specific footguns explicitly.
```

### 9.2 `memory/feedback_testing.md`

```markdown
---
name: Integration tests must hit a real database
description: Do not mock the database in integration tests — use a real one
type: feedback
---

Integration tests in `server/` must run against a real Postgres instance, not
against a mocked or in-memory replacement.

**Why:** Last quarter a mocked test suite passed cleanly while the production
migration failed because the mocks did not exercise real constraint
behaviour. The user explicitly called this out as a recurring concern.

**How to apply:** Whenever adding or modifying a test under `server/` that
exercises a query, repository, or migration, wire it through the existing
test-database fixture. Do not introduce `sqlmock` or hand-rolled stubs for
DB calls in integration tests. Pure unit tests of non-DB logic are still
fine to mock.
```

### 9.3 `memory/project_q2_freeze.md`

```markdown
---
name: Q2 mobile release freeze
description: Merge freeze for non-critical PRs starts 2026-05-07 (mobile cut)
type: project
---

A merge freeze for all non-critical changes begins on 2026-05-07 so the
mobile team can cut a release branch from a stable main.

**Why:** Mobile cannot rebuild on shifting infrastructure during their
release week; the user committed to the mobile lead that main would be
quiet from 2026-05-07 through 2026-05-14.

**How to apply:** Between 2026-05-07 and 2026-05-14, flag any non-critical
PR work — refactors, dependency bumps, doc-only changes — and ask whether
it can wait. Hotfixes and revert-of-broken-main changes are still allowed.
```

### 9.4 `memory/reference_grafana_latency.md`

```markdown
---
name: Oncall API latency dashboard
description: grafana.internal/d/api-latency — oncall watches this for request-path regressions
type: reference
---

The Grafana board at `grafana.internal/d/api-latency` is the dashboard that
oncall watches for request-handling latency. It is the most likely thing to
page someone when request-path code changes.

Check it before and after any change under `server/internal/handler/` or
`server/internal/middleware/` that could affect request latency.
```

### 9.5 `memory/MEMORY.md` (workspace-level index)

```markdown
## user
- [User role and stack background](user_role.md) — User is a senior Go engineer; new to the React side of this repo

## feedback
- [Integration tests must hit a real database](feedback_testing.md) — Do not mock the database in integration tests — use a real one

## project
- [Q2 mobile release freeze](project_q2_freeze.md) — Merge freeze for non-critical PRs starts 2026-05-07 (mobile cut)

## reference
- [Oncall API latency dashboard](reference_grafana_latency.md) — grafana.internal/d/api-latency — oncall watches this for request-path regressions
```

### 9.6 Repo-level memory example (`<repo>/memory/feedback_db_driver.md`)

This entry would be committed into the repository and loaded when any agent
checks out that repo, regardless of which workspace it is working in.

```markdown
---
name: Use pgx/v5 driver — never database/sql
description: All DB access in this repo must use pgx/v5 directly, not database/sql wrapper
type: feedback
---

All database access in this repository must go through `pgx/v5` directly.
Do not use the `database/sql` standard-library wrapper.

**Why:** The original team switched from `database/sql` to `pgx/v5` for
full COPY protocol support and array-type scanning. Code using
`database/sql` compiles but silently falls back to text encoding, which
breaks bulk inserts.

**How to apply:** Any new file under `internal/db/` must import
`github.com/jackc/pgx/v5` and use `pgx.Pool`, not `sql.DB`. Review any
PR that imports `database/sql` for this repo.
```

And its `<repo>/memory/MEMORY.md` index:

```markdown
## feedback
- [Use pgx/v5 driver — never database/sql](feedback_db_driver.md) — All DB access in this repo must use pgx/v5 directly, not database/sql wrapper
```

---

## 10. SQLite Backend (TES-28)

Starting with TES-28, workspace-level memory (`workdir/memory/`) is backed by a
SQLite database at `{WorkspacesRoot}/{WorkspaceID}/memory.db`. The database uses
**WAL mode** for concurrent-safe access by multiple parallel tasks.

### 10.1 Schema

| Table | Purpose |
|---|---|
| `memory_entries` | Primary store: id, workspace_id, type, name, description, body, created_at, updated_at, expires_at |
| `memory_fts_content` | FTS5 content table backing the virtual table |
| `memory_fts` | FTS5 virtual table indexed on name + description + body |

Vector search (`memory_vec`) is reserved for Phase 2 when `sqlite-vec` extension
is available via `MULTICA_SQLITE_VEC_PATH`.

### 10.2 CLI Access

Agents interact with the SQLite backend via the `mantica memory` command group:

```
mantica memory list [--type X] [--limit N]
mantica memory search <query> [--limit N]
mantica memory add --type X --name "..." --description "..." [--body "..."] [--expires-after 30d]
mantica memory gc [--older-than 30d]
mantica memory migrate
```

`mantica memory migrate` performs a one-time import of existing `memory/*.md`
files into the database. Agents in new workspaces should run `migrate` once to
import any legacy entries.

### 10.3 Markdown exports (backward compatibility)

Every write to the SQLite store simultaneously exports a human-readable
markdown file to `memory/entries/<id>.md` and rewrites `memory/MEMORY.md`.
Agents that still read the flat-file index work unchanged.

### 10.4 TTL / GC

Entries can be given a TTL via `--expires-after 30d`. The `gc` command deletes
expired entries and, optionally, project/feedback entries older than a given
duration. This replaces the previous requirement for manual index pruning.

### 10.5 Fallback for old workspaces

If `memory.db` does not exist (e.g. legacy workspace), the daemon creates it
automatically on next task start. Until then, agents fall back to reading
`memory/MEMORY.md` directly (AC-6 fallback).
