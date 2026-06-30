package issuebridge

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

// pollTestQueries is a Queries fake for the SyncDueConfigs path. It serves a
// fixed list of due configs and records every poll-mark call so tests can
// assert which configs were marked success vs failure. The import path
// (GetIssueSyncConfigByScope / integration / bridge items) is reused via the
// real ImportProjectIssues against the ListProjectIssues fake client.
type pollTestQueries struct {
	dueConfigs []db.IssueSyncConfig
	// Per-config import wiring: keyed by config ID.
	syncConfigs   map[pgtype.UUID]db.IssueSyncConfig
	integrations  map[pgtype.UUID]db.IssueIntegration
	existingItems map[pgtype.UUID]map[int64]bool // integration_id -> remote_iid set
	squads        map[pgtype.UUID]db.Squad
	createdItems  []db.CreateIssueBridgeItemParams
	successMarks  []pgtype.UUID
	failureMarks  []db.MarkIssueSyncPollFailureParams
	listDueErr    error
}

func (q *pollTestQueries) ListDueIssueSyncConfigs(context.Context) ([]db.IssueSyncConfig, error) {
	return q.dueConfigs, q.listDueErr
}
func (q *pollTestQueries) MarkIssueSyncPollSuccess(_ context.Context, id pgtype.UUID) (db.IssueSyncConfig, error) {
	q.successMarks = append(q.successMarks, id)
	return db.IssueSyncConfig{}, nil
}
func (q *pollTestQueries) MarkIssueSyncPollFailure(_ context.Context, arg db.MarkIssueSyncPollFailureParams) (db.IssueSyncConfig, error) {
	q.failureMarks = append(q.failureMarks, arg)
	return db.IssueSyncConfig{}, nil
}

// Import-path queries reused by ImportProjectIssues inside SyncDueConfigs.
func (q *pollTestQueries) GetIssueSyncConfigByScope(_ context.Context, _ db.GetIssueSyncConfigByScopeParams) (db.IssueSyncConfig, error) {
	// SyncDueConfigs already holds the config row; ImportProjectIssues only
	// re-fetches by scope. Serve the first matching one we know about.
	for _, c := range q.syncConfigs {
		return c, nil
	}
	return db.IssueSyncConfig{}, pgx.ErrNoRows
}
func (q *pollTestQueries) GetIssueIntegrationInWorkspace(_ context.Context, arg db.GetIssueIntegrationInWorkspaceParams) (db.IssueIntegration, error) {
	if itg, ok := q.integrations[arg.ID]; ok {
		return itg, nil
	}
	return db.IssueIntegration{}, errors.New("integration not found")
}
func (q *pollTestQueries) GetIssueBridgeItemByRemote(_ context.Context, arg db.GetIssueBridgeItemByRemoteParams) (db.IssueBridgeItem, error) {
	if set, ok := q.existingItems[arg.IntegrationID]; ok && set[arg.RemoteIid] {
		return db.IssueBridgeItem{IssueID: pgtype.UUID{Bytes: [16]byte{1}, Valid: true}}, nil
	}
	return db.IssueBridgeItem{}, pgx.ErrNoRows
}
func (q *pollTestQueries) CreateIssueBridgeItem(_ context.Context, arg db.CreateIssueBridgeItemParams) (db.IssueBridgeItem, error) {
	q.createdItems = append(q.createdItems, arg)
	return db.IssueBridgeItem{}, nil
}
func (q *pollTestQueries) GetAgent(_ context.Context, id pgtype.UUID) (db.Agent, error) {
	return db.Agent{ID: id}, nil
}
func (q *pollTestQueries) GetSquadInWorkspace(_ context.Context, arg db.GetSquadInWorkspaceParams) (db.Squad, error) {
	if squad, ok := q.squads[arg.ID]; ok {
		return squad, nil
	}
	return db.Squad{}, errors.New("squad not found")
}

