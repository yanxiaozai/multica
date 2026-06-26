package issuebridge

import (
	"context"
	"encoding/base64"
	"errors"
	"net/http"
	"net/http/httptest"
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
