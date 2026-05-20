package main

import (
	"context"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/xiaoquisme/mantica/server/internal/events"
	"github.com/xiaoquisme/mantica/server/internal/realtime"
	"github.com/xiaoquisme/mantica/server/internal/service"
	db "github.com/xiaoquisme/mantica/server/pkg/db/generated"
)

// setupRevertFixture creates an issue in the given status with a failed task
// that references it. Returns (issueID, agentID, taskID, runtimeID).
func setupRevertFixture(t *testing.T, issueStatus, taskStatus string, taskError string) (string, string, string, string) {
	t.Helper()
	ctx := context.Background()

	var agentID, runtimeID string
	if err := testPool.QueryRow(ctx, `
		SELECT a.id, a.runtime_id FROM agent a
		JOIN member m ON m.workspace_id = a.workspace_id
		JOIN "user" u ON u.id = m.user_id
		WHERE u.email = $1
		LIMIT 1
	`, integrationTestEmail).Scan(&agentID, &runtimeID); err != nil {
		t.Fatalf("failed to find test agent: %v", err)
	}

	var issueID string
	if err := testPool.QueryRow(ctx, `
		INSERT INTO issue (workspace_id, title, status, priority, creator_type, creator_id, assignee_type, assignee_id)
		SELECT $1, 'Auto-revert test issue', $2, 'none', 'member', m.user_id, 'agent', $3
		FROM member m WHERE m.workspace_id = $1 LIMIT 1
		RETURNING id
	`, testWorkspaceID, issueStatus, agentID).Scan(&issueID); err != nil {
		t.Fatalf("failed to create test issue: %v", err)
	}

	var taskID string
	if err := testPool.QueryRow(ctx, `
		INSERT INTO agent_task_queue (agent_id, runtime_id, issue_id, status, priority, error)
		VALUES ($1, $2, $3, $4, 0, $5)
		RETURNING id
	`, agentID, runtimeID, issueID, taskStatus, taskError).Scan(&taskID); err != nil {
		t.Fatalf("failed to create test task: %v", err)
	}

	return issueID, agentID, taskID, runtimeID
}

func cleanupRevertFixture(t *testing.T, issueID string) {
	t.Helper()
	ctx := context.Background()
	testPool.Exec(ctx, `DELETE FROM agent_task_queue WHERE issue_id = $1`, issueID)
	testPool.Exec(ctx, `DELETE FROM comment WHERE issue_id = $1`, issueID)
	testPool.Exec(ctx, `DELETE FROM issue WHERE id = $1`, issueID)
}

func loadTaskRow(t *testing.T, queries *db.Queries, taskID string) db.AgentTaskQueue {
	t.Helper()
	task, err := queries.GetAgentTask(context.Background(), parseUUIDFromString(t, taskID))
	if err != nil {
		t.Fatalf("failed to load task: %v", err)
	}
	return task
}

func parseUUIDFromString(t *testing.T, s string) pgtype.UUID {
	t.Helper()
	var u pgtype.UUID
	if err := u.Scan(s); err != nil {
		t.Fatalf("invalid uuid %q: %v", s, err)
	}
	return u
}

func loadIssueStatusAndAssignee(t *testing.T, issueID string) (string, *string) {
	t.Helper()
	var status string
	var assigneeType pgtype.Text
	if err := testPool.QueryRow(context.Background(), `SELECT status, assignee_type FROM issue WHERE id = $1`, issueID).Scan(&status, &assigneeType); err != nil {
		t.Fatalf("failed to read issue: %v", err)
	}
	if assigneeType.Valid {
		v := assigneeType.String
		return status, &v
	}
	return status, nil
}

func countAutoRevertComments(t *testing.T, issueID string) int {
	t.Helper()
	var count int
	if err := testPool.QueryRow(context.Background(), `
		SELECT count(*) FROM comment
		WHERE issue_id = $1 AND type = 'system' AND content LIKE 'Run %failed%status auto-reverted%'
	`, issueID).Scan(&count); err != nil {
		t.Fatalf("failed to count comments: %v", err)
	}
	return count
}

func newTaskServiceForTest() *service.TaskService {
	return service.NewTaskService(db.New(testPool), realtime.NewHub(), events.New())
}

func TestAutoRevertHappyPath(t *testing.T) {
	if testPool == nil {
		t.Skip("no database connection")
	}

	issueID, _, taskID, _ := setupRevertFixture(t, "in_dev", "failed", "claude timed out after 2h0m0s")
	t.Cleanup(func() { cleanupRevertFixture(t, issueID) })

	queries := db.New(testPool)
	ts := newTaskServiceForTest()
	task := loadTaskRow(t, queries, taskID)

	ts.AutoRevertIssueStatusOnFailure(context.Background(), task)

	status, _ := loadIssueStatusAndAssignee(t, issueID)
	if status != "ready_dev" {
		t.Fatalf("expected status 'ready_dev', got %q", status)
	}

	if got := countAutoRevertComments(t, issueID); got != 1 {
		t.Fatalf("expected 1 auto-revert comment, got %d", got)
	}

	var content string
	if err := testPool.QueryRow(context.Background(), `
		SELECT content FROM comment
		WHERE issue_id = $1 AND type = 'system' AND content LIKE 'Run %'
		LIMIT 1
	`, issueID).Scan(&content); err != nil {
		t.Fatalf("failed to load comment: %v", err)
	}
	if !strings.Contains(content, "ready_dev") {
		t.Fatalf("comment missing target status, got %q", content)
	}
	if !strings.Contains(content, "claude timed out") {
		t.Fatalf("comment missing error summary, got %q", content)
	}
}