// Unused but required by the Queries interface.
func (q *pollTestQueries) GetSkillByWorkspaceAndName(context.Context, db.GetSkillByWorkspaceAndNameParams) (db.Skill, error) {
	return db.Skill{}, errors.New("not implemented")
}
func (q *pollTestQueries) CreateSkill(context.Context, db.CreateSkillParams) (db.Skill, error) {
	return db.Skill{}, errors.New("not implemented")
}
func (q *pollTestQueries) ListIssueIntegrationsByWorkspace(context.Context, pgtype.UUID) ([]db.IssueIntegration, error) {
	return nil, nil
}
func (q *pollTestQueries) GetIssueIntegrationByProviderName(context.Context, db.GetIssueIntegrationByProviderNameParams) (db.IssueIntegration, error) {
	return db.IssueIntegration{}, errors.New("not implemented")
}
func (q *pollTestQueries) CreateIssueIntegration(context.Context, db.CreateIssueIntegrationParams) (db.IssueIntegration, error) {
	return db.IssueIntegration{}, errors.New("not implemented")
}
func (q *pollTestQueries) UpdateIssueIntegration(context.Context, db.UpdateIssueIntegrationParams) (db.IssueIntegration, error) {
	return db.IssueIntegration{}, errors.New("not implemented")
}
func (q *pollTestQueries) DeleteIssueIntegration(context.Context, db.DeleteIssueIntegrationParams) (pgtype.UUID, error) {
	return pgtype.UUID{}, errors.New("not implemented")
}

func TestSyncDueConfigs_MarksSuccessOnHappyPath(t *testing.T) {
	ctx := context.Background()
	wsID := testUUID(70)
	integID := testUUID(71)
	cfgID := testUUID(72)
	projectID := testUUID(73)

	cfg := db.IssueSyncConfig{
		ID: cfgID, WorkspaceID: wsID, IntegrationID: integID,
		ScopeType: "project", ScopeID: projectID, RemoteProjectRef: "g/p",
		StateMapping: []byte(`{"opened":"todo","closed":"done"}`),
	}
	q := &pollTestQueries{
		dueConfigs:  []db.IssueSyncConfig{cfg},
		syncConfigs: map[pgtype.UUID]db.IssueSyncConfig{cfgID: cfg},
		integrations: map[pgtype.UUID]db.IssueIntegration{
			integID: {ID: integID, WorkspaceID: wsID, Provider: ProviderGitLab, BaseUrl: "https://gitlab.example.com", EncryptedToken: sealedToken("tok")},
		},
		existingItems: map[pgtype.UUID]map[int64]bool{},
	}
	creator := &fakeIssueCreator{}
	client := &fakeGitLabClient{issues: []GitLabIssue{{IID: 1, Title: "new", State: "opened", UpdatedAt: time.Now()}}}
	svc := &Service{
		Queries: q, SecretBox: fakeSecretBox{}, IssueService: creator,
		ClientFactory: func(string, string) (GitLabClientAPI, error) { return client, nil },
	}

	stats, err := svc.SyncDueConfigs(ctx)
	if err != nil {
		t.Fatalf("SyncDueConfigs: %v", err)
	}
	if stats.Configs != 1 || stats.Imported != 1 {
		t.Fatalf("unexpected stats: %+v", stats)
	}
	if len(q.successMarks) != 1 || q.successMarks[0] != cfgID {
		t.Fatalf("expected one success mark on the config, got %+v", q.successMarks)
	}
	if len(q.failureMarks) != 0 {
		t.Fatalf("expected no failure marks, got %+v", q.failureMarks)
	}
}

