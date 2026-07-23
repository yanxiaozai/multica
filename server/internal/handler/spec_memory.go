package handler

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/logger"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

type SpecEpicResponse struct {
	ID          string `json:"id"`
	WorkspaceID string `json:"workspace_id"`
	Key         string `json:"key"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Stability   string `json:"stability"`
	CreatedAt   string `json:"created_at"`
	UpdatedAt   string `json:"updated_at"`
}

type SpecModuleResponse struct {
	ID          string `json:"id"`
	WorkspaceID string `json:"workspace_id"`
	EpicID      string `json:"epic_id"`
	Key         string `json:"key"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Stability   string `json:"stability"`
	CreatedAt   string `json:"created_at"`
	UpdatedAt   string `json:"updated_at"`
}

type SpecDocumentResponse struct {
	ID          string  `json:"id"`
	WorkspaceID string  `json:"workspace_id"`
	EpicID      *string `json:"epic_id"`
	ModuleID    *string `json:"module_id"`
	DocKind     string  `json:"doc_kind"`
	Title       string  `json:"title"`
	Body        string  `json:"body"`
	SourcePath  string  `json:"source_path"`
	CreatedAt   string  `json:"created_at"`
	UpdatedAt   string  `json:"updated_at"`
}

type SpecIssueStateResponse struct {
	ID              string   `json:"id"`
	WorkspaceID     string   `json:"workspace_id"`
	IssueID         string   `json:"issue_id"`
	Status          string   `json:"status"`
	Owner           string   `json:"owner"`
	CurrentStage    string   `json:"current_stage"`
	CurrentLoop     string   `json:"current_loop"`
	LastResult      string   `json:"last_result"`
	OpenQuestions   []string `json:"open_questions"`
	Blockers        []string `json:"blockers"`
	NextHandoff     string   `json:"next_handoff"`
	AuditMode       string   `json:"audit_mode"`
	AuditSkipped    bool     `json:"audit_skipped"`
	AuditSkipReason string   `json:"audit_skip_reason"`
	AuditSkippedBy  string   `json:"audit_skipped_by"`
	AuditSkippedAt  *string  `json:"audit_skipped_at"`
	CreatedAt       string   `json:"created_at"`
	UpdatedAt       string   `json:"updated_at"`
}

type SpecIssueMappingResponse struct {
	ID          string  `json:"id"`
	WorkspaceID string  `json:"workspace_id"`
	IssueID     string  `json:"issue_id"`
	EpicID      *string `json:"epic_id"`
	ModuleID    *string `json:"module_id"`
	MappingKind string  `json:"mapping_kind"`
	Reason      string  `json:"reason"`
	CreatedAt   string  `json:"created_at"`
	UpdatedAt   string  `json:"updated_at"`
}

type SpecDecisionResponse struct {
	ID          string  `json:"id"`
	WorkspaceID string  `json:"workspace_id"`
	EpicID      *string `json:"epic_id"`
	ModuleID    *string `json:"module_id"`
	Title       string  `json:"title"`
	Body        string  `json:"body"`
	Actor       string  `json:"actor"`
	SourcePath  string  `json:"source_path"`
	CreatedAt   string  `json:"created_at"`
	UpdatedAt   string  `json:"updated_at"`
}

type SpecIssueContextResponse struct {
	IssueID  string                     `json:"issue_id"`
	State    *SpecIssueStateResponse    `json:"state"`
	Mappings []SpecIssueMappingResponse `json:"mappings"`
	Docs     []SpecDocumentResponse     `json:"documents"`
	Metadata map[string]any             `json:"metadata"`
}

type UpdateIssueSpecMappingRequest struct {
	Primary *SpecIssueMappingInput  `json:"primary"`
	Related []SpecIssueMappingInput `json:"related"`
}

type SpecIssueMappingInput struct {
	EpicID   *string `json:"epic_id"`
	ModuleID *string `json:"module_id"`
	Reason   string  `json:"reason"`
}

type UpdateIssueSpecStateRequest struct {
	Status          string   `json:"status"`
	Owner           string   `json:"owner"`
	CurrentStage    string   `json:"current_stage"`
	CurrentLoop     string   `json:"current_loop"`
	LastResult      string   `json:"last_result"`
	OpenQuestions   []string `json:"open_questions"`
	Blockers        []string `json:"blockers"`
	NextHandoff     string   `json:"next_handoff"`
	AuditMode       string   `json:"audit_mode"`
	AuditSkipped    bool     `json:"audit_skipped"`
	AuditSkipReason string   `json:"audit_skip_reason"`
	AuditSkippedBy  string   `json:"audit_skipped_by"`
}

type UpdateSpecDocumentRequest struct {
	Title      string `json:"title"`
	Body       string `json:"body"`
	SourcePath string `json:"source_path"`
}

type CreateSpecDecisionRequest struct {
	EpicID     *string `json:"epic_id"`
	ModuleID   *string `json:"module_id"`
	Title      string  `json:"title"`
	Body       string  `json:"body"`
	Actor      string  `json:"actor"`
	SourcePath string  `json:"source_path"`
}

type SyncSpecFromFilesRequest struct {
	Epics             []SyncSpecEpicInput     `json:"epics"`
	Issues            []SyncSpecIssueInput    `json:"issues"`
	Decisions         []SyncSpecDecisionInput `json:"decisions"`
	SkippedIssueFiles []string                `json:"skipped_issue_files"`
}

type SyncSpecEpicInput struct {
	Key         string                  `json:"key"`
	Title       string                  `json:"title"`
	Description string                  `json:"description"`
	Stability   string                  `json:"stability"`
	Documents   []SyncSpecDocumentInput `json:"documents"`
	Modules     []SyncSpecModuleInput   `json:"modules"`
}

