package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/service"
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

type DelegateIssueDraftRequest struct {
	Prompt string   `json:"prompt"`
	Agents []string `json:"agents"`
}

type CancelIssueDraftRequest struct {
	Reason string `json:"reason"`
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

func (h *Handler) ListIssueDrafts(w http.ResponseWriter, r *http.Request) {
	workspaceID := h.resolveWorkspaceID(r)
	if _, ok := h.workspaceMember(w, r, workspaceID); !ok {
		return
	}
	workspaceUUID, ok := parseUUIDOrBadRequest(w, workspaceID, "workspace_id")
	if !ok {
		return
	}

	rows, err := h.Queries.ListIssueDraftSessions(r.Context(), db.ListIssueDraftSessionsParams{
		WorkspaceID: workspaceUUID,
		Limit:       50,
		Offset:      0,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list issue drafts")
		return
	}

	sessions := make([]IssueDraftSessionResponse, 0, len(rows))
	for _, row := range rows {
		if row.Status == "created" || row.Status == "cancelled" {
			continue
		}
		sessions = append(sessions, issueDraftSessionToResponse(row))
	}

	writeJSON(w, http.StatusOK, map[string]any{"sessions": sessions})
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

func (h *Handler) GetActiveIssueDraft(w http.ResponseWriter, r *http.Request) {
	workspaceID := h.resolveWorkspaceID(r)
	if _, ok := h.workspaceMember(w, r, workspaceID); !ok {
		return
	}
	workspaceUUID, ok := parseUUIDOrBadRequest(w, workspaceID, "workspace_id")
	if !ok {
		return
	}
	bundle, err := h.IssueDraftService.GetActiveSessionBundle(r.Context(), workspaceUUID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to get active issue draft")
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
	if _, _, _, err := h.delegateIssueDraft(r.Context(), workspaceUUID, sessionID, DelegateIssueDraftRequest{
		Prompt: "用户补充了新的需求信息。请基于最新完整对话继续澄清；如果信息已经足够，请给出可生成 issue 草稿的结论。",
	}); err != nil {
		_, _ = h.IssueDraftService.AppendMessage(r.Context(), issuedraft.AppendMessageInput{
			SessionID:   sessionID,
			WorkspaceID: workspaceUUID,
			AuthorType:  issuedraft.AuthorSystem,
			MessageType: issuedraft.MessageError,
			Content:     "已保存回复，但自动继续澄清失败：" + err.Error(),
		})
	}
	writeJSON(w, http.StatusCreated, map[string]any{"message": issueDraftMessageToResponse(msg)})
}

func (h *Handler) DelegateIssueDraft(w http.ResponseWriter, r *http.Request) {
	workspaceUUID, sessionID, ok := h.issueDraftRouteScope(w, r)
	if !ok {
		return
	}
	var req DelegateIssueDraftRequest
	if r.Body != nil {
		_ = json.NewDecoder(r.Body).Decode(&req)
	}
	updated, queued, alreadyActive, err := h.delegateIssueDraft(r.Context(), workspaceUUID, sessionID, req)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeError(w, http.StatusNotFound, "issue draft not found")
			return
		}
		if errors.Is(err, errNoIssueDraftDelegateAgents) {
			writeError(w, http.StatusConflict, "no squad agents were available to delegate")
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if alreadyActive {
		writeJSON(w, http.StatusAccepted, issueDraftBundleToResponse(updated))
		return
	}
	if queued == 0 {
		writeError(w, http.StatusConflict, "no squad agents were available to delegate")
		return
	}
	writeJSON(w, http.StatusAccepted, issueDraftBundleToResponse(updated))
}

var errNoIssueDraftDelegateAgents = errors.New("no squad agents were available to delegate")

func (h *Handler) delegateIssueDraft(ctx context.Context, workspaceUUID, sessionID pgtype.UUID, req DelegateIssueDraftRequest) (issuedraft.SessionBundle, int, bool, error) {
	bundle, err := h.IssueDraftService.GetSessionBundle(ctx, workspaceUUID, sessionID)
	if err != nil {
		return issuedraft.SessionBundle{}, 0, false, fmt.Errorf("issue draft not found: %w", err)
	}
	members, err := h.Queries.ListSquadMembers(ctx, bundle.Session.SquadID)
	if err != nil {
		return issuedraft.SessionBundle{}, 0, false, fmt.Errorf("failed to list squad members: %w", err)
	}
	agentFilter := make(map[string]struct{}, len(req.Agents))
	for _, id := range req.Agents {
		id = strings.TrimSpace(id)
		if id != "" {
			agentFilter[id] = struct{}{}
		}
	}
	if len(agentFilter) == 0 {
		agentFilter[uuidToString(bundle.Session.LeaderAgentID)] = struct{}{}
	}
	for _, task := range bundle.MemberTasks {
		if task.Status == "queued" || task.Status == "running" {
			if _, ok := agentFilter[uuidToString(task.AgentID)]; ok {
				return bundle, 0, true, nil
			}
		}
	}
	queued := 0
	for _, member := range members {
		if member.MemberType != "agent" {
			continue
		}
		agentID := uuidToString(member.MemberID)
		if len(agentFilter) > 0 {
			if _, ok := agentFilter[agentID]; !ok {
				continue
			}
		}
		agent, err := h.Queries.GetAgentInWorkspace(ctx, db.GetAgentInWorkspaceParams{
			ID:          member.MemberID,
			WorkspaceID: workspaceUUID,
		})
		if err != nil {
			continue
		}
		readScope := json.RawMessage(fmt.Sprintf(`{"primary_local_path":%q,"mode":"read_only"}`, bundle.Session.PrimaryLocalPathSnapshot))
		memberTask, err := h.Queries.CreateIssueDraftMemberTask(ctx, db.CreateIssueDraftMemberTaskParams{
			SessionID:   sessionID,
			WorkspaceID: workspaceUUID,
			AgentID:     member.MemberID,
			Status:      "queued",
			SkillBasis:  strings.TrimSpace(member.Role),
			ReadScope:   readScope,
		})
		if err != nil {
			continue
		}
		prompt := buildIssueDraftMemberPrompt(bundle, agent, member, req.Prompt)
		task, err := h.TaskService.EnqueueIssueDraftTask(ctx, agent, service.IssueDraftContext{
			WorkspaceID:      uuidToString(workspaceUUID),
			SessionID:        uuidToString(sessionID),
			ProjectID:        uuidToString(bundle.Session.ProjectID),
			SquadID:          uuidToString(bundle.Session.SquadID),
			MemberTaskID:     uuidToString(memberTask.ID),
			Role:             strings.TrimSpace(member.Role),
			Prompt:           prompt,
			PrimaryLocalPath: bundle.Session.PrimaryLocalPathSnapshot,
		})
		if err != nil {
			_, _ = h.Queries.UpdateIssueDraftMemberTaskStatus(ctx, db.UpdateIssueDraftMemberTaskStatusParams{
				ID:          memberTask.ID,
				WorkspaceID: workspaceUUID,
				Status:      "failed",
				Error:       err.Error(),
			})
			continue
		}
		if _, err := h.Queries.LinkIssueDraftMemberTaskQueueItem(ctx, db.LinkIssueDraftMemberTaskQueueItemParams{
			ID:          memberTask.ID,
			WorkspaceID: workspaceUUID,
			TaskID:      task.ID,
		}); err != nil {
			return issuedraft.SessionBundle{}, 0, false, fmt.Errorf("failed to link issue draft task: %w", err)
		}
		queued++
	}
	if queued == 0 {
		return issuedraft.SessionBundle{}, 0, false, errNoIssueDraftDelegateAgents
	}
	if _, err := h.IssueDraftService.AppendMessage(ctx, issuedraft.AppendMessageInput{
		SessionID:   sessionID,
		WorkspaceID: workspaceUUID,
		AuthorType:  issuedraft.AuthorSystem,
		MessageType: issuedraft.MessageStatus,
		Content:     fmt.Sprintf("已派发 %d 个主要负责人只读澄清任务。", queued),
	}); err != nil {
		return issuedraft.SessionBundle{}, 0, false, fmt.Errorf("failed to append issue draft status: %w", err)
	}
	if _, err := h.Queries.UpdateIssueDraftSessionStatus(ctx, db.UpdateIssueDraftSessionStatusParams{
		ID:          sessionID,
		WorkspaceID: workspaceUUID,
		Status:      issuedraft.StatusDelegating,
		LastError:   "",
	}); err != nil {
		return issuedraft.SessionBundle{}, 0, false, fmt.Errorf("failed to update issue draft: %w", err)
	}
	updated, err := h.IssueDraftService.GetSessionBundle(ctx, workspaceUUID, sessionID)
	if err != nil {
		return issuedraft.SessionBundle{}, 0, false, fmt.Errorf("failed to load issue draft: %w", err)
	}
	return updated, queued, false, nil
}

func (h *Handler) GenerateIssueDraft(w http.ResponseWriter, r *http.Request) {
	workspaceUUID, sessionID, ok := h.issueDraftRouteScope(w, r)
	if !ok {
		return
	}
	bundle, err := h.IssueDraftService.GenerateArtifacts(r.Context(), issuedraft.GenerateArtifactsInput{
		SessionID:   sessionID,
		WorkspaceID: workspaceUUID,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to generate issue draft")
		return
	}
	writeJSON(w, http.StatusOK, issueDraftBundleToResponse(bundle))
}

func (h *Handler) ConfirmIssueDraft(w http.ResponseWriter, r *http.Request) {
	workspaceUUID, sessionID, ok := h.issueDraftRouteScope(w, r)
	if !ok {
		return
	}
	member, ok := h.workspaceMember(w, r, uuidToString(workspaceUUID))
	if !ok {
		return
	}
	bundle, err := h.IssueDraftService.Confirm(r.Context(), issuedraft.ConfirmInput{
		SessionID:   sessionID,
		WorkspaceID: workspaceUUID,
		ConfirmedBy: member.ID,
	})
	if err != nil {
		writeError(w, http.StatusConflict, "issue draft is not ready to confirm")
		return
	}
	bundle, err = h.executeIssueDraftConfirmation(r.Context(), bundle, member.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to execute issue draft confirmation: "+err.Error())
		return
	}
	writeJSON(w, http.StatusAccepted, issueDraftBundleToResponse(bundle))
}

func (h *Handler) CancelIssueDraft(w http.ResponseWriter, r *http.Request) {
	workspaceUUID, sessionID, ok := h.issueDraftRouteScope(w, r)
	if !ok {
		return
	}
	member, ok := h.workspaceMember(w, r, uuidToString(workspaceUUID))
	if !ok {
		return
	}
	var req CancelIssueDraftRequest
	if r.Body != nil {
		_ = json.NewDecoder(r.Body).Decode(&req)
	}
	bundle, err := h.IssueDraftService.Cancel(r.Context(), issuedraft.CancelInput{
		SessionID:   sessionID,
		WorkspaceID: workspaceUUID,
		CancelledBy: member.ID,
		Reason:      req.Reason,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to cancel issue draft")
		return
	}
	writeJSON(w, http.StatusOK, issueDraftBundleToResponse(bundle))
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

func buildIssueDraftMemberPrompt(bundle issuedraft.SessionBundle, agent db.Agent, member db.SquadMember, extraPrompt string) string {
	var b strings.Builder
	b.WriteString("你正在参与一个 issue draft 澄清会话。请使用你的真实技能和已有 instructions，帮助用户把需求澄清到可创建 issue 的程度。\n\n")
	b.WriteString("硬性约束:\n")
	b.WriteString("- 这是只读任务，不要修改代码、不要创建/更新 issue、不要提交或推送 git。\n")
	b.WriteString("- 如果需要检查已有代码，只能读取目标项目主仓库，并在结论中引用文件路径/行号。\n")
	b.WriteString("- 明确区分事实、推断和仍需用户回答的问题。\n\n")
	b.WriteString("上下文:\n")
	b.WriteString("- Agent: ")
	b.WriteString(agent.Name)
	b.WriteByte('\n')
	b.WriteString("- 小队角色: ")
	b.WriteString(strings.TrimSpace(member.Role))
	b.WriteByte('\n')
	b.WriteString("- 目标项目主仓库: ")
	b.WriteString(bundle.Session.PrimaryLocalPathSnapshot)
	b.WriteString("\n\n")
	if strings.TrimSpace(extraPrompt) != "" {
		b.WriteString("本轮重点:\n")
		b.WriteString(strings.TrimSpace(extraPrompt))
		b.WriteString("\n\n")
	}
	b.WriteString("当前对话:\n")
	for _, msg := range bundle.Messages {
		content := strings.TrimSpace(msg.Content)
		if content == "" {
			continue
		}
		b.WriteString("- ")
		b.WriteString(msg.AuthorType)
		b.WriteString("/")
		b.WriteString(msg.MessageType)
		b.WriteString(": ")
		b.WriteString(content)
		b.WriteByte('\n')
	}
	b.WriteString("\n请输出:\n")
	b.WriteString("1. 你需要用户补充的问题，按优先级排列。\n")
	b.WriteString("2. 如果已检查代码，列出证据路径和影响判断。\n")
	b.WriteString("3. 你建议写入 issue 模板的验收标准/技术约束/风险。\n")
	return b.String()
}

func (h *Handler) executeIssueDraftConfirmation(ctx context.Context, bundle issuedraft.SessionBundle, confirmedBy pgtype.UUID) (issuedraft.SessionBundle, error) {
	session := bundle.Session
	detailedSpec, ok := latestIssueDraftArtifact(bundle.Artifacts, issuedraft.ArtifactDetailedSpec)
	if !ok {
		return bundle, fmt.Errorf("missing detailed spec artifact")
	}
	multicaIssue, ok := latestIssueDraftArtifact(bundle.Artifacts, issuedraft.ArtifactMulticaIssue)
	if !ok {
		return bundle, fmt.Errorf("missing multica issue artifact")
	}
	remoteIssue, ok := latestIssueDraftArtifact(bundle.Artifacts, issuedraft.ArtifactRemoteIssue)
	if !ok {
		return bundle, fmt.Errorf("missing remote issue artifact")
	}

	issueTitle := issueDraftTitle(detailedSpec.Content)
	if issueTitle == "" {
		issueTitle = "Issue draft"
	}
	if err := h.markIssueDraftStep(ctx, session.WorkspaceID, session.ID, "create_multica_issue", "running", "", nil, ""); err != nil {
		return bundle, err
	}
	created, err := h.IssueService.Create(ctx, service.IssueCreateParams{
		WorkspaceID:    session.WorkspaceID,
		Title:          issueTitle,
		Description:    pgtype.Text{String: multicaIssue.Content, Valid: strings.TrimSpace(multicaIssue.Content) != ""},
		Status:         "backlog",
		Priority:       "none",
		AssigneeType:   pgtype.Text{String: "squad", Valid: true},
		AssigneeID:     session.SquadID,
		CreatorType:    "member",
		CreatorID:      confirmedBy,
		ProjectID:      session.ProjectID,
		AllowDuplicate: true,
	}, service.IssueCreateOpts{
		ActorID:  uuidToString(confirmedBy),
		Platform: "issue_draft",
	})
	if err != nil {
		_ = h.failIssueDraftStep(ctx, session.WorkspaceID, session.ID, "create_multica_issue", err)
		return h.failIssueDraftSession(ctx, session.WorkspaceID, session.ID, err)
	}
	issue := created.Issue
	identifier := h.getIssuePrefix(ctx, session.WorkspaceID) + "-" + strconv.Itoa(int(issue.Number))
	if err := h.markIssueDraftStep(ctx, session.WorkspaceID, session.ID, "create_multica_issue", "succeeded", uuidToString(issue.ID), map[string]any{"identifier": identifier}, ""); err != nil {
		return bundle, err
	}

	if err := h.markIssueDraftStep(ctx, session.WorkspaceID, session.ID, "write_spec", "running", "", nil, ""); err != nil {
		return bundle, err
	}
	specPath, err := writeIssueDraftSpecFile(session.PrimaryLocalPathSnapshot, identifier, detailedSpec.Content)
	if err != nil {
		_ = h.failIssueDraftStep(ctx, session.WorkspaceID, session.ID, "write_spec", err)
		return h.failIssueDraftSession(ctx, session.WorkspaceID, session.ID, err)
	}
	if err := h.markIssueDraftStep(ctx, session.WorkspaceID, session.ID, "write_spec", "succeeded", specPath, map[string]any{"path": specPath}, ""); err != nil {
		return bundle, err
	}

	remoteURL := ""
	if err := h.markIssueDraftStep(ctx, session.WorkspaceID, session.ID, "create_remote_issue", "running", "", nil, ""); err != nil {
		return bundle, err
	}
	remoteURL, err = createRemoteIssue(ctx, session.PrimaryLocalPathSnapshot, issueTitle, remoteIssue.Content)
	if err != nil {
		_ = h.failIssueDraftStep(ctx, session.WorkspaceID, session.ID, "create_remote_issue", err)
		return h.failIssueDraftSession(ctx, session.WorkspaceID, session.ID, err)
	}
	if err := h.markIssueDraftStep(ctx, session.WorkspaceID, session.ID, "create_remote_issue", "succeeded", remoteURL, map[string]any{"url": remoteURL}, ""); err != nil {
		return bundle, err
	}

	if err := h.markIssueDraftStep(ctx, session.WorkspaceID, session.ID, "commit_and_push", "running", "", nil, ""); err != nil {
		return bundle, err
	}
	commitSHA, err := commitAndPushIssueDraftSpec(ctx, session.PrimaryLocalPathSnapshot, specPath, identifier)
	if err != nil {
		_ = h.failIssueDraftStep(ctx, session.WorkspaceID, session.ID, "commit_and_push", err)
		return h.failIssueDraftSession(ctx, session.WorkspaceID, session.ID, err)
	}
	if err := h.markIssueDraftStep(ctx, session.WorkspaceID, session.ID, "commit_and_push", "succeeded", commitSHA, map[string]any{"commit_sha": commitSHA}, ""); err != nil {
		return bundle, err
	}

	if err := h.markIssueDraftStep(ctx, session.WorkspaceID, session.ID, "link_outputs", "running", "", nil, ""); err != nil {
		return bundle, err
	}
	if _, err := h.Queries.MarkIssueDraftCreated(ctx, db.MarkIssueDraftCreatedParams{
		ID:             session.ID,
		WorkspaceID:    session.WorkspaceID,
		CreatedIssueID: issue.ID,
		RemoteIssueUrl: remoteURL,
		SpecFilePath:   specPath,
		GitCommitSha:   commitSHA,
	}); err != nil {
		_ = h.failIssueDraftStep(ctx, session.WorkspaceID, session.ID, "link_outputs", err)
		return h.failIssueDraftSession(ctx, session.WorkspaceID, session.ID, err)
	}
	if err := h.markIssueDraftStep(ctx, session.WorkspaceID, session.ID, "link_outputs", "succeeded", uuidToString(issue.ID), map[string]any{
		"issue_id":         uuidToString(issue.ID),
		"issue_identifier": identifier,
		"remote_issue_url": remoteURL,
		"spec_file_path":   specPath,
		"git_commit_sha":   commitSHA,
	}, ""); err != nil {
		return bundle, err
	}
	return h.IssueDraftService.GetSessionBundle(ctx, session.WorkspaceID, session.ID)
}

func latestIssueDraftArtifact(artifacts []db.IssueDraftArtifact, kind string) (db.IssueDraftArtifact, bool) {
	var latest db.IssueDraftArtifact
	found := false
	for _, artifact := range artifacts {
		if artifact.ArtifactType != kind {
			continue
		}
		if !found || artifact.Revision > latest.Revision {
			latest = artifact
			found = true
		}
	}
	return latest, found
}

func issueDraftTitle(content string) string {
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "# ") {
			return strings.TrimSpace(strings.TrimPrefix(line, "# "))
		}
	}
	return ""
}

func writeIssueDraftSpecFile(repoPath, identifier, content string) (string, error) {
	repoPath = filepath.Clean(strings.TrimSpace(repoPath))
	if repoPath == "." || repoPath == "" {
		return "", fmt.Errorf("target repository path is empty")
	}
	specDir := filepath.Join(repoPath, ".spec", "issues")
	if err := os.MkdirAll(specDir, 0o755); err != nil {
		return "", fmt.Errorf("create spec directory: %w", err)
	}
	specPath := filepath.Join(specDir, safeIssueDraftFilename(identifier)+".md")
	if err := os.WriteFile(specPath, []byte(strings.TrimSpace(content)+"\n"), 0o644); err != nil {
		return "", fmt.Errorf("write spec file: %w", err)
	}
	return specPath, nil
}

func safeIssueDraftFilename(value string) string {
	value = strings.TrimSpace(strings.ToLower(value))
	var b strings.Builder
	lastDash := false
	for _, r := range value {
		ok := (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9')
		if ok {
			b.WriteRune(r)
			lastDash = false
			continue
		}
		if !lastDash {
			b.WriteByte('-')
			lastDash = true
		}
	}
	out := strings.Trim(b.String(), "-")
	if out == "" {
		return "issue-draft"
	}
	return out
}

func createRemoteIssue(ctx context.Context, repoPath, title, body string) (string, error) {
	repoPath = filepath.Clean(strings.TrimSpace(repoPath))
	if repoPath == "." || repoPath == "" {
		return "", fmt.Errorf("target repository path is empty")
	}
	bodyFile, err := os.CreateTemp("", "multica-remote-issue-*.md")
	if err != nil {
		return "", fmt.Errorf("create remote issue body file: %w", err)
	}
	defer os.Remove(bodyFile.Name())
	if _, err := bodyFile.WriteString(strings.TrimSpace(body) + "\n"); err != nil {
		bodyFile.Close()
		return "", fmt.Errorf("write remote issue body file: %w", err)
	}
	if err := bodyFile.Close(); err != nil {
		return "", fmt.Errorf("close remote issue body file: %w", err)
	}
	remote, err := runCommand(ctx, repoPath, "git", "remote", "get-url", "origin")
	if err != nil {
		return "", err
	}
	remote = strings.TrimSpace(remote)
	switch {
	case strings.Contains(remote, "github.com"):
		out, err := runCommand(ctx, repoPath, "gh", "issue", "create", "--title", title, "--body-file", bodyFile.Name())
		if err != nil {
			return "", err
		}
		return strings.TrimSpace(out), nil
	case strings.Contains(remote, "gitlab"):
		out, err := runCommand(ctx, repoPath, "glab", "issue", "create", "--title", title, "--description-file", bodyFile.Name())
		if err != nil {
			return "", err
		}
		return strings.TrimSpace(out), nil
	default:
		return "", fmt.Errorf("unsupported git remote for remote issue creation: %s", remote)
	}
}

func commitAndPushIssueDraftSpec(ctx context.Context, repoPath, specPath, identifier string) (string, error) {
	if _, err := runCommand(ctx, repoPath, "git", "add", specPath); err != nil {
		return "", err
	}
	if _, err := runCommand(ctx, repoPath, "git", "commit", "-m", "docs: add spec for "+identifier); err != nil {
		return "", err
	}
	sha, err := runCommand(ctx, repoPath, "git", "rev-parse", "HEAD")
	if err != nil {
		return "", err
	}
	if _, err := runCommand(ctx, repoPath, "git", "push"); err != nil {
		return "", err
	}
	return strings.TrimSpace(sha), nil
}

func runCommand(ctx context.Context, dir, name string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = dir
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = strings.TrimSpace(stdout.String())
		}
		if msg == "" {
			msg = err.Error()
		}
		return "", fmt.Errorf("%s %s failed: %s", name, strings.Join(args, " "), msg)
	}
	return stdout.String(), nil
}

func (h *Handler) markIssueDraftStep(ctx context.Context, workspaceID, sessionID pgtype.UUID, step, status, externalID string, metadata map[string]any, errText string) error {
	raw := []byte(`{}`)
	if metadata != nil {
		var err error
		raw, err = json.Marshal(metadata)
		if err != nil {
			return fmt.Errorf("marshal step metadata: %w", err)
		}
	}
	_, err := h.Queries.UpsertIssueDraftConfirmStep(ctx, db.UpsertIssueDraftConfirmStepParams{
		SessionID:      sessionID,
		WorkspaceID:    workspaceID,
		Step:           step,
		Status:         status,
		ExternalID:     externalID,
		ResultMetadata: raw,
		Error:          errText,
	})
	if err != nil {
		return fmt.Errorf("update confirm step %s: %w", step, err)
	}
	return nil
}

func (h *Handler) failIssueDraftStep(ctx context.Context, workspaceID, sessionID pgtype.UUID, step string, stepErr error) error {
	return h.markIssueDraftStep(ctx, workspaceID, sessionID, step, "failed", "", nil, stepErr.Error())
}

func (h *Handler) failIssueDraftSession(ctx context.Context, workspaceID, sessionID pgtype.UUID, cause error) (issuedraft.SessionBundle, error) {
	_, _ = h.Queries.UpdateIssueDraftSessionStatus(ctx, db.UpdateIssueDraftSessionStatusParams{
		ID:          sessionID,
		WorkspaceID: workspaceID,
		Status:      issuedraft.StatusFailed,
		LastError:   cause.Error(),
	})
	bundle, err := h.IssueDraftService.GetSessionBundle(ctx, workspaceID, sessionID)
	if err != nil {
		return issuedraft.SessionBundle{}, cause
	}
	return bundle, cause
}
