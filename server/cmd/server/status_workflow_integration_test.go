package main

import (
	"fmt"
	"io"
	"testing"
)

// updateIssueStatus sends PUT /api/issues/:id with the given status and returns
// the decoded response map. Fails the test on non-200 status codes.
func updateIssueStatus(t *testing.T, issueID, status string) map[string]any {
	t.Helper()
	resp := authRequest(t, "PUT", "/api/issues/"+issueID, map[string]any{
		"status": status,
	})
	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		t.Fatalf("updateIssueStatus(%s→%s): expected 200, got %d: %s", issueID, status, resp.StatusCode, body)
	}
	var issue map[string]any
	readJSON(t, resp, &issue)
	return issue
}

// updateIssueStatusWithAgent sends PUT /api/issues/:id with X-Agent-ID header.
func updateIssueStatusWithAgent(t *testing.T, issueID, status, agentID string) map[string]any {
	t.Helper()
	resp := authRequestWithAgent(t, "PUT", "/api/issues/"+issueID, map[string]any{
		"status": status,
	}, agentID)
	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		t.Fatalf("updateIssueStatusWithAgent(%s→%s): expected 200, got %d: %s", issueID, status, resp.StatusCode, body)
	}
	var issue map[string]any
	readJSON(t, resp, &issue)
	return issue
}

// createAndCleanupIssue creates an issue and registers a t.Cleanup to delete it.
func createAndCleanupIssue(t *testing.T, title string) string {
	t.Helper()
	issueID := createIssue(t, title)
	t.Cleanup(func() {
		resp := authRequest(t, "DELETE", "/api/issues/"+issueID, nil)
		resp.Body.Close()
	})
	return issueID
}

// setIssueStatus is a shortcut to move an issue to a target status without assertions.
func setIssueStatus(t *testing.T, issueID, status string) {
	t.Helper()
	resp := authRequest(t, "PUT", "/api/issues/"+issueID, map[string]any{
		"status": status,
	})
	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		t.Fatalf("setIssueStatus(%s): expected 200, got %d: %s", status, resp.StatusCode, body)
	}
	resp.Body.Close()
}