type SyncSpecModuleInput struct {
	Key         string                  `json:"key"`
	Title       string                  `json:"title"`
	Description string                  `json:"description"`
	Stability   string                  `json:"stability"`
	Documents   []SyncSpecDocumentInput `json:"documents"`
}

type SyncSpecDocumentInput struct {
	DocKind    string `json:"doc_kind"`
	Title      string `json:"title"`
	Body       string `json:"body"`
	SourcePath string `json:"source_path"`
}

type SyncSpecDecisionInput struct {
	Title      string `json:"title"`
	Body       string `json:"body"`
	Actor      string `json:"actor"`
	SourcePath string `json:"source_path"`
}

type SyncSpecIssueInput struct {
	Issue         string    `json:"issue"`
	Title         string    `json:"title"`
	Primary       string    `json:"primary"`
	Related       []string  `json:"related"`
	Status        string    `json:"status"`
	Owner         string    `json:"owner"`
	CurrentStage  string    `json:"current_stage"`
	CurrentLoop   string    `json:"current_loop"`
	LastResult    string    `json:"last_result"`
	OpenQuestions []string  `json:"open_questions"`
	Blockers      []string  `json:"blockers"`
	NextHandoff   string    `json:"next_handoff"`
	Audit         SyncAudit `json:"audit"`
	UpdatedAt     string    `json:"updated_at"`
	SourcePath    string    `json:"source_path"`
}

type SyncAudit struct {
	Mode       string `json:"mode"`
	Skipped    bool   `json:"skipped"`
	SkipReason string `json:"skip_reason"`
	SkippedBy  string `json:"skipped_by"`
	SkippedAt  string `json:"skipped_at"`
}

type SyncSpecFromFilesResponse struct {
	Epics             int      `json:"epics"`
	Modules           int      `json:"modules"`
	Documents         int      `json:"documents"`
	Issues            int      `json:"issues"`
	IssueMappings     int      `json:"issue_mappings"`
	Decisions         int      `json:"decisions"`
	SkippedIssueFiles []string `json:"skipped_issue_files"`
}

type SyncSpecToFilesResponse = SyncSpecFromFilesRequest

func specEpicToResponse(epic db.SpecEpic) SpecEpicResponse {
	return SpecEpicResponse{
		ID:          uuidToString(epic.ID),
		WorkspaceID: uuidToString(epic.WorkspaceID),
		Key:         epic.Key,
		Title:       epic.Title,
		Description: epic.Description,
		Stability:   epic.Stability,
		CreatedAt:   timestampToString(epic.CreatedAt),
		UpdatedAt:   timestampToString(epic.UpdatedAt),
	}
}

func specModuleToResponse(module db.SpecModule) SpecModuleResponse {
	return SpecModuleResponse{
		ID:          uuidToString(module.ID),
		WorkspaceID: uuidToString(module.WorkspaceID),
		EpicID:      uuidToString(module.EpicID),
		Key:         module.Key,
		Title:       module.Title,
		Description: module.Description,
		Stability:   module.Stability,
		CreatedAt:   timestampToString(module.CreatedAt),
		UpdatedAt:   timestampToString(module.UpdatedAt),
	}
}

func specDocumentToResponse(doc db.SpecDocument) SpecDocumentResponse {
	return SpecDocumentResponse{
		ID:          uuidToString(doc.ID),
		WorkspaceID: uuidToString(doc.WorkspaceID),
		EpicID:      uuidToPtr(doc.EpicID),
		ModuleID:    uuidToPtr(doc.ModuleID),
		DocKind:     doc.DocKind,
		Title:       doc.Title,
		Body:        doc.Body,
		SourcePath:  doc.SourcePath,
		CreatedAt:   timestampToString(doc.CreatedAt),
		UpdatedAt:   timestampToString(doc.UpdatedAt),
	}
}

func specIssueStateToResponse(state db.SpecIssueState) SpecIssueStateResponse {
	return SpecIssueStateResponse{
		ID:              uuidToString(state.ID),
		WorkspaceID:     uuidToString(state.WorkspaceID),
		IssueID:         uuidToString(state.IssueID),
		Status:          state.Status,
		Owner:           state.Owner,
		CurrentStage:    state.CurrentStage,
		CurrentLoop:     state.CurrentLoop,
		LastResult:      state.LastResult,
		OpenQuestions:   decodeStringArray(state.OpenQuestions),
		Blockers:        decodeStringArray(state.Blockers),
		NextHandoff:     state.NextHandoff,
		AuditMode:       state.AuditMode,
		AuditSkipped:    state.AuditSkipped,
		AuditSkipReason: state.AuditSkipReason,
		AuditSkippedBy:  state.AuditSkippedBy,
		AuditSkippedAt:  timestampToPtr(state.AuditSkippedAt),
		CreatedAt:       timestampToString(state.CreatedAt),
		UpdatedAt:       timestampToString(state.UpdatedAt),
	}
}

func specIssueMappingToResponse(mapping db.SpecIssueMapping) SpecIssueMappingResponse {
	return SpecIssueMappingResponse{
		ID:          uuidToString(mapping.ID),
		WorkspaceID: uuidToString(mapping.WorkspaceID),
		IssueID:     uuidToString(mapping.IssueID),
		EpicID:      uuidToPtr(mapping.EpicID),
		ModuleID:    uuidToPtr(mapping.ModuleID),
		MappingKind: mapping.MappingKind,
		Reason:      mapping.Reason,
		CreatedAt:   timestampToString(mapping.CreatedAt),
		UpdatedAt:   timestampToString(mapping.UpdatedAt),
	}
}

