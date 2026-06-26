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
	"github.com/multica-ai/multica/server/internal/service/issuebridge"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

type IssueIntegrationResponse struct {
	ID                         string         `json:"id"`
	WorkspaceID                string         `json:"workspace_id"`
	Provider                   string         `json:"provider"`
	Name                       string         `json:"name"`
	BaseURL                    string         `json:"base_url"`
	DefaultIssueSkillID        *string        `json:"default_issue_skill_id"`
	PollingEnabled             bool           `json:"polling_enabled"`
	DefaultPollIntervalSeconds int32          `json:"default_poll_interval_seconds"`
	Config                     map[string]any `json:"config"`
	CreatedAt                  string         `json:"created_at"`
	UpdatedAt                  string         `json:"updated_at"`
}

type CreateGitLabIssueIntegrationRequest struct {
	Name                       string          `json:"name"`
	BaseURL                    string          `json:"base_url"`
	Token                      string          `json:"token"`
	DefaultIssueSkillID        *string         `json:"default_issue_skill_id"`
	PollingEnabled             bool            `json:"polling_enabled"`
	DefaultPollIntervalSeconds int32           `json:"default_poll_interval_seconds"`
	Config                     json.RawMessage `json:"config"`
}

type IssueSyncConfigResponse struct {
	ID                   string         `json:"id"`
	WorkspaceID          string         `json:"workspace_id"`
	IntegrationID        string         `json:"integration_id"`
	ScopeType            string         `json:"scope_type"`
	ScopeID              string         `json:"scope_id"`
	RemoteProjectRef     string         `json:"remote_project_ref"`
	SyncEnabled          bool           `json:"sync_enabled"`
	PollIntervalSeconds  *int32         `json:"poll_interval_seconds"`
	StateMapping         map[string]any `json:"state_mapping"`
	AutoAssignEnabled    bool           `json:"auto_assign_enabled"`
	DefaultAssigneeType  *string        `json:"default_assignee_type"`
	DefaultAssigneeID    *string        `json:"default_assignee_id"`
	LastPollAt           *string        `json:"last_poll_at"`
	LastSuccessfulPollAt *string        `json:"last_successful_poll_at"`
	LastError            string         `json:"last_error"`
	CreatedAt            string         `json:"created_at"`
	UpdatedAt            string         `json:"updated_at"`
}

type upsertIssueSyncConfigRequest struct {
	IntegrationID       string          `json:"integration_id"`
	ScopeType           string          `json:"scope_type"`
	ScopeID             string          `json:"scope_id"`
	RemoteProjectRef    string          `json:"remote_project_ref"`
	SyncEnabled         bool            `json:"sync_enabled"`
	PollIntervalSeconds *int32          `json:"poll_interval_seconds"`
	StateMapping        json.RawMessage `json:"state_mapping"`
	AutoAssignEnabled   bool            `json:"auto_assign_enabled"`
	DefaultAssigneeType *string         `json:"default_assignee_type"`
	DefaultAssigneeID   *string         `json:"default_assignee_id"`
}

func (h *Handler) ListIssueIntegrations(w http.ResponseWriter, r *http.Request) {
	svc, ok := h.issueBridgeService(w, false)
	if !ok {
		return
	}
	workspaceUUID, ok := h.issueBridgeWorkspaceUUID(w, r)
	if !ok {
		return
	}
	rows, err := svc.ListIntegrations(r.Context(), workspaceUUID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list issue integrations")
		return
	}
	resp := make([]IssueIntegrationResponse, len(rows))
	for i, row := range rows {
		resp[i] = issueIntegrationToResponse(row)
	}
	writeJSON(w, http.StatusOK, map[string]any{"integrations": resp, "total": len(resp)})
}

