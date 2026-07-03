package issuedraft

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

const localDirectoryResourceType = "local_directory"

type Service struct {
	Queries querier
}

func New(q *db.Queries) *Service {
	return &Service{Queries: q}
}

func NewWithQuerier(q querier) *Service {
	return &Service{Queries: q}
}

func (s *Service) CreateSession(ctx context.Context, in CreateSessionInput) (SessionBundle, error) {
	if _, err := s.Queries.GetProjectInWorkspace(ctx, db.GetProjectInWorkspaceParams{
		ID:          in.ProjectID,
		WorkspaceID: in.WorkspaceID,
	}); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return SessionBundle{}, ErrProjectNotFound
		}
		return SessionBundle{}, fmt.Errorf("get project: %w", err)
	}

	squad, err := s.Queries.GetSquadInWorkspace(ctx, db.GetSquadInWorkspaceParams{
		ID:          in.SquadID,
		WorkspaceID: in.WorkspaceID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return SessionBundle{}, ErrSquadNotFound
		}
		return SessionBundle{}, fmt.Errorf("get squad: %w", err)
	}
	if squad.ArchivedAt.Valid {
		return SessionBundle{}, ErrSquadArchived
	}

	repo, err := s.ResolvePrimaryRepository(ctx, in.ProjectID)
	if err != nil {
		return SessionBundle{}, err
	}

	session, err := s.Queries.CreateIssueDraftSession(ctx, db.CreateIssueDraftSessionParams{
		WorkspaceID:              in.WorkspaceID,
		ProjectID:                in.ProjectID,
		SquadID:                  in.SquadID,
		LeaderAgentID:            squad.LeaderID,
		PrimaryProjectResourceID: repo.Resource.ID,
		PrimaryLocalPathSnapshot: repo.LocalPath,
		CreatedBy:                in.CreatedByMemberID,
	})
	if err != nil {
		return SessionBundle{}, fmt.Errorf("create issue draft session: %w", err)
	}

	bundle := SessionBundle{Session: session}
	if strings.TrimSpace(in.InitialMessage) != "" {
		msg, err := s.AppendMessage(ctx, AppendMessageInput{
			SessionID:   session.ID,
			WorkspaceID: in.WorkspaceID,
			AuthorType:  AuthorMember,
			AuthorID:    in.CreatedByMemberID,
			MessageType: MessageUserMessage,
			Content:     in.InitialMessage,
		})
		if err != nil {
			return SessionBundle{}, err
		}
		bundle.Messages = []db.IssueDraftMessage{msg}
	}
	return bundle, nil
}

func (s *Service) GetSessionBundle(ctx context.Context, workspaceID, sessionID pgtype.UUID) (SessionBundle, error) {
	session, err := s.Queries.GetIssueDraftSessionInWorkspace(ctx, db.GetIssueDraftSessionInWorkspaceParams{
		ID:          sessionID,
		WorkspaceID: workspaceID,
	})
	if err != nil {
		return SessionBundle{}, fmt.Errorf("get issue draft session: %w", err)
	}
	messages, err := s.Queries.ListIssueDraftMessages(ctx, db.ListIssueDraftMessagesParams{
		SessionID:   sessionID,
		WorkspaceID: workspaceID,
	})
	if err != nil {
		return SessionBundle{}, fmt.Errorf("list issue draft messages: %w", err)
	}
	memberTasks, err := s.Queries.ListIssueDraftMemberTasks(ctx, db.ListIssueDraftMemberTasksParams{
		SessionID:   sessionID,
		WorkspaceID: workspaceID,
	})
	if err != nil {
		return SessionBundle{}, fmt.Errorf("list issue draft member tasks: %w", err)
	}
	artifacts, err := s.Queries.ListIssueDraftArtifacts(ctx, db.ListIssueDraftArtifactsParams{
		SessionID:   sessionID,
		WorkspaceID: workspaceID,
	})
	if err != nil {
		return SessionBundle{}, fmt.Errorf("list issue draft artifacts: %w", err)
	}
	confirmSteps, err := s.Queries.ListIssueDraftConfirmSteps(ctx, db.ListIssueDraftConfirmStepsParams{
		SessionID:   sessionID,
		WorkspaceID: workspaceID,
	})
	if err != nil {
		return SessionBundle{}, fmt.Errorf("list issue draft confirm steps: %w", err)
	}
	return SessionBundle{
		Session:      session,
		Messages:     messages,
		MemberTasks:  memberTasks,
		Artifacts:    artifacts,
		ConfirmSteps: confirmSteps,
	}, nil
}

func (s *Service) AppendMessage(ctx context.Context, in AppendMessageInput) (db.IssueDraftMessage, error) {
	metadata := in.Metadata
	if len(metadata) == 0 {
		metadata = []byte(`{}`)
	}
	msg, err := s.Queries.AppendIssueDraftMessage(ctx, db.AppendIssueDraftMessageParams{
		SessionID:   in.SessionID,
		WorkspaceID: in.WorkspaceID,
		AuthorType:  fallback(in.AuthorType, AuthorMember),
		AuthorID:    in.AuthorID,
		MessageType: fallback(in.MessageType, MessageUserMessage),
		Content:     strings.TrimSpace(in.Content),
		Metadata:    metadata,
	})
	if err != nil {
		return db.IssueDraftMessage{}, fmt.Errorf("append issue draft message: %w", err)
	}
	return msg, nil
}

func (s *Service) ResolvePrimaryRepository(ctx context.Context, projectID pgtype.UUID) (PrimaryRepository, error) {
	resources, err := s.Queries.ListProjectResources(ctx, projectID)
	if err != nil {
		return PrimaryRepository{}, fmt.Errorf("list project resources: %w", err)
	}
	for _, resource := range resources {
		if resource.ResourceType != localDirectoryResourceType {
			continue
		}
		repo, err := primaryRepositoryFromResource(resource)
		if err != nil {
			return PrimaryRepository{}, err
		}
		return repo, nil
	}
	return PrimaryRepository{}, ErrPrimaryRepositoryMissing
}

type localDirectoryRef struct {
	LocalPath string `json:"local_path"`
	DaemonID  string `json:"daemon_id"`
	Label     string `json:"label"`
}

func primaryRepositoryFromResource(resource db.ProjectResource) (PrimaryRepository, error) {
	var ref localDirectoryRef
	if err := json.Unmarshal(resource.ResourceRef, &ref); err != nil {
		return PrimaryRepository{}, fmt.Errorf("%w: parse local_directory resource_ref: %w", ErrInvalidProjectResource, err)
	}
	ref.LocalPath = strings.TrimSpace(ref.LocalPath)
	ref.DaemonID = strings.TrimSpace(ref.DaemonID)
	ref.Label = strings.TrimSpace(ref.Label)
	if ref.LocalPath == "" {
		return PrimaryRepository{}, fmt.Errorf("%w: local_directory local_path is empty", ErrInvalidProjectResource)
	}
	if ref.DaemonID == "" {
		return PrimaryRepository{}, fmt.Errorf("%w: local_directory daemon_id is empty", ErrInvalidProjectResource)
	}
	return PrimaryRepository{
		Resource:  resource,
		LocalPath: ref.LocalPath,
		DaemonID:  ref.DaemonID,
		Label:     ref.Label,
	}, nil
}