func specDecisionToResponse(decision db.SpecDecision) SpecDecisionResponse {
	return SpecDecisionResponse{
		ID:          uuidToString(decision.ID),
		WorkspaceID: uuidToString(decision.WorkspaceID),
		EpicID:      uuidToPtr(decision.EpicID),
		ModuleID:    uuidToPtr(decision.ModuleID),
		Title:       decision.Title,
		Body:        decision.Body,
		Actor:       decision.Actor,
		SourcePath:  decision.SourcePath,
		CreatedAt:   timestampToString(decision.CreatedAt),
		UpdatedAt:   timestampToString(decision.UpdatedAt),
	}
}

func (h *Handler) ListSpecEpics(w http.ResponseWriter, r *http.Request) {
	workspaceID := h.resolveWorkspaceID(r)
	wsUUID, ok := parseUUIDOrBadRequest(w, workspaceID, "workspace_id")
	if !ok {
		return
	}
	epics, err := h.Queries.ListSpecEpics(r.Context(), wsUUID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list spec epics")
		return
	}
	resp := make([]SpecEpicResponse, len(epics))
	for i, epic := range epics {
		resp[i] = specEpicToResponse(epic)
	}
	writeJSON(w, http.StatusOK, map[string]any{"epics": resp})
}

func (h *Handler) ListSpecModules(w http.ResponseWriter, r *http.Request) {
	workspaceID := h.resolveWorkspaceID(r)
	wsUUID, ok := parseUUIDOrBadRequest(w, workspaceID, "workspace_id")
	if !ok {
		return
	}
	epicID, ok := parseUUIDOrBadRequest(w, chi.URLParam(r, "epicId"), "epic id")
	if !ok {
		return
	}
	modules, err := h.Queries.ListSpecModules(r.Context(), db.ListSpecModulesParams{
		WorkspaceID: wsUUID,
		EpicID:      epicID,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list spec modules")
		return
	}
	resp := make([]SpecModuleResponse, len(modules))
	for i, module := range modules {
		resp[i] = specModuleToResponse(module)
	}
	writeJSON(w, http.StatusOK, map[string]any{"modules": resp})
}

func (h *Handler) ListSpecEpicDocuments(w http.ResponseWriter, r *http.Request) {
	workspaceID := h.resolveWorkspaceID(r)
	wsUUID, ok := parseUUIDOrBadRequest(w, workspaceID, "workspace_id")
	if !ok {
		return
	}
	epicID, ok := parseUUIDOrBadRequest(w, chi.URLParam(r, "epicId"), "epic id")
	if !ok {
		return
	}
	docs, err := h.Queries.ListSpecDocumentsByEpic(r.Context(), db.ListSpecDocumentsByEpicParams{
		WorkspaceID: wsUUID,
		EpicID:      epicID,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list spec documents")
		return
	}
	resp := make([]SpecDocumentResponse, len(docs))
	for i, doc := range docs {
		resp[i] = specDocumentToResponse(doc)
	}
	writeJSON(w, http.StatusOK, map[string]any{"documents": resp})
}

func (h *Handler) ListSpecModuleDocuments(w http.ResponseWriter, r *http.Request) {
	workspaceID := h.resolveWorkspaceID(r)
	wsUUID, ok := parseUUIDOrBadRequest(w, workspaceID, "workspace_id")
	if !ok {
		return
	}
	moduleID, ok := parseUUIDOrBadRequest(w, chi.URLParam(r, "moduleId"), "module id")
	if !ok {
		return
	}
	docs, err := h.Queries.ListSpecDocumentsByModule(r.Context(), db.ListSpecDocumentsByModuleParams{
		WorkspaceID: wsUUID,
		ModuleID:    moduleID,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list spec documents")
		return
	}
	resp := make([]SpecDocumentResponse, len(docs))
	for i, doc := range docs {
		resp[i] = specDocumentToResponse(doc)
	}
	writeJSON(w, http.StatusOK, map[string]any{"documents": resp})
}

func (h *Handler) GetIssueSpec(w http.ResponseWriter, r *http.Request) {
	issue, ok := h.loadIssueForUser(w, r, chi.URLParam(r, "id"))
	if !ok {
		return
	}
	var stateResp *SpecIssueStateResponse
	state, err := h.Queries.GetSpecIssueState(r.Context(), db.GetSpecIssueStateParams{
		WorkspaceID: issue.WorkspaceID,
		IssueID:     issue.ID,
	})
	if err == nil {
		resp := specIssueStateToResponse(state)
		stateResp = &resp
	} else if !errors.Is(err, pgx.ErrNoRows) {
		writeError(w, http.StatusInternalServerError, "failed to load issue spec state")
		return
	}
	mappings, err := h.Queries.ListSpecIssueMappings(r.Context(), db.ListSpecIssueMappingsParams{
		WorkspaceID: issue.WorkspaceID,
		IssueID:     issue.ID,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load issue spec mappings")
		return
	}
	mappingResp := make([]SpecIssueMappingResponse, len(mappings))
	for i, mapping := range mappings {
		mappingResp[i] = specIssueMappingToResponse(mapping)
	}
	docs, err := h.issueSpecDocuments(r, issue.WorkspaceID, mappings)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load linked spec documents")
		return
	}
	writeJSON(w, http.StatusOK, SpecIssueContextResponse{
		IssueID:  uuidToString(issue.ID),
		State:    stateResp,
		Mappings: mappingResp,
		Docs:     docs,
		Metadata: parseIssueMetadata(issue.Metadata),
	})
}

func (h *Handler) UpdateIssueSpecMapping(w http.ResponseWriter, r *http.Request) {
	issue, ok := h.loadIssueForUser(w, r, chi.URLParam(r, "id"))
	if !ok {
		return
	}
	userID, ok := requireUserID(w, r)
	if !ok {
		return
	}
	var req UpdateIssueSpecMappingRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Primary == nil {
		writeError(w, http.StatusBadRequest, "primary mapping is required")
		return
	}
	if err := h.Queries.DeleteSpecIssueMappings(r.Context(), db.DeleteSpecIssueMappingsParams{
		WorkspaceID: issue.WorkspaceID,
		IssueID:     issue.ID,
	}); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to clear issue spec mappings")
		return
	}
	primary, err := h.upsertIssueSpecMapping(r, issue, "primary", *req.Primary)
	if err != nil {
		writeSpecMemoryWriteError(w, r, err, "update issue spec mapping")
		return
	}
	for _, related := range req.Related {
		if _, err := h.upsertIssueSpecMapping(r, issue, "related", related); err != nil {
			writeSpecMemoryWriteError(w, r, err, "update related issue spec mapping")
			return
		}
	}
	if err := h.syncIssueSpecMetadata(r, issue, userID); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to sync issue spec metadata")
		return
	}
	resp := specIssueMappingToResponse(primary)
	writeJSON(w, http.StatusOK, map[string]any{"primary": resp})
}

func (h *Handler) UpdateIssueSpecState(w http.ResponseWriter, r *http.Request) {
	issue, ok := h.loadIssueForUser(w, r, chi.URLParam(r, "id"))
	if !ok {
		return
	}
	userID, ok := requireUserID(w, r)
	if !ok {
		return
	}
	var req UpdateIssueSpecStateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	req.applyDefaults()
	if !validSpecIssueStateRequest(w, req) {
		return
	}
	openQuestions, _ := json.Marshal(req.OpenQuestions)
	blockers, _ := json.Marshal(req.Blockers)
	state, err := h.Queries.UpsertSpecIssueState(r.Context(), db.UpsertSpecIssueStateParams{
		WorkspaceID:     issue.WorkspaceID,
		IssueID:         issue.ID,
		Status:          req.Status,
		Owner:           req.Owner,
		CurrentStage:    req.CurrentStage,
		CurrentLoop:     req.CurrentLoop,
		LastResult:      req.LastResult,
		OpenQuestions:   openQuestions,
		Blockers:        blockers,
		NextHandoff:     req.NextHandoff,
		AuditMode:       req.AuditMode,
		AuditSkipped:    req.AuditSkipped,
		AuditSkipReason: req.AuditSkipReason,
		AuditSkippedBy:  req.AuditSkippedBy,
	})
	if err != nil {
		writeSpecMemoryWriteError(w, r, err, "update issue spec state")
		return
	}
	if err := h.syncIssueSpecMetadata(r, issue, userID); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to sync issue spec metadata")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"state": specIssueStateToResponse(state)})
}

func (h *Handler) UpdateSpecModuleDocument(w http.ResponseWriter, r *http.Request) {
	workspaceID := h.resolveWorkspaceID(r)
	wsUUID, ok := parseUUIDOrBadRequest(w, workspaceID, "workspace_id")
	if !ok {
		return
	}
	moduleID, ok := parseUUIDOrBadRequest(w, chi.URLParam(r, "moduleId"), "module id")
	if !ok {
		return
	}
	docKind := chi.URLParam(r, "docKind")
	if !validSpecDocumentKind(docKind) {
		writeError(w, http.StatusBadRequest, "invalid document kind")
		return
	}
	var req UpdateSpecDocumentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	doc, err := h.Queries.UpsertSpecModuleDocument(r.Context(), db.UpsertSpecModuleDocumentParams{
		WorkspaceID: wsUUID,
		ModuleID:    moduleID,
		DocKind:     docKind,
		Title:       req.Title,
		Body:        req.Body,
		SourcePath:  req.SourcePath,
	})
	if err != nil {
		writeSpecMemoryWriteError(w, r, err, "update spec document")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"document": specDocumentToResponse(doc)})
}

