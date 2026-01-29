---
name: Git Commit Helper
description: Create well-formatted git commits following conventional commit standards
version: 1.0.0
metadata:
  emoji: "📝"
  requiresBinaries:
    - git
  tags:
    - git
    - developer-tools
---

## Instructions

When the user asks you to create a commit or commit their changes, follow these guidelines:

### Step 1: Review Changes

1. Run `git status` to see what files have changed
2. Run `git diff` to see the actual changes
3. If there are staged changes, also run `git diff --staged`

### Step 2: Analyze and Group Changes

Group related changes into logical commits:
- Feature additions
- Bug fixes
- Refactoring (no functional change)
- Documentation
- Tests
- Configuration/dependencies

### Step 3: Create Atomic Commits

For each logical group of changes:

1. Stage only the relevant files: `git add <file1> <file2>`
2. Create a commit with conventional message format

### Commit Message Format

Use conventional commits:
- `feat`: New feature
- `fix`: Bug fix
- `refactor`: Code refactoring (no functional change)
- `docs`: Documentation changes
- `test`: Adding or updating tests
- `chore`: Build, config, dependencies

Format: `<type>(<scope>): <description>`

Example: `feat(auth): add user login endpoint`

### Rules

- Each commit should be independently meaningful and buildable
- Related test files should be committed with their implementation
- Never create empty commits
- Never combine unrelated changes in one commit
- Keep commit messages concise but descriptive
- If all changes are related to one logical unit, a single commit is fine

### Example

If the user modified:
- `src/api/user.ts` (added new endpoint)
- `src/api/user.test.ts` (tests for new endpoint)
- `src/utils/format.ts` (refactored helper)
- `README.md` (updated docs)

Create three commits:
1. `git add src/api/user.ts src/api/user.test.ts && git commit -m "feat(api): add user profile endpoint"`
2. `git add src/utils/format.ts && git commit -m "refactor(utils): simplify date formatting logic"`
3. `git add README.md && git commit -m "docs: update API documentation"`
