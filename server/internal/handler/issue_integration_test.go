package handler

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/multica-ai/multica/server/internal/middleware"
	"github.com/multica-ai/multica/server/internal/service/issuebridge"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

func TestCreateGitLabIssueIntegrationCreatesDefaultSkill(t *testing.T) {
	if testHandler == nil || testPool == nil {
		t.Skip("handler test fixture not available")
	}
	ctx := context.Background()
	withIssueBridgeService(t, nil)
	cleanupIssueBridgeRows(t)

	w := httptest.NewRecorder()
	req := newRequest(http.MethodPost, "/api/issue-integrations/gitlab", map[string]any{
		"name":     "handler-test-create-default-skill",
		"base_url": "https://gitlab.example.com",
		"token":    "secret-token",
	})
	testHandler.CreateGitLabIssueIntegration(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("status %d body %s", w.Code, w.Body.String())
	}
	var resp IssueIntegrationResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.DefaultIssueSkillID == nil || *resp.DefaultIssueSkillID == "" {
		t.Fatalf("expected default_issue_skill_id, got %+v", resp)
	}
	var skillName string
	if err := testPool.QueryRow(ctx, `SELECT name FROM skill WHERE id = $1`, *resp.DefaultIssueSkillID).Scan(&skillName); err != nil {
		t.Fatalf("load created skill: %v", err)
	}
	if skillName != issuebridge.DefaultIssueCreatorName {
		t.Fatalf("expected default skill %q, got %q", issuebridge.DefaultIssueCreatorName, skillName)
	}
}