func (h *Handler) CreateSpecDecision(w http.ResponseWriter, r *http.Request) {
	workspaceID := h.resolveWorkspaceID(r)
	wsUUID, ok := parseUUIDOrBadRequest(w, workspaceID, "workspace_id")
	if !ok {
		return
	}
	var req CreateSpecDecisionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if strings.TrimSpace(req.Title) == "" {
		writeError(w, http.StatusBadRequest, "title is required")
		return
	}
	epicID, ok := nullableUUIDFromString(w, req.EpicID, "epic_id")
	if !ok {
		return
	}
	moduleID, ok := nullableUUIDFromString(w, req.ModuleID, "module_id")
	if !ok {
		return
	}
	decision, err := h.Queries.CreateSpecDecision(r.Context(), db.CreateSpecDecisionParams{
		WorkspaceID: wsUUID,
		EpicID:      epicID,
		ModuleID:    moduleID,
		Title:       req.Title,
		Body:        req.Body,
		Actor:       req.Actor,
		SourcePath:  req.SourcePath,
	})
	if err != nil {
		writeSpecMemoryWriteError(w, r, err, "create spec decision")
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"decision": specDecisionToResponse(decision)})
}

func (h *Handler) SyncSpecFromFiles(w http.ResponseWriter, r *http.Request) {
	workspaceID := h.resolveWorkspaceID(r)
	wsUUID, ok := parseUUIDOrBadRequest(w, workspaceID, "workspace_id")
	if !ok {
		return
	}
	var req SyncSpecFromFilesRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	resp := SyncSpecFromFilesResponse{
		SkippedIssueFiles: req.SkippedIssueFiles,
	}
	epicIDs := map[string]pgtype.UUID{}
	moduleIDs := map[string]pgtype.UUID{}
	for _, epicIn := range req.Epics {
		if strings.TrimSpace(epicIn.Key) == "" {
			writeError(w, http.StatusBadRequest, "epic key is required")
			return
		}
		if !validSpecStability(epicIn.Stability) {
			writeError(w, http.StatusBadRequest, "invalid epic stability")
			return
		}
		epic, err := h.Queries.UpsertSpecEpic(r.Context(), db.UpsertSpecEpicParams{
			WorkspaceID: wsUUID,
			Key:         strings.TrimSpace(epicIn.Key),
			Title:       firstNonEmptyString(epicIn.Title, epicIn.Key),
			Description: epicIn.Description,
			Stability:   firstNonEmptyString(epicIn.Stability, "draft"),
		})
		if err != nil {
			writeSpecMemoryWriteError(w, r, err, "sync spec epic")
			return
		}
		epicIDs[epicIn.Key] = epic.ID
		resp.Epics++
		for _, docIn := range epicIn.Documents {
			if !validSpecDocumentKind(docIn.DocKind) {
				writeError(w, http.StatusBadRequest, "invalid epic document kind")
				return
			}
			if _, err := h.Queries.UpsertSpecEpicDocument(r.Context(), db.UpsertSpecEpicDocumentParams{
				WorkspaceID: wsUUID,
				EpicID:      epic.ID,
				DocKind:     docIn.DocKind,
				Title:       firstNonEmptyString(docIn.Title, docIn.DocKind),
				Body:        docIn.Body,
				SourcePath:  docIn.SourcePath,
			}); err != nil {
				writeSpecMemoryWriteError(w, r, err, "sync spec epic document")
				return
			}
			resp.Documents++
		}
		for _, moduleIn := range epicIn.Modules {
			if strings.TrimSpace(moduleIn.Key) == "" {
				writeError(w, http.StatusBadRequest, "module key is required")
				return
			}
			if !validSpecStability(moduleIn.Stability) {
				writeError(w, http.StatusBadRequest, "invalid module stability")
				return
			}
			module, err := h.Queries.UpsertSpecModule(r.Context(), db.UpsertSpecModuleParams{
				WorkspaceID: wsUUID,
				EpicID:      epic.ID,
				Key:         strings.TrimSpace(moduleIn.Key),
				Title:       firstNonEmptyString(moduleIn.Title, moduleIn.Key),
				Description: moduleIn.Description,
				Stability:   firstNonEmptyString(moduleIn.Stability, "draft"),
			})
			if err != nil {
				writeSpecMemoryWriteError(w, r, err, "sync spec module")
				return
			}
			moduleIDs[epicIn.Key+"/"+moduleIn.Key] = module.ID
			resp.Modules++
			for _, docIn := range moduleIn.Documents {
				if !validSpecDocumentKind(docIn.DocKind) {
					writeError(w, http.StatusBadRequest, "invalid module document kind")
					return
				}
				if _, err := h.Queries.UpsertSpecModuleDocument(r.Context(), db.UpsertSpecModuleDocumentParams{
					WorkspaceID: wsUUID,
					ModuleID:    module.ID,
					DocKind:     docIn.DocKind,
					Title:       firstNonEmptyString(docIn.Title, docIn.DocKind),
					Body:        docIn.Body,
					SourcePath:  docIn.SourcePath,
				}); err != nil {
					writeSpecMemoryWriteError(w, r, err, "sync spec module document")
					return
				}
				resp.Documents++
			}
		}
	}
	for _, issueIn := range req.Issues {
		issue, ok := h.resolveSpecSyncIssue(r.Context(), issueIn.Issue, workspaceID)
		if !ok {
			resp.SkippedIssueFiles = append(resp.SkippedIssueFiles, issueIn.SourcePath)
			continue
		}
		stateReq := UpdateIssueSpecStateRequest{
			Status:          issueIn.Status,
			Owner:           issueIn.Owner,
			CurrentStage:    issueIn.CurrentStage,
			CurrentLoop:     issueIn.CurrentLoop,
			LastResult:      issueIn.LastResult,
			OpenQuestions:   issueIn.OpenQuestions,
			Blockers:        issueIn.Blockers,
			NextHandoff:     issueIn.NextHandoff,
			AuditMode:       issueIn.Audit.Mode,
			AuditSkipped:    issueIn.Audit.Skipped,
			AuditSkipReason: issueIn.Audit.SkipReason,
			AuditSkippedBy:  issueIn.Audit.SkippedBy,
		}
		stateReq.applyDefaults()
		if !validSpecIssueStateRequest(w, stateReq) {
			return
		}
		openQuestions, _ := json.Marshal(stateReq.OpenQuestions)
		blockers, _ := json.Marshal(stateReq.Blockers)
		if _, err := h.Queries.UpsertSpecIssueState(r.Context(), db.UpsertSpecIssueStateParams{
			WorkspaceID:     wsUUID,
			IssueID:         issue.ID,
			Status:          stateReq.Status,
			Owner:           stateReq.Owner,
			CurrentStage:    stateReq.CurrentStage,
			CurrentLoop:     stateReq.CurrentLoop,
			LastResult:      stateReq.LastResult,
			OpenQuestions:   openQuestions,
			Blockers:        blockers,
			NextHandoff:     stateReq.NextHandoff,
			AuditMode:       stateReq.AuditMode,
			AuditSkipped:    stateReq.AuditSkipped,
			AuditSkipReason: stateReq.AuditSkipReason,
			AuditSkippedBy:  stateReq.AuditSkippedBy,
		}); err != nil {
			writeSpecMemoryWriteError(w, r, err, "sync spec issue state")
			return
		}
		resp.Issues++
		mappingCount, err := h.syncSpecIssueMappingsFromFiles(r, issue, issueIn, epicIDs, moduleIDs)
		if err != nil {
			writeSpecMemoryWriteError(w, r, err, "sync spec issue mappings")
			return
		}
		resp.IssueMappings += mappingCount
	}
	for _, decisionIn := range req.Decisions {
		if strings.TrimSpace(decisionIn.Title) == "" {
			writeError(w, http.StatusBadRequest, "decision title is required")
			return
		}
		if _, err := h.Queries.CreateSpecDecision(r.Context(), db.CreateSpecDecisionParams{
			WorkspaceID: wsUUID,
			EpicID:      pgtype.UUID{},
			ModuleID:    pgtype.UUID{},
			Title:       decisionIn.Title,
			Body:        decisionIn.Body,
			Actor:       decisionIn.Actor,
			SourcePath:  decisionIn.SourcePath,
		}); err != nil {
			writeSpecMemoryWriteError(w, r, err, "sync spec decision")
			return
		}
		resp.Decisions++
	}
	writeJSON(w, http.StatusOK, resp)
}

