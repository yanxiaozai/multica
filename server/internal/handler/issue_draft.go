package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/service/issuedraft"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

type CreateIssueDraftRequest struct {
	ProjectID      string `json:"project_id"`
	SquadID        string `json:"squad_id"`
	InitialMessage string `json:"initial_message"`
}

type AppendIssueDraftMessageRequest struct {
	Content  string          `json:"content"`
	Metadata json.RawMessage `json:"metadata"`
}

type IssueDraftSessionResponse struct {
	ID                       string  `json:"id"`
	WorkspaceID              string  `json:"workspace_id"`
	ProjectID                string  `json:"project_id"`
	SquadID                  string  `json:"squad_id"`
	LeaderAgentID            string  `json:"leader_agent_id"`
	PrimaryProjectResourceID string  `json:"primary_project_resource_id"`
	PrimaryLocalPathSnapshot string  `json:"primary_local_path_snapshot"`
	Status                   string  `json:"status"`
	CreatedBy                string  `json:"created_by"`
	CreatedIssueID           *string `json:"created_issue_id"`
	RemoteIssueURL           string  `json:"remote_issue_url"`
	SpecFilePath             string  `json:"spec_file_path"`
	GitCommitSHA             string  `json:"git_commit_sha"`
	LastError                string  `json:"last_error"`
	CreatedAt                string  `json:"created_at"`
	UpdatedAt                string  `json:"updated_at"`
}

type IssueDraftMessageResponse struct {
	ID          string          `json:"id"`
	SessionID   string          `json:"session_id"`
	WorkspaceID string          `json:"workspace_id"`
	AuthorType  string          `json:"author_type"`
	AuthorID    *string         `json:"author_id"`
	MessageType string          `json:"message_type"`
	Content     string          `json:"content"`
	Metadata    json.RawMessage `json:"metadata"`
	CreatedAt   string          `json:"created_at"`
}

type IssueDraftMemberTaskResponse struct {
	ID          string          `json:"id"`
	SessionID   string          `json:"session_id"`
	WorkspaceID string          `json:"workspace_id"`
	AgentID     string          `json:"agent_id"`
	TaskID      *string         `json:"task_id"`
	Status      string          `json:"status"`
	SkillBasis  string          `json:"skill_basis"`
	ReadScope   json.RawMessage `json:"read_scope"`
	Findings    string          `json:"findings"`
	Error       string          `json:"error"`
	CreatedAt   string          `json:"created_at"`
	UpdatedAt   string          `json:"updated_at"`
}

type IssueDraftArtifactResponse struct {
	ID                 string  `json:"id"`
	SessionID          string  `json:"session_id"`
	WorkspaceID        string  `json:"workspace_id"`
	ArtifactType       string  `json:"artifact_type"`
	Revision           int32   `json:"revision"`
	Content            string  `json:"content"`
	GeneratedByAgentID *string `json:"generated_by_agent_id"`
	CreatedAt          string  `json:"created_at"`
}

type IssueDraftConfirmStepResponse struct {
	SessionID      string          `json:"session_id"`
	WorkspaceID    string          `json:"workspace_id"`
	Step           string          `json:"step"`
	Status         string          `json:"status"`
	ExternalID     string          `json:"external_id"`
	ResultMetadata json.RawMessage `json:"result_metadata"`
	Error          string          `json:"error"`
	CreatedAt      string          `json:"created_at"`
	UpdatedAt      string          `json:"updated_at"`
}

type IssueDraftBundleResponse struct {
	Session      IssueDraftSessionResponse       `json:"session"`
	Messages     []IssueDraftMessageResponse     `json:"messages"`
	MemberTasks  []IssueDraftMemberTaskResponse  `json:"member_tasks"`
	Artifacts    []IssueDraftArtifactResponse    `json:"artifacts"`
	ConfirmSteps []IssueDraftConfirmStepResponse `json:"confirm_steps"`
}

func (h *Handler) CreateIssueDraft(w http.ResponseWriter, r *http.Request) {
	workspaceID := h.resolveWorkspaceID(r)
	member, ok := h.workspaceMember(w, r, workspaceID)
	if !ok {
		return
	}
	workspaceUUID, ok := parseUUIDOrBadRequest(w, workspaceID, "workspace_id")
	if !ok {
		return
	}

	var req CreateIssueDraftRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	projectID, ok := parseUUIDOrBadRequest(w, strings.TrimSpace(req.ProjectID), "project_id")
	if !ok {
		return
	}
	squadID, ok := parseUUIDOrBadRequest(w, strings.TrimSpace(req.SquadID), "squad_id")
	if !ok {
		return
	}

	bundle, err := h.IssueDraftService.CreateSession(r.Context(), issuedraft.CreateSessionInput{
		WorkspaceID:       workspaceUUID,
		ProjectID:         projectID,
		SquadID:           squadID,
		CreatedByMemberID: member.ID,
		InitialMessage:    req.InitialMessage,
	})
	if err != nil {
		h.writeIssueDraftServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, issueDraftBundleToResponse(bundle))
}