func TestListIssueIntegrationsDoesNotReturnToken(t *testing.T) {
	if testHandler == nil || testPool == nil {
		t.Skip("handler test fixture not available")
	}
	withIssueBridgeService(t, nil)
	cleanupIssueBridgeRows(t)
	ctx := context.Background()
	_, err := testHandler.Queries.CreateIssueIntegration(ctx, db.CreateIssueIntegrationParams{
		WorkspaceID:                parseUUID(testWorkspaceID),
		Provider:                   issuebridge.ProviderGitLab,
		Name:                       "handler-test-list-redaction",
		BaseUrl:                    "https://gitlab.example.com",
		EncryptedToken:             "encrypted-secret-token",
		DefaultPollIntervalSeconds: issuebridge.DefaultPollIntervalSeconds,
		Config:                     []byte(`{"visible":true}`),
	})
	if err != nil {
		t.Fatalf("create integration: %v", err)
	}

	w := httptest.NewRecorder()
	testHandler.ListIssueIntegrations(w, newRequest(http.MethodGet, "/api/issue-integrations", nil))

	if w.Code != http.StatusOK {
		t.Fatalf("status %d body %s", w.Code, w.Body.String())
	}
	body := w.Body.String()
	if strings.Contains(body, "encrypted_token") || strings.Contains(body, "encrypted-secret-token") || strings.Contains(body, "secret-token") {
		t.Fatalf("response leaked token material: %s", body)
	}
	var decoded struct {
		Integrations []map[string]any `json:"integrations"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &decoded); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	for _, integration := range decoded.Integrations {
		if _, ok := integration["token"]; ok {
			t.Fatalf("response exposed token field: %+v", integration)
		}
		if _, ok := integration["encrypted_token"]; ok {
			t.Fatalf("response exposed encrypted_token field: %+v", integration)
		}
	}
}

func TestCreateIssueSyncConfigRequiresWorkspaceScopedIntegration(t *testing.T) {
	if testHandler == nil || testPool == nil {
		t.Skip("handler test fixture not available")
	}
	withIssueBridgeService(t, nil)
	cleanupIssueBridgeRows(t)
	ctx := context.Background()

	var otherWorkspaceID string
	_, _ = testPool.Exec(ctx, `DELETE FROM workspace WHERE slug = 'issue-bridge-other-workspace'`)
	if err := testPool.QueryRow(ctx, `
		INSERT INTO workspace (name, slug, description, issue_prefix)
		VALUES ('Issue Bridge Other', 'issue-bridge-other-workspace', '', 'IBO')
		RETURNING id
	`).Scan(&otherWorkspaceID); err != nil {
		t.Fatalf("create other workspace: %v", err)
	}
	t.Cleanup(func() {
		_, _ = testPool.Exec(context.Background(), `DELETE FROM workspace WHERE id = $1`, otherWorkspaceID)
	})
	otherIntegration, err := testHandler.Queries.CreateIssueIntegration(ctx, db.CreateIssueIntegrationParams{
		WorkspaceID:                parseUUID(otherWorkspaceID),
		Provider:                   issuebridge.ProviderGitLab,
		Name:                       "handler-test-cross-workspace",
		BaseUrl:                    "https://gitlab.example.com",
		EncryptedToken:             "encrypted-other-token",
		DefaultPollIntervalSeconds: issuebridge.DefaultPollIntervalSeconds,
		Config:                     []byte(`{}`),
	})
	if err != nil {
		t.Fatalf("create other integration: %v", err)
	}
	projectID := createIssueBridgeTestProject(t, "Issue bridge scoped project")

	w := httptest.NewRecorder()
	req := newRequest(http.MethodPost, "/api/issue-sync-configs", map[string]any{
		"integration_id":     uuidToString(otherIntegration.ID),
		"scope_type":         "project",
		"scope_id":           projectID,
		"remote_project_ref": "group/project",
	})
	testHandler.CreateIssueSyncConfig(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("status %d body %s", w.Code, w.Body.String())
	}
}

func TestTestIssueIntegrationReturnsGitLabUser(t *testing.T) {
	if testHandler == nil || testPool == nil {
		t.Skip("handler test fixture not available")
	}
	cleanupIssueBridgeRows(t)
	gitlabServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v4/user" {
			http.NotFound(w, r)
			return
		}
		if r.Header.Get("Authorization") != "Bearer secret-token" {
			http.Error(w, "bad auth", http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":42,"username":"octavia","name":"Octavia Butler"}`))
	}))
	t.Cleanup(gitlabServer.Close)
	svc := issuebridge.NewService(testHandler.Queries, handlerIssueBridgeTestBox{})
	svc.ClientFactory = func(baseURL, token string) (issuebridge.GitLabClientAPI, error) {
		return issuebridge.NewGitLabClientWithHTTPClient(baseURL, token, gitlabServer.Client())
	}
	withIssueBridgeService(t, svc)

	row, err := svc.CreateGitLabIntegration(context.Background(), issuebridge.SaveIntegrationInput{
		WorkspaceID:                parseUUID(testWorkspaceID),
		Name:                       "handler-test-gitlab-user",
		BaseURL:                    gitlabServer.URL,
		Token:                      "secret-token",
		DefaultPollIntervalSeconds: issuebridge.DefaultPollIntervalSeconds,
		Config:                     []byte(`{}`),
	})
	if err != nil {
		t.Fatalf("create integration: %v", err)
	}

	w := httptest.NewRecorder()
	req := withURLParam(newRequest(http.MethodPost, "/api/issue-integrations/"+uuidToString(row.ID)+"/test", nil), "id", uuidToString(row.ID))
	testHandler.TestIssueIntegration(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status %d body %s", w.Code, w.Body.String())
	}
	var resp struct {
		OK       bool   `json:"ok"`
		Provider string `json:"provider"`
		Username string `json:"username"`
		Name     string `json:"name"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if !resp.OK || resp.Provider != issuebridge.ProviderGitLab || resp.Username != "octavia" || resp.Name != "Octavia Butler" {
		t.Fatalf("unexpected response: %+v", resp)
	}
}

func TestDeleteIssueIntegrationReturnsNotFound(t *testing.T) {
	if testHandler == nil || testPool == nil {
		t.Skip("handler test fixture not available")
	}
	withIssueBridgeService(t, nil)

	w := httptest.NewRecorder()
	missingID := "00000000-0000-0000-0000-000000000001"
	req := withURLParam(newRequest(http.MethodDelete, "/api/issue-integrations/"+missingID, nil), "id", missingID)
	testHandler.DeleteIssueIntegration(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("status %d body %s", w.Code, w.Body.String())
	}
}

func TestIssueBridgeRoutesRoleGating(t *testing.T) {
	if testHandler == nil || testPool == nil {
		t.Skip("handler test fixture not available")
	}
	ctx := context.Background()
	withIssueBridgeService(t, nil)

	const slug = "issue-bridge-routes-role-gating"
	_, _ = testPool.Exec(ctx, `DELETE FROM workspace WHERE slug = $1`, slug)

	var wsID string
	if err := testPool.QueryRow(ctx, `
INSERT INTO workspace (name, slug, description, issue_prefix)
VALUES ($1, $2, $3, $4)
RETURNING id
`, "Issue Bridge Routes Role Gating", slug, "issue bridge routes role gating", "IBR").Scan(&wsID); err != nil {
		t.Fatalf("create workspace: %v", err)
	}

	mkUser := func(t *testing.T, label string) string {
		t.Helper()
		var id string
		email := fmt.Sprintf("issue-bridge-routes-%s-%s@multica.ai", slug, label)
		if err := testPool.QueryRow(ctx, `
INSERT INTO "user" (name, email) VALUES ($1, $2) RETURNING id
`, "IBR "+label, email).Scan(&id); err != nil {
			t.Fatalf("create user %s: %v", label, err)
		}
		return id
	}
	adminUserID := mkUser(t, "admin")
	memberUserID := mkUser(t, "member")
	outsiderUserID := mkUser(t, "outsider")

	for _, m := range []struct {
		userID string
		role   string
	}{
		{adminUserID, "admin"},
		{memberUserID, "member"},
	} {
		if _, err := testPool.Exec(ctx, `
INSERT INTO member (workspace_id, user_id, role) VALUES ($1, $2, $3)
`, wsID, m.userID, m.role); err != nil {
			t.Fatalf("insert member (%s): %v", m.role, err)
		}
	}

	t.Cleanup(func() {
		_, _ = testPool.Exec(context.Background(), `DELETE FROM workspace WHERE id = $1`, wsID)
		for _, uid := range []string{adminUserID, memberUserID, outsiderUserID} {
			_, _ = testPool.Exec(context.Background(), `DELETE FROM "user" WHERE id = $1`, uid)
		}
	})

	router := chi.NewRouter()
	router.Group(func(r chi.Router) {
		r.Use(middleware.RequireWorkspaceMember(testHandler.Queries))
		r.Get("/api/issue-integrations", testHandler.ListIssueIntegrations)
		r.Group(func(r chi.Router) {
			r.Use(RequireHumanActor)
			r.Use(middleware.RequireWorkspaceRole(testHandler.Queries, "owner", "admin"))
			r.Post("/api/issue-integrations/gitlab", testHandler.CreateGitLabIssueIntegration)
		})
	})

	exercise := func(t *testing.T, method, path, userID, actorSource string) int {
		t.Helper()
		req := httptest.NewRequest(method, path, strings.NewReader(`{"name":"GitLab","base_url":"https://gitlab.example.com","token":"secret-token"}`))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Workspace-ID", wsID)
		if userID != "" {
			req.Header.Set("X-User-ID", userID)
		}
		if actorSource != "" {
			req.Header.Set("X-Actor-Source", actorSource)
		}
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		return rec.Code
	}

	t.Run("read endpoints are member visible", func(t *testing.T) {
		if code := exercise(t, http.MethodGet, "/api/issue-integrations", memberUserID, ""); code != http.StatusOK {
			t.Fatalf("member GET integrations: want 200, got %d", code)
		}
	})

	t.Run("write endpoints require admin", func(t *testing.T) {
		if code := exercise(t, http.MethodPost, "/api/issue-integrations/gitlab", memberUserID, ""); code != http.StatusForbidden {
			t.Fatalf("member POST integration: want 403, got %d", code)
		}
		if code := exercise(t, http.MethodPost, "/api/issue-integrations/gitlab", outsiderUserID, ""); code != http.StatusNotFound {
			t.Fatalf("outsider POST integration: want 404, got %d", code)
		}
	})

	t.Run("write endpoints reject task tokens before admin role succeeds", func(t *testing.T) {
		if code := exercise(t, http.MethodPost, "/api/issue-integrations/gitlab", adminUserID, "task_token"); code != http.StatusForbidden {
			t.Fatalf("task-token admin POST integration: want 403, got %d", code)
		}
	})
}

func withIssueBridgeService(t *testing.T, svc *issuebridge.Service) {
	t.Helper()
	prev := testHandler.IssueBridgeService
	if svc == nil {
		svc = issuebridge.NewService(testHandler.Queries, handlerIssueBridgeTestBox{})
	}
	testHandler.IssueBridgeService = svc
	t.Cleanup(func() {
		testHandler.IssueBridgeService = prev
	})
}

func cleanupIssueBridgeRows(t *testing.T) {
	t.Helper()
	_, _ = testPool.Exec(context.Background(), `DELETE FROM issue_integration WHERE workspace_id = $1 AND name LIKE 'handler-test-%'`, testWorkspaceID)
	t.Cleanup(func() {
		_, _ = testPool.Exec(context.Background(), `DELETE FROM issue_integration WHERE workspace_id = $1 AND name LIKE 'handler-test-%'`, testWorkspaceID)
	})
}

func createIssueBridgeTestProject(t *testing.T, title string) string {
	t.Helper()
	project, err := testHandler.Queries.CreateProject(context.Background(), db.CreateProjectParams{
		WorkspaceID: parseUUID(testWorkspaceID),
		Title:       title,
		Status:      "planned",
		Priority:    "none",
	})
	if err != nil {
		t.Fatalf("create project: %v", err)
	}
	projectID := uuidToString(project.ID)
	t.Cleanup(func() {
		_, _ = testPool.Exec(context.Background(), `DELETE FROM project WHERE id = $1`, projectID)
	})
	return projectID
}

type handlerIssueBridgeTestBox struct{}

func (handlerIssueBridgeTestBox) Seal(plaintext []byte) ([]byte, error) {
	return []byte("sealed:" + base64.StdEncoding.EncodeToString(plaintext)), nil
}

func (handlerIssueBridgeTestBox) Open(sealed []byte) ([]byte, error) {
	const prefix = "sealed:"
	raw := string(sealed)
	if !strings.HasPrefix(raw, prefix) {
		return nil, errors.New("bad seal")
	}
	return base64.StdEncoding.DecodeString(raw[len(prefix):])
}

var _ issuebridge.SecretBox = handlerIssueBridgeTestBox{}