func (h *Handler) CreateGitLabIssueIntegration(w http.ResponseWriter, r *http.Request) {
	svc, ok := h.issueBridgeService(w, true)
	if !ok {
		return
	}
	workspaceUUID, ok := h.issueBridgeWorkspaceUUID(w, r)
	if !ok {
		return
	}
	userID, ok := requireUserID(w, r)
	if !ok {
		return
	}
	userUUID, ok := parseUUIDOrBadRequest(w, userID, "user id")
	if !ok {
		return
	}
	var req CreateGitLabIssueIntegrationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	defaultSkillID, ok := h.defaultIssueSkillIDForCreate(w, r, svc, workspaceUUID, userUUID, req.DefaultIssueSkillID)
	if !ok {
		return
	}
	row, err := svc.CreateGitLabIntegration(r.Context(), issuebridge.SaveIntegrationInput{
		WorkspaceID:                workspaceUUID,
		Name:                       req.Name,
		BaseURL:                    req.BaseURL,
		Token:                      req.Token,
		DefaultIssueSkillID:        defaultSkillID,
		PollingEnabled:             req.PollingEnabled,
		DefaultPollIntervalSeconds: req.DefaultPollIntervalSeconds,
		Config:                     req.Config,
	})
	if err != nil {
		h.writeIssueBridgeWriteError(w, err, "create issue integration")
		return
	}
	writeJSON(w, http.StatusCreated, issueIntegrationToResponse(row))
}

func (h *Handler) UpdateIssueIntegration(w http.ResponseWriter, r *http.Request) {
	svc, ok := h.issueBridgeService(w, false)
	if !ok {
		return
	}
	workspaceUUID, ok := h.issueBridgeWorkspaceUUID(w, r)
	if !ok {
		return
	}
	integrationID, ok := parseUUIDOrBadRequest(w, chi.URLParam(r, "id"), "issue integration id")
	if !ok {
		return
	}
	var req CreateGitLabIssueIntegrationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if strings.TrimSpace(req.Token) != "" && svc.SecretBox == nil {
		writeError(w, http.StatusServiceUnavailable, "issue bridge token storage is not configured")
		return
	}
	defaultSkillID, ok := h.parseOptionalSkillInWorkspace(w, r, workspaceUUID, req.DefaultIssueSkillID)
	if !ok {
		return
	}
	row, err := svc.UpdateGitLabIntegration(r.Context(), integrationID, issuebridge.SaveIntegrationInput{
		WorkspaceID:                workspaceUUID,
		Name:                       req.Name,
		BaseURL:                    req.BaseURL,
		Token:                      req.Token,
		DefaultIssueSkillID:        defaultSkillID,
		PollingEnabled:             req.PollingEnabled,
		DefaultPollIntervalSeconds: req.DefaultPollIntervalSeconds,
		Config:                     req.Config,
	})
	if err != nil {
		h.writeIssueBridgeWriteError(w, err, "update issue integration")
		return
	}
	writeJSON(w, http.StatusOK, issueIntegrationToResponse(row))
}

