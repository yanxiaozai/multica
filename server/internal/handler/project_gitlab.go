package handler

import (
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"github.com/multica-ai/multica/server/internal/service/issuebridge"
)

// ImportProjectGitLabIssuesResponse tallies a one-shot GitLab import.
type ImportProjectGitLabIssuesResponse struct {
	Imported int      `json:"imported"`
	Skipped  int      `json:"skipped"`
	Failed   int      `json:"failed"`
	Errors   []string `json:"errors,omitempty"`
}

// ImportProjectGitLabIssues pulls GitLab issues assigned to the connection
// owner into the project's issue list. Requires the project to already have a
// GitLab sync config (integration + remote_project_ref) — configured from the
// project's GitLab sync panel. Idempotent: re-running skips already-imported
// issues.
func (h *Handler) ImportProjectGitLabIssues(w http.ResponseWriter, r *http.Request) {
	svc, ok := h.issueBridgeService(w, true)
	if !ok {
		return
	}
	workspaceID := h.resolveWorkspaceID(r)
	wsUUID, ok := parseUUIDOrBadRequest(w, workspaceID, "workspace id")
	if !ok {
		return
	}
	projectID, ok := parseUUIDOrBadRequest(w, chi.URLParam(r, "id"), "project id")
	if !ok {
		return
	}
	// Verify the project exists in this workspace before importing, so a
	// bad project id surfaces as 404 rather than a confusing sync error.
	if _, err := h.Queries.GetProjectInWorkspace(r.Context(), db.GetProjectInWorkspaceParams{
		ID: projectID, WorkspaceID: wsUUID,
	}); err != nil {
		writeError(w, http.StatusNotFound, "project not found")
		return
	}
	// Membership gate: only workspace members can trigger an import.
	if _, ok := h.workspaceMember(w, r, workspaceID); !ok {
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

	result, err := svc.ImportProjectIssues(r.Context(), wsUUID, projectID, userUUID, issuebridge.ImportOpts{
		AssignedToMe: true,
		State:        "all",
	})
	if err != nil {
		switch {
		case errors.Is(err, issuebridge.ErrSyncConfigMissing):
			writeError(w, http.StatusConflict, err.Error())
		case errors.Is(err, issuebridge.ErrIssueServiceMissing):
			writeError(w, http.StatusServiceUnavailable, err.Error())
		default:
			writeError(w, http.StatusBadGateway, "failed to import gitlab issues: "+err.Error())
		}
		return
	}
	writeJSON(w, http.StatusOK, ImportProjectGitLabIssuesResponse{
		Imported: result.Imported,
		Skipped:  result.Skipped,
		Failed:   result.Failed,
		Errors:   result.Errors,
	})
}
