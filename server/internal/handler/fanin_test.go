package handler

import (
	"context"
	"net/http/httptest"
	"testing"

	"github.com/xiaoquisme/mantica/server/internal/pipeline"
)

// createFanInIssue inserts an issue directly via SQL, bypassing the API. This
// lets fan-in tests deterministically seed a parent in a specific in_* status
// (which the normal pipeline would not allow without an agent assigned).
func createFanInIssue(t *testing.T, ctx context.Context, title, status string, parentID *string) string {
	t.Helper()
	var id string
	var err error
	if parentID == nil {
		err = testPool.QueryRow(ctx,
			`INSERT INTO issue (workspace_id, title, status, priority, creator_type, creator_id, number, position)
			 VALUES ($1, $2, $3, 'none', 'member', $4, nextval('issue_number_seq'::regclass), 0)
			 RETURNING id`,
			testWorkspaceID, title, status, testUserID,
		).Scan(&id)
	} else {
		err = testPool.QueryRow(ctx,
			`INSERT INTO issue (workspace_id, title, status, priority, creator_type, creator_id, number, position, parent_issue_id)
			 VALUES ($1, $2, $3, 'none', 'member', $4, nextval('issue_number_seq'::regclass), 0, $5)
			 RETURNING id`,
			testWorkspaceID, title, status, testUserID, *parentID,
		).Scan(&id)
	}
	if err != nil {
		// issue_number_seq may not exist — fall back to a simple monotonic number.
		var n int32
		_ = testPool.QueryRow(ctx, `SELECT COALESCE(MAX(number), 0) + 1 FROM issue WHERE workspace_id = $1`, testWorkspaceID).Scan(&n)
		if parentID == nil {
			err = testPool.QueryRow(ctx,
				`INSERT INTO issue (workspace_id, title, status, priority, creator_type, creator_id, number, position)
				 VALUES ($1, $2, $3, 'none', 'member', $4, $5, 0)
				 RETURNING id`,
				testWorkspaceID, title, status, testUserID, n,
			).Scan(&id)
		} else {
			err = testPool.QueryRow(ctx,
				`INSERT INTO issue (workspace_id, title, status, priority, creator_type, creator_id, number, position, parent_issue_id)
				 VALUES ($1, $2, $3, 'none', 'member', $4, $5, 0, $6)
				 RETURNING id`,
				testWorkspaceID, title, status, testUserID, n, *parentID,
			).Scan(&id)
		}
	}
	if err != nil {
		t.Fatalf("failed to insert fan-in test issue: %v", err)
	}
	return id
}

func setIssueStatus(t *testing.T, ctx context.Context, issueID, status string) {
	t.Helper()
	_, err := testPool.Exec(ctx, `UPDATE issue SET status = $2 WHERE id = $1`, issueID, status)
	if err != nil {
		t.Fatalf("failed to update issue status: %v", err)
	}
}

func getIssueStatus(t *testing.T, ctx context.Context, issueID string) string {
	t.Helper()
	var s string
	if err := testPool.QueryRow(ctx, `SELECT status FROM issue WHERE id = $1`, issueID).Scan(&s); err != nil {
		t.Fatalf("failed to read issue status: %v", err)
	}
	return s
}

func deleteIssues(ctx context.Context, ids ...string) {
	for _, id := range ids {
		testPool.Exec(ctx, `DELETE FROM issue WHERE id = $1`, id)
	}
}

// TestFanIn_RuleConfig is a small unit check that the canonical pipeline
// stages each have a fan-in rule wired up.
func TestFanIn_RuleConfig(t *testing.T) {
	wantStages := []string{"in_arch_design", "in_dev", "in_review", "in_test"}
	for _, s := range wantStages {
		rule, ok := pipeline.FanInRuleFor(s)
		if !ok {
			t.Errorf("FanInRuleFor(%q) missing", s)
			continue
		}
		if rule.NextStatus == "" {
			t.Errorf("FanInRuleFor(%q): empty NextStatus", s)
		}
		if !rule.IsTerminal("done") {
			t.Errorf("FanInRuleFor(%q): expected 'done' to be terminal", s)
		}
	}
	if _, ok := pipeline.FanInRuleFor("backlog"); ok {
		t.Errorf("FanInRuleFor(backlog) should be unset")
	}
}

