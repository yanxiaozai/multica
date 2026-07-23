package handler

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

type CreateSquadInstructionsGenerationRequest struct {
	Mode  string `json:"mode"`
	Draft string `json:"draft"`
}

type SquadInstructionsGenerationJobResponse struct {
	ID           string  `json:"id"`
	JobID        string  `json:"job_id,omitempty"`
	TaskID       *string `json:"task_id"`
	Status       string  `json:"status"`
	Mode         string  `json:"mode"`
	Instructions string  `json:"instructions"`
	Error        *string `json:"error"`
	CreatedAt    string  `json:"created_at"`
	UpdatedAt    string  `json:"updated_at"`
	CompletedAt  *string `json:"completed_at"`
}

func squadInstructionsGenerationJobToResponse(job db.SquadInstructionsGenerationJob) SquadInstructionsGenerationJobResponse {
	id := uuidToString(job.ID)
	return SquadInstructionsGenerationJobResponse{
		ID:           id,
		JobID:        id,
		TaskID:       uuidToPtr(job.TaskID),
		Status:       job.Status,
		Mode:         job.Mode,
		Instructions: job.Instructions,
		Error:        textToPtr(job.Error),
		CreatedAt:    timestampToString(job.CreatedAt),
		UpdatedAt:    timestampToString(job.UpdatedAt),
		CompletedAt:  timestampToPtr(job.CompletedAt),
	}
}

func (h *Handler) CreateSquadInstructionsGenerationJob(w http.ResponseWriter, r *http.Request) {
	userID, ok := requireUserID(w, r)
	if !ok {
		return
	}
	workspaceID := workspaceIDFromURL(r, "workspaceId")
	if _, ok := h.requireWorkspaceRole(w, r, workspaceID, "workspace not found", "owner", "admin"); !ok {
		return
	}

	squad, _, ok := h.loadSquadInWorkspace(w, r)
	if !ok {
		return
	}

	var req CreateSquadInstructionsGenerationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Mode == "" {
		req.Mode = "agent_docs"
	}
	if req.Mode != "agent_docs" {
		writeError(w, http.StatusBadRequest, "mode must be agent_docs")
		return
	}

	leader, err := h.Queries.GetAgent(r.Context(), squad.LeaderID)
	if err != nil || leader.ArchivedAt.Valid {
		writeError(w, http.StatusBadRequest, "leader agent is archived")
		return
	}
	if !leader.RuntimeID.Valid {
		writeError(w, http.StatusBadRequest, "leader agent has no runtime")
		return
	}

	prompt, err := h.buildSquadInstructionsGenerationPrompt(r.Context(), squad, req.Draft)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to build generation prompt")
		return
	}

	job, err := h.Queries.CreateSquadInstructionsGenerationJob(r.Context(), db.CreateSquadInstructionsGenerationJobParams{
		WorkspaceID: util.MustParseUUID(workspaceID),
		SquadID:     squad.ID,
		CreatedBy:   util.MustParseUUID(userID),
		Mode:        req.Mode,
		Draft:       req.Draft,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create generation job")
		return
	}

	task, err := h.TaskService.EnqueueSquadInstructionsGenerationTask(r.Context(), leader, job, prompt)
	if err != nil {
		_, _ = h.Queries.FailSquadInstructionsGenerationJob(r.Context(), db.FailSquadInstructionsGenerationJobParams{
			ID:    job.ID,
			Error: pgtype.Text{String: err.Error(), Valid: true},
		})
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	job, err = h.Queries.AttachSquadInstructionsGenerationTask(r.Context(), db.AttachSquadInstructionsGenerationTaskParams{
		ID:     job.ID,
		TaskID: task.ID,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to attach generation task")
		return
	}

	writeJSON(w, http.StatusCreated, squadInstructionsGenerationJobToResponse(job))
}

func (h *Handler) GetSquadInstructionsGenerationJob(w http.ResponseWriter, r *http.Request) {
	squad, workspaceID, ok := h.loadSquadInWorkspace(w, r)
	if !ok {
		return
	}
	if _, ok := h.workspaceMember(w, r, workspaceID); !ok {
		return
	}

	jobID, ok := parseUUIDOrBadRequest(w, chi.URLParam(r, "jobId"), "generation job id")
	if !ok {
		return
	}
	job, err := h.Queries.GetSquadInstructionsGenerationJobInWorkspace(r.Context(), db.GetSquadInstructionsGenerationJobInWorkspaceParams{
		ID:          jobID,
		SquadID:     squad.ID,
		WorkspaceID: util.MustParseUUID(workspaceID),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeError(w, http.StatusNotFound, "generation job not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to load generation job")
		return
	}
	writeJSON(w, http.StatusOK, squadInstructionsGenerationJobToResponse(job))
}
