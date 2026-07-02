package handler

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

const (
	agentLearningScopePersonalAgent          = "personal_agent"
	agentLearningScopeWorkspaceSkill         = "workspace_skill"
	agentLearningScopeBuiltinSkillCandidate  = "builtin_skill_candidate"
	agentLearningRiskSafe                    = "safe"
	agentLearningRiskReview                  = "review"
	agentLearningRiskManual                  = "manual"
	agentLearningStatusPending               = "pending"
	agentLearningTargetAgent                 = "agent"
	agentLearningTargetSkill                 = "skill"
	agentLearningTargetBuiltinSkill          = "builtin_skill"
	agentEvolutionContentSectionTitle        = "Agent Evolution"
	agentEvolutionApplicationContentMaxBytes = 512 * 1024
)

var errAgentEvolutionResponseWritten = errors.New("agent evolution response written")

type AgentLearningReportResponse struct {
	ID          string                             `json:"id"`
	WorkspaceID string                             `json:"workspace_id"`
	IssueID     *string                            `json:"issue_id"`
	TaskID      *string                            `json:"task_id"`
	AgentID     string                             `json:"agent_id"`
	Summary     string                             `json:"summary"`
	Metadata    any                                `json:"metadata"`
	Suggestions []AgentEvolutionSuggestionResponse `json:"suggestions"`
	CreatedAt   string                             `json:"created_at"`
	UpdatedAt   string                             `json:"updated_at"`
}

type AgentEvolutionSuggestionResponse struct {
	ID              string                              `json:"id"`
	ReportID        string                              `json:"report_id"`
	WorkspaceID     string                              `json:"workspace_id"`
	Scope           string                              `json:"scope"`
	Risk            string                              `json:"risk"`
	Status          string                              `json:"status"`
	TargetType      string                              `json:"target_type"`
	TargetID        *string                             `json:"target_id"`
	Title           string                              `json:"title"`
	Rationale       string                              `json:"rationale"`
	ProposedContent string                              `json:"proposed_content"`
	Metadata        any                                 `json:"metadata"`
	Applications    []AgentEvolutionApplicationResponse `json:"applications"`
	CreatedAt       string                              `json:"created_at"`
	UpdatedAt       string                              `json:"updated_at"`
	AppliedAt       *string                             `json:"applied_at"`
	DismissedAt     *string                             `json:"dismissed_at"`
}

type AgentEvolutionApplicationResponse struct {
	ID            string  `json:"id"`
	SuggestionID  string  `json:"suggestion_id"`
	WorkspaceID   string  `json:"workspace_id"`
	AppliedBy     *string `json:"applied_by"`
	TargetType    string  `json:"target_type"`
	TargetID      string  `json:"target_id"`
	BeforeContent string  `json:"before_content"`
	AfterContent  string  `json:"after_content"`
	CreatedAt     string  `json:"created_at"`
}

type CreateAgentLearningReportRequest struct {
	WorkspaceID string                                  `json:"workspace_id"`
	IssueID     *string                                 `json:"issue_id"`
	TaskID      *string                                 `json:"task_id"`
	AgentID     string                                  `json:"agent_id"`
	Summary     string                                  `json:"summary"`
	Metadata    any                                     `json:"metadata"`
	Suggestions []CreateAgentEvolutionSuggestionRequest `json:"suggestions"`
}

type CreateAgentEvolutionSuggestionRequest struct {
	Scope           string `json:"scope"`
	Risk            string `json:"risk"`
	TargetType      string `json:"target_type"`
	TargetID        string `json:"target_id"`
	Title           string `json:"title"`
	Rationale       string `json:"rationale"`
	ProposedContent string `json:"proposed_content"`
	Metadata        any    `json:"metadata"`
}

func agentLearningReportToResponse(report db.AgentLearningReport, suggestions []AgentEvolutionSuggestionResponse) AgentLearningReportResponse {
	return AgentLearningReportResponse{
		ID:          uuidToString(report.ID),
		WorkspaceID: uuidToString(report.WorkspaceID),
		IssueID:     uuidToPtr(report.IssueID),
		TaskID:      uuidToPtr(report.TaskID),
		AgentID:     uuidToString(report.AgentID),
		Summary:     report.Summary,
		Metadata:    decodeJSONBObject(report.Metadata),
		Suggestions: suggestions,
		CreatedAt:   timestampToString(report.CreatedAt),
		UpdatedAt:   timestampToString(report.UpdatedAt),
	}
}

