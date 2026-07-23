package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCreateCommentRejectsStaleAgentMention(t *testing.T) {
	if testHandler == nil || testPool == nil {
		t.Skip("database not available")
	}
	ctx := context.Background()
	issueID := createCommentTriggerPreviewIssue(t, "stale mention create", "", "")
	staleAgentID := "01afc25a-108d-4094-9689-16500b2f1e05"

	w := httptest.NewRecorder()
	r := newRequest(http.MethodPost, "/api/issues/"+issueID+"/comments", map[string]any{
		"content": "handoff to [@alex](mention://agent/" + staleAgentID + ")",
	})
	r = withURLParam(r, "id", issueID)
	testHandler.CreateComment(w, r)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("CreateComment: expected 400 for stale agent mention, got %d: %s", w.Code, w.Body.String())
	}

	var count int
	if err := testPool.QueryRow(ctx, `
		SELECT count(*) FROM comment
		WHERE issue_id = $1 AND content LIKE '%' || $2 || '%'
	`, issueID, staleAgentID).Scan(&count); err != nil {
		t.Fatalf("count stale comments: %v", err)
	}
	if count != 0 {
		t.Fatalf("stale mention comment was stored %d time(s), want 0", count)
	}
}

func TestUpdateCommentRejectsStaleAgentMention(t *testing.T) {
	if testHandler == nil || testPool == nil {
		t.Skip("database not available")
	}
	issueID := createCommentTriggerPreviewIssue(t, "stale mention update", "", "")
	commentID := postCommentForTriggerPreviewTest(t, issueID, map[string]any{
		"content": "plain comment",
	})
	staleAgentID := "01afc25a-108d-4094-9689-16500b2f1e05"

	w := httptest.NewRecorder()
	r := newRequest(http.MethodPut, "/api/comments/"+commentID, map[string]any{
		"content": "handoff to [@alex](mention://agent/" + staleAgentID + ")",
	})
	r = withURLParam(r, "commentId", commentID)
	testHandler.UpdateComment(w, r)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("UpdateComment: expected 400 for stale agent mention, got %d: %s", w.Code, w.Body.String())
	}

	var resp struct {
		Content string `json:"content"`
	}
	row := testPool.QueryRow(context.Background(), `SELECT content FROM comment WHERE id = $1`, commentID)
	if err := row.Scan(&resp.Content); err != nil {
		t.Fatalf("load stored comment: %v", err)
	}
	if resp.Content != "plain comment" {
		b, _ := json.Marshal(resp)
		t.Fatalf("stored comment changed after rejected update: %s", string(b))
	}
}