// TestStatusTransitionViaAPI verifies all legal sequential status transitions
// via PUT /api/issues/:id using table-driven tests.
func TestStatusTransitionViaAPI(t *testing.T) {
	tests := []struct {
		name   string
		from   string
		to     string
		setup  []string // intermediate statuses to reach "from" starting from backlog
	}{
		// Sequential workflow transitions
		{"backlog to classifying", "backlog", "classifying", nil},
		{"classifying to ready_analyze", "classifying", "ready_analyze", []string{"classifying"}},
		{"classifying to ready_arch_design (tech card)", "classifying", "ready_arch_design", []string{"classifying"}},
		{"ready_analyze to in_analyze", "ready_analyze", "in_analyze", []string{"classifying", "ready_analyze"}},
		{"in_analyze to ready_arch_design", "in_analyze", "ready_arch_design", []string{"classifying", "ready_analyze", "in_analyze"}},
		{"ready_arch_design to in_arch_design", "ready_arch_design", "in_arch_design", []string{"classifying", "ready_arch_design"}},
		{"in_arch_design to ready_dev", "in_arch_design", "ready_dev", []string{"classifying", "ready_arch_design", "in_arch_design"}},
		{"ready_dev to in_dev", "ready_dev", "in_dev", []string{"classifying", "ready_arch_design", "in_arch_design", "ready_dev"}},
		{"in_dev to ready_review", "in_dev", "ready_review", []string{"classifying", "ready_arch_design", "in_arch_design", "ready_dev", "in_dev"}},
		{"ready_review to in_review", "ready_review", "in_review", []string{"classifying", "ready_arch_design", "in_arch_design", "ready_dev", "in_dev", "ready_review"}},
		{"in_review to ready_test", "in_review", "ready_test", []string{"classifying", "ready_arch_design", "in_arch_design", "ready_dev", "in_dev", "ready_review", "in_review"}},
		{"ready_test to in_test", "ready_test", "in_test", []string{"classifying", "ready_arch_design", "in_arch_design", "ready_dev", "in_dev", "ready_review", "in_review", "ready_test"}},
		{"in_test to done", "in_test", "done", []string{"classifying", "ready_arch_design", "in_arch_design", "ready_dev", "in_dev", "ready_review", "in_review", "ready_test", "in_test"}},

		// Blocked transitions from in_* statuses
		{"in_analyze to blocked", "in_analyze", "blocked", []string{"classifying", "ready_analyze", "in_analyze"}},
		{"in_arch_design to blocked", "in_arch_design", "blocked", []string{"classifying", "ready_arch_design", "in_arch_design"}},
		{"in_dev to blocked", "in_dev", "blocked", []string{"classifying", "ready_arch_design", "in_arch_design", "ready_dev", "in_dev"}},
		{"in_review to blocked", "in_review", "blocked", []string{"classifying", "ready_arch_design", "in_arch_design", "ready_dev", "in_dev", "ready_review", "in_review"}},
		{"in_test to blocked", "in_test", "blocked", []string{"classifying", "ready_arch_design", "in_arch_design", "ready_dev", "in_dev", "ready_review", "in_review", "ready_test", "in_test"}},

		// Blocked recovery
		{"blocked to backlog", "blocked", "backlog", []string{"blocked"}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			issueID := createAndCleanupIssue(t, fmt.Sprintf("status transition: %s", tc.name))

			// Walk through setup statuses to reach the "from" state.
			for _, s := range tc.setup {
				setIssueStatus(t, issueID, s)
			}

			// Perform the transition under test.
			updated := updateIssueStatus(t, issueID, tc.to)
			if got := updated["status"]; got != tc.to {
				t.Errorf("expected status %q, got %q", tc.to, got)
			}
		})
	}
}

// TestAgentStatusTransitionFlow simulates each agent role pushing an issue
// through its portion of the workflow via the API with X-Agent-ID header.
func TestAgentStatusTransitionFlow(t *testing.T) {
	agentID := getAgentID(t)

	tests := []struct {
		name        string
		transitions []string // statuses to transition through in order
	}{
		{
			"Classifier Agent: backlog → classifying → ready_analyze",
			[]string{"classifying", "ready_analyze"},
		},
		{
			"BA Agent: ready_analyze → in_analyze → ready_arch_design",
			[]string{"classifying", "ready_analyze", "in_analyze", "ready_arch_design"},
		},
		{
			"TL Agent: ready_arch_design → in_arch_design → ready_dev",
			[]string{"classifying", "ready_arch_design", "in_arch_design", "ready_dev"},
		},
		{
			"Dev Agent: ready_dev → in_dev → ready_review",
			[]string{"classifying", "ready_arch_design", "in_arch_design", "ready_dev", "in_dev", "ready_review"},
		},
		{
			"Review Agent: ready_review → in_review → ready_test",
			[]string{"classifying", "ready_arch_design", "in_arch_design", "ready_dev", "in_dev", "ready_review", "in_review", "ready_test"},
		},
		{
			"QA Agent: ready_test → in_test → done",
			[]string{"classifying", "ready_arch_design", "in_arch_design", "ready_dev", "in_dev", "ready_review", "in_review", "ready_test", "in_test", "done"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			issueID := createAndCleanupIssue(t, fmt.Sprintf("agent flow: %s", tc.name))

			for _, status := range tc.transitions {
				updated := updateIssueStatusWithAgent(t, issueID, status, agentID)
				if got := updated["status"]; got != status {
					t.Fatalf("expected status %q, got %q", status, got)
				}
			}
		})
	}
}

