package issuebridge

import (
	"context"
	"encoding/base64"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

func TestEnsureDefaultIssueCreatorReusesExistingSkill(t *testing.T) {
	ctx := context.Background()
	workspaceID := testUUID(1)
	creatorID := testUUID(2)
	existing := db.Skill{
		ID:          testUUID(3),
		WorkspaceID: workspaceID,
		Name:        DefaultIssueCreatorName,
		Description: "custom description",
		Content:     "custom content",
	}
	queries := &fakeQueries{skill: existing}
	svc := NewService(queries, nil)

	got, err := svc.EnsureDefaultIssueCreator(ctx, workspaceID, creatorID)
	if err != nil {
		t.Fatalf("EnsureDefaultIssueCreator: %v", err)
	}
	if got.ID != existing.ID {
		t.Fatalf("expected existing skill, got %+v", got)
	}
	if queries.createSkillCalls != 0 {
		t.Fatalf("expected no skill creation, got %d calls", queries.createSkillCalls)
	}
	if queries.getSkillArg.WorkspaceID != workspaceID || queries.getSkillArg.Name != DefaultIssueCreatorName {
		t.Fatalf("unexpected lookup arg: %+v", queries.getSkillArg)
	}
}

func TestEnsureDefaultIssueCreatorCreatesMissingSkill(t *testing.T) {
	ctx := context.Background()
	workspaceID := testUUID(4)
	creatorID := testUUID(5)
	created := db.Skill{
		ID:          testUUID(6),
		WorkspaceID: workspaceID,
		Name:        DefaultIssueCreatorName,
		Description: DefaultIssueCreatorDescription,
		Content:     DefaultIssueCreatorContent,
		CreatedBy:   creatorID,
	}
	queries := &fakeQueries{
		getSkillErr: pgx.ErrNoRows,
		created:     created,
	}
	svc := NewService(queries, nil)

	got, err := svc.EnsureDefaultIssueCreator(ctx, workspaceID, creatorID)
	if err != nil {
		t.Fatalf("EnsureDefaultIssueCreator: %v", err)
	}
	if got.ID != created.ID {
		t.Fatalf("expected created skill, got %+v", got)
	}
	if queries.createSkillCalls != 1 {
		t.Fatalf("expected one skill creation, got %d calls", queries.createSkillCalls)
	}
	arg := queries.createSkillArg
	if arg.WorkspaceID != workspaceID || arg.CreatedBy != creatorID {
		t.Fatalf("unexpected create ownership: %+v", arg)
	}
	if arg.Name != DefaultIssueCreatorName {
		t.Fatalf("unexpected skill name: %q", arg.Name)
	}
	if arg.Description != DefaultIssueCreatorDescription {
		t.Fatalf("unexpected skill description: %q", arg.Description)
	}
	if arg.Content != DefaultIssueCreatorContent {
		t.Fatalf("unexpected skill content")
	}
	if string(arg.Config) != "{}" {
		t.Fatalf("unexpected skill config: %s", arg.Config)
	}
}

func TestGitLabClientTestConnectionUsesBearerToken(t *testing.T) {
	ctx := context.Background()
	var gotAuth string
	var gotPath string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotAuth = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"id":1}`))
	}))
	t.Cleanup(server.Close)

	client, err := NewGitLabClientWithHTTPClient(server.URL+"/", "secret-token", server.Client())
	if err != nil {
		t.Fatalf("NewGitLabClientWithHTTPClient: %v", err)
	}
	user, err := client.TestConnection(ctx)
	if err != nil {
		t.Fatalf("TestConnection: %v", err)
	}
	if user.ID != 1 {
		t.Fatalf("unexpected user: %+v", user)
	}
	if gotPath != "/api/v4/user" {
		t.Fatalf("unexpected path: %s", gotPath)
	}
	if gotAuth != "Bearer secret-token" {
		t.Fatalf("unexpected authorization header: %q", gotAuth)
	}
}

func TestNormalizeGitLabBaseURLAcceptsApiV4Path(t *testing.T) {
	got, err := NormalizeGitLabBaseURL("https://gitlab.example.com/api/v4/")
	if err != nil {
		t.Fatalf("NormalizeGitLabBaseURL: %v", err)
	}
	if got != "https://gitlab.example.com" {
		t.Fatalf("got %q", got)
	}
}

func TestNormalizeGitLabBaseURLRejectsInvalidURL(t *testing.T) {
	tests := []string{
		"",
		"gitlab.example.com",
		"ftp://gitlab.example.com",
		"https://",
		"https://user:pass@gitlab.example.com",
		"https://gitlab.example.com/group/project",
		"://bad",
	}
	for _, tt := range tests {
		t.Run(tt, func(t *testing.T) {
			if got, err := NormalizeGitLabBaseURL(tt); err == nil {
				t.Fatalf("expected error, got %q", got)
			}
		})
	}
}

func TestGitLabClientRejectsBlankToken(t *testing.T) {
	client, err := NewGitLabClient("https://gitlab.example.com", " ")
	if err != nil {
		t.Fatalf("NewGitLabClient: %v", err)
	}
	if _, err := client.TestConnection(context.Background()); err == nil {
		t.Fatal("expected blank token error")
	}
}

func TestGitLabClientNon2xxFails(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "nope", http.StatusUnauthorized)
	}))
	t.Cleanup(server.Close)

	client, err := NewGitLabClientWithHTTPClient(server.URL, "secret-token", server.Client())
	if err != nil {
		t.Fatalf("NewGitLabClientWithHTTPClient: %v", err)
	}
	if _, err := client.TestConnection(context.Background()); err == nil {
		t.Fatal("expected non-2xx error")
	}
}

// TestListProjectIssuesPaginatesAndAppliesScope verifies the client follows
// GitLab's X-Next-Page header, URL-encodes the project ref, and forwards the
// assigned_to_me scope + bearer token the import path relies on.
func TestListProjectIssuesPaginatesAndAppliesScope(t *testing.T) {
	ctx := context.Background()
	var (
		gotPaths   []string
		gotScopes  []string
		gotAuth    string
		gotEscaped string
	)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPaths = append(gotPaths, r.URL.Path)
		gotScopes = append(gotScopes, r.URL.Query().Get("scope"))
		gotAuth = r.Header.Get("Authorization")
		// EscapedPath preserves the %2F encoding of the project ref;
		// r.URL.Path is the decoded form (slashes) and would hide whether
		// we actually encoded "group/sub/proj" as a single path segment.
		gotEscaped = r.URL.EscapedPath()
		w.Header().Set("Content-Type", "application/json")
		page := r.URL.Query().Get("page")
		if page == "" || page == "1" {
			w.Header().Set("X-Next-Page", "2")
			_, _ = w.Write([]byte(`[{"iid":1,"title":"a","state":"opened","updated_at":"2026-01-01T00:00:00Z"}]`))
			return
		}
		// page 2: short page → stop pagination.
		_, _ = w.Write([]byte(`[{"iid":2,"title":"b","state":"closed","updated_at":"2026-01-02T00:00:00Z"}]`))
	}))
	t.Cleanup(server.Close)

	client, err := NewGitLabClientWithHTTPClient(server.URL, "tok", server.Client())
	if err != nil {
		t.Fatalf("NewGitLabClient: %v", err)
	}
	issues, err := client.ListProjectIssues(ctx, "group/sub/proj", ListIssuesOpts{AssignedToMe: true})
	if err != nil {
		t.Fatalf("ListProjectIssues: %v", err)
	}
	if len(issues) != 2 {
		t.Fatalf("expected 2 issues across pages, got %d", len(issues))
	}
	if len(gotPaths) != 2 {
		t.Fatalf("expected 2 paginated requests, got %d", len(gotPaths))
	}
	if gotAuth != "Bearer tok" {
		t.Errorf("auth = %q", gotAuth)
	}
	for _, s := range gotScopes {
		if s != "assigned_to_me" {
			t.Errorf("scope = %q, want assigned_to_me", s)
		}
	}
	// The project ref's slashes must be percent-encoded in the request path
	// so GitLab routes it as a single project ref (group/sub/proj), not 3
	// unrelated path segments.
	if !strings.Contains(gotEscaped, "/projects/group%2Fsub%2Fproj/issues") {
		t.Errorf("project ref not URL-encoded in path: %s", gotEscaped)
	}
}

func TestValidatePollInterval(t *testing.T) {
	if got, err := ValidatePollInterval(0); err != nil || got != DefaultPollIntervalSeconds {
		t.Fatalf("default interval got %d err %v", got, err)
	}
	if _, err := ValidatePollInterval(MinPollIntervalSeconds - 1); err == nil {
		t.Fatal("expected short interval error")
	}
	if got, err := ValidatePollInterval(120); err != nil || got != 120 {
		t.Fatalf("explicit interval got %d err %v", got, err)
	}
}

func TestCleanConfig(t *testing.T) {
	got, err := CleanConfig(nil)
	if err != nil {
		t.Fatalf("CleanConfig nil: %v", err)
	}
	if string(got) != "{}" {
		t.Fatalf("nil config got %s", got)
	}
	if _, err := CleanConfig([]byte(`[]`)); err == nil {
		t.Fatal("expected non-object config error")
	}
	if _, err := CleanConfig([]byte(`{bad}`)); err == nil {
		t.Fatal("expected malformed config error")
	}
}

func TestTokenEncryptDecryptTrimsStoredCiphertext(t *testing.T) {
	box := fakeSecretBox{}
	svc := &Service{SecretBox: box}
	encrypted, err := svc.encryptToken(" secret-token ")
	if err != nil {
		t.Fatalf("encryptToken: %v", err)
	}
	got, err := svc.decryptToken("\n" + encrypted + "\n")
	if err != nil {
		t.Fatalf("decryptToken: %v", err)
	}
	if got != "secret-token" {
		t.Fatalf("got %q", got)
	}
}

type fakeQueries struct {
	getSkillArg      db.GetSkillByWorkspaceAndNameParams
	getSkillErr      error
	skill            db.Skill
	createSkillArg   db.CreateSkillParams
	createSkillCalls int
	created          db.Skill
}

func (f *fakeQueries) GetSkillByWorkspaceAndName(_ context.Context, arg db.GetSkillByWorkspaceAndNameParams) (db.Skill, error) {
	f.getSkillArg = arg
	if f.getSkillErr != nil {
		return db.Skill{}, f.getSkillErr
	}
	return f.skill, nil
}

func (f *fakeQueries) CreateSkill(_ context.Context, arg db.CreateSkillParams) (db.Skill, error) {
	f.createSkillArg = arg
	f.createSkillCalls++
	if f.created.ID.Valid {
		return f.created, nil
	}
	return db.Skill{}, errors.New("unexpected CreateSkill call")
}

func (f *fakeQueries) ListIssueIntegrationsByWorkspace(context.Context, pgtype.UUID) ([]db.IssueIntegration, error) {
	return nil, errors.New("not implemented")
}

func (f *fakeQueries) GetIssueIntegrationInWorkspace(context.Context, db.GetIssueIntegrationInWorkspaceParams) (db.IssueIntegration, error) {
	return db.IssueIntegration{}, errors.New("not implemented")
}

func (f *fakeQueries) GetIssueIntegrationByProviderName(context.Context, db.GetIssueIntegrationByProviderNameParams) (db.IssueIntegration, error) {
	return db.IssueIntegration{}, errors.New("not implemented")
}

func (f *fakeQueries) CreateIssueIntegration(context.Context, db.CreateIssueIntegrationParams) (db.IssueIntegration, error) {
	return db.IssueIntegration{}, errors.New("not implemented")
}

func (f *fakeQueries) UpdateIssueIntegration(context.Context, db.UpdateIssueIntegrationParams) (db.IssueIntegration, error) {
	return db.IssueIntegration{}, errors.New("not implemented")
}

func (f *fakeQueries) DeleteIssueIntegration(context.Context, db.DeleteIssueIntegrationParams) (pgtype.UUID, error) {
	return pgtype.UUID{}, errors.New("not implemented")
}

// Import-path queries are exercised by sync_test.go via a richer fake; these
// stubs just keep fakeQueries satisfying the expanded Queries interface.
func (f *fakeQueries) GetIssueSyncConfigByScope(context.Context, db.GetIssueSyncConfigByScopeParams) (db.IssueSyncConfig, error) {
	return db.IssueSyncConfig{}, pgx.ErrNoRows
}

func (f *fakeQueries) GetIssueBridgeItemByRemote(context.Context, db.GetIssueBridgeItemByRemoteParams) (db.IssueBridgeItem, error) {
	return db.IssueBridgeItem{}, pgx.ErrNoRows
}

func (f *fakeQueries) CreateIssueBridgeItem(context.Context, db.CreateIssueBridgeItemParams) (db.IssueBridgeItem, error) {
	return db.IssueBridgeItem{}, errors.New("not implemented")
}

func (f *fakeQueries) GetAgent(context.Context, pgtype.UUID) (db.Agent, error) {
	return db.Agent{}, errors.New("not implemented")
}

func (f *fakeQueries) GetSquadInWorkspace(context.Context, db.GetSquadInWorkspaceParams) (db.Squad, error) {
	return db.Squad{}, errors.New("not implemented")
}

// Polling-path queries (Phase B). Stubs; the real coverage is in
// sync_poll_test.go via pollTestQueries.
func (f *fakeQueries) ListDueIssueSyncConfigs(context.Context) ([]db.IssueSyncConfig, error) {
	return nil, nil
}

func (f *fakeQueries) MarkIssueSyncPollSuccess(context.Context, pgtype.UUID) (db.IssueSyncConfig, error) {
	return db.IssueSyncConfig{}, errors.New("not implemented")
}

func (f *fakeQueries) MarkIssueSyncPollFailure(context.Context, db.MarkIssueSyncPollFailureParams) (db.IssueSyncConfig, error) {
	return db.IssueSyncConfig{}, errors.New("not implemented")
}

type fakeSecretBox struct{}

func (fakeSecretBox) Seal(plaintext []byte) ([]byte, error) {
	return []byte("sealed:" + base64.StdEncoding.EncodeToString(plaintext)), nil
}

func (fakeSecretBox) Open(sealed []byte) ([]byte, error) {
	const prefix = "sealed:"
	raw := string(sealed)
	if len(raw) < len(prefix) || raw[:len(prefix)] != prefix {
		return nil, errors.New("bad seal")
	}
	return base64.StdEncoding.DecodeString(raw[len(prefix):])
}

func testUUID(last byte) pgtype.UUID {
	return pgtype.UUID{
		Bytes: [16]byte{15: last},
		Valid: true,
	}
}

func TestNormalizeProjectRef(t *testing.T) {
	cases := []struct{ in, want string }{
		{"group/proj", "group/proj"},
		{"  group/proj  ", "group/proj"},
		{"/group/proj/", "group/proj"},
		{"group/proj.git", "group/proj"},
		{"https://gitlab.example.com/group/proj", "group/proj"},
		{"https://gitlab.example.com/group/sub/proj/-/issues", "group/sub/proj"},
		{"https://gitlab.com/group/proj.git", "group/proj"},
		{"", ""},
		{"   ", ""},
		{"42", "42"}, // numeric project id passes through
	}
	for _, c := range cases {
		if got := NormalizeProjectRef(c.in); got != c.want {
			t.Errorf("NormalizeProjectRef(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

// TestListProjectIssuesSurfaces404Body confirms a 404 carries GitLab's actual
// message body + the actionable hint, not a bare "status 404".
func TestListProjectIssuesSurfaces404Body(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"message":"404 Project Not Found"}`))
	}))
	t.Cleanup(server.Close)

	client, err := NewGitLabClientWithHTTPClient(server.URL, "tok", server.Client())
	if err != nil {
		t.Fatalf("NewGitLabClient: %v", err)
	}
	_, err = client.ListProjectIssues(context.Background(), "group/missing", ListIssuesOpts{})
	if err == nil {
		t.Fatal("expected 404 error, got nil")
	}
	msg := err.Error()
	if !strings.Contains(msg, "404") {
		t.Errorf("error missing status: %q", msg)
	}
	if !strings.Contains(msg, "404 Project Not Found") {
		t.Errorf("error missing GitLab body: %q", msg)
	}
	if !strings.Contains(msg, "token can access") {
		t.Errorf("error missing access hint: %q", msg)
	}
}