func agentEvolutionSuggestionToResponse(s db.AgentEvolutionSuggestion, applications []AgentEvolutionApplicationResponse) AgentEvolutionSuggestionResponse {
	return AgentEvolutionSuggestionResponse{
		ID:              uuidToString(s.ID),
		ReportID:        uuidToString(s.ReportID),
		WorkspaceID:     uuidToString(s.WorkspaceID),
		Scope:           s.Scope,
		Risk:            s.Risk,
		Status:          s.Status,
		TargetType:      s.TargetType,
		TargetID:        uuidToPtr(s.TargetID),
		Title:           s.Title,
		Rationale:       s.Rationale,
		ProposedContent: s.ProposedContent,
		Metadata:        decodeJSONBObject(s.Metadata),
		Applications:    applications,
		CreatedAt:       timestampToString(s.CreatedAt),
		UpdatedAt:       timestampToString(s.UpdatedAt),
		AppliedAt:       timestampToPtr(s.AppliedAt),
		DismissedAt:     timestampToPtr(s.DismissedAt),
	}
}

func agentEvolutionApplicationToResponse(a db.AgentEvolutionApplication) AgentEvolutionApplicationResponse {
	return AgentEvolutionApplicationResponse{
		ID:            uuidToString(a.ID),
		SuggestionID:  uuidToString(a.SuggestionID),
		WorkspaceID:   uuidToString(a.WorkspaceID),
		AppliedBy:     uuidToPtr(a.AppliedBy),
		TargetType:    a.TargetType,
		TargetID:      uuidToString(a.TargetID),
		BeforeContent: a.BeforeContent,
		AfterContent:  a.AfterContent,
		CreatedAt:     timestampToString(a.CreatedAt),
	}
}

func decodeJSONBObject(raw []byte) any {
	var value any
	if len(raw) > 0 {
		_ = json.Unmarshal(raw, &value)
	}
	if value == nil {
		return map[string]any{}
	}
	return value
}

func agentLearningNullableUUID(w http.ResponseWriter, raw *string, fieldName string) (pgtype.UUID, bool) {
	if raw == nil || strings.TrimSpace(*raw) == "" {
		return pgtype.UUID{}, true
	}
	return parseUUIDOrBadRequest(w, *raw, fieldName)
}

func jsonMetadataBytes(value any) []byte {
	if value == nil {
		return []byte(`{}`)
	}
	b, err := json.Marshal(value)
	if err != nil || len(b) == 0 || string(b) == "null" {
		return []byte(`{}`)
	}
	return b
}

func validAgentLearningScope(scope string) bool {
	switch scope {
	case agentLearningScopePersonalAgent, agentLearningScopeWorkspaceSkill, agentLearningScopeBuiltinSkillCandidate:
		return true
	default:
		return false
	}
}

func validAgentLearningRisk(risk string) bool {
	switch risk {
	case agentLearningRiskSafe, agentLearningRiskReview, agentLearningRiskManual:
		return true
	default:
		return false
	}
}

func targetTypeForScope(scope string) string {
	switch scope {
	case agentLearningScopePersonalAgent:
		return agentLearningTargetAgent
	case agentLearningScopeWorkspaceSkill:
		return agentLearningTargetSkill
	case agentLearningScopeBuiltinSkillCandidate:
		return agentLearningTargetBuiltinSkill
	default:
		return ""
	}
}

