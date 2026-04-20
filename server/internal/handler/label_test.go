package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
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