func TestSyncDueConfigs_AutoAcceptClaimsBeforeCreatingLocalIssue(t *testing.T) {
	ctx := context.Background()
	wsID := testUUID(170)
	integID := testUUID(171)
	cfgID := testUUID(172)
	projectID := testUUID(173)
	agentID := testUUID(174)

	cfg := db.IssueSyncConfig{
		ID: cfgID, WorkspaceID: wsID, IntegrationID: integID,
		ScopeType: "project", ScopeID: projectID, RemoteProjectRef: "g/p",
		StateMapping:        []byte(`{"opened":"todo","closed":"done"}`),
		SyncMode:            SyncModeAutoAccept,
		AutoAcceptLabel:     "ai-auto",
		AutoAssignEnabled:   true,
		DefaultAssigneeType: pgtype.Text{String: "agent", Valid: true},
		DefaultAssigneeID:   agentID,
	}
	q := &pollTestQueries{
		dueConfigs:  []db.IssueSyncConfig{cfg},
		syncConfigs: map[pgtype.UUID]db.IssueSyncConfig{cfgID: cfg},
		integrations: map[pgtype.UUID]db.IssueIntegration{
			integID: {ID: integID, WorkspaceID: wsID, Provider: ProviderGitLab, BaseUrl: "https://gitlab.example.com", EncryptedToken: sealedToken("tok")},
		},
		existingItems: map[pgtype.UUID]map[int64]bool{},
	}
	creator := &fakeIssueCreator{}
	client := &fakeGitLabClient{
		user: GitLabUser{ID: 99, Username: "bot", Name: "Bot"},
		issues: []GitLabIssue{{
			IID: 7, Title: "auto", State: "opened", Labels: []string{"ai-auto"}, UpdatedAt: time.Now(),
		}},
	}
	svc := &Service{
		Queries: q, SecretBox: fakeSecretBox{}, IssueService: creator,
		ClientFactory: func(string, string) (GitLabClientAPI, error) { return client, nil },
	}

	stats, err := svc.SyncDueConfigs(ctx)
	if err != nil {
		t.Fatalf("SyncDueConfigs: %v", err)
	}
	if stats.Imported != 1 || stats.Failed != 0 || stats.ErrConfigs != 0 {
		t.Fatalf("stats = %+v, want imported=1 failed=0 err_configs=0", stats)
	}
	if len(client.assigned) != 1 || client.assigned[0] != 7 {
		t.Fatalf("assigned = %v, want [7]", client.assigned)
	}
	if len(client.notes) != 1 {
		t.Fatalf("expected gitlab comment, got %d", len(client.notes))
	}
	if len(creator.calls) != 1 {
		t.Fatalf("create calls = %d, want 1", len(creator.calls))
	}
	if creator.calls[0].AssigneeType.String != "agent" || creator.calls[0].AssigneeID != agentID {
		t.Fatalf("assignee = %q/%v, want agent/%v", creator.calls[0].AssigneeType.String, creator.calls[0].AssigneeID, agentID)
	}
	if creator.calls[0].Status != "todo" {
		t.Fatalf("status = %q, want todo", creator.calls[0].Status)
	}
	if creator.calls[0].CreatorType != "agent" || creator.calls[0].CreatorID != agentID {
		t.Fatalf("creator = %q/%v, want agent/%v", creator.calls[0].CreatorType, creator.calls[0].CreatorID, agentID)
	}
	if len(q.createdItems) != 1 || q.createdItems[0].RemoteIid != 7 {
		t.Fatalf("created bridge items = %+v, want remote iid 7", q.createdItems)
	}
	if len(q.successMarks) != 1 || len(q.failureMarks) != 0 {
		t.Fatalf("success=%d failure=%d, want success=1 failure=0", len(q.successMarks), len(q.failureMarks))
	}
}

