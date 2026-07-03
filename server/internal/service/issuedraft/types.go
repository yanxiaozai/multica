package issuedraft

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

type querier interface {
	CreateIssueDraftSession(ctx context.Context, arg db.CreateIssueDraftSessionParams) (db.IssueDraftSession, error)
	GetIssueDraftSessionInWorkspace(ctx context.Context, arg db.GetIssueDraftSessionInWorkspaceParams) (db.IssueDraftSession, error)
	GetActiveIssueDraftSession(ctx context.Context, workspaceID pgtype.UUID) (db.IssueDraftSession, error)
	GetProjectInWorkspace(ctx context.Context, arg db.GetProjectInWorkspaceParams) (db.Project, error)
	GetSquadInWorkspace(ctx context.Context, arg db.GetSquadInWorkspaceParams) (db.Squad, error)
	GetAgentTask(ctx context.Context, id pgtype.UUID) (db.AgentTaskQueue, error)
	ListProjectResources(ctx context.Context, projectID pgtype.UUID) ([]db.ProjectResource, error)
	UpdateIssueDraftSessionStatus(ctx context.Context, arg db.UpdateIssueDraftSessionStatusParams) (db.IssueDraftSession, error)
	AppendIssueDraftMessage(ctx context.Context, arg db.AppendIssueDraftMessageParams) (db.IssueDraftMessage, error)
	ListIssueDraftMessages(ctx context.Context, arg db.ListIssueDraftMessagesParams) ([]db.IssueDraftMessage, error)
	ListIssueDraftMemberTasks(ctx context.Context, arg db.ListIssueDraftMemberTasksParams) ([]db.IssueDraftMemberTask, error)
	UpdateIssueDraftMemberTaskStatus(ctx context.Context, arg db.UpdateIssueDraftMemberTaskStatusParams) (db.IssueDraftMemberTask, error)
	ListIssueDraftArtifacts(ctx context.Context, arg db.ListIssueDraftArtifactsParams) ([]db.IssueDraftArtifact, error)
	ListIssueDraftConfirmSteps(ctx context.Context, arg db.ListIssueDraftConfirmStepsParams) ([]db.IssueDraftConfirmStep, error)
	GetNextIssueDraftArtifactRevision(ctx context.Context, arg db.GetNextIssueDraftArtifactRevisionParams) (int32, error)
	CreateIssueDraftArtifact(ctx context.Context, arg db.CreateIssueDraftArtifactParams) (db.IssueDraftArtifact, error)
	GetLatestIssueDraftArtifact(ctx context.Context, arg db.GetLatestIssueDraftArtifactParams) (db.IssueDraftArtifact, error)
	UpsertIssueDraftConfirmStep(ctx context.Context, arg db.UpsertIssueDraftConfirmStepParams) (db.IssueDraftConfirmStep, error)
}

const (
	StatusClarifying     = "clarifying"
	StatusDelegating     = "delegating"
	StatusDrafting       = "drafting"
	StatusReadyForReview = "ready_for_review"
	StatusCreating       = "creating"
	StatusCreated        = "created"
	StatusFailed         = "failed"
	StatusCancelled      = "cancelled"

	AuthorMember = "member"
	AuthorAgent  = "agent"
	AuthorSystem = "system"

	MessageUserMessage  = "user_message"
	MessageQuestion     = "question"
	MessageAnswer       = "answer"
	MessageFinding      = "finding"
	MessageDraftPreview = "draft_preview"
	MessageStatus       = "status"
	MessageError        = "error"

	ArtifactDetailedSpec = "detailed_spec"
	ArtifactMulticaIssue = "multica_issue"
	ArtifactRemoteIssue  = "remote_issue"
)

var (
	ErrProjectNotFound          = errors.New("project not found in workspace")
	ErrSquadNotFound            = errors.New("squad not found in workspace")
	ErrSquadArchived            = errors.New("squad is archived")
	ErrPrimaryRepositoryMissing = errors.New("project has no local_directory project resource")
	ErrInvalidProjectResource   = errors.New("project resource is invalid")
)

type CreateSessionInput struct {
	WorkspaceID       pgtype.UUID
	ProjectID         pgtype.UUID
	SquadID           pgtype.UUID
	CreatedByMemberID pgtype.UUID
	InitialMessage    string
}

type AppendMessageInput struct {
	SessionID   pgtype.UUID
	WorkspaceID pgtype.UUID
	AuthorType  string
	AuthorID    pgtype.UUID
	MessageType string
	Content     string
	Metadata    []byte
}

type GenerateArtifactsInput struct {
	SessionID   pgtype.UUID
	WorkspaceID pgtype.UUID
	GeneratedBy pgtype.UUID
}

type ConfirmInput struct {
	SessionID   pgtype.UUID
	WorkspaceID pgtype.UUID
	ConfirmedBy pgtype.UUID
}

type CancelInput struct {
	SessionID   pgtype.UUID
	WorkspaceID pgtype.UUID
	CancelledBy pgtype.UUID
	Reason      string
}

type SessionBundle struct {
	Session      db.IssueDraftSession
	Messages     []db.IssueDraftMessage
	MemberTasks  []db.IssueDraftMemberTask
	Artifacts    []db.IssueDraftArtifact
	ConfirmSteps []db.IssueDraftConfirmStep
}

type PrimaryRepository struct {
	Resource  db.ProjectResource
	LocalPath string
	DaemonID  string
	Label     string
}