func (h *Handler) SyncSpecToFiles(w http.ResponseWriter, r *http.Request) {
	workspaceID := h.resolveWorkspaceID(r)
	wsUUID, ok := parseUUIDOrBadRequest(w, workspaceID, "workspace_id")
	if !ok {
		return
	}
	epics, err := h.Queries.ListSpecEpics(r.Context(), wsUUID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list spec epics")
		return
	}
	resp := SyncSpecToFilesResponse{}
	epicRefs := map[string]string{}
	moduleRefs := map[string]string{}
	for _, epic := range epics {
		epicRefs[uuidToString(epic.ID)] = epic.Key
		epicOut := SyncSpecEpicInput{
			Key:         epic.Key,
			Title:       epic.Title,
			Description: epic.Description,
			Stability:   epic.Stability,
		}
		epicDocs, err := h.Queries.ListSpecDocumentsByEpic(r.Context(), db.ListSpecDocumentsByEpicParams{
			WorkspaceID: wsUUID,
			EpicID:      epic.ID,
		})
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to list spec epic documents")
			return
		}
		for _, doc := range epicDocs {
			epicOut.Documents = append(epicOut.Documents, specDocumentToSyncInput(doc))
		}
		modules, err := h.Queries.ListSpecModules(r.Context(), db.ListSpecModulesParams{
			WorkspaceID: wsUUID,
			EpicID:      epic.ID,
		})
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to list spec modules")
			return
		}
		for _, module := range modules {
			moduleRefs[uuidToString(module.ID)] = epic.Key + "/" + module.Key
			moduleOut := SyncSpecModuleInput{
				Key:         module.Key,
				Title:       module.Title,
				Description: module.Description,
				Stability:   module.Stability,
			}
			docs, err := h.Queries.ListSpecDocumentsByModule(r.Context(), db.ListSpecDocumentsByModuleParams{
				WorkspaceID: wsUUID,
				ModuleID:    module.ID,
			})
			if err != nil {
				writeError(w, http.StatusInternalServerError, "failed to list spec module documents")
				return
			}
			for _, doc := range docs {
				moduleOut.Documents = append(moduleOut.Documents, specDocumentToSyncInput(doc))
			}
			epicOut.Modules = append(epicOut.Modules, moduleOut)
		}
		resp.Epics = append(resp.Epics, epicOut)
	}
	decisions, err := h.Queries.ListSpecDecisions(r.Context(), db.ListSpecDecisionsParams{WorkspaceID: wsUUID})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list spec decisions")
		return
	}
	for _, decision := range decisions {
		resp.Decisions = append(resp.Decisions, SyncSpecDecisionInput{
			Title:      decision.Title,
			Body:       decision.Body,
			Actor:      decision.Actor,
			SourcePath: decision.SourcePath,
		})
	}
	issueStates, err := h.Queries.ListSpecIssueStates(r.Context(), wsUUID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list spec issue states")
		return
	}
	for _, state := range issueStates {
		issueRef := uuidToString(state.IssueID)
		if state.IssuePrefix != "" && state.IssueNumber > 0 {
			issueRef = state.IssuePrefix + "-" + strconv.Itoa(int(state.IssueNumber))
		}
		mappings, err := h.Queries.ListSpecIssueMappings(r.Context(), db.ListSpecIssueMappingsParams{
			WorkspaceID: wsUUID,
			IssueID:     state.IssueID,
		})
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to list spec issue mappings")
			return
		}
		issueOut := SyncSpecIssueInput{
			Issue:         issueRef,
			Title:         state.IssueTitle,
			Status:        state.Status,
			Owner:         state.Owner,
			CurrentStage:  state.CurrentStage,
			CurrentLoop:   state.CurrentLoop,
			LastResult:    state.LastResult,
			OpenQuestions: decodeStringArray(state.OpenQuestions),
			Blockers:      decodeStringArray(state.Blockers),
			NextHandoff:   state.NextHandoff,
			Audit: SyncAudit{
				Mode:       state.AuditMode,
				Skipped:    state.AuditSkipped,
				SkipReason: state.AuditSkipReason,
				SkippedBy:  state.AuditSkippedBy,
				SkippedAt:  timestampToString(state.AuditSkippedAt),
			},
			UpdatedAt:  timestampToString(state.UpdatedAt),
			SourcePath: ".spec/issues/" + issueRef + ".md",
		}
		for _, mapping := range mappings {
			ref := specMappingRef(mapping, epicRefs, moduleRefs)
			if ref == "" {
				continue
			}
			if mapping.MappingKind == "primary" {
				issueOut.Primary = ref
				continue
			}
			if mapping.Reason != "" {
				ref += " - " + mapping.Reason
			}
			issueOut.Related = append(issueOut.Related, ref)
		}
		resp.Issues = append(resp.Issues, issueOut)
	}
	writeJSON(w, http.StatusOK, resp)
}