func TestSyncDueConfigs_AutoAcceptSquadUsesLeaderAsCreator(t *testing.T) {
	ctx := context.Background()
	wsID := testUUID(180)
	integID := testUUID(181)
	cfgID := testUUID(182)
	projectID := testUUID(183)
	squadID := testUUID(184)
	leaderID := testUUID(185)

	cfg := db.IssueSyncConfig{
		ID: cfgID, WorkspaceID: wsID, IntegrationID: integID,
		ScopeType: "project", ScopeID: projectID, RemoteProjectRef: "g/p",
		StateMapping:        []byte(`{"opened":"todo","closed":"done"}`),
		SyncMode:            SyncModeAutoAccept,
		AutoAcceptLabel:     "ai-auto",
		AutoAssignEnabled:   true,
		DefaultAssigneeType: pgtype.Text{String: "squad", Valid: true},
		DefaultAssigneeID:   squadID,
	}
	q := &pollTestQueries{
		dueConfigs:  []db.IssueSyncConfig{cfg},
		syncConfigs: map[pgtype.UUID]db.IssueSyncConfig{cfgID: cfg},
		integrations: map[pgtype.UUID]db.IssueIntegration{
			integID: {ID: integID, WorkspaceID: wsID, Provider: ProviderGitLab, BaseUrl: "https://gitlab.example.com", EncryptedToken: sealedToken("tok")},
		},
		existingItems: map[pgtype.UUID]map[int64]bool{},
		squads: map[pgtype.UUID]db.Squad{
			squadID: {ID: squadID, WorkspaceID: wsID, LeaderID: leaderID},
		},
	}
	creator := &fakeIssueCreator{}
	client := &fakeGitLabClient{
		user: GitLabUser{ID: 99, Username: "bot", Name: "Bot"},
		issues: []GitLabIssue{{
			IID: 8, Title: "auto squad", State: "opened", Labels: []string{"ai-auto"}, UpdatedAt: time.Now(),
		}},
	}
	svc := &Service{
		Queries: q, SecretBox: fakeSecretBox{}, IssueService: creator,
		ClientFactory: func(string, string) (GitLabClientAPI, error) { return client, nil },
	}

	stats, err := svc.SyncDueConfigs(ctx)
	if err != nil {
		t.Fatalf("SyncDueConfigs: %v", err)
	}
	if stats.Imported != 1 || stats.Failed != 0 {
		t.Fatalf("stats = %+v, want imported=1 failed=0", stats)
	}
	if len(creator.calls) != 1 {
		t.Fatalf("create calls = %d, want 1", len(creator.calls))
	}
	call := creator.calls[0]
	if call.AssigneeType.String != "squad" || call.AssigneeID != squadID {
		t.Fatalf("assignee = %q/%v, want squad/%v", call.AssigneeType.String, call.AssigneeID, squadID)
	}
	if call.CreatorType != "agent" || call.CreatorID != leaderID {
		t.Fatalf("creator = %q/%v, want agent/%v", call.CreatorType, call.CreatorID, leaderID)
	}
}

func TestSyncDueConfigs_AutoAcceptRecoversAlreadyClaimedIssue(t *testing.T) {
	ctx := context.Background()
	wsID := testUUID(190)
	integID := testUUID(191)
	cfgID := testUUID(192)
	projectID := testUUID(193)
	agentID := testUUID(194)

	cfg := db.IssueSyncConfig{
		ID: cfgID, WorkspaceID: wsID, IntegrationID: integID,
		ScopeType: "project", ScopeID: projectID, RemoteProjectRef: "g/p",
		StateMapping:        []byte(`{"opened":"todo","closed":"done"}`),
		SyncMode:            SyncModeAutoAccept,
		AutoAcceptLabel:     "ai-auto",
		AutoAssignEnabled:   true,
		DefaultAssigneeType: pgtype.Text{String: "agent", Valid: true},
		DefaultAssigneeID:   agentID,
	}
	q := &pollTestQueries{
		dueConfigs:  []db.IssueSyncConfig{cfg},
		syncConfigs: map[pgtype.UUID]db.IssueSyncConfig{cfgID: cfg},
		integrations: map[pgtype.UUID]db.IssueIntegration{
			integID: {ID: integID, WorkspaceID: wsID, Provider: ProviderGitLab, BaseUrl: "https://gitlab.example.com", EncryptedToken: sealedToken("tok")},
		},
		existingItems: map[pgtype.UUID]map[int64]bool{},
	}
	creator := &fakeIssueCreator{}
	client := &fakeGitLabClient{
		user: GitLabUser{ID: 99, Username: "bot", Name: "Bot"},
		issues: []GitLabIssue{{
			IID: 9, Title: "already claimed", State: "opened", Labels: []string{"ai-auto"},
			Assignees: []GitLabUser{{ID: 99, Username: "bot", Name: "Bot"}},
			UpdatedAt: time.Now(),
		}},
	}
	svc := &Service{
		Queries: q, SecretBox: fakeSecretBox{}, IssueService: creator,
		ClientFactory: func(string, string) (GitLabClientAPI, error) { return client, nil },
	}

	stats, err := svc.SyncDueConfigs(ctx)
	if err != nil {
		t.Fatalf("SyncDueConfigs: %v", err)
	}
	if stats.Imported != 1 || stats.Failed != 0 {
		t.Fatalf("stats = %+v, want imported=1 failed=0", stats)
	}
	if len(client.assigned) != 0 {
		t.Fatalf("assigned = %v, want no new claim for already claimed issue", client.assigned)
	}
	if len(client.listOpts) != 2 || !client.listOpts[0].Unassigned || !client.listOpts[1].AssignedToMe {
		t.Fatalf("list opts = %+v, want unassigned scan then assigned-to-me recovery scan", client.listOpts)
	}
	if len(q.createdItems) != 1 || q.createdItems[0].RemoteIid != 9 {
		t.Fatalf("created bridge items = %+v, want remote iid 9", q.createdItems)
	}
}