func (h *Handler) GetIssueDraft(w http.ResponseWriter, r *http.Request) {
	workspaceUUID, sessionID, ok := h.issueDraftRouteScope(w, r)
	if !ok {
		return
	}
	bundle, err := h.IssueDraftService.GetSessionBundle(r.Context(), workspaceUUID, sessionID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeError(w, http.StatusNotFound, "issue draft not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to get issue draft")
		return
	}
	writeJSON(w, http.StatusOK, issueDraftBundleToResponse(bundle))
}

func (h *Handler) AppendIssueDraftMessage(w http.ResponseWriter, r *http.Request) {
	workspaceUUID, sessionID, ok := h.issueDraftRouteScope(w, r)
	if !ok {
		return
	}
	member, ok := h.workspaceMember(w, r, uuidToString(workspaceUUID))
	if !ok {
		return
	}

	var req AppendIssueDraftMessageRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if strings.TrimSpace(req.Content) == "" {
		writeError(w, http.StatusBadRequest, "content is required")
		return
	}
	msg, err := h.IssueDraftService.AppendMessage(r.Context(), issuedraft.AppendMessageInput{
		SessionID:   sessionID,
		WorkspaceID: workspaceUUID,
		AuthorType:  issuedraft.AuthorMember,
		AuthorID:    member.ID,
		MessageType: issuedraft.MessageUserMessage,
		Content:     req.Content,
		Metadata:    req.Metadata,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to append issue draft message")
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"message": issueDraftMessageToResponse(msg)})
}

func (h *Handler) DelegateIssueDraft(w http.ResponseWriter, r *http.Request) {
	writeError(w, http.StatusNotImplemented, "issue draft delegation is not implemented yet")
}

func (h *Handler) GenerateIssueDraft(w http.ResponseWriter, r *http.Request) {
	writeError(w, http.StatusNotImplemented, "issue draft generation is not implemented yet")
}

func (h *Handler) ConfirmIssueDraft(w http.ResponseWriter, r *http.Request) {
	writeError(w, http.StatusNotImplemented, "issue draft confirmation is not implemented yet")
}

func (h *Handler) CancelIssueDraft(w http.ResponseWriter, r *http.Request) {
	writeError(w, http.StatusNotImplemented, "issue draft cancellation is not implemented yet")
}

func (h *Handler) issueDraftRouteScope(w http.ResponseWriter, r *http.Request) (pgtype.UUID, pgtype.UUID, bool) {
	workspaceID := h.resolveWorkspaceID(r)
	if _, ok := h.workspaceMember(w, r, workspaceID); !ok {
		return pgtype.UUID{}, pgtype.UUID{}, false
	}
	workspaceUUID, ok := parseUUIDOrBadRequest(w, workspaceID, "workspace_id")
	if !ok {
		return pgtype.UUID{}, pgtype.UUID{}, false
	}
	sessionID, ok := parseUUIDOrBadRequest(w, chi.URLParam(r, "id"), "issue draft id")
	if !ok {
		return pgtype.UUID{}, pgtype.UUID{}, false
	}
	return workspaceUUID, sessionID, true
}

func (h *Handler) writeIssueDraftServiceError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, issuedraft.ErrProjectNotFound):
		writeError(w, http.StatusNotFound, "project not found")
	case errors.Is(err, issuedraft.ErrSquadNotFound):
		writeError(w, http.StatusNotFound, "squad not found")
	case errors.Is(err, issuedraft.ErrSquadArchived):
		writeError(w, http.StatusConflict, "squad is archived")
	case errors.Is(err, issuedraft.ErrPrimaryRepositoryMissing):
		writeError(w, http.StatusBadRequest, "project has no local_directory resource")
	case errors.Is(err, issuedraft.ErrInvalidProjectResource):
		writeError(w, http.StatusBadRequest, err.Error())
	default:
		writeError(w, http.StatusInternalServerError, "failed to create issue draft")
	}
}

