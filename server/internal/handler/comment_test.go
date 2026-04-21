package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestCreateReplyComment_IdentifierURL verifies that posting a reply comment
// succeeds when the issue URL uses the identifier format (e.g. "HAN-1") rather
// than the UUID. This is the regression case for TES-132 where the parent-comment
// ownership check compared a UUID against a raw identifier string, always failing.
func TestCreateReplyComment_IdentifierURL(t *testing.T) {
	ctx := context.Background()

	// Create an issue so we have a real identifier to use in the URL.
	w := httptest.NewRecorder()
	req := newRequest("POST", "/api/issues?workspace_id="+testWorkspaceID, map[string]any{
		"title": "Reply comment identifier URL test",
	})
	testHandler.CreateIssue(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("CreateIssue: expected 201, got %d: %s", w.Code, w.Body.String())
	}
	var issue IssueResponse
	json.NewDecoder(w.Body).Decode(&issue)
	t.Cleanup(func() {
		testPool.Exec(ctx, `DELETE FROM issue WHERE id = $1`, issue.ID)
	})

	identifier := issue.Identifier
	if identifier == "" {
		t.Fatal("created issue has no identifier")
	}

	// Post a top-level comment using the UUID (known-good path) to obtain a parent_id.
	w = httptest.NewRecorder()
	req = newRequest("POST", "/api/issues/"+issue.ID+"/comments", map[string]any{
		"content": "Top-level comment",
	})
	req = withURLParam(req, "id", issue.ID)
	testHandler.CreateComment(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("CreateComment (top-level): expected 201, got %d: %s", w.Code, w.Body.String())
	}
	var parent CommentResponse
	json.NewDecoder(w.Body).Decode(&parent)

	// Post a reply using the identifier in the URL param — this is the regression path.
	w = httptest.NewRecorder()
	req = newRequest("POST", "/api/issues/"+identifier+"/comments", map[string]any{
		"content":   "Reply via identifier URL",
		"parent_id": parent.ID,
	})
	req = withURLParam(req, "id", identifier)
	testHandler.CreateComment(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("CreateComment (reply via identifier): expected 201, got %d: %s", w.Code, w.Body.String())
	}
}