func TestSyncDueConfigs_MarksFailureWhenImportErrors(t *testing.T) {
	ctx := context.Background()
	wsID := testUUID(80)
	integID := testUUID(81)
	cfgID := testUUID(82)
	projectID := testUUID(83)
	cfg := db.IssueSyncConfig{
		ID: cfgID, WorkspaceID: wsID, IntegrationID: integID,
		ScopeType: "project", ScopeID: projectID, RemoteProjectRef: "g/p",
		StateMapping: []byte(`{"opened":"todo","closed":"done"}`),
	}
	q := &pollTestQueries{
		dueConfigs:  []db.IssueSyncConfig{cfg},
		syncConfigs: map[pgtype.UUID]db.IssueSyncConfig{cfgID: cfg},
		// No integration entry → GetIssueIntegrationInWorkspace errors →
		// ImportProjectIssues returns an error → SyncDueConfigs marks failure.
		integrations: map[pgtype.UUID]db.IssueIntegration{},
	}
	svc := &Service{
		Queries: q, SecretBox: fakeSecretBox{}, IssueService: &fakeIssueCreator{},
		ClientFactory: func(string, string) (GitLabClientAPI, error) { return &fakeGitLabClient{}, nil },
	}

	stats, err := svc.SyncDueConfigs(ctx)
	if err != nil {
		t.Fatalf("SyncDueConfigs itself must not error on a per-config failure: %v", err)
	}
	if stats.ErrConfigs != 1 || stats.Imported != 0 {
		t.Fatalf("expected err_configs=1 imported=0, got %+v", stats)
	}
	if len(q.failureMarks) != 1 || q.failureMarks[0].ID != cfgID || q.failureMarks[0].LastError == "" {
		t.Fatalf("expected one non-empty failure mark, got %+v", q.failureMarks)
	}
	if len(q.successMarks) != 0 {
		t.Fatalf("expected no success marks, got %+v", q.successMarks)
	}
}

func TestSyncDueConfigs_SkipsNonProjectScope(t *testing.T) {
	ctx := context.Background()
	// repo_resource-scoped config is not yet supported by the importer; it
	// should be skipped without being marked success or failure.
	repoCfg := db.IssueSyncConfig{
		ID: testUUID(90), ScopeType: "repo_resource", ScopeID: testUUID(91),
	}
	q := &pollTestQueries{dueConfigs: []db.IssueSyncConfig{repoCfg}}
	svc := &Service{
		Queries: q, SecretBox: fakeSecretBox{}, IssueService: &fakeIssueCreator{},
		ClientFactory: func(string, string) (GitLabClientAPI, error) { return &fakeGitLabClient{}, nil },
	}

	stats, err := svc.SyncDueConfigs(ctx)
	if err != nil {
		t.Fatalf("SyncDueConfigs: %v", err)
	}
	if stats.Configs != 1 || stats.Imported != 0 || stats.ErrConfigs != 0 {
		t.Fatalf("expected the repo_resource config skipped, got %+v", stats)
	}
	if len(q.successMarks) != 0 || len(q.failureMarks) != 0 {
		t.Fatalf("repo_resource config must not be marked, got success=%+v failure=%+v", q.successMarks, q.failureMarks)
	}
}