func specDocumentToSyncInput(doc db.SpecDocument) SyncSpecDocumentInput {
	return SyncSpecDocumentInput{
		DocKind:    doc.DocKind,
		Title:      doc.Title,
		Body:       doc.Body,
		SourcePath: doc.SourcePath,
	}
}

func specMappingRef(mapping db.SpecIssueMapping, epicRefs, moduleRefs map[string]string) string {
	if mapping.ModuleID.Valid {
		return moduleRefs[uuidToString(mapping.ModuleID)]
	}
	if mapping.EpicID.Valid {
		return epicRefs[uuidToString(mapping.EpicID)]
	}
	return ""
}

func (h *Handler) resolveSpecSyncIssue(ctx context.Context, issueRef, workspaceID string) (db.Issue, bool) {
	issueRef = strings.TrimSpace(issueRef)
	if issueRef == "" {
		return db.Issue{}, false
	}
	if issue, ok := h.resolveIssueByIdentifier(ctx, issueRef, workspaceID); ok {
		return issue, true
	}
	issueUUID, err := util.ParseUUID(issueRef)
	if err != nil {
		return db.Issue{}, false
	}
	wsUUID, err := util.ParseUUID(workspaceID)
	if err != nil {
		return db.Issue{}, false
	}
	issue, err := h.Queries.GetIssueInWorkspace(ctx, db.GetIssueInWorkspaceParams{
		ID:          issueUUID,
		WorkspaceID: wsUUID,
	})
	return issue, err == nil
}

