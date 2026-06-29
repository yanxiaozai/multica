package issuebridge

import (
	"context"
	"encoding/base64"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/service"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

// syncTestQueries is a Queries fake seeded with the import-path data the
// test wants to exercise. It records the issue-creation + mapping args so
// tests can assert on what the bridge would have persisted.
type syncTestQueries struct {
	// Inputs
	syncConfig    db.IssueSyncConfig
	syncConfigErr error
	integration   db.IssueIntegration
	// Dedup state: keyed by remote_iid, value = whether it already exists.
	existing map[int64]bool
	// Records
	createdBridgeItems []db.CreateIssueBridgeItemParams
}

func (q *syncTestQueries) GetIssueSyncConfigByScope(_ context.Context, _ db.GetIssueSyncConfigByScopeParams) (db.IssueSyncConfig, error) {
	return q.syncConfig, q.syncConfigErr
}
func (q *syncTestQueries) GetIssueIntegrationInWorkspace(context.Context, db.GetIssueIntegrationInWorkspaceParams) (db.IssueIntegration, error) {
	return q.integration, nil
}
func (q *syncTestQueries) GetIssueBridgeItemByRemote(_ context.Context, arg db.GetIssueBridgeItemByRemoteParams) (db.IssueBridgeItem, error) {
	if q.existing[arg.RemoteIid] {
		return db.IssueBridgeItem{IssueID: pgtype.UUID{Bytes: [16]byte{1}, Valid: true}}, nil
	}
	return db.IssueBridgeItem{}, pgx.ErrNoRows
}
func (q *syncTestQueries) CreateIssueBridgeItem(_ context.Context, arg db.CreateIssueBridgeItemParams) (db.IssueBridgeItem, error) {
	q.createdBridgeItems = append(q.createdBridgeItems, arg)
	return db.IssueBridgeItem{}, nil
}

// Polling-path stubs — ImportProjectIssues (Phase A) doesn't call these, so
// they keep syncTestQueries satisfying the expanded Queries interface.
func (q *syncTestQueries) ListDueIssueSyncConfigs(context.Context) ([]db.IssueSyncConfig, error) {
	return nil, nil
}
func (q *syncTestQueries) MarkIssueSyncPollSuccess(context.Context, pgtype.UUID) (db.IssueSyncConfig, error) {
	return db.IssueSyncConfig{}, errors.New("not implemented")
}
func (q *syncTestQueries) MarkIssueSyncPollFailure(context.Context, db.MarkIssueSyncPollFailureParams) (db.IssueSyncConfig, error) {
	return db.IssueSyncConfig{}, errors.New("not implemented")
}

// Unused by ImportProjectIssues but required by the Queries interface.
func (q *syncTestQueries) GetSkillByWorkspaceAndName(context.Context, db.GetSkillByWorkspaceAndNameParams) (db.Skill, error) {
	return db.Skill{}, errors.New("not implemented")
}
func (q *syncTestQueries) CreateSkill(context.Context, db.CreateSkillParams) (db.Skill, error) {
	return db.Skill{}, errors.New("not implemented")
}
func (q *syncTestQueries) ListIssueIntegrationsByWorkspace(context.Context, pgtype.UUID) ([]db.IssueIntegration, error) {
	return nil, nil
}
func (q *syncTestQueries) GetIssueIntegrationByProviderName(context.Context, db.GetIssueIntegrationByProviderNameParams) (db.IssueIntegration, error) {
	return db.IssueIntegration{}, errors.New("not implemented")
}
func (q *syncTestQueries) CreateIssueIntegration(context.Context, db.CreateIssueIntegrationParams) (db.IssueIntegration, error) {
	return db.IssueIntegration{}, errors.New("not implemented")
}
func (q *syncTestQueries) UpdateIssueIntegration(context.Context, db.UpdateIssueIntegrationParams) (db.IssueIntegration, error) {
	return db.IssueIntegration{}, errors.New("not implemented")
}
func (q *syncTestQueries) DeleteIssueIntegration(context.Context, db.DeleteIssueIntegrationParams) (pgtype.UUID, error) {
	return pgtype.UUID{}, errors.New("not implemented")
}

// fakeIssueCreator records each Create call so tests can assert params
// (status via state-mapping, project_id, auto-assignee). Each call mints a
// fresh issue UUID so the mapping write has a valid issue_id.
type fakeIssueCreator struct {
	calls []service.IssueCreateParams
	err   error
	// nextN differentiates the issue UUIDs minted across sequential calls.
	nextN byte
}

func (f *fakeIssueCreator) Create(_ context.Context, p service.IssueCreateParams, _ service.IssueCreateOpts) (service.IssueCreateResult, error) {
	if f.err != nil {
		return service.IssueCreateResult{}, f.err
	}
	f.calls = append(f.calls, p)
	f.nextN++
	return service.IssueCreateResult{
		Issue: db.Issue{ID: pgtype.UUID{Bytes: [16]byte{f.nextN}, Valid: true}},
	}, nil
}

// fakeGitLabClient serves a fixed list of remote issues for ListProjectIssues.
type fakeGitLabClient struct {
	issues []GitLabIssue
	err    error
}

func (f *fakeGitLabClient) TestConnection(context.Context) (GitLabUser, error) {
	return GitLabUser{}, nil
}
func (f *fakeGitLabClient) ListProjectIssues(_ context.Context, _ string, _ ListIssuesOpts) ([]GitLabIssue, error) {
	return f.issues, f.err
}

func newSyncService(q *syncTestQueries, creator *fakeIssueCreator, client *fakeGitLabClient) *Service {
	s := &Service{
		Queries:       q,
		SecretBox:     fakeSecretBox{},
		IssueService:  creator,
		ClientFactory: func(string, string) (GitLabClientAPI, error) { return client, nil },
	}
	return s
}

func seedIntegration(wsID, integID pgtype.UUID) db.IssueIntegration {
	return db.IssueIntegration{
		ID:             integID,
		WorkspaceID:    wsID,
		Provider:       ProviderGitLab,
		Name:           "GitLab",
		BaseUrl:        "https://gitlab.example.com",
		EncryptedToken: "sealed:dG9rZW4=", // fakeSecretBox seals base64 of plaintext; see fakeSecretBox.Seal
	}
}

// Token ciphertext that decryptToken can actually open. encryptToken seals
// then base64-encodes the sealed bytes, so the test must mirror both steps
// (not just Seal) — otherwise decryptToken's base64 decode fails.
func sealedToken(plaintext string) string {
	box := fakeSecretBox{}
	sealed, _ := box.Seal([]byte(plaintext))
	return base64.StdEncoding.EncodeToString(sealed)
}

func TestImportProjectIssues_CreatesNewIssuesWithStateMapping(t *testing.T) {
	ctx := context.Background()
	wsID := testUUID(1)
	projectID := testUUID(2)
	actor := testUUID(3)
	integID := testUUID(4)

	q := &syncTestQueries{
		syncConfig: db.IssueSyncConfig{
			ID:                testUUID(5),
			WorkspaceID:       wsID,
			IntegrationID:     integID,
			ScopeType:         "project",
			ScopeID:           projectID,
			RemoteProjectRef:  "group/proj",
			StateMapping:      []byte(`{"opened":"todo","closed":"done"}`),
			AutoAssignEnabled: false,
		},
		integration: seedIntegration(wsID, integID),
		existing:    map[int64]bool{},
	}
	q.integration.EncryptedToken = sealedToken("tok")
	creator := &fakeIssueCreator{}
	client := &fakeGitLabClient{issues: []GitLabIssue{
		{IID: 1, Title: "Bug one", Description: "d1", State: "opened", WebURL: "https://gitlab.example.com/group/proj/-/issues/1", UpdatedAt: time.Now()},
		{IID: 2, Title: "Closed one", Description: "d2", State: "closed", WebURL: "https://gitlab.example.com/group/proj/-/issues/2", UpdatedAt: time.Now()},
	}}
	svc := newSyncService(q, creator, client)

	res, err := svc.ImportProjectIssues(ctx, wsID, projectID, actor, ImportOpts{AssignedToMe: true, State: "all"})
	if err != nil {
		t.Fatalf("ImportProjectIssues: %v", err)
	}
	if res.Imported != 2 || res.Skipped != 0 || res.Failed != 0 {
		t.Fatalf("unexpected tallies: %+v", res)
	}
	if len(creator.calls) != 2 {
		t.Fatalf("expected 2 issue creates, got %d", len(creator.calls))
	}
	// State mapping: opened→todo, closed→done.
	if creator.calls[0].Status != "todo" {
		t.Errorf("iid 1 status = %q, want todo", creator.calls[0].Status)
	}
	if creator.calls[1].Status != "done" {
		t.Errorf("iid 2 status = %q, want done", creator.calls[1].Status)
	}
	// All imported issues belong to the project and the triggering member.
	for i, c := range creator.calls {
		if c.ProjectID != projectID {
			t.Errorf("call %d project = %+v, want %v", i, c.ProjectID, projectID)
		}
		if c.CreatorType != "member" || c.CreatorID != actor {
			t.Errorf("call %d creator = %q/%+v, want member/%v", i, c.CreatorType, c.CreatorID, actor)
		}
	}
	// Both mapping rows recorded with the remote iid + url.
	if len(q.createdBridgeItems) != 2 || q.createdBridgeItems[0].RemoteIid != 1 || q.createdBridgeItems[1].RemoteIid != 2 {
		t.Fatalf("unexpected bridge items: %+v", q.createdBridgeItems)
	}
}

func TestImportProjectIssues_SkipsAlreadyMapped(t *testing.T) {
	ctx := context.Background()
	wsID := testUUID(10)
	projectID := testUUID(11)
	integID := testUUID(12)

	q := &syncTestQueries{
		syncConfig:   db.IssueSyncConfig{ID: testUUID(13), WorkspaceID: wsID, IntegrationID: integID, ScopeType: "project", ScopeID: projectID, RemoteProjectRef: "g/p", StateMapping: []byte(`{"opened":"backlog","closed":"done"}`)},
		integration:  seedIntegration(wsID, integID),
		existing:     map[int64]bool{7: true}, // iid 7 already imported
	}
	q.integration.EncryptedToken = sealedToken("tok")
	creator := &fakeIssueCreator{}
	client := &fakeGitLabClient{issues: []GitLabIssue{
		{IID: 7, Title: "already here", State: "opened"},
		{IID: 8, Title: "new", State: "opened"},
	}}
	svc := newSyncService(q, creator, client)

	res, err := svc.ImportProjectIssues(ctx, wsID, projectID, testUUID(14), ImportOpts{AssignedToMe: true})
	if err != nil {
		t.Fatalf("ImportProjectIssues: %v", err)
	}
	if res.Imported != 1 || res.Skipped != 1 {
		t.Fatalf("expected imported=1 skipped=1, got %+v", res)
	}
	if len(creator.calls) != 1 || creator.calls[0].Title != "new" {
		t.Fatalf("expected only the new issue created, got %+v", creator.calls)
	}
}

func TestImportProjectIssues_AutoAssignStampsAgent(t *testing.T) {
	ctx := context.Background()
	wsID := testUUID(20)
	projectID := testUUID(21)
	integID := testUUID(22)
	agentID := testUUID(23)

	q := &syncTestQueries{
		syncConfig: db.IssueSyncConfig{
			ID: testUUID(24), WorkspaceID: wsID, IntegrationID: integID,
			ScopeType: "project", ScopeID: projectID, RemoteProjectRef: "g/p",
			StateMapping: []byte(`{"opened":"todo","closed":"done"}`),
			AutoAssignEnabled: true, DefaultAssigneeType: pgtype.Text{String: "agent", Valid: true},
			DefaultAssigneeID: agentID,
		},
		integration: seedIntegration(wsID, integID),
		existing:    map[int64]bool{},
	}
	q.integration.EncryptedToken = sealedToken("tok")
	creator := &fakeIssueCreator{}
	client := &fakeGitLabClient{issues: []GitLabIssue{{IID: 1, Title: "x", State: "opened"}}}
	svc := newSyncService(q, creator, client)

	if _, err := svc.ImportProjectIssues(ctx, wsID, projectID, testUUID(25), ImportOpts{AssignedToMe: true}); err != nil {
		t.Fatalf("ImportProjectIssues: %v", err)
	}
	if len(creator.calls) != 1 {
		t.Fatalf("expected 1 create, got %d", len(creator.calls))
	}
	c := creator.calls[0]
	if c.AssigneeType.String != "agent" || c.AssigneeID != agentID {
		t.Errorf("auto-assign = %q/%+v, want agent/%v", c.AssigneeType.String, c.AssigneeID, agentID)
	}
}

func TestImportProjectIssues_MissingSyncConfigReturnsTypedError(t *testing.T) {
	ctx := context.Background()
	q := &syncTestQueries{syncConfigErr: pgx.ErrNoRows}
	svc := newSyncService(q, &fakeIssueCreator{}, &fakeGitLabClient{})

	_, err := svc.ImportProjectIssues(ctx, testUUID(30), testUUID(31), testUUID(32), ImportOpts{AssignedToMe: true})
	if !errors.Is(err, ErrSyncConfigMissing) {
		t.Fatalf("expected ErrSyncConfigMissing, got %v", err)
	}
}

func TestImportProjectIssues_PartialCreateFailureContinues(t *testing.T) {
	ctx := context.Background()
	wsID := testUUID(40)
	projectID := testUUID(41)
	integID := testUUID(42)

	q := &syncTestQueries{
		syncConfig:   db.IssueSyncConfig{ID: testUUID(43), WorkspaceID: wsID, IntegrationID: integID, ScopeType: "project", ScopeID: projectID, RemoteProjectRef: "g/p", StateMapping: []byte(`{"opened":"backlog","closed":"done"}`)},
		integration:  seedIntegration(wsID, integID),
		existing:     map[int64]bool{},
	}
	q.integration.EncryptedToken = sealedToken("tok")
	// First create fails, second succeeds — the batch must continue past
	// the failure rather than aborting the whole import.
	creator := &conditionalFailCreator{failOn: 1}
	client := &fakeGitLabClient{issues: []GitLabIssue{
		{IID: 1, Title: "will fail", State: "opened"},
		{IID: 2, Title: "will pass", State: "opened"},
	}}
	svc := &Service{
		Queries:       q,
		SecretBox:     fakeSecretBox{},
		IssueService:  creator,
		ClientFactory: func(string, string) (GitLabClientAPI, error) { return client, nil },
	}

	res, err := svc.ImportProjectIssues(ctx, wsID, projectID, testUUID(44), ImportOpts{AssignedToMe: true})
	if err != nil {
		t.Fatalf("ImportProjectIssues: %v", err)
	}
	if res.Imported != 1 || res.Failed != 1 {
		t.Fatalf("expected imported=1 failed=1, got %+v", res)
	}
}

// conditionalFailCreator fails on the Nth call (1-indexed) and succeeds
// otherwise.
type conditionalFailCreator struct{ failOn byte }

func (c *conditionalFailCreator) Create(_ context.Context, p service.IssueCreateParams, _ service.IssueCreateOpts) (service.IssueCreateResult, error) {
	if c.failOn == 1 {
		c.failOn = 0 // only first call fails
		return service.IssueCreateResult{}, errors.New("simulated create failure")
	}
	return service.IssueCreateResult{Issue: db.Issue{ID: pgtype.UUID{Bytes: [16]byte{9}, Valid: true}}}, nil
}

func TestMapRemoteState_FallbackAndMapping(t *testing.T) {
	m := map[string]string{"opened": "todo", "closed": "done"}
	if got := mapRemoteState(m, "opened"); got != "todo" {
		t.Errorf("opened = %q", got)
	}
	if got := mapRemoteState(m, "closed"); got != "done" {
		t.Errorf("closed = %q", got)
	}
	// Unknown GitLab state falls back to backlog, never "".
	if got := mapRemoteState(m, "locked"); got != "backlog" {
		t.Errorf("unknown = %q, want backlog", got)
	}
	if got := mapRemoteState(map[string]string{}, "opened"); got != "backlog" {
		t.Errorf("empty mapping = %q, want backlog", got)
	}
}