// TestBlockedTransitionFlow verifies that blocked status can be entered from
// and recovered to various in-progress statuses.
func TestBlockedTransitionFlow(t *testing.T) {
	tests := []struct {
		name   string
		status string
		setup  []string // path from backlog to reach the status
	}{
		{"in_dev", "in_dev", []string{"classifying", "ready_arch_design", "in_arch_design", "ready_dev", "in_dev"}},
		{"in_review", "in_review", []string{"classifying", "ready_arch_design", "in_arch_design", "ready_dev", "in_dev", "ready_review", "in_review"}},
		{"in_test", "in_test", []string{"classifying", "ready_arch_design", "in_arch_design", "ready_dev", "in_dev", "ready_review", "in_review", "ready_test", "in_test"}},
	}

	for _, tc := range tests {
		t.Run(fmt.Sprintf("%s to blocked and back", tc.name), func(t *testing.T) {
			issueID := createAndCleanupIssue(t, fmt.Sprintf("blocked flow: %s", tc.name))

			// Setup: walk to the target status.
			for _, s := range tc.setup {
				setIssueStatus(t, issueID, s)
			}

			// Transition to blocked.
			blocked := updateIssueStatus(t, issueID, "blocked")
			if got := blocked["status"]; got != "blocked" {
				t.Fatalf("expected status 'blocked', got %q", got)
			}

			// Recover back to the original status.
			recovered := updateIssueStatus(t, issueID, tc.status)
			if got := recovered["status"]; got != tc.status {
				t.Fatalf("expected status %q after recovery, got %q", tc.status, got)
			}
		})
	}
}

// TestCancelledTransition verifies that issues can be cancelled from various
// states and that cancelled issues remain readable via the API.
func TestCancelledTransition(t *testing.T) {
	t.Run("backlog to cancelled", func(t *testing.T) {
		issueID := createAndCleanupIssue(t, "cancel from backlog")

		updated := updateIssueStatus(t, issueID, "cancelled")
		if got := updated["status"]; got != "cancelled" {
			t.Errorf("expected status 'cancelled', got %q", got)
		}
	})

	t.Run("in_dev to cancelled", func(t *testing.T) {
		issueID := createAndCleanupIssue(t, "cancel from in_dev")

		// Walk to in_dev.
		for _, s := range []string{"classifying", "ready_arch_design", "in_arch_design", "ready_dev", "in_dev"} {
			setIssueStatus(t, issueID, s)
		}

		updated := updateIssueStatus(t, issueID, "cancelled")
		if got := updated["status"]; got != "cancelled" {
			t.Errorf("expected status 'cancelled', got %q", got)
		}
	})

	t.Run("cancelled issue is still readable", func(t *testing.T) {
		issueID := createAndCleanupIssue(t, "read cancelled issue")

		setIssueStatus(t, issueID, "cancelled")

		// GET the issue and verify it's still accessible.
		resp := authRequest(t, "GET", "/api/issues/"+issueID, nil)
		if resp.StatusCode != 200 {
			body, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			t.Fatalf("expected 200 reading cancelled issue, got %d: %s", resp.StatusCode, body)
		}
		var issue map[string]any
		readJSON(t, resp, &issue)
		if got := issue["status"]; got != "cancelled" {
			t.Errorf("expected status 'cancelled' on GET, got %q", got)
		}
	})
}

// TestStatusPersistsAfterUpdate verifies that a status change via PUT is
// persisted and returned correctly by a subsequent GET.
func TestStatusPersistsAfterUpdate(t *testing.T) {
	issueID := createAndCleanupIssue(t, "status persistence test")

	// Walk to in_dev.
	for _, s := range []string{"classifying", "ready_arch_design", "in_arch_design", "ready_dev", "in_dev"} {
		setIssueStatus(t, issueID, s)
	}

	// GET the issue and verify the status persisted.
	resp := authRequest(t, "GET", "/api/issues/"+issueID, nil)
	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		t.Fatalf("expected 200, got %d: %s", resp.StatusCode, body)
	}
	var issue map[string]any
	readJSON(t, resp, &issue)
	if got := issue["status"]; got != "in_dev" {
		t.Errorf("expected status 'in_dev' persisted via GET, got %q", got)
	}
}