func (h *Handler) syncSpecIssueMappingsFromFiles(
	r *http.Request,
	issue db.Issue,
	issueIn SyncSpecIssueInput,
	epicIDs map[string]pgtype.UUID,
	moduleIDs map[string]pgtype.UUID,
) (int, error) {
	if issueIn.Primary == "" && len(issueIn.Related) == 0 {
		return 0, nil
	}
	if err := h.Queries.DeleteSpecIssueMappings(r.Context(), db.DeleteSpecIssueMappingsParams{
		WorkspaceID: issue.WorkspaceID,
		IssueID:     issue.ID,
	}); err != nil {
		return 0, err
	}
	count := 0
	if mapping, ok := mappingInputFromSpecRef(issueIn.Primary, epicIDs, moduleIDs, ""); ok {
		if _, err := h.upsertIssueSpecMapping(r, issue, "primary", mapping); err != nil {
			return count, err
		}
		count++
	}
	for _, related := range issueIn.Related {
		ref, reason, _ := strings.Cut(related, " - ")
		if mapping, ok := mappingInputFromSpecRef(ref, epicIDs, moduleIDs, reason); ok {
			if _, err := h.upsertIssueSpecMapping(r, issue, "related", mapping); err != nil {
				return count, err
			}
			count++
		}
	}
	return count, nil
}

func mappingInputFromSpecRef(ref string, epicIDs map[string]pgtype.UUID, moduleIDs map[string]pgtype.UUID, reason string) (SpecIssueMappingInput, bool) {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return SpecIssueMappingInput{}, false
	}
	if moduleID, ok := moduleIDs[ref]; ok {
		id := uuidToString(moduleID)
		return SpecIssueMappingInput{ModuleID: &id, Reason: strings.TrimSpace(reason)}, true
	}
	if epicID, ok := epicIDs[ref]; ok {
		id := uuidToString(epicID)
		return SpecIssueMappingInput{EpicID: &id, Reason: strings.TrimSpace(reason)}, true
	}
	return SpecIssueMappingInput{}, false
}

func (h *Handler) issueSpecDocuments(r *http.Request, workspaceID pgtype.UUID, mappings []db.SpecIssueMapping) ([]SpecDocumentResponse, error) {
	seen := map[string]bool{}
	var out []SpecDocumentResponse
	for _, mapping := range mappings {
		if !mapping.ModuleID.Valid {
			continue
		}
		key := uuidToString(mapping.ModuleID)
		if seen[key] {
			continue
		}
		seen[key] = true
		docs, err := h.Queries.ListSpecDocumentsByModule(r.Context(), db.ListSpecDocumentsByModuleParams{
			WorkspaceID: workspaceID,
			ModuleID:    mapping.ModuleID,
		})
		if err != nil {
			return nil, err
		}
		for _, doc := range docs {
			out = append(out, specDocumentToResponse(doc))
		}
	}
	return out, nil
}

func (h *Handler) upsertIssueSpecMapping(r *http.Request, issue db.Issue, kind string, in SpecIssueMappingInput) (db.SpecIssueMapping, error) {
	epicID, ok := nullableUUIDFromString(nil, in.EpicID, "epic_id")
	if !ok {
		return db.SpecIssueMapping{}, errors.New("invalid epic_id")
	}
	moduleID, ok := nullableUUIDFromString(nil, in.ModuleID, "module_id")
	if !ok {
		return db.SpecIssueMapping{}, errors.New("invalid module_id")
	}
	if !epicID.Valid && !moduleID.Valid {
		return db.SpecIssueMapping{}, errors.New("mapping requires epic_id or module_id")
	}
	if kind == "primary" {
		return h.Queries.UpsertSpecIssuePrimaryMapping(r.Context(), db.UpsertSpecIssuePrimaryMappingParams{
			WorkspaceID: issue.WorkspaceID,
			IssueID:     issue.ID,
			EpicID:      epicID,
			ModuleID:    moduleID,
			Reason:      in.Reason,
		})
	}
	return h.Queries.UpsertSpecIssueRelatedMapping(r.Context(), db.UpsertSpecIssueRelatedMappingParams{
		WorkspaceID: issue.WorkspaceID,
		IssueID:     issue.ID,
		EpicID:      epicID,
		ModuleID:    moduleID,
		Reason:      in.Reason,
	})
}