func TestAutoRevertIdempotent(t *testing.T) {
	if testPool == nil {
		t.Skip("no database connection")
	}

	issueID, _, taskID, _ := setupRevertFixture(t, "in_review", "failed", "boom")
	t.Cleanup(func() { cleanupRevertFixture(t, issueID) })

	queries := db.New(testPool)
	ts := newTaskServiceForTest()
	task := loadTaskRow(t, queries, taskID)

	ts.AutoRevertIssueStatusOnFailure(context.Background(), task)
	ts.AutoRevertIssueStatusOnFailure(context.Background(), task)

	status, _ := loadIssueStatusAndAssignee(t, issueID)
	if status != "ready_review" {
		t.Fatalf("expected status 'ready_review', got %q", status)
	}
	if got := countAutoRevertComments(t, issueID); got != 1 {
		t.Fatalf("expected 1 comment after duplicate revert, got %d", got)
	}
}

func TestAutoRevertNoOpWhenStatusMovedOn(t *testing.T) {
	if testPool == nil {
		t.Skip("no database connection")
	}

	// Issue is at ready_review (agent set it there before crashing) — not in_*.
	issueID, _, taskID, _ := setupRevertFixture(t, "ready_review", "failed", "boom")
	t.Cleanup(func() { cleanupRevertFixture(t, issueID) })

	queries := db.New(testPool)
	ts := newTaskServiceForTest()
	task := loadTaskRow(t, queries, taskID)

	ts.AutoRevertIssueStatusOnFailure(context.Background(), task)

	status, _ := loadIssueStatusAndAssignee(t, issueID)
	if status != "ready_review" {
		t.Fatalf("expected status to remain 'ready_review', got %q", status)
	}
	if got := countAutoRevertComments(t, issueID); got != 0 {
		t.Fatalf("expected no auto-revert comment when status not in_*, got %d", got)
	}
}

func TestAutoRevertClassifierClearsAssignee(t *testing.T) {
	if testPool == nil {
		t.Skip("no database connection")
	}

	issueID, _, taskID, _ := setupRevertFixture(t, "classifying", "failed", "classifier crashed")
	t.Cleanup(func() { cleanupRevertFixture(t, issueID) })

	queries := db.New(testPool)
	ts := newTaskServiceForTest()
	task := loadTaskRow(t, queries, taskID)

	ts.AutoRevertIssueStatusOnFailure(context.Background(), task)

	status, assignee := loadIssueStatusAndAssignee(t, issueID)
	if status != "backlog" {
		t.Fatalf("expected classifier revert → 'backlog', got %q", status)
	}
	if assignee != nil {
		t.Fatalf("expected assignee cleared on classifier revert, got %q", *assignee)
	}
	if got := countAutoRevertComments(t, issueID); got != 1 {
		t.Fatalf("expected 1 auto-revert comment, got %d", got)
	}
}

func TestAutoRevertSkipsNonIssueTask(t *testing.T) {
	// Chat tasks have no IssueID; auto-revert must short-circuit and not panic.
	ts := newTaskServiceForTest()
	ts.AutoRevertIssueStatusOnFailure(context.Background(), db.AgentTaskQueue{
		IssueID: pgtype.UUID{Valid: false},
	})
}

func TestAutoRevertViaSweeper(t *testing.T) {
	if testPool == nil {
		t.Skip("no database connection")
	}

	// Set up issue in_dev with a running task aged 3 hours so the sweeper picks it up.
	ctx := context.Background()
	var agentID, runtimeID string
	if err := testPool.QueryRow(ctx, `
		SELECT a.id, a.runtime_id FROM agent a
		JOIN member m ON m.workspace_id = a.workspace_id
		JOIN "user" u ON u.id = m.user_id
		WHERE u.email = $1
		LIMIT 1
	`, integrationTestEmail).Scan(&agentID, &runtimeID); err != nil {
		t.Fatalf("failed to find test agent: %v", err)
	}

	var issueID string
	if err := testPool.QueryRow(ctx, `
		INSERT INTO issue (workspace_id, title, status, priority, creator_type, creator_id, assignee_type, assignee_id)
		SELECT $1, 'Sweeper revert test issue', 'in_dev', 'none', 'member', m.user_id, 'agent', $2
		FROM member m WHERE m.workspace_id = $1 LIMIT 1
		RETURNING id
	`, testWorkspaceID, agentID).Scan(&issueID); err != nil {
		t.Fatalf("failed to create issue: %v", err)
	}
	t.Cleanup(func() { cleanupRevertFixture(t, issueID) })

	var taskID string
	if err := testPool.QueryRow(ctx, `
		INSERT INTO agent_task_queue (agent_id, runtime_id, issue_id, status, priority, dispatched_at, started_at)
		VALUES ($1, $2, $3, 'running', 0, now() - interval '3 hours', now() - interval '3 hours')
		RETURNING id
	`, agentID, runtimeID, issueID).Scan(&taskID); err != nil {
		t.Fatalf("failed to create task: %v", err)
	}

	queries := db.New(testPool)
	ts := newTaskServiceForTest()

	failed, err := queries.FailStaleTasks(ctx, db.FailStaleTasksParams{
		DispatchTimeoutSecs: 300.0,
		RunningTimeoutSecs:  1.0,
	})
	if err != nil {
		t.Fatalf("FailStaleTasks: %v", err)
	}
	if len(failed) == 0 {
		t.Fatal("expected at least one stale task")
	}

	broadcastFailedTasks(ctx, queries, events.New(), ts, failed)

	status, _ := loadIssueStatusAndAssignee(t, issueID)
	if status != "ready_dev" {
		t.Fatalf("expected sweeper-driven revert to 'ready_dev', got %q", status)
	}
	if got := countAutoRevertComments(t, issueID); got != 1 {
		t.Fatalf("expected 1 system comment from sweeper revert, got %d", got)
	}
}
