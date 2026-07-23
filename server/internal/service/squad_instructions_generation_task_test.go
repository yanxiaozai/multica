package service

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/multica-ai/multica/server/internal/events"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

type squadInstructionsGenerationTaskFixture struct {
	queries     *db.Queries
	pool        *pgxpool.Pool
	service     *TaskService
	workspaceID pgtype.UUID
	userID      pgtype.UUID
	runtimeID   pgtype.UUID
	agentID     pgtype.UUID
	squadID     pgtype.UUID
	taskID      pgtype.UUID
	jobID       pgtype.UUID
}

func newSquadInstructionsGenerationTaskFixture(t *testing.T) squadInstructionsGenerationTaskFixture {
	t.Helper()
	ctx := context.Background()
	pool := newTaskClaimRacePool(t)
	queries := db.New(pool)
	svc := NewTaskService(queries, pool, nil, events.New())

	suffix := time.Now().UnixNano()
	email := fmt.Sprintf("squad-gen-%d@multica.test", suffix)

	var userID pgtype.UUID
	if err := pool.QueryRow(ctx, `
		INSERT INTO "user" (name, email)
		VALUES ($1, $2)
		RETURNING id
	`, "Squad Generation Tester", email).Scan(&userID); err != nil {
		t.Fatalf("create user: %v", err)
	}
	t.Cleanup(func() {
		pool.Exec(context.Background(), `DELETE FROM "user" WHERE id = $1`, userID)
	})

	var workspaceID pgtype.UUID
	if err := pool.QueryRow(ctx, `
		INSERT INTO workspace (name, slug, description, issue_prefix)
		VALUES ($1, $2, '', 'SGT')
		RETURNING id
	`, "Squad Generation Workspace", fmt.Sprintf("squad-generation-%d", suffix)).Scan(&workspaceID); err != nil {
		t.Fatalf("create workspace: %v", err)
	}
	t.Cleanup(func() {
		pool.Exec(context.Background(), `DELETE FROM workspace WHERE id = $1`, workspaceID)
	})

	if _, err := pool.Exec(ctx, `
		INSERT INTO member (workspace_id, user_id, role)
		VALUES ($1, $2, 'owner')
	`, workspaceID, userID); err != nil {
		t.Fatalf("create member: %v", err)
	}

	var runtimeID pgtype.UUID
	if err := pool.QueryRow(ctx, `
		INSERT INTO agent_runtime (
			workspace_id, daemon_id, name, runtime_mode, provider, status, device_info, metadata, owner_id, last_seen_at
		)
		VALUES ($1, NULL, 'Generation Runtime', 'cloud', 'test', 'online', 'test runtime', '{}'::jsonb, $2, now())
		RETURNING id
	`, workspaceID, userID).Scan(&runtimeID); err != nil {
		t.Fatalf("create runtime: %v", err)
	}

	var agentID pgtype.UUID
	if err := pool.QueryRow(ctx, `
		INSERT INTO agent (
			workspace_id, name, description, runtime_mode, runtime_config,
			runtime_id, visibility, max_concurrent_tasks, owner_id,
			instructions, custom_env, custom_args, mcp_config
		)
		VALUES ($1, 'Generation Leader', '', 'cloud', '{}'::jsonb, $2, 'workspace', 1, $3, '', '{}'::jsonb, '[]'::jsonb, '{}'::jsonb)
		RETURNING id
	`, workspaceID, runtimeID, userID).Scan(&agentID); err != nil {
		t.Fatalf("create agent: %v", err)
	}

	var squadID pgtype.UUID
	if err := pool.QueryRow(ctx, `
		INSERT INTO squad (workspace_id, name, description, leader_id, creator_id)
		VALUES ($1, 'Generation Squad', '', $2, $3)
		RETURNING id
	`, workspaceID, agentID, userID).Scan(&squadID); err != nil {
		t.Fatalf("create squad: %v", err)
	}

	job, err := queries.CreateSquadInstructionsGenerationJob(ctx, db.CreateSquadInstructionsGenerationJobParams{
		WorkspaceID: workspaceID,
		SquadID:     squadID,
		CreatedBy:   userID,
		Mode:        "agent_docs",
		Draft:       "",
	})
	if err != nil {
		t.Fatalf("create job: %v", err)
	}

	contextJSON, err := json.Marshal(SquadInstructionsGenerationContext{
		Type:            SquadInstructionsGenerationContextType,
		WorkspaceID:     util.UUIDToString(workspaceID),
		SquadID:         util.UUIDToString(squadID),
		GenerationJobID: util.UUIDToString(job.ID),
		Prompt:          "generate instructions",
	})
	if err != nil {
		t.Fatalf("marshal context: %v", err)
	}

	var taskID pgtype.UUID
	if err := pool.QueryRow(ctx, `
		INSERT INTO agent_task_queue (agent_id, runtime_id, status, priority, context, started_at)
		VALUES ($1, $2, 'running', 2, $3, now())
		RETURNING id
	`, agentID, runtimeID, contextJSON).Scan(&taskID); err != nil {
		t.Fatalf("create task: %v", err)
	}
	if _, err := queries.AttachSquadInstructionsGenerationTask(ctx, db.AttachSquadInstructionsGenerationTaskParams{
		ID:     job.ID,
		TaskID: taskID,
	}); err != nil {
		t.Fatalf("attach task: %v", err)
	}

	return squadInstructionsGenerationTaskFixture{
		queries:     queries,
		pool:        pool,
		service:     svc,
		workspaceID: workspaceID,
		userID:      userID,
		runtimeID:   runtimeID,
		agentID:     agentID,
		squadID:     squadID,
		taskID:      taskID,
		jobID:       job.ID,
	}
}