// TestFanIn_AllChildrenDone_ParentAdvances seeds a parent in in_dev with two
// children and marks them both done, then verifies the parent advances to
// the configured next status (ready_review).
func TestFanIn_AllChildrenDone_ParentAdvances(t *testing.T) {
	ctx := context.Background()
	parentID := createFanInIssue(t, ctx, "fan-in parent: all done", "in_dev", nil)
	childA := createFanInIssue(t, ctx, "child A", "in_dev", &parentID)
	childB := createFanInIssue(t, ctx, "child B", "in_dev", &parentID)
	t.Cleanup(func() { deleteIssues(ctx, childA, childB, parentID) })

	// First child done — parent should NOT advance yet.
	setIssueStatus(t, ctx, childA, "done")
	testHandler.evaluateParentFanIn(ctx, parseUUID(parentID))
	if got := getIssueStatus(t, ctx, parentID); got != "in_dev" {
		t.Fatalf("after one child done: parent status = %q, want %q", got, "in_dev")
	}

	// Second child done — parent should advance to ready_review.
	setIssueStatus(t, ctx, childB, "done")
	testHandler.evaluateParentFanIn(ctx, parseUUID(parentID))
	if got := getIssueStatus(t, ctx, parentID); got != "ready_review" {
		t.Fatalf("after all children done: parent status = %q, want %q", got, "ready_review")
	}
}

// TestFanIn_ChildBlocked_ParentBlocked verifies that a single blocked child
// flips the parent to "blocked" even when other children are still in flight.
func TestFanIn_ChildBlocked_ParentBlocked(t *testing.T) {
	ctx := context.Background()
	parentID := createFanInIssue(t, ctx, "fan-in parent: blocked", "in_dev", nil)
	childA := createFanInIssue(t, ctx, "child A", "in_dev", &parentID)
	childB := createFanInIssue(t, ctx, "child B", "in_dev", &parentID)
	t.Cleanup(func() { deleteIssues(ctx, childA, childB, parentID) })

	setIssueStatus(t, ctx, childA, "blocked")
	testHandler.evaluateParentFanIn(ctx, parseUUID(parentID))
	if got := getIssueStatus(t, ctx, parentID); got != "blocked" {
		t.Fatalf("with blocked child: parent status = %q, want %q", got, "blocked")
	}
}

// TestFanIn_MixedChildren_NoOp verifies the parent stays put when at least
// one child is neither terminal nor blocked.
func TestFanIn_MixedChildren_NoOp(t *testing.T) {
	ctx := context.Background()
	parentID := createFanInIssue(t, ctx, "fan-in parent: mixed", "in_dev", nil)
	childA := createFanInIssue(t, ctx, "child A", "done", &parentID)
	childB := createFanInIssue(t, ctx, "child B", "in_dev", &parentID)
	t.Cleanup(func() { deleteIssues(ctx, childA, childB, parentID) })

	testHandler.evaluateParentFanIn(ctx, parseUUID(parentID))
	if got := getIssueStatus(t, ctx, parentID); got != "in_dev" {
		t.Fatalf("with mixed children: parent status = %q, want %q (unchanged)", got, "in_dev")
	}
}

// TestFanIn_NoChildren_NoOp verifies a parent with zero children is left
// alone — the story is explicitly out of scope for non-parallel pipelines.
func TestFanIn_NoChildren_NoOp(t *testing.T) {
	ctx := context.Background()
	parentID := createFanInIssue(t, ctx, "fan-in parent: childless", "in_dev", nil)
	t.Cleanup(func() { deleteIssues(ctx, parentID) })

	testHandler.evaluateParentFanIn(ctx, parseUUID(parentID))
	if got := getIssueStatus(t, ctx, parentID); got != "in_dev" {
		t.Fatalf("childless parent: status = %q, want %q (unchanged)", got, "in_dev")
	}
}

// TestFanIn_ParentNotInRule_NoOp verifies parents in stages without a fan-in
// rule (e.g. backlog, done, ready_*) are not auto-advanced.
func TestFanIn_ParentNotInRule_NoOp(t *testing.T) {
	ctx := context.Background()
	parentID := createFanInIssue(t, ctx, "fan-in parent: backlog", "backlog", nil)
	childA := createFanInIssue(t, ctx, "child A", "done", &parentID)
	t.Cleanup(func() { deleteIssues(ctx, childA, parentID) })

	testHandler.evaluateParentFanIn(ctx, parseUUID(parentID))
	if got := getIssueStatus(t, ctx, parentID); got != "backlog" {
		t.Fatalf("backlog parent: status = %q, want %q (unchanged)", got, "backlog")
	}
}