func issueDraftBundleToResponse(bundle issuedraft.SessionBundle) IssueDraftBundleResponse {
	messages := make([]IssueDraftMessageResponse, 0, len(bundle.Messages))
	for _, msg := range bundle.Messages {
		messages = append(messages, issueDraftMessageToResponse(msg))
	}
	memberTasks := make([]IssueDraftMemberTaskResponse, 0, len(bundle.MemberTasks))
	for _, task := range bundle.MemberTasks {
		memberTasks = append(memberTasks, issueDraftMemberTaskToResponse(task))
	}
	artifacts := make([]IssueDraftArtifactResponse, 0, len(bundle.Artifacts))
	for _, artifact := range bundle.Artifacts {
		artifacts = append(artifacts, issueDraftArtifactToResponse(artifact))
	}
	confirmSteps := make([]IssueDraftConfirmStepResponse, 0, len(bundle.ConfirmSteps))
	for _, step := range bundle.ConfirmSteps {
		confirmSteps = append(confirmSteps, issueDraftConfirmStepToResponse(step))
	}
	return IssueDraftBundleResponse{
		Session:      issueDraftSessionToResponse(bundle.Session),
		Messages:     messages,
		MemberTasks:  memberTasks,
		Artifacts:    artifacts,
		ConfirmSteps: confirmSteps,
	}
}

func issueDraftSessionToResponse(session db.IssueDraftSession) IssueDraftSessionResponse {
	return IssueDraftSessionResponse{
		ID:                       uuidToString(session.ID),
		WorkspaceID:              uuidToString(session.WorkspaceID),
		ProjectID:                uuidToString(session.ProjectID),
		SquadID:                  uuidToString(session.SquadID),
		LeaderAgentID:            uuidToString(session.LeaderAgentID),
		PrimaryProjectResourceID: uuidToString(session.PrimaryProjectResourceID),
		PrimaryLocalPathSnapshot: session.PrimaryLocalPathSnapshot,
		Status:                   session.Status,
		CreatedBy:                uuidToString(session.CreatedBy),
		CreatedIssueID:           uuidToPtr(session.CreatedIssueID),
		RemoteIssueURL:           session.RemoteIssueUrl,
		SpecFilePath:             session.SpecFilePath,
		GitCommitSHA:             session.GitCommitSha,
		LastError:                session.LastError,
		CreatedAt:                timestampToString(session.CreatedAt),
		UpdatedAt:                timestampToString(session.UpdatedAt),
	}
}

func issueDraftMessageToResponse(msg db.IssueDraftMessage) IssueDraftMessageResponse {
	return IssueDraftMessageResponse{
		ID:          uuidToString(msg.ID),
		SessionID:   uuidToString(msg.SessionID),
		WorkspaceID: uuidToString(msg.WorkspaceID),
		AuthorType:  msg.AuthorType,
		AuthorID:    uuidToPtr(msg.AuthorID),
		MessageType: msg.MessageType,
		Content:     msg.Content,
		Metadata:    rawJSONOrObject(msg.Metadata),
		CreatedAt:   timestampToString(msg.CreatedAt),
	}
}

func issueDraftMemberTaskToResponse(task db.IssueDraftMemberTask) IssueDraftMemberTaskResponse {
	return IssueDraftMemberTaskResponse{
		ID:          uuidToString(task.ID),
		SessionID:   uuidToString(task.SessionID),
		WorkspaceID: uuidToString(task.WorkspaceID),
		AgentID:     uuidToString(task.AgentID),
		TaskID:      uuidToPtr(task.TaskID),
		Status:      task.Status,
		SkillBasis:  task.SkillBasis,
		ReadScope:   rawJSONOrObject(task.ReadScope),
		Findings:    task.Findings,
		Error:       task.Error,
		CreatedAt:   timestampToString(task.CreatedAt),
		UpdatedAt:   timestampToString(task.UpdatedAt),
	}
}

func issueDraftArtifactToResponse(artifact db.IssueDraftArtifact) IssueDraftArtifactResponse {
	return IssueDraftArtifactResponse{
		ID:                 uuidToString(artifact.ID),
		SessionID:          uuidToString(artifact.SessionID),
		WorkspaceID:        uuidToString(artifact.WorkspaceID),
		ArtifactType:       artifact.ArtifactType,
		Revision:           artifact.Revision,
		Content:            artifact.Content,
		GeneratedByAgentID: uuidToPtr(artifact.GeneratedByAgentID),
		CreatedAt:          timestampToString(artifact.CreatedAt),
	}
}

func issueDraftConfirmStepToResponse(step db.IssueDraftConfirmStep) IssueDraftConfirmStepResponse {
	return IssueDraftConfirmStepResponse{
		SessionID:      uuidToString(step.SessionID),
		WorkspaceID:    uuidToString(step.WorkspaceID),
		Step:           step.Step,
		Status:         step.Status,
		ExternalID:     step.ExternalID,
		ResultMetadata: rawJSONOrObject(step.ResultMetadata),
		Error:          step.Error,
		CreatedAt:      timestampToString(step.CreatedAt),
		UpdatedAt:      timestampToString(step.UpdatedAt),
	}
}

func rawJSONOrObject(raw []byte) json.RawMessage {
	if len(raw) == 0 || !json.Valid(raw) {
		return json.RawMessage(`{}`)
	}
	return json.RawMessage(raw)
}