func (h *Handler) syncIssueSpecMetadata(r *http.Request, issue db.Issue, userID string) error {
	mappings, err := h.Queries.ListSpecIssueMappings(r.Context(), db.ListSpecIssueMappingsParams{
		WorkspaceID: issue.WorkspaceID,
		IssueID:     issue.ID,
	})
	if err != nil {
		return err
	}
	metadata := parseIssueMetadata(issue.Metadata)
	metadata["spec_issue_file"] = ".spec/issues/" + uuidToString(issue.ID) + ".md"
	var related []string
	for _, mapping := range mappings {
		value := mappingMetadataValue(mapping)
		if value == "" {
			continue
		}
		if mapping.MappingKind == "primary" {
			metadata["spec_primary"] = value
			continue
		}
		related = append(related, value)
	}
	metadata["spec_related"] = strings.Join(related, ",")
	if state, err := h.Queries.GetSpecIssueState(r.Context(), db.GetSpecIssueStateParams{WorkspaceID: issue.WorkspaceID, IssueID: issue.ID}); err == nil {
		metadata["audit_mode"] = state.AuditMode
	}
	workspaceID := uuidToString(issue.WorkspaceID)
	updated := issue
	for _, key := range []string{"spec_primary", "spec_related", "spec_issue_file", "audit_mode"} {
		value, ok := metadata[key]
		if !ok {
			continue
		}
		raw, _ := json.Marshal(value)
		next, err := h.Queries.SetIssueMetadataKey(r.Context(), db.SetIssueMetadataKeyParams{
			ID:          issue.ID,
			WorkspaceID: issue.WorkspaceID,
			Key:         key,
			Value:       raw,
		})
		if err != nil {
			return err
		}
		updated = next
	}
	actorType, actorID := h.resolveActor(r, userID, workspaceID)
	h.publish(protocol.EventIssueMetadataChanged, workspaceID, actorType, actorID, map[string]any{
		"issue_id": uuidToString(issue.ID),
		"metadata": parseIssueMetadata(updated.Metadata),
	})
	return nil
}

func mappingMetadataValue(mapping db.SpecIssueMapping) string {
	if mapping.ModuleID.Valid {
		return uuidToString(mapping.ModuleID)
	}
	if mapping.EpicID.Valid {
		return uuidToString(mapping.EpicID)
	}
	return ""
}

func (req *UpdateIssueSpecStateRequest) applyDefaults() {
	if req.Status == "" {
		req.Status = "todo"
	}
	if req.CurrentStage == "" {
		req.CurrentStage = "requirements"
	}
	if req.CurrentLoop == "" {
		req.CurrentLoop = req.CurrentStage
	}
	if req.LastResult == "" {
		req.LastResult = "pending"
	}
	if req.AuditMode == "" {
		req.AuditMode = "required"
	}
}

func validSpecIssueStateRequest(w http.ResponseWriter, req UpdateIssueSpecStateRequest) bool {
	if !stringIn(req.Status, []string{"todo", "in_progress", "review", "blocked", "done"}) {
		writeError(w, http.StatusBadRequest, "invalid status")
		return false
	}
	if !stringIn(req.CurrentStage, []string{"requirements", "design", "implementation", "testing", "review", "acceptance"}) {
		writeError(w, http.StatusBadRequest, "invalid current_stage")
		return false
	}
	if !stringIn(req.CurrentLoop, []string{"requirements", "design", "implementation", "design-audit", "fix", "re-audit", "testing", "code-review", "acceptance"}) {
		writeError(w, http.StatusBadRequest, "invalid current_loop")
		return false
	}
	if !stringIn(req.LastResult, []string{"pending", "passed", "failed", "conditional"}) {
		writeError(w, http.StatusBadRequest, "invalid last_result")
		return false
	}
	if !stringIn(req.AuditMode, []string{"required", "skipped"}) {
		writeError(w, http.StatusBadRequest, "invalid audit_mode")
		return false
	}
	if req.AuditSkipped && strings.TrimSpace(req.AuditSkipReason) == "" {
		writeError(w, http.StatusBadRequest, "audit_skip_reason is required when audit is skipped")
		return false
	}
	return true
}

func validSpecDocumentKind(kind string) bool {
	return stringIn(kind, []string{"index", "plan", "risks", "requirements", "design", "architecture", "backend", "frontend", "implementation", "testing", "review", "acceptance", "glossary"})
}

func validSpecStability(stability string) bool {
	if strings.TrimSpace(stability) == "" {
		return true
	}
	return stringIn(stability, []string{"draft", "active", "stable", "deprecated"})
}

func firstNonEmptyString(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func stringIn(value string, allowed []string) bool {
	for _, item := range allowed {
		if value == item {
			return true
		}
	}
	return false
}

func decodeStringArray(raw []byte) []string {
	if len(raw) == 0 {
		return []string{}
	}
	var out []string
	if err := json.Unmarshal(raw, &out); err != nil {
		return []string{}
	}
	return out
}

func nullableUUIDFromString(w http.ResponseWriter, value *string, field string) (pgtype.UUID, bool) {
	if value == nil || strings.TrimSpace(*value) == "" {
		return pgtype.UUID{}, true
	}
	if w == nil {
		var id pgtype.UUID
		if err := id.Scan(strings.TrimSpace(*value)); err != nil {
			return pgtype.UUID{}, false
		}
		return id, true
	}
	id, ok := parseUUIDOrBadRequest(w, strings.TrimSpace(*value), field)
	if !ok {
		return pgtype.UUID{}, false
	}
	return id, true
}

func writeSpecMemoryWriteError(w http.ResponseWriter, r *http.Request, err error, action string) {
	if errors.Is(err, pgx.ErrNoRows) {
		writeError(w, http.StatusNotFound, "spec memory resource not found")
		return
	}
	if isCheckViolation(err) {
		writeError(w, http.StatusBadRequest, "spec memory "+action+" rejected by protocol constraints")
		return
	}
	slog.Warn("spec memory "+action+" failed", append(logger.RequestAttrs(r), "error", err)...)
	writeError(w, http.StatusInternalServerError, "failed to "+action)
}