func (h *Handler) CreateAgentLearningReport(w http.ResponseWriter, r *http.Request) {
	workspaceID := h.resolveWorkspaceID(r)
	var req CreateAgentLearningReportRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if workspaceID == "" {
		workspaceID = strings.TrimSpace(req.WorkspaceID)
	}
	member, ok := h.requireWorkspaceMember(w, r, workspaceID, "workspace not found")
	if !ok {
		return
	}
	workspaceUUID, ok := parseUUIDOrBadRequest(w, workspaceID, "workspace_id")
	if !ok {
		return
	}
	agentUUID, ok := parseUUIDOrBadRequest(w, req.AgentID, "agent_id")
	if !ok {
		return
	}
	agentRow, err := h.Queries.GetAgentInWorkspace(r.Context(), db.GetAgentInWorkspaceParams{ID: agentUUID, WorkspaceID: workspaceUUID})
	if err != nil {
		writeError(w, http.StatusNotFound, "agent not found")
		return
	}
	issueUUID, ok := agentLearningNullableUUID(w, req.IssueID, "issue_id")
	if !ok {
		return
	}
	if issueUUID.Valid {
		if _, err := h.Queries.GetIssueInWorkspace(r.Context(), db.GetIssueInWorkspaceParams{ID: issueUUID, WorkspaceID: workspaceUUID}); err != nil {
			writeError(w, http.StatusNotFound, "issue not found")
			return
		}
	}
	taskUUID, ok := agentLearningNullableUUID(w, req.TaskID, "task_id")
	if !ok {
		return
	}
	if taskUUID.Valid {
		if _, err := h.Queries.GetAgentTaskInWorkspace(r.Context(), db.GetAgentTaskInWorkspaceParams{ID: taskUUID, WorkspaceID: workspaceUUID}); err != nil {
			writeError(w, http.StatusNotFound, "task not found")
			return
		}
	}

	tx, err := h.TxStarter.Begin(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to start transaction")
		return
	}
	defer tx.Rollback(r.Context())
	qtx := h.Queries.WithTx(tx)

	report, err := qtx.CreateAgentLearningReport(r.Context(), db.CreateAgentLearningReportParams{
		WorkspaceID: workspaceUUID,
		IssueID:     issueUUID,
		TaskID:      taskUUID,
		AgentID:     agentUUID,
		Summary:     sanitizeNullBytes(req.Summary),
		Metadata:    jsonMetadataBytes(req.Metadata),
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create learning report")
		return
	}

	for _, draft := range req.Suggestions {
		created, ok := h.createAgentEvolutionSuggestionFromRequest(w, r, qtx, workspaceUUID, report.ID, draft)
		if !ok {
			return
		}
		if shouldAutoApplyEvolutionSuggestion(agentRow, created) {
			if _, err := h.applyEvolutionSuggestionInTx(w, r, qtx, member, created); err != nil {
				if errors.Is(err, errAgentEvolutionResponseWritten) {
					return
				}
				writeError(w, http.StatusInternalServerError, "failed to auto-apply evolution suggestion")
				return
			}
		}
	}
	if err := tx.Commit(r.Context()); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to commit learning report")
		return
	}

	resp, err := h.learningReportWithSuggestions(r, workspaceUUID, report)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load learning report")
		return
	}
	writeJSON(w, http.StatusCreated, resp)
}

func shouldAutoApplyEvolutionSuggestion(agent db.Agent, suggestion db.AgentEvolutionSuggestion) bool {
	return agent.AgentEvolutionEnabled &&
		suggestion.Scope == agentLearningScopePersonalAgent &&
		suggestion.Risk == agentLearningRiskSafe &&
		suggestion.Status == agentLearningStatusPending &&
		uuidToString(suggestion.TargetID) == uuidToString(agent.ID)
}