func (h *Handler) DeleteIssueIntegration(w http.ResponseWriter, r *http.Request) {
	svc, ok := h.issueBridgeService(w, false)
	if !ok {
		return
	}
	workspaceUUID, ok := h.issueBridgeWorkspaceUUID(w, r)
	if !ok {
		return
	}
	integrationID, ok := parseUUIDOrBadRequest(w, chi.URLParam(r, "id"), "issue integration id")
	if !ok {
		return
	}
	if err := svc.DeleteIntegration(r.Context(), workspaceUUID, integrationID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeError(w, http.StatusNotFound, "issue integration not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to delete issue integration")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) TestIssueIntegration(w http.ResponseWriter, r *http.Request) {
	svc, ok := h.issueBridgeService(w, true)
	if !ok {
		return
	}
	workspaceUUID, ok := h.issueBridgeWorkspaceUUID(w, r)
	if !ok {
		return
	}
	integrationID, ok := parseUUIDOrBadRequest(w, chi.URLParam(r, "id"), "issue integration id")
	if !ok {
		return
	}
	integration, err := svc.GetIntegration(r.Context(), workspaceUUID, integrationID)
	if err != nil {
		writeError(w, http.StatusNotFound, "issue integration not found")
		return
	}
	user, err := svc.TestStoredGitLabConnection(r.Context(), integration)
	if err != nil {
		writeError(w, http.StatusBadGateway, "failed to test issue integration: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":       true,
		"provider": integration.Provider,
		"username": user.Username,
		"name":     user.Name,
	})
}

func (h *Handler) ListIssueSyncConfigs(w http.ResponseWriter, r *http.Request) {
	workspaceUUID, ok := h.issueBridgeWorkspaceUUID(w, r)
	if !ok {
		return
	}
	rows, err := h.Queries.ListIssueSyncConfigsByWorkspace(r.Context(), workspaceUUID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list issue sync configs")
		return
	}
	resp := make([]IssueSyncConfigResponse, len(rows))
	for i, row := range rows {
		resp[i] = issueSyncConfigToResponse(row)
	}
	writeJSON(w, http.StatusOK, map[string]any{"sync_configs": resp, "total": len(resp)})
}

func (h *Handler) CreateIssueSyncConfig(w http.ResponseWriter, r *http.Request) {
	workspaceUUID, ok := h.issueBridgeWorkspaceUUID(w, r)
	if !ok {
		return
	}
	var req upsertIssueSyncConfigRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	input, ok := h.normalizeIssueSyncConfigRequest(w, r, workspaceUUID, req, nil)
	if !ok {
		return
	}
	row, err := h.Queries.CreateIssueSyncConfig(r.Context(), db.CreateIssueSyncConfigParams{
		WorkspaceID:         workspaceUUID,
		IntegrationID:       input.integrationID,
		ScopeType:           input.scopeType,
		ScopeID:             input.scopeID,
		RemoteProjectRef:    input.remoteProjectRef,
		SyncEnabled:         input.syncEnabled,
		PollIntervalSeconds: input.pollIntervalSeconds,
		StateMapping:        input.stateMapping,
		AutoAssignEnabled:   input.autoAssignEnabled,
		DefaultAssigneeType: input.defaultAssigneeType,
		DefaultAssigneeID:   input.defaultAssigneeID,
	})
	if err != nil {
		h.writeIssueBridgeWriteError(w, err, "create issue sync config")
		return
	}
	writeJSON(w, http.StatusCreated, issueSyncConfigToResponse(row))
}

func (h *Handler) UpdateIssueSyncConfig(w http.ResponseWriter, r *http.Request) {
	workspaceUUID, ok := h.issueBridgeWorkspaceUUID(w, r)
	if !ok {
		return
	}
	configID, ok := parseUUIDOrBadRequest(w, chi.URLParam(r, "id"), "issue sync config id")
	if !ok {
		return
	}
	existing, err := h.Queries.GetIssueSyncConfigInWorkspace(r.Context(), db.GetIssueSyncConfigInWorkspaceParams{
		ID:          configID,
		WorkspaceID: workspaceUUID,
	})
	if err != nil {
		writeError(w, http.StatusNotFound, "issue sync config not found")
		return
	}
	var req upsertIssueSyncConfigRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	input, ok := h.normalizeIssueSyncConfigRequest(w, r, workspaceUUID, req, &existing)
	if !ok {
		return
	}
	row, err := h.Queries.UpdateIssueSyncConfig(r.Context(), db.UpdateIssueSyncConfigParams{
		ID:                  configID,
		WorkspaceID:         workspaceUUID,
		RemoteProjectRef:    input.remoteProjectRef,
		SyncEnabled:         input.syncEnabled,
		PollIntervalSeconds: input.pollIntervalSeconds,
		StateMapping:        input.stateMapping,
		AutoAssignEnabled:   input.autoAssignEnabled,
		DefaultAssigneeType: input.defaultAssigneeType,
		DefaultAssigneeID:   input.defaultAssigneeID,
	})
	if err != nil {
		h.writeIssueBridgeWriteError(w, err, "update issue sync config")
		return
	}
	writeJSON(w, http.StatusOK, issueSyncConfigToResponse(row))
}

func (h *Handler) DeleteIssueSyncConfig(w http.ResponseWriter, r *http.Request) {
	workspaceUUID, ok := h.issueBridgeWorkspaceUUID(w, r)
	if !ok {
		return
	}
	configID, ok := parseUUIDOrBadRequest(w, chi.URLParam(r, "id"), "issue sync config id")
	if !ok {
		return
	}
	if _, err := h.Queries.DeleteIssueSyncConfig(r.Context(), db.DeleteIssueSyncConfigParams{
		ID:          configID,
		WorkspaceID: workspaceUUID,
	}); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeError(w, http.StatusNotFound, "issue sync config not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to delete issue sync config")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

type normalizedIssueSyncConfigInput struct {
	integrationID       pgtype.UUID
	scopeType           string
	scopeID             pgtype.UUID
	remoteProjectRef    string
	syncEnabled         bool
	pollIntervalSeconds pgtype.Int4
	stateMapping        []byte
	autoAssignEnabled   bool
	defaultAssigneeType pgtype.Text
	defaultAssigneeID   pgtype.UUID
}

func (h *Handler) normalizeIssueSyncConfigRequest(w http.ResponseWriter, r *http.Request, workspaceID pgtype.UUID, req upsertIssueSyncConfigRequest, existing *db.IssueSyncConfig) (normalizedIssueSyncConfigInput, bool) {
	var input normalizedIssueSyncConfigInput
	if existing == nil {
		integrationID, ok := parseUUIDOrBadRequest(w, req.IntegrationID, "integration_id")
		if !ok {
			return input, false
		}
		scopeID, ok := parseUUIDOrBadRequest(w, req.ScopeID, "scope_id")
		if !ok {
			return input, false
		}
		input.integrationID = integrationID
		input.scopeType = strings.TrimSpace(req.ScopeType)
		input.scopeID = scopeID
	} else {
		input.integrationID = existing.IntegrationID
		input.scopeType = existing.ScopeType
		input.scopeID = existing.ScopeID
	}
	if _, err := h.Queries.GetIssueIntegrationInWorkspace(r.Context(), db.GetIssueIntegrationInWorkspaceParams{
		ID:          input.integrationID,
		WorkspaceID: workspaceID,
	}); err != nil {
		writeError(w, http.StatusNotFound, "issue integration not found")
		return input, false
	}
	if !h.validateIssueSyncScope(w, r, workspaceID, input.scopeType, input.scopeID) {
		return input, false
	}
	input.remoteProjectRef = strings.TrimSpace(req.RemoteProjectRef)
	if input.remoteProjectRef == "" {
		writeError(w, http.StatusBadRequest, "remote_project_ref is required")
		return input, false
	}
	input.syncEnabled = req.SyncEnabled
	var ok bool
	input.pollIntervalSeconds, ok = issueSyncPollInterval(w, req.PollIntervalSeconds)
	if !ok {
		return input, false
	}
	stateMapping, ok := issueBridgeJSONObject(w, req.StateMapping, []byte(`{"opened":"backlog","closed":"done"}`), "state_mapping")
	if !ok {
		return input, false
	}
	input.stateMapping = stateMapping
	input.autoAssignEnabled = req.AutoAssignEnabled
	input.defaultAssigneeType, input.defaultAssigneeID, ok = h.issueSyncAssignee(w, r, workspaceID, req)
	if !ok {
		return input, false
	}
	return input, true
}

func (h *Handler) validateIssueSyncScope(w http.ResponseWriter, r *http.Request, workspaceID pgtype.UUID, scopeType string, scopeID pgtype.UUID) bool {
	switch scopeType {
	case "project":
		if _, err := h.Queries.GetProjectInWorkspace(r.Context(), db.GetProjectInWorkspaceParams{
			ID:          scopeID,
			WorkspaceID: workspaceID,
		}); err != nil {
			writeError(w, http.StatusNotFound, "project not found")
			return false
		}
		return true
	case "repo_resource":
		if _, err := h.Queries.GetProjectResourceInWorkspace(r.Context(), db.GetProjectResourceInWorkspaceParams{
			ID:          scopeID,
			WorkspaceID: workspaceID,
		}); err != nil {
			writeError(w, http.StatusNotFound, "project resource not found")
			return false
		}
		return true
	default:
		writeError(w, http.StatusBadRequest, "scope_type must be project or repo_resource")
		return false
	}
}

func (h *Handler) issueSyncAssignee(w http.ResponseWriter, r *http.Request, workspaceID pgtype.UUID, req upsertIssueSyncConfigRequest) (pgtype.Text, pgtype.UUID, bool) {
	if !req.AutoAssignEnabled {
		return pgtype.Text{}, pgtype.UUID{}, true
	}
	if req.DefaultAssigneeType == nil || strings.TrimSpace(*req.DefaultAssigneeType) == "" {
		writeError(w, http.StatusBadRequest, "default_assignee_type is required when auto_assign_enabled is true")
		return pgtype.Text{}, pgtype.UUID{}, false
	}
	if req.DefaultAssigneeID == nil || strings.TrimSpace(*req.DefaultAssigneeID) == "" {
		writeError(w, http.StatusBadRequest, "default_assignee_id is required when auto_assign_enabled is true")
		return pgtype.Text{}, pgtype.UUID{}, false
	}
	assigneeType := strings.TrimSpace(*req.DefaultAssigneeType)
	assigneeID, ok := parseUUIDOrBadRequest(w, strings.TrimSpace(*req.DefaultAssigneeID), "default_assignee_id")
	if !ok {
		return pgtype.Text{}, pgtype.UUID{}, false
	}
	switch assigneeType {
	case "agent":
		if _, err := h.Queries.GetAgentInWorkspace(r.Context(), db.GetAgentInWorkspaceParams{
			ID:          assigneeID,
			WorkspaceID: workspaceID,
		}); err != nil {
			writeError(w, http.StatusNotFound, "agent not found")
			return pgtype.Text{}, pgtype.UUID{}, false
		}
	case "squad":
		if _, err := h.Queries.GetSquadInWorkspace(r.Context(), db.GetSquadInWorkspaceParams{
			ID:          assigneeID,
			WorkspaceID: workspaceID,
		}); err != nil {
			writeError(w, http.StatusNotFound, "squad not found")
			return pgtype.Text{}, pgtype.UUID{}, false
		}
	default:
		writeError(w, http.StatusBadRequest, "default_assignee_type must be agent or squad")
		return pgtype.Text{}, pgtype.UUID{}, false
	}
	return pgtype.Text{String: assigneeType, Valid: true}, assigneeID, true
}

func (h *Handler) issueBridgeService(w http.ResponseWriter, tokenRequired bool) (*issuebridge.Service, bool) {
	svc := h.IssueBridgeService
	if svc == nil {
		writeError(w, http.StatusServiceUnavailable, "issue bridge service is not configured")
		return nil, false
	}
	if tokenRequired && svc.SecretBox == nil {
		writeError(w, http.StatusServiceUnavailable, "issue bridge token storage is not configured")
		return nil, false
	}
	return svc, true
}

func (h *Handler) issueBridgeWorkspaceUUID(w http.ResponseWriter, r *http.Request) (pgtype.UUID, bool) {
	workspaceID := h.resolveWorkspaceID(r)
	if workspaceID == "" {
		writeError(w, http.StatusBadRequest, "workspace_id is required")
		return pgtype.UUID{}, false
	}
	return parseUUIDOrBadRequest(w, workspaceID, "workspace_id")
}

func (h *Handler) defaultIssueSkillIDForCreate(w http.ResponseWriter, r *http.Request, svc *issuebridge.Service, workspaceID, userID pgtype.UUID, requested *string) (pgtype.UUID, bool) {
	if requested != nil && strings.TrimSpace(*requested) != "" {
		return h.parseOptionalSkillInWorkspace(w, r, workspaceID, requested)
	}
	skill, err := svc.EnsureDefaultIssueCreator(r.Context(), workspaceID, userID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to ensure default issue skill")
		return pgtype.UUID{}, false
	}
	return skill.ID, true
}

func (h *Handler) parseOptionalSkillInWorkspace(w http.ResponseWriter, r *http.Request, workspaceID pgtype.UUID, raw *string) (pgtype.UUID, bool) {
	if raw == nil || strings.TrimSpace(*raw) == "" {
		return pgtype.UUID{}, true
	}
	skillID, ok := parseUUIDOrBadRequest(w, strings.TrimSpace(*raw), "default_issue_skill_id")
	if !ok {
		return pgtype.UUID{}, false
	}
	if _, err := h.Queries.GetSkillInWorkspace(r.Context(), db.GetSkillInWorkspaceParams{
		ID:          skillID,
		WorkspaceID: workspaceID,
	}); err != nil {
		writeError(w, http.StatusNotFound, "default issue skill not found")
		return pgtype.UUID{}, false
	}
	return skillID, true
}

func issueIntegrationToResponse(row db.IssueIntegration) IssueIntegrationResponse {
	config := decodeJSONObject(row.Config)
	return IssueIntegrationResponse{
		ID:                         uuidToString(row.ID),
		WorkspaceID:                uuidToString(row.WorkspaceID),
		Provider:                   row.Provider,
		Name:                       row.Name,
		BaseURL:                    row.BaseUrl,
		DefaultIssueSkillID:        uuidToPtr(row.DefaultIssueSkillID),
		PollingEnabled:             row.PollingEnabled,
		DefaultPollIntervalSeconds: row.DefaultPollIntervalSeconds,
		Config:                     config,
		CreatedAt:                  timestampToString(row.CreatedAt),
		UpdatedAt:                  timestampToString(row.UpdatedAt),
	}
}

func issueSyncConfigToResponse(row db.IssueSyncConfig) IssueSyncConfigResponse {
	return IssueSyncConfigResponse{
		ID:                   uuidToString(row.ID),
		WorkspaceID:          uuidToString(row.WorkspaceID),
		IntegrationID:        uuidToString(row.IntegrationID),
		ScopeType:            row.ScopeType,
		ScopeID:              uuidToString(row.ScopeID),
		RemoteProjectRef:     row.RemoteProjectRef,
		SyncEnabled:          row.SyncEnabled,
		PollIntervalSeconds:  int4ToPtr(row.PollIntervalSeconds),
		StateMapping:         decodeJSONObject(row.StateMapping),
		AutoAssignEnabled:    row.AutoAssignEnabled,
		DefaultAssigneeType:  textToPtr(row.DefaultAssigneeType),
		DefaultAssigneeID:    uuidToPtr(row.DefaultAssigneeID),
		LastPollAt:           timestampToPtr(row.LastPollAt),
		LastSuccessfulPollAt: timestampToPtr(row.LastSuccessfulPollAt),
		LastError:            row.LastError,
		CreatedAt:            timestampToString(row.CreatedAt),
		UpdatedAt:            timestampToString(row.UpdatedAt),
	}
}

func decodeJSONObject(raw []byte) map[string]any {
	out := map[string]any{}
	if len(raw) == 0 {
		return out
	}
	if err := json.Unmarshal(raw, &out); err != nil || out == nil {
		return map[string]any{}
	}
	return out
}

func issueBridgeJSONObject(w http.ResponseWriter, raw json.RawMessage, fallback []byte, field string) ([]byte, bool) {
	if len(strings.TrimSpace(string(raw))) == 0 {
		return fallback, true
	}
	var decoded map[string]any
	if err := json.Unmarshal(raw, &decoded); err != nil {
		writeError(w, http.StatusBadRequest, field+" must be a valid JSON object")
		return nil, false
	}
	if decoded == nil {
		writeError(w, http.StatusBadRequest, field+" must be a JSON object")
		return nil, false
	}
	cleaned, err := json.Marshal(decoded)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to encode "+field)
		return nil, false
	}
	return cleaned, true
}

func issueSyncPollInterval(w http.ResponseWriter, seconds *int32) (pgtype.Int4, bool) {
	if seconds == nil {
		return pgtype.Int4{}, true
	}
	if *seconds < issuebridge.MinPollIntervalSeconds {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("poll_interval_seconds must be at least %d", issuebridge.MinPollIntervalSeconds))
		return pgtype.Int4{}, false
	}
	return pgtype.Int4{Int32: *seconds, Valid: true}, true
}

func (h *Handler) writeIssueBridgeWriteError(w http.ResponseWriter, err error, action string) {
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		writeError(w, http.StatusNotFound, "issue integration not found")
	case isUniqueViolation(err):
		writeError(w, http.StatusConflict, "issue bridge record already exists")
	case isCheckViolation(err):
		writeError(w, http.StatusBadRequest, "issue bridge record rejected: a field value failed a database constraint")
	case strings.Contains(err.Error(), "required"),
		strings.Contains(err.Error(), "must "),
		strings.Contains(err.Error(), "invalid"),
		strings.Contains(err.Error(), "poll interval"):
		writeError(w, http.StatusBadRequest, err.Error())
	default:
		writeError(w, http.StatusInternalServerError, "failed to "+action)
	}
}
