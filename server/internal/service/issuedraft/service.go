package issuedraft

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"github.com/multica-ai/multica/server/pkg/protocol"
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
	memberTasks = s.reconcileMemberTasks(ctx, workspaceID, sessionID, memberTasks)
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

func (s *Service) GetActiveSessionBundle(ctx context.Context, workspaceID pgtype.UUID) (SessionBundle, error) {
	session, err := s.Queries.GetActiveIssueDraftSession(ctx, workspaceID)
	if err != nil {
		return SessionBundle{}, fmt.Errorf("get active issue draft session: %w", err)
	}
	return s.GetSessionBundle(ctx, workspaceID, session.ID)
}

func (s *Service) reconcileMemberTasks(ctx context.Context, workspaceID, sessionID pgtype.UUID, memberTasks []db.IssueDraftMemberTask) []db.IssueDraftMemberTask {
	for i, memberTask := range memberTasks {
		if memberTask.Status != "queued" && memberTask.Status != "running" {
			continue
		}
		if !memberTask.TaskID.Valid {
			continue
		}
		task, err := s.Queries.GetAgentTask(ctx, memberTask.TaskID)
		if err != nil {
			continue
		}
		switch task.Status {
		case "running":
			updated, err := s.Queries.UpdateIssueDraftMemberTaskStatus(ctx, db.UpdateIssueDraftMemberTaskStatusParams{
				ID:          memberTask.ID,
				WorkspaceID: workspaceID,
				Status:      "running",
				Findings:    memberTask.Findings,
				Error:       "",
			})
			if err == nil {
				memberTasks[i] = updated
			}
		case "completed":
			body := issueDraftTaskOutput(task.Result)
			if body == "" {
				body = "澄清任务已完成，但没有返回可展示内容。"
			}
			updated, err := s.Queries.UpdateIssueDraftMemberTaskStatus(ctx, db.UpdateIssueDraftMemberTaskStatusParams{
				ID:          memberTask.ID,
				WorkspaceID: workspaceID,
				Status:      "completed",
				Findings:    body,
				Error:       "",
			})
			if err == nil {
				memberTasks[i] = updated
				_, _ = s.Queries.AppendIssueDraftMessage(ctx, db.AppendIssueDraftMessageParams{
					SessionID:   sessionID,
					WorkspaceID: workspaceID,
					AuthorType:  AuthorAgent,
					AuthorID:    task.AgentID,
					MessageType: MessageFinding,
					Content:     body,
					Metadata:    issueDraftTaskMetadata(task),
				})
				_, _ = s.Queries.UpdateIssueDraftSessionStatus(ctx, db.UpdateIssueDraftSessionStatusParams{
					ID:          sessionID,
					WorkspaceID: workspaceID,
					Status:      StatusClarifying,
					LastError:   "",
				})
			}
		case "failed", "cancelled":
			errMsg := strings.TrimSpace(task.Error.String)
			if errMsg == "" {
				errMsg = "澄清任务失败，但没有返回错误详情。"
			}
			updated, err := s.Queries.UpdateIssueDraftMemberTaskStatus(ctx, db.UpdateIssueDraftMemberTaskStatusParams{
				ID:          memberTask.ID,
				WorkspaceID: workspaceID,
				Status:      "failed",
				Findings:    "",
				Error:       errMsg,
			})
			if err == nil {
				memberTasks[i] = updated
				_, _ = s.Queries.AppendIssueDraftMessage(ctx, db.AppendIssueDraftMessageParams{
					SessionID:   sessionID,
					WorkspaceID: workspaceID,
					AuthorType:  AuthorAgent,
					AuthorID:    task.AgentID,
					MessageType: MessageError,
					Content:     errMsg,
					Metadata:    issueDraftTaskMetadata(task),
				})
				_, _ = s.Queries.UpdateIssueDraftSessionStatus(ctx, db.UpdateIssueDraftSessionStatusParams{
					ID:          sessionID,
					WorkspaceID: workspaceID,
					Status:      StatusClarifying,
					LastError:   errMsg,
				})
			}
		}
	}
	return memberTasks
}

func issueDraftTaskOutput(result []byte) string {
	var payload protocol.TaskCompletedPayload
	if err := json.Unmarshal(result, &payload); err != nil {
		return ""
	}
	return strings.TrimSpace(util.UnescapeBackslashEscapes(payload.Output))
}

