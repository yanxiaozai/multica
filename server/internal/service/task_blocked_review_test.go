package service

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/multica-ai/multica/server/internal/events"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"github.com/multica-ai/multica/server/pkg/taskfailure"
)

func TestFailTask_AgentBlockedPromotesIssueToReview(t *testing.T) {
	ctx := context.Background()
	pool := newTaskClaimRacePool(t)
	queries := db.New(pool)

	workspaceID, userID, agentID, runtimeID, issueID, taskID := createBlockedReviewFixture(t, ctx, pool)
	svc := NewTaskService(queries, pool, nil, events.New())

	if _, err := svc.FailTask(ctx, util.MustParseUUID(taskID), "external screenshot environment is unavailable", "", "", taskfailure.ReasonAgentBlocked.String()); err != nil {
		t.Fatalf("FailTask: %v", err)
	}

	var status string
	if err := pool.QueryRow(ctx, `SELECT status FROM issue WHERE id = $1`, issueID).Scan(&status); err != nil {
		t.Fatalf("query issue status: %v", err)
	}
	if status != "in_review" {
		t.Fatalf("issue status = %q, want in_review", status)
	}

	_ = workspaceID
	_ = userID
	_ = agentID
	_ = runtimeID
}

func createBlockedReviewFixture(t *testing.T, ctx context.Context, pool db.DBTX) (workspaceID, userID, agentID, runtimeID, issueID, taskID string) {
	t.Helper()

	suffix := time.Now().UnixNano()
	if err := pool.QueryRow(ctx, `
		INSERT INTO "user" (name, email)
		VALUES ($1, $2)
		RETURNING id
	`, "Blocked Review Test", fmt.Sprintf("blocked-review-%d@multica.ai", suffix)).Scan(&userID); err != nil {
		t.Fatalf("create user: %v", err)
	}

	if err := pool.QueryRow(ctx, `
		INSERT INTO workspace (name, slug, description, issue_prefix)
		VALUES ($1, $2, $3, $4)
		RETURNING id
	`, "Blocked Review Test", fmt.Sprintf("blocked-review-%d", suffix), "temporary blocked review test workspace", "BRT").Scan(&workspaceID); err != nil {
		t.Fatalf("create workspace: %v", err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO member (workspace_id, user_id, role)
		VALUES ($1, $2, 'owner')
	`, workspaceID, userID); err != nil {
		t.Fatalf("create member: %v", err)
	}

	if err := pool.QueryRow(ctx, `
		INSERT INTO agent_runtime (
			workspace_id, daemon_id, name, runtime_mode, provider,
			status, device_info, metadata, last_seen_at, visibility, owner_id
		)
		VALUES ($1, NULL, $2, 'cloud', 'blocked_review_test', 'online', 'test runtime', '{}'::jsonb, now(), 'private', $3)
		RETURNING id
	`, workspaceID, "Blocked Review Runtime", userID).Scan(&runtimeID); err != nil {
		t.Fatalf("create runtime: %v", err)
	}

	if err := pool.QueryRow(ctx, `
		INSERT INTO agent (
			workspace_id, name, description, runtime_mode, runtime_config,
			runtime_id, visibility, max_concurrent_tasks, owner_id
		)
		VALUES ($1, $2, '', 'cloud', '{}'::jsonb, $3, 'private', 1, $4)
		RETURNING id
	`, workspaceID, "Blocked Review Agent", runtimeID, userID).Scan(&agentID); err != nil {
		t.Fatalf("create agent: %v", err)
	}

	if err := pool.QueryRow(ctx, `
		INSERT INTO issue (workspace_id, title, status, priority, creator_id, creator_type, assignee_type, assignee_id, number, position)
		VALUES ($1, 'blocked review issue', 'in_progress', 'none', $2, 'member', 'agent', $3, 910001, 0)
		RETURNING id
	`, workspaceID, userID, agentID).Scan(&issueID); err != nil {
		t.Fatalf("create issue: %v", err)
	}

	if err := pool.QueryRow(ctx, `
		INSERT INTO agent_task_queue (agent_id, issue_id, runtime_id, status, priority, context, started_at)
		VALUES ($1, $2, $3, 'running', 0, '{}'::jsonb, now())
		RETURNING id
	`, agentID, issueID, runtimeID).Scan(&taskID); err != nil {
		t.Fatalf("create task: %v", err)
	}

	t.Cleanup(func() {
		cleanupCtx := context.Background()
		pool.Exec(cleanupCtx, `DELETE FROM comment WHERE issue_id = $1`, issueID)
		pool.Exec(cleanupCtx, `DELETE FROM agent_task_queue WHERE issue_id = $1`, issueID)
		pool.Exec(cleanupCtx, `DELETE FROM issue WHERE id = $1`, issueID)
		pool.Exec(cleanupCtx, `DELETE FROM agent WHERE id = $1`, agentID)
		pool.Exec(cleanupCtx, `DELETE FROM agent_runtime WHERE id = $1`, runtimeID)
		pool.Exec(cleanupCtx, `DELETE FROM member WHERE workspace_id = $1 AND user_id = $2`, workspaceID, userID)
		pool.Exec(cleanupCtx, `DELETE FROM workspace WHERE id = $1`, workspaceID)
		pool.Exec(cleanupCtx, `DELETE FROM "user" WHERE id = $1`, userID)
	})

	return workspaceID, userID, agentID, runtimeID, issueID, taskID
}