func TestCompleteTask_SquadInstructionsGenerationWritesJobResult(t *testing.T) {
	ctx := context.Background()
	f := newSquadInstructionsGenerationTaskFixture(t)
	payload, err := json.Marshal(protocol.TaskCompletedPayload{
		TaskID: util.UUIDToString(f.taskID),
		Output: "## Delegation Strategy\\n\\n- Route backend work to Backend.",
	})
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}

	if _, err := f.service.CompleteTask(ctx, f.taskID, payload, "", ""); err != nil {
		t.Fatalf("complete task: %v", err)
	}

	job, err := f.queries.GetSquadInstructionsGenerationJobByTask(ctx, f.taskID)
	if err != nil {
		t.Fatalf("load job: %v", err)
	}
	if job.Status != "completed" {
		t.Fatalf("job status = %q, want completed", job.Status)
	}
	if job.Instructions != "## Delegation Strategy\n\n- Route backend work to Backend." {
		t.Fatalf("unexpected instructions: %q", job.Instructions)
	}
}

func TestClaimTask_SquadInstructionsGenerationMarksJobRunning(t *testing.T) {
	ctx := context.Background()
	f := newSquadInstructionsGenerationTaskFixture(t)

	if _, err := f.pool.Exec(ctx, `
		UPDATE agent_task_queue
		SET status = 'queued', started_at = NULL, dispatched_at = NULL, prepare_lease_expires_at = NULL
		WHERE id = $1
	`, f.taskID); err != nil {
		t.Fatalf("reset task to queued: %v", err)
	}
	before, err := f.queries.GetSquadInstructionsGenerationJobByTask(ctx, f.taskID)
	if err != nil {
		t.Fatalf("load job before claim: %v", err)
	}
	if before.Status != "queued" {
		t.Fatalf("job status before claim = %q, want queued", before.Status)
	}

	claimed, err := f.service.ClaimTaskForRuntime(ctx, f.runtimeID)
	if err != nil {
		t.Fatalf("claim task: %v", err)
	}
	if claimed == nil || claimed.ID != f.taskID {
		t.Fatalf("claimed task = %+v, want %s", claimed, util.UUIDToString(f.taskID))
	}

	job, err := f.queries.GetSquadInstructionsGenerationJobByTask(ctx, f.taskID)
	if err != nil {
		t.Fatalf("load job: %v", err)
	}
	if job.Status != "running" {
		t.Fatalf("job status = %q, want running", job.Status)
	}
}

func TestResolveTaskWorkspaceID_SquadInstructionsGeneration(t *testing.T) {
	ctx := context.Background()
	f := newSquadInstructionsGenerationTaskFixture(t)

	task, err := f.queries.GetAgentTask(ctx, f.taskID)
	if err != nil {
		t.Fatalf("load task: %v", err)
	}

	got := f.service.ResolveTaskWorkspaceID(ctx, task)
	want := util.UUIDToString(f.workspaceID)
	if got != want {
		t.Fatalf("ResolveTaskWorkspaceID = %q, want %q", got, want)
	}
}

func TestCompleteTask_SquadInstructionsGenerationEmptyOutputFailsJob(t *testing.T) {
	ctx := context.Background()
	f := newSquadInstructionsGenerationTaskFixture(t)
	payload, err := json.Marshal(protocol.TaskCompletedPayload{TaskID: util.UUIDToString(f.taskID)})
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}

	if _, err := f.service.CompleteTask(ctx, f.taskID, payload, "", ""); err != nil {
		t.Fatalf("complete task: %v", err)
	}

	job, err := f.queries.GetSquadInstructionsGenerationJobByTask(ctx, f.taskID)
	if err != nil {
		t.Fatalf("load job: %v", err)
	}
	if job.Status != "failed" || job.Error.String != "AI generation completed without output" {
		t.Fatalf("unexpected job failure state: status=%q error=%q", job.Status, job.Error.String)
	}
}

func TestFailTask_SquadInstructionsGenerationFailsJob(t *testing.T) {
	ctx := context.Background()
	f := newSquadInstructionsGenerationTaskFixture(t)

	if _, err := f.service.FailTask(ctx, f.taskID, "provider failed", "", "", "agent_error"); err != nil {
		t.Fatalf("fail task: %v", err)
	}

	job, err := f.queries.GetSquadInstructionsGenerationJobByTask(ctx, f.taskID)
	if err != nil {
		t.Fatalf("load job: %v", err)
	}
	if job.Status != "failed" || job.Error.String != "provider failed" {
		t.Fatalf("unexpected job failure state: status=%q error=%q", job.Status, job.Error.String)
	}
}