// TestFanIn_Idempotent verifies a second evaluation after the parent has
// already advanced is a no-op (concurrent-completion race protection).
func TestFanIn_Idempotent(t *testing.T) {
	ctx := context.Background()
	parentID := createFanInIssue(t, ctx, "fan-in parent: idempotent", "in_dev", nil)
	childA := createFanInIssue(t, ctx, "child A", "done", &parentID)
	t.Cleanup(func() { deleteIssues(ctx, childA, parentID) })

	testHandler.evaluateParentFanIn(ctx, parseUUID(parentID))
	if got := getIssueStatus(t, ctx, parentID); got != "ready_review" {
		t.Fatalf("first eval: parent status = %q, want %q", got, "ready_review")
	}

	// Second call — parent is now in ready_review (no fan-in rule), so this
	// must be a no-op rather than re-advancing or erroring.
	testHandler.evaluateParentFanIn(ctx, parseUUID(parentID))
	if got := getIssueStatus(t, ctx, parentID); got != "ready_review" {
		t.Fatalf("second eval: parent status = %q, want %q (must remain unchanged)", got, "ready_review")
	}
}

// TestFanIn_TerminalChain_PropagatesToGrandparent verifies that when fan-in
// flips a parent to a terminal status (e.g. in_test → done), the grandparent
// is also re-evaluated and advances if its remaining children are all done.
func TestFanIn_TerminalChain_PropagatesToGrandparent(t *testing.T) {
	ctx := context.Background()
	grandparentID := createFanInIssue(t, ctx, "fan-in grandparent", "in_test", nil)
	parentID := createFanInIssue(t, ctx, "fan-in parent (in_test)", "in_test", &grandparentID)
	childA := createFanInIssue(t, ctx, "leaf child", "in_test", &parentID)
	t.Cleanup(func() { deleteIssues(ctx, childA, parentID, grandparentID) })

	// Move leaf to done; parent should fan-in to done, then grandparent should
	// also fan-in to done because its only child (parent) is now done.
	setIssueStatus(t, ctx, childA, "done")
	testHandler.evaluateParentFanIn(ctx, parseUUID(parentID))

	if got := getIssueStatus(t, ctx, parentID); got != "done" {
		t.Fatalf("parent status = %q, want %q", got, "done")
	}
	if got := getIssueStatus(t, ctx, grandparentID); got != "done" {
		t.Fatalf("grandparent status = %q, want %q (terminal-chain propagation)", got, "done")
	}
}

// TestFanIn_BlockPropagatesToGrandparent verifies a blocked grandchild
// blocks the parent and then the grandparent.
func TestFanIn_BlockPropagatesToGrandparent(t *testing.T) {
	ctx := context.Background()
	grandparentID := createFanInIssue(t, ctx, "fan-in grandparent", "in_dev", nil)
	parentID := createFanInIssue(t, ctx, "fan-in parent", "in_dev", &grandparentID)
	childA := createFanInIssue(t, ctx, "leaf child", "blocked", &parentID)
	t.Cleanup(func() { deleteIssues(ctx, childA, parentID, grandparentID) })

	testHandler.evaluateParentFanIn(ctx, parseUUID(parentID))
	if got := getIssueStatus(t, ctx, parentID); got != "blocked" {
		t.Fatalf("parent status = %q, want %q", got, "blocked")
	}
	if got := getIssueStatus(t, ctx, grandparentID); got != "blocked" {
		t.Fatalf("grandparent status = %q, want %q (block propagation)", got, "blocked")
	}
}

// TestFanIn_HookedFromUpdateIssue verifies the fan-in trigger fires when a
// child issue's status is changed via the normal UpdateIssue HTTP handler —
// the integration point that matters most in production.
func TestFanIn_HookedFromUpdateIssue(t *testing.T) {
	ctx := context.Background()
	parentID := createFanInIssue(t, ctx, "fan-in parent: via http", "in_dev", nil)
	childA := createFanInIssue(t, ctx, "child A", "in_dev", &parentID)
	t.Cleanup(func() { deleteIssues(ctx, childA, parentID) })

	w := httptest.NewRecorder()
	req := newRequest("PUT", "/api/issues/"+childA, map[string]any{"status": "done"})
	req = withURLParam(req, "id", childA)
	testHandler.UpdateIssue(w, req)
	if w.Code != 200 {
		t.Fatalf("UpdateIssue: expected 200, got %d: %s", w.Code, w.Body.String())
	}
	if got := getIssueStatus(t, ctx, parentID); got != "ready_review" {
		t.Fatalf("after child PUT done: parent status = %q, want %q", got, "ready_review")
	}
}
