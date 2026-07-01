package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/multica-ai/multica/server/internal/middleware"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

func requestAsWorkspaceRole(req *http.Request, role string) *http.Request {
	return requestAsWorkspaceUserRole(req, testUserID, role)
}

func requestAsWorkspaceUserRole(req *http.Request, userID, role string) *http.Request {
	return req.WithContext(middleware.SetMemberContext(req.Context(), testWorkspaceID, db.Member{
		WorkspaceID: util.MustParseUUID(testWorkspaceID),
		UserID:      util.MustParseUUID(userID),
		Role:        role,
	}))
}

func createSquadGenerationTestMember(t *testing.T, role string) string {
	t.Helper()
	ctx := context.Background()
	email := fmt.Sprintf("squad-generation-%s@multica.test", uuid.NewString())

	var userID string
	if err := testPool.QueryRow(ctx, `
		INSERT INTO "user" (name, email)
		VALUES ('Squad Generation Member', $1)
		RETURNING id
	`, email).Scan(&userID); err != nil {
		t.Fatalf("create squad generation member user: %v", err)
	}
	t.Cleanup(func() {
		testPool.Exec(context.Background(), `DELETE FROM "user" WHERE id = $1`, userID)
	})

	if _, err := testPool.Exec(ctx, `
		INSERT INTO member (workspace_id, user_id, role)
		VALUES ($1, $2, $3)
	`, testWorkspaceID, userID, role); err != nil {
		t.Fatalf("create squad generation member: %v", err)
	}

	return userID
}

func withSquadGenerationJobParams(req *http.Request, squadID, jobID string) *http.Request {
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", squadID)
	rctx.URLParams.Add("jobId", jobID)
	return req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
}

func TestSquadInstructionsGenerationCreateQueuesTask(t *testing.T) {
	ctx := context.Background()
	leaderID, _ := seededLeaderAgent(t)
	setAgentDocForGenerationTest(t, leaderID, "Leader description.", "leader document for generation")
	squad := seedSquadForBriefing(t, leaderID, "AI Generation Handler Squad", "")

	memberAgent := createHandlerTestAgent(t, "Generation Member Agent", []byte("[]"))
	setAgentDocForGenerationTest(t, memberAgent, "Member description.", "member document for generation")
	addAgentMember(t, squad.ID, memberAgent, "implementation")

	req := newRequest(http.MethodPost, "/api/squads/"+util.UUIDToString(squad.ID)+"/instructions/generation-jobs", map[string]any{
		"mode":  "agent_docs",
		"draft": "current draft text",
	})
	req = withURLParam(req, "id", util.UUIDToString(squad.ID))
	req = requestAsWorkspaceRole(req, "owner")

	w := httptest.NewRecorder()
	testHandler.CreateSquadInstructionsGenerationJob(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("CreateSquadInstructionsGenerationJob status = %d, body = %s", w.Code, w.Body.String())
	}

	var resp SquadInstructionsGenerationJobResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.ID == "" || resp.JobID != resp.ID {
		t.Fatalf("job id mismatch: %+v", resp)
	}
	if resp.Status != "queued" || resp.Mode != "agent_docs" {
		t.Fatalf("unexpected response: %+v", resp)
	}
	if resp.TaskID == nil || *resp.TaskID == "" {
		t.Fatalf("expected task id in response: %+v", resp)
	}
	t.Cleanup(func() {
		testPool.Exec(ctx, `DELETE FROM agent_task_queue WHERE id = $1`, *resp.TaskID)
	})

	task, err := testHandler.Queries.GetAgentTask(ctx, util.MustParseUUID(*resp.TaskID))
	if err != nil {
		t.Fatalf("load queued task: %v", err)
	}
	var taskContext struct {
		Type   string `json:"type"`
		Prompt string `json:"prompt"`
	}
	if err := json.Unmarshal(task.Context, &taskContext); err != nil {
		t.Fatalf("decode task context: %v", err)
	}
	if taskContext.Type != "squad_instructions_generation" ||
		!strings.Contains(taskContext.Prompt, "member document for generation") {
		t.Fatalf("task context missing generation prompt: %s", string(task.Context))
	}
}

