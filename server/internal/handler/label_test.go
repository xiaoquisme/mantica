package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/xiaoquisme/mantica/server/internal/events"
	"github.com/xiaoquisme/mantica/server/pkg/protocol"
)

// withURLParams sets multiple chi URL parameters on the request at once.
func withURLParams(req *http.Request, kv ...string) *http.Request {
	rctx := chi.NewRouteContext()
	for i := 0; i+1 < len(kv); i += 2 {
		rctx.URLParams.Add(kv[i], kv[i+1])
	}
	return req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
}

func TestCreateLabel_ValidRequest(t *testing.T) {
	ctx := context.Background()

	w := httptest.NewRecorder()
	req := newRequest("POST", "/api/workspaces/"+testWorkspaceID+"/labels", map[string]any{
		"name":  "Bug",
		"color": "#ef4444",
	})
	req = withURLParam(req, "id", testWorkspaceID)
	testHandler.CreateLabel(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("CreateLabel: expected 201, got %d: %s", w.Code, w.Body.String())
	}

	var resp LabelResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.ID == "" {
		t.Fatal("CreateLabel: expected non-empty id")
	}
	if resp.Name != "Bug" {
		t.Errorf("CreateLabel: name: want %q, got %q", "Bug", resp.Name)
	}
	if resp.Color != "#ef4444" {
		t.Errorf("CreateLabel: color: want %q, got %q", "#ef4444", resp.Color)
	}

	t.Cleanup(func() {
		testPool.Exec(ctx, `DELETE FROM issue_label WHERE id = $1`, resp.ID)
	})

	var count int
	if err := testPool.QueryRow(ctx, `SELECT count(*) FROM issue_label WHERE id = $1`, resp.ID).Scan(&count); err != nil {
		t.Fatalf("DB check: %v", err)
	}
	if count != 1 {
		t.Errorf("CreateLabel: expected label persisted in DB, got count=%d", count)
	}
}