func issueDraftTaskMetadata(task db.AgentTaskQueue) []byte {
	metadata, err := json.Marshal(map[string]string{
		"task_id":  util.UUIDToString(task.ID),
		"agent_id": util.UUIDToString(task.AgentID),
	})
	if err != nil {
		return nil
	}
	return metadata
}

func (s *Service) AppendMessage(ctx context.Context, in AppendMessageInput) (db.IssueDraftMessage, error) {
	if _, err := s.Queries.GetIssueDraftSessionInWorkspace(ctx, db.GetIssueDraftSessionInWorkspaceParams{
		ID:          in.SessionID,
		WorkspaceID: in.WorkspaceID,
	}); err != nil {
		return db.IssueDraftMessage{}, fmt.Errorf("get issue draft session: %w", err)
	}
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

func (s *Service) GenerateArtifacts(ctx context.Context, in GenerateArtifactsInput) (SessionBundle, error) {
	session, err := s.Queries.GetIssueDraftSessionInWorkspace(ctx, db.GetIssueDraftSessionInWorkspaceParams{
		ID:          in.SessionID,
		WorkspaceID: in.WorkspaceID,
	})
	if err != nil {
		return SessionBundle{}, fmt.Errorf("get issue draft session: %w", err)
	}
	messages, err := s.Queries.ListIssueDraftMessages(ctx, db.ListIssueDraftMessagesParams{
		SessionID:   in.SessionID,
		WorkspaceID: in.WorkspaceID,
	})
	if err != nil {
		return SessionBundle{}, fmt.Errorf("list issue draft messages: %w", err)
	}
	memberTasks, err := s.Queries.ListIssueDraftMemberTasks(ctx, db.ListIssueDraftMemberTasksParams{
		SessionID:   in.SessionID,
		WorkspaceID: in.WorkspaceID,
	})
	if err != nil {
		return SessionBundle{}, fmt.Errorf("list issue draft member tasks: %w", err)
	}

	template := buildTemplateInput(session, messages, memberTasks)
	detailed := RenderDetailedSpec(template)
	multicaIssue := RenderMulticaIssue(template)
	remoteIssue := RenderRemoteIssue(template)

	for _, artifact := range []struct {
		kind    string
		content string
	}{
		{ArtifactDetailedSpec, detailed},
		{ArtifactMulticaIssue, multicaIssue},
		{ArtifactRemoteIssue, remoteIssue},
	} {
		if _, err := s.createArtifact(ctx, in.WorkspaceID, in.SessionID, artifact.kind, artifact.content, in.GeneratedBy); err != nil {
			return SessionBundle{}, err
		}
	}
	if _, err := s.Queries.UpdateIssueDraftSessionStatus(ctx, db.UpdateIssueDraftSessionStatusParams{
		ID:          in.SessionID,
		WorkspaceID: in.WorkspaceID,
		Status:      StatusReadyForReview,
		LastError:   "",
	}); err != nil {
		return SessionBundle{}, fmt.Errorf("mark issue draft ready: %w", err)
	}
	if _, err := s.AppendMessage(ctx, AppendMessageInput{
		SessionID:   in.SessionID,
		WorkspaceID: in.WorkspaceID,
		AuthorType:  AuthorSystem,
		MessageType: MessageDraftPreview,
		Content:     "已生成详细 .spec 草稿、Multica issue 草稿和远端 issue 简版草稿，请确认后再执行创建。",
	}); err != nil {
		return SessionBundle{}, err
	}
	return s.GetSessionBundle(ctx, in.WorkspaceID, in.SessionID)
}

func (s *Service) Confirm(ctx context.Context, in ConfirmInput) (SessionBundle, error) {
	if _, err := s.requireLatestArtifact(ctx, in.WorkspaceID, in.SessionID, ArtifactDetailedSpec); err != nil {
		return SessionBundle{}, err
	}
	if _, err := s.requireLatestArtifact(ctx, in.WorkspaceID, in.SessionID, ArtifactRemoteIssue); err != nil {
		return SessionBundle{}, err
	}
	for _, step := range []string{"write_spec", "create_multica_issue", "create_remote_issue", "commit_and_push", "link_outputs"} {
		if _, err := s.Queries.UpsertIssueDraftConfirmStep(ctx, db.UpsertIssueDraftConfirmStepParams{
			SessionID:      in.SessionID,
			WorkspaceID:    in.WorkspaceID,
			Step:           step,
			Status:         "pending",
			ResultMetadata: []byte(`{}`),
		}); err != nil {
			return SessionBundle{}, fmt.Errorf("upsert confirm step %s: %w", step, err)
		}
	}
	if _, err := s.Queries.UpdateIssueDraftSessionStatus(ctx, db.UpdateIssueDraftSessionStatusParams{
		ID:          in.SessionID,
		WorkspaceID: in.WorkspaceID,
		Status:      StatusCreating,
		LastError:   "",
	}); err != nil {
		return SessionBundle{}, fmt.Errorf("mark issue draft creating: %w", err)
	}
	if _, err := s.AppendMessage(ctx, AppendMessageInput{
		SessionID:   in.SessionID,
		WorkspaceID: in.WorkspaceID,
		AuthorType:  AuthorSystem,
		MessageType: MessageStatus,
		Content:     "确认已接收，执行步骤已排队：写入目标仓库 .spec、创建 Multica issue、创建远端 issue、提交并推送目标仓库、回填关联信息。",
	}); err != nil {
		return SessionBundle{}, err
	}
	return s.GetSessionBundle(ctx, in.WorkspaceID, in.SessionID)
}

func (s *Service) Cancel(ctx context.Context, in CancelInput) (SessionBundle, error) {
	if _, err := s.Queries.UpdateIssueDraftSessionStatus(ctx, db.UpdateIssueDraftSessionStatusParams{
		ID:          in.SessionID,
		WorkspaceID: in.WorkspaceID,
		Status:      StatusCancelled,
		LastError:   "",
	}); err != nil {
		return SessionBundle{}, fmt.Errorf("cancel issue draft: %w", err)
	}
	content := "已取消本次 issue 草稿。"
	if strings.TrimSpace(in.Reason) != "" {
		content += "\n\n原因: " + strings.TrimSpace(in.Reason)
	}
	if _, err := s.AppendMessage(ctx, AppendMessageInput{
		SessionID:   in.SessionID,
		WorkspaceID: in.WorkspaceID,
		AuthorType:  AuthorSystem,
		MessageType: MessageStatus,
		Content:     content,
	}); err != nil {
		return SessionBundle{}, err
	}
	return s.GetSessionBundle(ctx, in.WorkspaceID, in.SessionID)
}

func (s *Service) createArtifact(ctx context.Context, workspaceID, sessionID pgtype.UUID, kind, content string, generatedBy pgtype.UUID) (db.IssueDraftArtifact, error) {
	revision, err := s.Queries.GetNextIssueDraftArtifactRevision(ctx, db.GetNextIssueDraftArtifactRevisionParams{
		SessionID:    sessionID,
		WorkspaceID:  workspaceID,
		ArtifactType: kind,
	})
	if err != nil {
		return db.IssueDraftArtifact{}, fmt.Errorf("get next artifact revision: %w", err)
	}
	artifact, err := s.Queries.CreateIssueDraftArtifact(ctx, db.CreateIssueDraftArtifactParams{
		SessionID:          sessionID,
		WorkspaceID:        workspaceID,
		ArtifactType:       kind,
		Revision:           revision,
		Content:            content,
		GeneratedByAgentID: generatedBy,
	})
	if err != nil {
		return db.IssueDraftArtifact{}, fmt.Errorf("create issue draft artifact: %w", err)
	}
	return artifact, nil
}

func (s *Service) requireLatestArtifact(ctx context.Context, workspaceID, sessionID pgtype.UUID, kind string) (db.IssueDraftArtifact, error) {
	artifact, err := s.Queries.GetLatestIssueDraftArtifact(ctx, db.GetLatestIssueDraftArtifactParams{
		SessionID:    sessionID,
		WorkspaceID:  workspaceID,
		ArtifactType: kind,
	})
	if err != nil {
		return db.IssueDraftArtifact{}, fmt.Errorf("latest %s artifact is required: %w", kind, err)
	}
	return artifact, nil
}

func buildTemplateInput(session db.IssueDraftSession, messages []db.IssueDraftMessage, memberTasks []db.IssueDraftMemberTask) TemplateInput {
	userText := collectMessages(messages, AuthorMember)
	findings := collectFindings(memberTasks)
	title := firstNonEmptyLine(userText)
	if title == "" {
		title = "新建 issue 需求草稿"
	}
	background := "用户原始需求:\n\n" + fallback(userText, "TBD")
	if findings != "" {
		background += "\n\n小队成员发现:\n\n" + findings
	}
	specPathHint := strings.TrimRight(session.PrimaryLocalPathSnapshot, "/") + "/.spec/"
	goal := "根据用户需求交付: " + fallback(strings.TrimPrefix(userText, "- "), title)
	return TemplateInput{
		Title:                title,
		Background:           background,
		DuplicateSearch:      "待 PM Agent 基于现有 issue / .spec 记忆完成查重；当前草稿保留查重结论入口。",
		SplitDecision:        "默认按一个独立需求处理；若小队追问后发现存在可并行交付的子目标，再拆分为父子 issue。",
		Goal:                 goal,
		NonGoal:              "不在远端 issue 中写入详细实施计划；不允许澄清阶段任务修改代码、提交 git 或创建外部资源。",
		ImpactScope:          fmt.Sprintf("目标项目主仓库: `%s`\n详细版本写入: `%s`", session.PrimaryLocalPathSnapshot, specPathHint),
		UserScenario:         "用户提出需求后，系统进入澄清会话，成员根据技能补充问题/代码证据，最终生成可确认的标准模板。\n\n用户需求摘要:\n\n" + fallback(userText, "TBD"),
		AcceptanceCriteria:   []string{"满足用户原始需求: " + title, "必要的影响范围、验收标准、技术约束已被澄清", "生成的详细版本写入目标项目 .spec", "远端 issue 和提交信息不包含详细实施计划", "用户确认后才进入创建/同步步骤"},
		TechnicalConstraints: "React Query 管理服务端 issue draft 状态；Zustand 只保存本地选择/草稿 UI 状态；小队成员任务必须使用 issue_draft 只读上下文。",
		DesignRecord:         "草稿会话持久化为 issue_draft_session；成员任务、对话消息、artifact、确认步骤分别记录，便于审计与重试。",
		ImplementationPlan:   "1. 创建 issue draft session 并绑定项目主仓库 local_directory resource。\n2. 按小队成员创建只读 member task，并下发包含真实技能/仓库路径/会话上下文的 prompt。\n3. 汇总对话与 findings，生成 detailed_spec / multica_issue / remote_issue 三类 artifact。\n4. 用户确认后，详细 spec 写入目标仓库 .spec，远端 issue 使用简版正文。\n5. 执行创建 issue、提交并推送目标仓库，记录每个确认步骤结果。",
		VerificationPlan:     "go test ./internal/service/issuedraft ./internal/handler\npnpm --filter @multica/core typecheck\npnpm --filter @multica/views typecheck",
		RisksAndRollback:     "风险: 目标项目缺少 local_directory resource、成员 runtime 离线、远端 issue/git 操作失败。回滚: 保留 artifact 和 confirm step，可取消 session 或重试失败步骤。",
		RelatedInfo:          "由 Multica issue draft flow 自动生成。",
		PMAgent:              "issue draft PM flow",
		ArchitectAgent:       "issue draft ARCH flow",
		PMCreatedAt:          pgTime(session.CreatedAt),
		ArchitectAlignedAt:   time.Now(),
	}
}

func collectMessages(messages []db.IssueDraftMessage, authorType string) string {
	var b strings.Builder
	for _, msg := range messages {
		if msg.AuthorType != authorType && authorType != "" {
			continue
		}
		content := strings.TrimSpace(msg.Content)
		if content == "" {
			continue
		}
		b.WriteString("- ")
		b.WriteString(content)
		b.WriteByte('\n')
	}
	return strings.TrimSpace(b.String())
}

func collectFindings(tasks []db.IssueDraftMemberTask) string {
	var b strings.Builder
	for _, task := range tasks {
		finding := strings.TrimSpace(task.Findings)
		if finding == "" {
			continue
		}
		b.WriteString("- ")
		b.WriteString(finding)
		b.WriteByte('\n')
	}
	return strings.TrimSpace(b.String())
}

func firstNonEmptyLine(value string) string {
	for _, line := range strings.Split(value, "\n") {
		line = strings.Trim(strings.TrimSpace(line), "-# ")
		if line == "" {
			continue
		}
		if len([]rune(line)) > 80 {
			runes := []rune(line)
			return string(runes[:80])
		}
		return line
	}
	return ""
}

func pgTime(ts pgtype.Timestamptz) time.Time {
	if !ts.Valid {
		return time.Time{}
	}
	return ts.Time
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
