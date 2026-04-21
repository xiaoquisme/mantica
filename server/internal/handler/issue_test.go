package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGetIssue_ByIdentifier(t *testing.T) {
	ctx := context.Background()

	// Create an issue to get its assigned identifier.
	w := httptest.NewRecorder()
	req := newRequest("POST", "/api/issues?workspace_id="+testWorkspaceID, map[string]any{
		"title":  "Identifier lookup test",
		"status": "backlog",
	})
	testHandler.CreateIssue(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("CreateIssue: %d: %s", w.Code, w.Body.String())
	}
	var created IssueResponse
	json.NewDecoder(w.Body).Decode(&created)
	t.Cleanup(func() {
		testPool.Exec(ctx, `DELETE FROM issue WHERE id = $1`, created.ID)
	})

	identifier := created.Identifier
	if identifier == "" {
		t.Fatal("created issue has no identifier")
	}

	// GET by identifier.
	w2 := httptest.NewRecorder()
	req2 := newRequest("GET", "/api/issues/"+identifier, nil)
	req2 = withURLParam(req2, "id", identifier)
	testHandler.GetIssue(w2, req2)
	if w2.Code != http.StatusOK {
		t.Fatalf("GetIssue by identifier: expected 200, got %d: %s", w2.Code, w2.Body.String())
	}
	var got IssueResponse
	json.NewDecoder(w2.Body).Decode(&got)
	if got.ID != created.ID {
		t.Errorf("got issue ID %s, want %s", got.ID, created.ID)
	}
	if got.Identifier != identifier {
		t.Errorf("got identifier %s, want %s", got.Identifier, identifier)
	}

	// 404 for non-existent identifier.
	w3 := httptest.NewRecorder()
	req3 := newRequest("GET", "/api/issues/HAN-999999", nil)
	req3 = withURLParam(req3, "id", "HAN-999999")
	testHandler.GetIssue(w3, req3)
	if w3.Code != http.StatusNotFound {
		t.Errorf("expected 404 for missing identifier, got %d", w3.Code)
	}
}

// TestListIssues_LabelsAttached verifies that attachLabelsToResponses correctly
// populates labels on issues returned by the list endpoint.
func TestListIssues_LabelsAttached(t *testing.T) {
	ctx := context.Background()

	// Create an issue.
	w := httptest.NewRecorder()
	req := newRequest("POST", "/api/issues?workspace_id="+testWorkspaceID, map[string]any{
		"title":  "Label batch-fetch test issue",
		"status": "backlog",
	})
	testHandler.CreateIssue(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("CreateIssue: expected 201, got %d: %s", w.Code, w.Body.String())
	}
	var issue IssueResponse
	json.NewDecoder(w.Body).Decode(&issue)
	issueID := issue.ID

	// Create a label directly in the DB (no sqlc generated query for creation).
	var labelID string
	err := testPool.QueryRow(ctx,
		`INSERT INTO issue_label (workspace_id, name, color) VALUES ($1, $2, $3) RETURNING id`,
		testWorkspaceID, "Test Label", "#3b82f6",
	).Scan(&labelID)
	if err != nil {
		t.Fatalf("insert issue_label: %v", err)
	}

	t.Cleanup(func() {
		testPool.Exec(ctx, `DELETE FROM issue_to_label WHERE issue_id = $1`, issueID)
		testPool.Exec(ctx, `DELETE FROM issue_label WHERE id = $1`, labelID)
		testPool.Exec(ctx, `DELETE FROM issue WHERE id = $1`, issueID)
	})

	// Associate label with issue.
	_, err = testPool.Exec(ctx,
		`INSERT INTO issue_to_label (issue_id, label_id) VALUES ($1, $2)`,
		issueID, labelID,
	)
	if err != nil {
		t.Fatalf("insert issue_to_label: %v", err)
	}

	// Call ListIssues — labels must be present in the response.
	w = httptest.NewRecorder()
	req = newRequest("GET", "/api/issues?workspace_id="+testWorkspaceID, nil)
	testHandler.ListIssues(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("ListIssues: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var listResp struct {
		Issues []IssueResponse `json:"issues"`
		Total  int             `json:"total"`
	}
	if err := json.NewDecoder(w.Body).Decode(&listResp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	var found *IssueResponse
	for i := range listResp.Issues {
		if listResp.Issues[i].ID == issueID {
			found = &listResp.Issues[i]
			break
		}
	}
	if found == nil {
		t.Fatalf("issue %s not found in list response", issueID)
	}
	if len(found.Labels) != 1 {
		t.Fatalf("expected 1 label on issue, got %d", len(found.Labels))
	}
	if found.Labels[0].Name != "Test Label" {
		t.Errorf("label name: want %q, got %q", "Test Label", found.Labels[0].Name)
	}
	if found.Labels[0].Color != "#3b82f6" {
		t.Errorf("label color: want %q, got %q", "#3b82f6", found.Labels[0].Color)
	}
}

// TestListIssues_NoLabels verifies that issues with no labels have a nil/empty
// Labels field, not a populated slice.
func TestListIssues_NoLabels(t *testing.T) {
	ctx := context.Background()

	w := httptest.NewRecorder()
	req := newRequest("POST", "/api/issues?workspace_id="+testWorkspaceID, map[string]any{
		"title":  "No-label test issue",
		"status": "backlog",
	})
	testHandler.CreateIssue(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("CreateIssue: expected 201, got %d: %s", w.Code, w.Body.String())
	}
	var issue IssueResponse
	json.NewDecoder(w.Body).Decode(&issue)
	issueID := issue.ID

	t.Cleanup(func() {
		testPool.Exec(ctx, `DELETE FROM issue WHERE id = $1`, issueID)
	})

	w = httptest.NewRecorder()
	req = newRequest("GET", "/api/issues?workspace_id="+testWorkspaceID, nil)
	testHandler.ListIssues(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("ListIssues: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var listResp struct {
		Issues []IssueResponse `json:"issues"`
	}
	json.NewDecoder(w.Body).Decode(&listResp)

	for _, iss := range listResp.Issues {
		if iss.ID == issueID {
			if len(iss.Labels) != 0 {
				t.Errorf("expected no labels on issue, got %d", len(iss.Labels))
			}
			return
		}
	}
	t.Fatalf("issue %s not found in list response", issueID)
}

// TestAttachLabelsToResponses_Empty verifies that the helper is a no-op on an
// empty slice (no panic, no DB call needed).
func TestAttachLabelsToResponses_Empty(t *testing.T) {
	testHandler.attachLabelsToResponses(context.Background(), nil)
	testHandler.attachLabelsToResponses(context.Background(), []IssueResponse{})
}