func (h *Handler) createAgentEvolutionSuggestionFromRequest(
	w http.ResponseWriter,
	r *http.Request,
	qtx *db.Queries,
	workspaceUUID pgtype.UUID,
	reportID pgtype.UUID,
	req CreateAgentEvolutionSuggestionRequest,
) (db.AgentEvolutionSuggestion, bool) {
	scope := strings.TrimSpace(req.Scope)
	if !validAgentLearningScope(scope) {
		writeError(w, http.StatusBadRequest, "invalid suggestion scope")
		return db.AgentEvolutionSuggestion{}, false
	}
	risk := strings.TrimSpace(req.Risk)
	if risk == "" {
		risk = agentLearningRiskReview
	}
	if !validAgentLearningRisk(risk) {
		writeError(w, http.StatusBadRequest, "invalid suggestion risk")
		return db.AgentEvolutionSuggestion{}, false
	}
	targetType := strings.TrimSpace(req.TargetType)
	if targetType == "" {
		targetType = targetTypeForScope(scope)
	}
	if targetType != targetTypeForScope(scope) {
		writeError(w, http.StatusBadRequest, "target_type does not match suggestion scope")
		return db.AgentEvolutionSuggestion{}, false
	}
	var targetID pgtype.UUID
	if targetType != agentLearningTargetBuiltinSkill {
		parsed, ok := parseUUIDOrBadRequest(w, req.TargetID, "target_id")
		if !ok {
			return db.AgentEvolutionSuggestion{}, false
		}
		targetID = parsed
		if !h.validateEvolutionTarget(r, workspaceUUID, targetType, targetID) {
			writeError(w, http.StatusNotFound, "target not found")
			return db.AgentEvolutionSuggestion{}, false
		}
	}
	created, err := qtx.CreateAgentEvolutionSuggestion(r.Context(), db.CreateAgentEvolutionSuggestionParams{
		ReportID:        reportID,
		WorkspaceID:     workspaceUUID,
		Scope:           scope,
		Risk:            risk,
		Status:          agentLearningStatusPending,
		TargetType:      targetType,
		TargetID:        targetID,
		Title:           sanitizeNullBytes(strings.TrimSpace(req.Title)),
		Rationale:       sanitizeNullBytes(strings.TrimSpace(req.Rationale)),
		ProposedContent: sanitizeNullBytes(strings.TrimSpace(req.ProposedContent)),
		Metadata:        jsonMetadataBytes(req.Metadata),
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create evolution suggestion")
		return db.AgentEvolutionSuggestion{}, false
	}
	return created, true
}

func (h *Handler) validateEvolutionTarget(r *http.Request, workspaceID pgtype.UUID, targetType string, targetID pgtype.UUID) bool {
	switch targetType {
	case agentLearningTargetAgent:
		_, err := h.Queries.GetAgentInWorkspace(r.Context(), db.GetAgentInWorkspaceParams{ID: targetID, WorkspaceID: workspaceID})
		return err == nil
	case agentLearningTargetSkill:
		_, err := h.Queries.GetSkillInWorkspace(r.Context(), db.GetSkillInWorkspaceParams{ID: targetID, WorkspaceID: workspaceID})
		return err == nil
	default:
		return false
	}
}

func (h *Handler) ListIssueLearningReports(w http.ResponseWriter, r *http.Request) {
	issue, ok := h.loadIssueForUser(w, r, chi.URLParam(r, "id"))
	if !ok {
		return
	}
	reports, err := h.Queries.ListAgentLearningReportsByIssue(r.Context(), db.ListAgentLearningReportsByIssueParams{
		WorkspaceID: issue.WorkspaceID,
		IssueID:     issue.ID,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list learning reports")
		return
	}
	h.writeLearningReportList(w, r, issue.WorkspaceID, reports)
}

func (h *Handler) ListAgentLearningReports(w http.ResponseWriter, r *http.Request) {
	agent, ok := h.loadAgentForUser(w, r, chi.URLParam(r, "id"))
	if !ok {
		return
	}
	reports, err := h.Queries.ListAgentLearningReportsByAgent(r.Context(), db.ListAgentLearningReportsByAgentParams{
		WorkspaceID: agent.WorkspaceID,
		AgentID:     agent.ID,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list learning reports")
		return
	}
	h.writeLearningReportList(w, r, agent.WorkspaceID, reports)
}

func (h *Handler) GetAgentLearningReport(w http.ResponseWriter, r *http.Request) {
	workspaceID := h.resolveWorkspaceID(r)
	if _, ok := h.requireWorkspaceMember(w, r, workspaceID, "workspace not found"); !ok {
		return
	}
	workspaceUUID, ok := parseUUIDOrBadRequest(w, workspaceID, "workspace_id")
	if !ok {
		return
	}
	reportID, ok := parseUUIDOrBadRequest(w, chi.URLParam(r, "id"), "report id")
	if !ok {
		return
	}
	report, err := h.Queries.GetAgentLearningReportInWorkspace(r.Context(), db.GetAgentLearningReportInWorkspaceParams{
		ID:          reportID,
		WorkspaceID: workspaceUUID,
	})
	if err != nil {
		writeError(w, http.StatusNotFound, "learning report not found")
		return
	}
	resp, err := h.learningReportWithSuggestions(r, workspaceUUID, report)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load learning report")
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

func (h *Handler) writeLearningReportList(w http.ResponseWriter, r *http.Request, workspaceID pgtype.UUID, reports []db.AgentLearningReport) {
	resp := make([]AgentLearningReportResponse, 0, len(reports))
	for _, report := range reports {
		item, err := h.learningReportWithSuggestions(r, workspaceID, report)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to load learning reports")
			return
		}
		resp = append(resp, item)
	}
	writeJSON(w, http.StatusOK, resp)
}

func (h *Handler) learningReportWithSuggestions(r *http.Request, workspaceID pgtype.UUID, report db.AgentLearningReport) (AgentLearningReportResponse, error) {
	rows, err := h.Queries.ListAgentEvolutionSuggestionsByReport(r.Context(), db.ListAgentEvolutionSuggestionsByReportParams{
		WorkspaceID: workspaceID,
		ReportID:    report.ID,
	})
	if err != nil {
		return AgentLearningReportResponse{}, err
	}
	suggestions := make([]AgentEvolutionSuggestionResponse, 0, len(rows))
	for _, row := range rows {
		appRows, err := h.Queries.ListAgentEvolutionApplicationsBySuggestion(r.Context(), db.ListAgentEvolutionApplicationsBySuggestionParams{
			WorkspaceID:  workspaceID,
			SuggestionID: row.ID,
		})
		if err != nil {
			return AgentLearningReportResponse{}, err
		}
		apps := make([]AgentEvolutionApplicationResponse, 0, len(appRows))
		for _, app := range appRows {
			apps = append(apps, agentEvolutionApplicationToResponse(app))
		}
		suggestions = append(suggestions, agentEvolutionSuggestionToResponse(row, apps))
	}
	return agentLearningReportToResponse(report, suggestions), nil
}

func (h *Handler) ApplyAgentEvolutionSuggestion(w http.ResponseWriter, r *http.Request) {
	workspaceID := h.resolveWorkspaceID(r)
	member, ok := h.requireWorkspaceMember(w, r, workspaceID, "workspace not found")
	if !ok {
		return
	}
	workspaceUUID, ok := parseUUIDOrBadRequest(w, workspaceID, "workspace_id")
	if !ok {
		return
	}
	suggestionID, ok := parseUUIDOrBadRequest(w, chi.URLParam(r, "id"), "suggestion id")
	if !ok {
		return
	}
	suggestion, err := h.Queries.GetAgentEvolutionSuggestionInWorkspace(r.Context(), db.GetAgentEvolutionSuggestionInWorkspaceParams{
		ID:          suggestionID,
		WorkspaceID: workspaceUUID,
	})
	if err != nil {
		writeError(w, http.StatusNotFound, "suggestion not found")
		return
	}
	if suggestion.Status != agentLearningStatusPending {
		writeError(w, http.StatusConflict, "suggestion is not pending")
		return
	}
	if suggestion.Risk == agentLearningRiskManual {
		writeError(w, http.StatusBadRequest, "manual risk suggestions cannot be applied in v1")
		return
	}
	if suggestion.Scope == agentLearningScopeBuiltinSkillCandidate {
		writeError(w, http.StatusBadRequest, "builtin skill candidates are recorded only in v1")
		return
	}
	if !suggestion.TargetID.Valid {
		writeError(w, http.StatusBadRequest, "suggestion target is required")
		return
	}
	if strings.TrimSpace(suggestion.ProposedContent) == "" {
		writeError(w, http.StatusBadRequest, "suggestion proposed_content is required")
		return
	}

	tx, err := h.TxStarter.Begin(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to start transaction")
		return
	}
	defer tx.Rollback(r.Context())
	qtx := h.Queries.WithTx(tx)

	app, err := h.applyEvolutionSuggestionInTx(w, r, qtx, member, suggestion)
	if err != nil {
		if errors.Is(err, errAgentEvolutionResponseWritten) {
			return
		}
		if errors.Is(err, pgx.ErrNoRows) {
			writeError(w, http.StatusConflict, "suggestion is not pending")
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if err := tx.Commit(r.Context()); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to commit evolution suggestion")
		return
	}
	writeJSON(w, http.StatusOK, agentEvolutionApplicationToResponse(app))
}

func (h *Handler) applyEvolutionSuggestionInTx(
	w http.ResponseWriter,
	r *http.Request,
	qtx *db.Queries,
	member db.Member,
	suggestion db.AgentEvolutionSuggestion,
) (db.AgentEvolutionApplication, error) {
	switch suggestion.Scope {
	case agentLearningScopePersonalAgent:
		agent, err := qtx.GetAgentInWorkspace(r.Context(), db.GetAgentInWorkspaceParams{
			ID:          suggestion.TargetID,
			WorkspaceID: suggestion.WorkspaceID,
		})
		if err != nil {
			writeError(w, http.StatusNotFound, "target agent not found")
			return db.AgentEvolutionApplication{}, errAgentEvolutionResponseWritten
		}
		if !roleAllowed(member.Role, "owner", "admin") && uuidToString(agent.OwnerID) != requestUserID(r) {
			writeError(w, http.StatusForbidden, "only the agent owner or workspace admin can evolve this agent")
			return db.AgentEvolutionApplication{}, errAgentEvolutionResponseWritten
		}
		before := agent.Instructions
		after, err := appendEvolutionContent(before, suggestion)
		if err != nil {
			return db.AgentEvolutionApplication{}, err
		}
		if _, err := qtx.UpdateAgent(r.Context(), db.UpdateAgentParams{
			ID:           agent.ID,
			Instructions: pgtype.Text{String: after, Valid: true},
		}); err != nil {
			return db.AgentEvolutionApplication{}, fmt.Errorf("failed to update agent instructions")
		}
		if _, err := qtx.MarkAgentEvolutionSuggestionApplied(r.Context(), db.MarkAgentEvolutionSuggestionAppliedParams{
			ID:          suggestion.ID,
			WorkspaceID: suggestion.WorkspaceID,
		}); err != nil {
			return db.AgentEvolutionApplication{}, err
		}
		return qtx.CreateAgentEvolutionApplication(r.Context(), db.CreateAgentEvolutionApplicationParams{
			SuggestionID:  suggestion.ID,
			WorkspaceID:   suggestion.WorkspaceID,
			AppliedBy:     member.UserID,
			TargetType:    agentLearningTargetAgent,
			TargetID:      agent.ID,
			BeforeContent: before,
			AfterContent:  after,
		})

	case agentLearningScopeWorkspaceSkill:
		skill, err := qtx.GetSkillInWorkspace(r.Context(), db.GetSkillInWorkspaceParams{
			ID:          suggestion.TargetID,
			WorkspaceID: suggestion.WorkspaceID,
		})
		if err != nil {
			writeError(w, http.StatusNotFound, "target skill not found")
			return db.AgentEvolutionApplication{}, errAgentEvolutionResponseWritten
		}
		if !roleAllowed(member.Role, "owner", "admin") && uuidToString(skill.CreatedBy) != requestUserID(r) {
			writeError(w, http.StatusForbidden, "only the skill creator or workspace admin can evolve this skill")
			return db.AgentEvolutionApplication{}, errAgentEvolutionResponseWritten
		}
		before := skill.Content
		after, err := appendEvolutionContent(before, suggestion)
		if err != nil {
			return db.AgentEvolutionApplication{}, err
		}
		if _, err := qtx.UpdateSkill(r.Context(), db.UpdateSkillParams{
			ID:      skill.ID,
			Content: pgtype.Text{String: after, Valid: true},
		}); err != nil {
			return db.AgentEvolutionApplication{}, fmt.Errorf("failed to update skill content")
		}
		if _, err := qtx.MarkAgentEvolutionSuggestionApplied(r.Context(), db.MarkAgentEvolutionSuggestionAppliedParams{
			ID:          suggestion.ID,
			WorkspaceID: suggestion.WorkspaceID,
		}); err != nil {
			return db.AgentEvolutionApplication{}, err
		}
		return qtx.CreateAgentEvolutionApplication(r.Context(), db.CreateAgentEvolutionApplicationParams{
			SuggestionID:  suggestion.ID,
			WorkspaceID:   suggestion.WorkspaceID,
			AppliedBy:     member.UserID,
			TargetType:    agentLearningTargetSkill,
			TargetID:      skill.ID,
			BeforeContent: before,
			AfterContent:  after,
		})
	default:
		writeError(w, http.StatusBadRequest, "unsupported suggestion scope")
		return db.AgentEvolutionApplication{}, errAgentEvolutionResponseWritten
	}
}

func appendEvolutionContent(existing string, suggestion db.AgentEvolutionSuggestion) (string, error) {
	content := strings.TrimSpace(suggestion.ProposedContent)
	if content == "" {
		return existing, nil
	}
	title := strings.TrimSpace(suggestion.Title)
	if title == "" {
		title = "Learning"
	}
	block := fmt.Sprintf(
		"\n\n## %s\n\n<!-- agent-evolution:suggestion=%s report=%s -->\n### %s\n\n%s\n",
		agentEvolutionContentSectionTitle,
		uuidToString(suggestion.ID),
		uuidToString(suggestion.ReportID),
		title,
		content,
	)
	next := strings.TrimRight(existing, "\n") + block
	if len(next) > agentEvolutionApplicationContentMaxBytes {
		return "", fmt.Errorf("evolved content would exceed size limit")
	}
	return next, nil
}