func TestSquadInstructionsGenerationRejectsInvalidMode(t *testing.T) {
	leaderID, _ := seededLeaderAgent(t)
	squad := seedSquadForBriefing(t, leaderID, "AI Generation Invalid Mode Squad", "")

	req := newRequest(http.MethodPost, "/api/squads/"+util.UUIDToString(squad.ID)+"/instructions/generation-jobs", map[string]any{
		"mode": "template",
	})
	req = withURLParam(req, "id", util.UUIDToString(squad.ID))
	req = requestAsWorkspaceRole(req, "owner")

	w := httptest.NewRecorder()
	testHandler.CreateSquadInstructionsGenerationJob(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400, body = %s", w.Code, w.Body.String())
	}
}

func TestSquadInstructionsGenerationRejectsArchivedLeader(t *testing.T) {
	leaderID := createHandlerTestAgent(t, "Generation No Runtime Leader", []byte("[]"))
	if _, err := testPool.Exec(context.Background(), `UPDATE agent SET archived_at = now() WHERE id = $1`, leaderID); err != nil {
		t.Fatalf("archive leader: %v", err)
	}
	squad := seedSquadForBriefing(t, leaderID, "AI Generation Archived Leader Squad", "")

	req := newRequest(http.MethodPost, "/api/squads/"+util.UUIDToString(squad.ID)+"/instructions/generation-jobs", map[string]any{
		"mode": "agent_docs",
	})
	req = withURLParam(req, "id", util.UUIDToString(squad.ID))
	req = requestAsWorkspaceRole(req, "owner")

	w := httptest.NewRecorder()
	testHandler.CreateSquadInstructionsGenerationJob(w, req)
	if w.Code != http.StatusBadRequest || !strings.Contains(w.Body.String(), "leader agent is archived") {
		t.Fatalf("status/body = %d/%s, want archived-leader 400", w.Code, w.Body.String())
	}
}

func TestSquadInstructionsGenerationCreateRequiresOwnerOrAdmin(t *testing.T) {
	leaderID, _ := seededLeaderAgent(t)
	squad := seedSquadForBriefing(t, leaderID, "AI Generation Role Squad", "")

	req := newRequest(http.MethodPost, "/api/squads/"+util.UUIDToString(squad.ID)+"/instructions/generation-jobs", map[string]any{
		"mode": "agent_docs",
	})
	req = withURLParam(req, "id", util.UUIDToString(squad.ID))
	memberID := createSquadGenerationTestMember(t, "member")
	req = newRequestAs(memberID, http.MethodPost, "/api/squads/"+util.UUIDToString(squad.ID)+"/instructions/generation-jobs", map[string]any{
		"mode": "agent_docs",
	})
	req = withURLParam(req, "id", util.UUIDToString(squad.ID))
	req = requestAsWorkspaceUserRole(req, memberID, "member")

	w := httptest.NewRecorder()
	testHandler.CreateSquadInstructionsGenerationJob(w, req)
	if w.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403, body = %s", w.Code, w.Body.String())
	}
}

func TestSquadInstructionsGenerationGetReturnsJob(t *testing.T) {
	ctx := context.Background()
	leaderID, _ := seededLeaderAgent(t)
	squad := seedSquadForBriefing(t, leaderID, "AI Generation Get Squad", "")

	job, err := testHandler.Queries.CreateSquadInstructionsGenerationJob(ctx, db.CreateSquadInstructionsGenerationJobParams{
		WorkspaceID: util.MustParseUUID(testWorkspaceID),
		SquadID:     squad.ID,
		CreatedBy:   util.MustParseUUID(testUserID),
		Mode:        "agent_docs",
		Draft:       "draft",
	})
	if err != nil {
		t.Fatalf("create job: %v", err)
	}

	req := newRequest(http.MethodGet, "/api/squads/"+util.UUIDToString(squad.ID)+"/instructions/generation-jobs/"+util.UUIDToString(job.ID), nil)
	req = withSquadGenerationJobParams(req, util.UUIDToString(squad.ID), util.UUIDToString(job.ID))
	req = requestAsWorkspaceRole(req, "member")

	w := httptest.NewRecorder()
	testHandler.GetSquadInstructionsGenerationJob(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("GetSquadInstructionsGenerationJob status = %d, body = %s", w.Code, w.Body.String())
	}

	var resp SquadInstructionsGenerationJobResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.ID != util.UUIDToString(job.ID) || resp.Status != "queued" {
		t.Fatalf("unexpected response: %+v", resp)
	}
}