func TestCreateLabel_EmptyName(t *testing.T) {
	w := httptest.NewRecorder()
	req := newRequest("POST", "/api/workspaces/"+testWorkspaceID+"/labels", map[string]any{
		"name":  "",
		"color": "#ef4444",
	})
	req = withURLParam(req, "id", testWorkspaceID)
	testHandler.CreateLabel(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("CreateLabel (empty name): expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestCreateLabel_InvalidColor(t *testing.T) {
	w := httptest.NewRecorder()
	req := newRequest("POST", "/api/workspaces/"+testWorkspaceID+"/labels", map[string]any{
		"name":  "Bug",
		"color": "red",
	})
	req = withURLParam(req, "id", testWorkspaceID)
	testHandler.CreateLabel(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("CreateLabel (invalid color): expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestDeleteLabel_Existing(t *testing.T) {
	ctx := context.Background()

	var labelID string
	if err := testPool.QueryRow(ctx,
		`INSERT INTO issue_label (workspace_id, name, color) VALUES ($1, $2, $3) RETURNING id`,
		testWorkspaceID, "Delete Me", "#22c55e",
	).Scan(&labelID); err != nil {
		t.Fatalf("insert label: %v", err)
	}
	t.Cleanup(func() {
		testPool.Exec(ctx, `DELETE FROM issue_label WHERE id = $1`, labelID)
	})

	w := httptest.NewRecorder()
	req := newRequest("DELETE", "/api/workspaces/"+testWorkspaceID+"/labels/"+labelID, nil)
	req = withURLParams(req, "id", testWorkspaceID, "labelId", labelID)
	testHandler.DeleteLabel(w, req)
	if w.Code != http.StatusNoContent {
		t.Fatalf("DeleteLabel: expected 204, got %d: %s", w.Code, w.Body.String())
	}

	var count int
	if err := testPool.QueryRow(ctx, `SELECT count(*) FROM issue_label WHERE id = $1`, labelID).Scan(&count); err != nil {
		t.Fatalf("DB check: %v", err)
	}
	if count != 0 {
		t.Errorf("DeleteLabel: expected label removed from DB, got count=%d", count)
	}
}

func TestDeleteLabel_NotFound(t *testing.T) {
	const nonExistentID = "00000000-0000-0000-0000-000000000099"

	w := httptest.NewRecorder()
	req := newRequest("DELETE", "/api/workspaces/"+testWorkspaceID+"/labels/"+nonExistentID, nil)
	req = withURLParams(req, "id", testWorkspaceID, "labelId", nonExistentID)
	testHandler.DeleteLabel(w, req)
	if w.Code != http.StatusNotFound {
		t.Fatalf("DeleteLabel (not found): expected 404, got %d: %s", w.Code, w.Body.String())
	}
}

// TestUpdateIssueLabels_WSPayload verifies that UpdateIssueLabels publishes a WS event
// with the { issue: { id, labels } } shape expected by the frontend onIssueUpdated handler.
func TestUpdateIssueLabels_WSPayload(t *testing.T) {
	ctx := context.Background()

	// Create an issue to attach labels to.
	var issueID string
	if err := testPool.QueryRow(ctx, `
		INSERT INTO issue (workspace_id, title, status, priority, creator_type, creator_id, number, position)
		VALUES ($1, 'WS payload test', 'backlog', 'none', 'member', $2, 9998, 0)
		RETURNING id
	`, testWorkspaceID, testUserID).Scan(&issueID); err != nil {
		t.Fatalf("insert issue: %v", err)
	}

	// Create a label to assign.
	var labelID string
	if err := testPool.QueryRow(ctx,
		`INSERT INTO issue_label (workspace_id, name, color) VALUES ($1, $2, $3) RETURNING id`,
		testWorkspaceID, "WS-Test-Label", "#3b82f6",
	).Scan(&labelID); err != nil {
		t.Fatalf("insert label: %v", err)
	}

	t.Cleanup(func() {
		testPool.Exec(ctx, `DELETE FROM issue WHERE id = $1`, issueID)
		testPool.Exec(ctx, `DELETE FROM issue_label WHERE id = $1`, labelID)
	})

	// Subscribe to the event bus to capture the published event.
	var capturedEvent events.Event
	testHandler.Bus.Subscribe(protocol.EventIssueUpdated, func(e events.Event) {
		capturedEvent = e
	})

	w := httptest.NewRecorder()
	req := newRequest("PUT", "/api/issues/"+issueID+"/labels", map[string]any{
		"label_ids": []string{labelID},
	})
	req = withURLParam(req, "id", issueID)
	testHandler.UpdateIssueLabels(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("UpdateIssueLabels: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	// Verify the event was published with the correct payload shape.
	if capturedEvent.Type != protocol.EventIssueUpdated {
		t.Fatalf("expected event type %q, got %q", protocol.EventIssueUpdated, capturedEvent.Type)
	}

	payload, ok := capturedEvent.Payload.(map[string]any)
	if !ok {
		t.Fatalf("expected payload to be map[string]any, got %T", capturedEvent.Payload)
	}

	issue, ok := payload["issue"].(map[string]any)
	if !ok {
		t.Fatalf("expected payload[\"issue\"] to be map[string]any, got %T — missing or wrong key", payload["issue"])
	}

	if issue["id"] != issueID {
		t.Errorf("payload[\"issue\"][\"id\"]: want %q, got %v", issueID, issue["id"])
	}

	labels, ok := issue["labels"].([]LabelResponse)
	if !ok {
		t.Fatalf("expected payload[\"issue\"][\"labels\"] to be []LabelResponse, got %T", issue["labels"])
	}
	if len(labels) != 1 {
		t.Fatalf("expected 1 label in WS payload, got %d", len(labels))
	}
	if labels[0].ID != labelID {
		t.Errorf("WS label id: want %q, got %q", labelID, labels[0].ID)
	}
}
