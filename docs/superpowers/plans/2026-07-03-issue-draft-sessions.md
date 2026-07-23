# Issue Draft Sessions Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build a dedicated Issue Draft Session flow that lets users clarify issues with a squad, delegate read-only analysis to real squad member agents, review structured drafts, then confirm creation with Multica issue, target-repo `.spec` commit/push, and concise remote Git issue output.

**Architecture:** Add a new backend domain (`issuedraft`) with database tables, service orchestration, and handler routes. Reuse `IssueService.Create`, project resources, squad/member queries, task queue infrastructure, and issue bridge boundaries instead of inventing parallel systems. Add `packages/core/issue-drafts` for typed API/query hooks and shared `packages/views/issue-drafts` screens for web/desktop wiring.

**Tech Stack:** Go 1.26, Chi handlers, pgx/sqlc migrations, PostgreSQL JSONB, TanStack Query, Zustand for local UI state only, React shared views, existing daemon/task execution path, existing issue bridge/provider services.

---

## Scope Check

This feature spans storage, API, agent orchestration, confirmation side effects, and UI. Implement it in vertical phases that each compile and test independently:

1. Backend read/write model and draft CRUD.
2. Leader/member task orchestration with read-only prompts.
3. Artifact generation and confirmation idempotency.
4. Core API client and schemas.
5. Shared UI flow and platform route wiring.
6. Remote Git/provider and daemon write/push integration hardening.

Do not attempt all phases in one commit.

## File Map

Create:

- `server/internal/service/issuedraft/service.go` - service entry point for session CRUD, resource resolution, artifact validation, and confirmation orchestration.
- `server/internal/service/issuedraft/templates.go` - detailed `.spec`, Multica issue, and remote issue template helpers.
- `server/internal/service/issuedraft/types.go` - service-level input/result structs and status constants.
- `server/internal/handler/issue_draft.go` - HTTP request/response structs and handler methods.
- `server/internal/handler/issue_draft_test.go` - handler boundary tests.
- `server/pkg/db/queries/issue_draft.sql` - sqlc queries.
- `packages/core/issue-drafts/types.ts` - frontend-facing types.
- `packages/core/issue-drafts/queries.ts` - query keys and query options.
- `packages/core/issue-drafts/mutations.ts` - mutation hooks.
- `packages/core/issue-drafts/index.ts` - exports.
- `packages/views/issue-drafts/issue-draft-page.tsx` - shared draft session page.
- `packages/views/issue-drafts/components/draft-preview-tabs.tsx` - artifact preview tabs.
- `packages/views/issue-drafts/components/member-findings-panel.tsx` - delegated member task status.
- `packages/views/issue-drafts/components/draft-composer.tsx` - user reply composer.
- `packages/views/issue-drafts/index.ts` - exports.

Modify:

- `server/internal/migrations/migrations.go` - add migration for issue draft tables.
- `server/cmd/server/router.go` - register `/api/issue-drafts` routes.
- `server/internal/handler/handler.go` or composition root - add `IssueDraftService` dependency.
- `server/internal/service/task.go` - add draft task contexts and enqueue helpers.
- `server/internal/daemon/types.go` - expose draft task fields to daemon claim.
- `server/internal/daemon/prompt.go` - build leader/member draft prompts.
- `server/internal/handler/agent.go` - populate draft task claim payload fields.
- `packages/core/api/client.ts` - add issue draft methods.
- `packages/core/api/schemas.ts` - add zod schemas.
- `packages/core/api/schemas.test.ts` - malformed-response tests.
- `packages/core/index.ts` - export issue draft module.
- `packages/views/package.json` - export `./issue-drafts` to shared consumers.
- `apps/web/app/[workspaceSlug]/(dashboard)/issue-drafts/[id]/page.tsx` - add web route to shared draft page.
- `apps/desktop/src/renderer/src/routes.tsx` - add desktop session route.
- `packages/views/modals/create-issue-dialog.tsx` or modal entry point - add "Clarify with squad" path.

## Task 1: Database Model and sqlc Queries

**Files:**
- Modify: `server/internal/migrations/migrations.go`
- Create: `server/pkg/db/queries/issue_draft.sql`
- Generated: `server/pkg/db/generated/*.go`

- [ ] **Step 1: Add failing query generation expectation**

Before adding SQL, run:

```bash
make sqlc
```

Expected: no generated issue draft query types exist, and later code that imports `CreateIssueDraftSessionParams` would not compile. This establishes that generation is required after SQL is added.

- [ ] **Step 2: Add migration**

Add a migration function in `server/internal/migrations/migrations.go` following the existing migration style. The migration must create these tables:

```sql
CREATE TABLE IF NOT EXISTS issue_draft_session (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id uuid NOT NULL REFERENCES workspace(id) ON DELETE CASCADE,
    project_id uuid NOT NULL REFERENCES project(id) ON DELETE CASCADE,
    squad_id uuid NOT NULL REFERENCES squad(id) ON DELETE RESTRICT,
    leader_agent_id uuid NOT NULL REFERENCES agent(id) ON DELETE RESTRICT,
    primary_project_resource_id uuid NOT NULL REFERENCES project_resource(id) ON DELETE RESTRICT,
    primary_local_path_snapshot text NOT NULL,
    status text NOT NULL DEFAULT 'clarifying',
    created_by uuid NOT NULL REFERENCES app_user(id) ON DELETE RESTRICT,
    created_issue_id uuid REFERENCES issue(id) ON DELETE SET NULL,
    remote_issue_url text NOT NULL DEFAULT '',
    spec_file_path text NOT NULL DEFAULT '',
    git_commit_sha text NOT NULL DEFAULT '',
    last_error text NOT NULL DEFAULT '',
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT issue_draft_session_status_chk CHECK (
        status IN ('clarifying', 'delegating', 'drafting', 'ready_for_review', 'creating', 'created', 'failed', 'cancelled')
    )
);

CREATE TABLE IF NOT EXISTS issue_draft_message (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    session_id uuid NOT NULL REFERENCES issue_draft_session(id) ON DELETE CASCADE,
    workspace_id uuid NOT NULL REFERENCES workspace(id) ON DELETE CASCADE,
    author_type text NOT NULL,
    author_id uuid,
    message_type text NOT NULL,
    content text NOT NULL,
    metadata jsonb NOT NULL DEFAULT '{}'::jsonb,
    created_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT issue_draft_message_author_type_chk CHECK (author_type IN ('member', 'agent', 'system')),
    CONSTRAINT issue_draft_message_type_chk CHECK (
        message_type IN ('user_message', 'question', 'answer', 'finding', 'draft_preview', 'status', 'error')
    )
);

CREATE TABLE IF NOT EXISTS issue_draft_member_task (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    session_id uuid NOT NULL REFERENCES issue_draft_session(id) ON DELETE CASCADE,
    workspace_id uuid NOT NULL REFERENCES workspace(id) ON DELETE CASCADE,
    agent_id uuid NOT NULL REFERENCES agent(id) ON DELETE RESTRICT,
    task_id uuid REFERENCES agent_task_queue(id) ON DELETE SET NULL,
    status text NOT NULL DEFAULT 'queued',
    skill_basis text NOT NULL DEFAULT '',
    read_scope jsonb NOT NULL DEFAULT '{}'::jsonb,
    findings text NOT NULL DEFAULT '',
    error text NOT NULL DEFAULT '',
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT issue_draft_member_task_status_chk CHECK (
        status IN ('queued', 'running', 'completed', 'failed', 'cancelled')
    )
);

CREATE TABLE IF NOT EXISTS issue_draft_artifact (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    session_id uuid NOT NULL REFERENCES issue_draft_session(id) ON DELETE CASCADE,
    workspace_id uuid NOT NULL REFERENCES workspace(id) ON DELETE CASCADE,
    artifact_type text NOT NULL,
    revision integer NOT NULL,
    content text NOT NULL,
    generated_by_agent_id uuid REFERENCES agent(id) ON DELETE SET NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT issue_draft_artifact_type_chk CHECK (
        artifact_type IN ('detailed_spec', 'multica_issue', 'remote_issue')
    ),
    UNIQUE (session_id, artifact_type, revision)
);

CREATE TABLE IF NOT EXISTS issue_draft_confirm_step (
    session_id uuid NOT NULL REFERENCES issue_draft_session(id) ON DELETE CASCADE,
    workspace_id uuid NOT NULL REFERENCES workspace(id) ON DELETE CASCADE,
    step text NOT NULL,
    status text NOT NULL DEFAULT 'pending',
    external_id text NOT NULL DEFAULT '',
    result_metadata jsonb NOT NULL DEFAULT '{}'::jsonb,
    error text NOT NULL DEFAULT '',
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (session_id, step),
    CONSTRAINT issue_draft_confirm_step_name_chk CHECK (
        step IN ('create_multica_issue', 'write_spec', 'commit_and_push', 'create_remote_issue', 'link_outputs')
    ),
    CONSTRAINT issue_draft_confirm_step_status_chk CHECK (
        status IN ('pending', 'running', 'succeeded', 'failed')
    )
);

CREATE INDEX IF NOT EXISTS idx_issue_draft_session_workspace_created
    ON issue_draft_session (workspace_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_issue_draft_message_session_created
    ON issue_draft_message (session_id, created_at ASC);
CREATE INDEX IF NOT EXISTS idx_issue_draft_member_task_session
    ON issue_draft_member_task (session_id, created_at ASC);
CREATE INDEX IF NOT EXISTS idx_issue_draft_artifact_session_type_revision
    ON issue_draft_artifact (session_id, artifact_type, revision DESC);
```

- [ ] **Step 3: Add sqlc queries**

Create `server/pkg/db/queries/issue_draft.sql`:

```sql
-- name: CreateIssueDraftSession :one
INSERT INTO issue_draft_session (
    workspace_id, project_id, squad_id, leader_agent_id,
    primary_project_resource_id, primary_local_path_snapshot, created_by
) VALUES (
    $1, $2, $3, $4, $5, $6, $7
) RETURNING *;

-- name: GetIssueDraftSessionInWorkspace :one
SELECT * FROM issue_draft_session
WHERE id = $1 AND workspace_id = $2;

-- name: ListIssueDraftSessions :many
SELECT * FROM issue_draft_session
WHERE workspace_id = $1
ORDER BY created_at DESC
LIMIT $2 OFFSET $3;

-- name: UpdateIssueDraftSessionStatus :one
UPDATE issue_draft_session
SET status = $3, last_error = $4, updated_at = now()
WHERE id = $1 AND workspace_id = $2
RETURNING *;

-- name: MarkIssueDraftCreated :one
UPDATE issue_draft_session
SET status = 'created',
    created_issue_id = $3,
    remote_issue_url = $4,
    spec_file_path = $5,
    git_commit_sha = $6,
    last_error = '',
    updated_at = now()
WHERE id = $1 AND workspace_id = $2
RETURNING *;

-- name: CreateIssueDraftMessage :one
INSERT INTO issue_draft_message (
    session_id, workspace_id, author_type, author_id, message_type, content, metadata
) VALUES (
    $1, $2, $3, $4, $5, $6, $7
) RETURNING *;

-- name: ListIssueDraftMessages :many
SELECT * FROM issue_draft_message
WHERE session_id = $1 AND workspace_id = $2
ORDER BY created_at ASC;

-- name: CreateIssueDraftMemberTask :one
INSERT INTO issue_draft_member_task (
    session_id, workspace_id, agent_id, skill_basis, read_scope
) VALUES (
    $1, $2, $3, $4, $5
) RETURNING *;

-- name: UpdateIssueDraftMemberTaskQueuedTask :one
UPDATE issue_draft_member_task
SET task_id = $3, status = 'queued', updated_at = now()
WHERE id = $1 AND workspace_id = $2
RETURNING *;

-- name: CompleteIssueDraftMemberTask :one
UPDATE issue_draft_member_task
SET status = 'completed', findings = $3, error = '', updated_at = now()
WHERE id = $1 AND workspace_id = $2
RETURNING *;

-- name: FailIssueDraftMemberTask :one
UPDATE issue_draft_member_task
SET status = 'failed', error = $3, updated_at = now()
WHERE id = $1 AND workspace_id = $2
RETURNING *;

-- name: ListIssueDraftMemberTasks :many
SELECT * FROM issue_draft_member_task
WHERE session_id = $1 AND workspace_id = $2
ORDER BY created_at ASC;

-- name: CreateIssueDraftArtifact :one
INSERT INTO issue_draft_artifact (
    session_id, workspace_id, artifact_type, revision, content, generated_by_agent_id
) VALUES (
    $1, $2, $3, $4, $5, $6
) RETURNING *;

-- name: ListLatestIssueDraftArtifacts :many
SELECT DISTINCT ON (artifact_type) *
FROM issue_draft_artifact
WHERE session_id = $1 AND workspace_id = $2
ORDER BY artifact_type, revision DESC;

-- name: GetIssueDraftArtifact :one
SELECT * FROM issue_draft_artifact
WHERE session_id = $1 AND workspace_id = $2 AND artifact_type = $3
ORDER BY revision DESC
LIMIT 1;

-- name: UpsertIssueDraftConfirmStep :one
INSERT INTO issue_draft_confirm_step (
    session_id, workspace_id, step, status, external_id, result_metadata, error
) VALUES (
    $1, $2, $3, $4, $5, $6, $7
)
ON CONFLICT (session_id, step)
DO UPDATE SET
    status = EXCLUDED.status,
    external_id = EXCLUDED.external_id,
    result_metadata = EXCLUDED.result_metadata,
    error = EXCLUDED.error,
    updated_at = now()
RETURNING *;

-- name: ListIssueDraftConfirmSteps :many
SELECT * FROM issue_draft_confirm_step
WHERE session_id = $1 AND workspace_id = $2
ORDER BY created_at ASC;
```

- [ ] **Step 4: Generate sqlc**

Run:

```bash
make sqlc
```

Expected: generated Go methods exist for every query above.

- [ ] **Step 5: Commit**

```bash
git add server/internal/migrations/migrations.go server/pkg/db/queries/issue_draft.sql server/pkg/db/generated
git commit -m "feat(issue-drafts): add draft session storage"
```

## Task 2: Backend Service Skeleton and Resource Resolution

**Files:**
- Create: `server/internal/service/issuedraft/types.go`
- Create: `server/internal/service/issuedraft/templates.go`
- Create: `server/internal/service/issuedraft/service.go`
- Test: `server/internal/service/issuedraft/service_test.go`

- [ ] **Step 1: Write service tests for primary local repo resolution**

Create `server/internal/service/issuedraft/service_test.go` with table tests that assert:

```go
func TestCreateSessionRequiresPrimaryLocalDirectory(t *testing.T) {
    // Arrange a workspace, project, squad leader, and no local_directory resources.
    // Call Service.CreateSession.
    // Assert ErrPrimaryLocalRepositoryMissing.
}

func TestCreateSessionUsesFirstPositionedLocalDirectory(t *testing.T) {
    // Arrange two local_directory resources with positions 20 and 10.
    // Call Service.CreateSession.
    // Assert PrimaryProjectResourceID is the position 10 resource and
    // PrimaryLocalPathSnapshot equals its resource_ref.local_path.
}
```

Use the repository's existing handler/service test fixture style. Keep tests in Go; do not mock SQL with string parsing.

- [ ] **Step 2: Add service types**

Create `server/internal/service/issuedraft/types.go`:

```go
package issuedraft

import (
    "errors"

    "github.com/jackc/pgx/v5/pgtype"
    db "github.com/multica-ai/multica/server/pkg/db/generated"
)

const (
    StatusClarifying     = "clarifying"
    StatusDelegating     = "delegating"
    StatusDrafting       = "drafting"
    StatusReadyForReview = "ready_for_review"
    StatusCreating       = "creating"
    StatusCreated        = "created"
    StatusFailed         = "failed"
    StatusCancelled      = "cancelled"

    ArtifactDetailedSpec = "detailed_spec"
    ArtifactMulticaIssue = "multica_issue"
    ArtifactRemoteIssue  = "remote_issue"
)

var (
    ErrPrimaryLocalRepositoryMissing = errors.New("project has no primary local repository")
    ErrProjectNotFound               = errors.New("project not found in workspace")
    ErrSquadNotFound                 = errors.New("squad not found in workspace")
)

type CreateSessionInput struct {
    WorkspaceID   pgtype.UUID
    ProjectID     pgtype.UUID
    SquadID       pgtype.UUID
    CreatedBy     pgtype.UUID
    InitialPrompt string
}

type SessionBundle struct {
    Session     db.IssueDraftSession
    Messages    []db.IssueDraftMessage
    MemberTasks []db.IssueDraftMemberTask
    Artifacts   []db.IssueDraftArtifact
    Steps       []db.IssueDraftConfirmStep
}
```

- [ ] **Step 3: Add templates**

Create `server/internal/service/issuedraft/templates.go`:

```go
package issuedraft

const DetailedSpecTemplate = `# {{TITLE}}

## 1. Background

## 2. History Query Record

## 2.1 Split Decision

## 3. Goals

## 4. Non-Goals

## 5. Impact Scope

## 6. User Scenarios

## 7. Acceptance Criteria
- [ ]

## 8. Technical Constraints

## 9. Design and Decision Record

## 10. Implementation Task Breakdown

## 11. Verification Plan

## 12. Risks and Rollback

## 13. Related Information

## 14. Agent Work Record
`

const RemoteIssueTemplate = `## Background

## Goal

## Non-Goals

## Scope

## User Scenarios

## Acceptance Criteria
- [ ]

## References
- Multica issue:
- Detailed spec:
`
```

- [ ] **Step 4: Implement service skeleton**

Create `server/internal/service/issuedraft/service.go`:

```go
package issuedraft

import (
    "context"
    "encoding/json"
    "fmt"
    "strings"

    "github.com/jackc/pgx/v5"
    "github.com/jackc/pgx/v5/pgtype"
    db "github.com/multica-ai/multica/server/pkg/db/generated"
)

type Queries interface {
    GetProjectInWorkspace(context.Context, db.GetProjectInWorkspaceParams) (db.Project, error)
    ListProjectResources(context.Context, pgtype.UUID) ([]db.ProjectResource, error)
    GetSquadInWorkspace(context.Context, db.GetSquadInWorkspaceParams) (db.Squad, error)
    CreateIssueDraftSession(context.Context, db.CreateIssueDraftSessionParams) (db.IssueDraftSession, error)
    CreateIssueDraftMessage(context.Context, db.CreateIssueDraftMessageParams) (db.IssueDraftMessage, error)
    GetIssueDraftSessionInWorkspace(context.Context, db.GetIssueDraftSessionInWorkspaceParams) (db.IssueDraftSession, error)
    ListIssueDraftMessages(context.Context, db.ListIssueDraftMessagesParams) ([]db.IssueDraftMessage, error)
    ListIssueDraftMemberTasks(context.Context, db.ListIssueDraftMemberTasksParams) ([]db.IssueDraftMemberTask, error)
    ListLatestIssueDraftArtifacts(context.Context, db.ListLatestIssueDraftArtifactsParams) ([]db.IssueDraftArtifact, error)
    ListIssueDraftConfirmSteps(context.Context, db.ListIssueDraftConfirmStepsParams) ([]db.IssueDraftConfirmStep, error)
}

type Service struct {
    Queries Queries
}

func NewService(q Queries) *Service {
    return &Service{Queries: q}
}

func (s *Service) CreateSession(ctx context.Context, in CreateSessionInput) (SessionBundle, error) {
    if _, err := s.Queries.GetProjectInWorkspace(ctx, db.GetProjectInWorkspaceParams{
        ID: in.ProjectID, WorkspaceID: in.WorkspaceID,
    }); err != nil {
        if err == pgx.ErrNoRows {
            return SessionBundle{}, ErrProjectNotFound
        }
        return SessionBundle{}, err
    }
    squad, err := s.Queries.GetSquadInWorkspace(ctx, db.GetSquadInWorkspaceParams{
        ID: in.SquadID, WorkspaceID: in.WorkspaceID,
    })
    if err != nil {
        if err == pgx.ErrNoRows {
            return SessionBundle{}, ErrSquadNotFound
        }
        return SessionBundle{}, err
    }
    resource, localPath, err := s.primaryLocalDirectory(ctx, in.ProjectID)
    if err != nil {
        return SessionBundle{}, err
    }
    session, err := s.Queries.CreateIssueDraftSession(ctx, db.CreateIssueDraftSessionParams{
        WorkspaceID:              in.WorkspaceID,
        ProjectID:                in.ProjectID,
        SquadID:                  in.SquadID,
        LeaderAgentID:            squad.LeaderID,
        PrimaryProjectResourceID: resource.ID,
        PrimaryLocalPathSnapshot: localPath,
        CreatedBy:                in.CreatedBy,
    })
    if err != nil {
        return SessionBundle{}, err
    }
    if strings.TrimSpace(in.InitialPrompt) != "" {
        if _, err := s.Queries.CreateIssueDraftMessage(ctx, db.CreateIssueDraftMessageParams{
            SessionID:   session.ID,
            WorkspaceID: in.WorkspaceID,
            AuthorType:  "member",
            AuthorID:    in.CreatedBy,
            MessageType: "user_message",
            Content:     strings.TrimSpace(in.InitialPrompt),
            Metadata:    []byte("{}"),
        }); err != nil {
            return SessionBundle{}, err
        }
    }
    return s.GetSession(ctx, in.WorkspaceID, session.ID)
}

func (s *Service) GetSession(ctx context.Context, workspaceID, sessionID pgtype.UUID) (SessionBundle, error) {
    session, err := s.Queries.GetIssueDraftSessionInWorkspace(ctx, db.GetIssueDraftSessionInWorkspaceParams{
        ID: sessionID, WorkspaceID: workspaceID,
    })
    if err != nil {
        return SessionBundle{}, err
    }
    messages, err := s.Queries.ListIssueDraftMessages(ctx, db.ListIssueDraftMessagesParams{SessionID: sessionID, WorkspaceID: workspaceID})
    if err != nil {
        return SessionBundle{}, err
    }
    tasks, err := s.Queries.ListIssueDraftMemberTasks(ctx, db.ListIssueDraftMemberTasksParams{SessionID: sessionID, WorkspaceID: workspaceID})
    if err != nil {
        return SessionBundle{}, err
    }
    artifacts, err := s.Queries.ListLatestIssueDraftArtifacts(ctx, db.ListLatestIssueDraftArtifactsParams{SessionID: sessionID, WorkspaceID: workspaceID})
    if err != nil {
        return SessionBundle{}, err
    }
    steps, err := s.Queries.ListIssueDraftConfirmSteps(ctx, db.ListIssueDraftConfirmStepsParams{SessionID: sessionID, WorkspaceID: workspaceID})
    if err != nil {
        return SessionBundle{}, err
    }
    return SessionBundle{Session: session, Messages: messages, MemberTasks: tasks, Artifacts: artifacts, Steps: steps}, nil
}

func (s *Service) primaryLocalDirectory(ctx context.Context, projectID pgtype.UUID) (db.ProjectResource, string, error) {
    resources, err := s.Queries.ListProjectResources(ctx, projectID)
    if err != nil {
        return db.ProjectResource{}, "", err
    }
    for _, r := range resources {
        if r.ResourceType != "local_directory" {
            continue
        }
        var ref struct {
            LocalPath string `json:"local_path"`
        }
        if err := json.Unmarshal(r.ResourceRef, &ref); err != nil {
            return db.ProjectResource{}, "", fmt.Errorf("parse local_directory resource_ref: %w", err)
        }
        if strings.TrimSpace(ref.LocalPath) == "" {
            continue
        }
        return r, ref.LocalPath, nil
    }
    return db.ProjectResource{}, "", ErrPrimaryLocalRepositoryMissing
}
```

- [ ] **Step 5: Run service tests**

```bash
go test ./server/internal/service/issuedraft
```

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add server/internal/service/issuedraft
git commit -m "feat(issue-drafts): add draft service skeleton"
```

## Task 3: HTTP Handlers and Routes

**Files:**
- Create: `server/internal/handler/issue_draft.go`
- Test: `server/internal/handler/issue_draft_test.go`
- Modify: `server/internal/handler/handler.go`
- Modify: `server/cmd/server/router.go`

- [ ] **Step 1: Write handler tests**

Create tests that assert:

```go
func TestCreateIssueDraftRequiresPrimaryLocalDirectory(t *testing.T) {
    // POST /api/issue-drafts with project_id and squad_id.
    // Project has no local_directory.
    // Expect 400 and stable message "project has no primary local repository".
}

func TestCreateIssueDraftReturnsSessionBundle(t *testing.T) {
    // Project has a local_directory resource and squad.
    // Expect 201 with session, initial user message, empty member_tasks/artifacts/steps.
}
```

- [ ] **Step 2: Add handler structs and helpers**

Create `server/internal/handler/issue_draft.go`:

```go
package handler

import (
    "encoding/json"
    "errors"
    "net/http"

    "github.com/jackc/pgx/v5/pgtype"
    "github.com/multica-ai/multica/server/internal/service/issuedraft"
)

type CreateIssueDraftRequest struct {
    ProjectID     string `json:"project_id"`
    SquadID       string `json:"squad_id"`
    InitialPrompt string `json:"initial_prompt"`
}

type IssueDraftBundleResponse struct {
    Session     db.IssueDraftSession       `json:"session"`
    Messages    []db.IssueDraftMessage     `json:"messages"`
    MemberTasks []db.IssueDraftMemberTask  `json:"member_tasks"`
    Artifacts   []db.IssueDraftArtifact    `json:"artifacts"`
    Steps       []db.IssueDraftConfirmStep `json:"steps"`
}

func (h *Handler) CreateIssueDraft(w http.ResponseWriter, r *http.Request) {
    var req CreateIssueDraftRequest
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        writeError(w, http.StatusBadRequest, "invalid request body")
        return
    }
    workspaceID := h.resolveWorkspaceID(r)
    wsUUID, ok := parseUUIDOrBadRequest(w, workspaceID, "workspace_id")
    if !ok { return }
    userID, ok := requireUserID(w, r)
    if !ok { return }
    userUUID, ok := parseUUIDOrBadRequest(w, userID, "user_id")
    if !ok { return }
    projectUUID, ok := parseUUIDOrBadRequest(w, req.ProjectID, "project_id")
    if !ok { return }
    squadUUID, ok := parseUUIDOrBadRequest(w, req.SquadID, "squad_id")
    if !ok { return }

    bundle, err := h.IssueDraftService.CreateSession(r.Context(), issuedraft.CreateSessionInput{
        WorkspaceID: wsUUID,
        ProjectID: projectUUID,
        SquadID: squadUUID,
        CreatedBy: userUUID,
        InitialPrompt: req.InitialPrompt,
    })
    if err != nil {
        h.writeIssueDraftError(w, err)
        return
    }
    writeJSON(w, http.StatusCreated, issueDraftBundleResponse(bundle))
}

func (h *Handler) GetIssueDraft(w http.ResponseWriter, r *http.Request) {
    workspaceID := h.resolveWorkspaceID(r)
    wsUUID, ok := parseUUIDOrBadRequest(w, workspaceID, "workspace_id")
    if !ok { return }
    draftID := chi.URLParam(r, "id")
    draftUUID, ok := parseUUIDOrBadRequest(w, draftID, "id")
    if !ok { return }
    bundle, err := h.IssueDraftService.GetSession(r.Context(), wsUUID, draftUUID)
    if err != nil {
        h.writeIssueDraftError(w, err)
        return
    }
    writeJSON(w, http.StatusOK, issueDraftBundleResponse(bundle))
}

func issueDraftBundleResponse(bundle issuedraft.SessionBundle) IssueDraftBundleResponse {
    return IssueDraftBundleResponse{
        Session: bundle.Session,
        Messages: bundle.Messages,
        MemberTasks: bundle.MemberTasks,
        Artifacts: bundle.Artifacts,
        Steps: bundle.Steps,
    }
}

func (h *Handler) writeIssueDraftError(w http.ResponseWriter, err error) {
    switch {
    case errors.Is(err, issuedraft.ErrPrimaryLocalRepositoryMissing):
        writeError(w, http.StatusBadRequest, "project has no primary local repository")
    case errors.Is(err, issuedraft.ErrProjectNotFound), errors.Is(err, issuedraft.ErrSquadNotFound):
        writeError(w, http.StatusNotFound, err.Error())
    default:
        writeError(w, http.StatusInternalServerError, "issue draft request failed")
    }
}

var _ = pgtype.UUID{}
```

Add imports for `github.com/go-chi/chi/v5` and `db "github.com/multica-ai/multica/server/pkg/db/generated"` when creating this file.

- [ ] **Step 3: Wire handler dependency**

Add `IssueDraftService *issuedraft.Service` to the main `Handler` struct and initialize it in server composition next to `IssueService` / `IssueBridgeService`.

- [ ] **Step 4: Register routes**

In `server/cmd/server/router.go`, register authenticated workspace routes:

```go
r.Route("/api/issue-drafts", func(r chi.Router) {
    r.Post("/", h.CreateIssueDraft)
    r.Get("/{id}", h.GetIssueDraft)
})
```

Match existing role gating. Regular workspace members can create/read their own workspace drafts; outsiders must 404/403 consistently with issue routes.

- [ ] **Step 5: Run handler tests**

```bash
go test ./server/internal/handler -run 'TestCreateIssueDraft'
```

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add server/internal/handler/issue_draft.go server/internal/handler/issue_draft_test.go server/internal/handler/handler.go server/cmd/server/router.go
git commit -m "feat(issue-drafts): expose draft session API"
```

## Task 4: Core API Client, Schemas, and Query Hooks

**Files:**
- Modify: `packages/core/api/client.ts`
- Modify: `packages/core/api/schemas.ts`
- Test: `packages/core/api/schemas.test.ts`
- Create: `packages/core/issue-drafts/types.ts`
- Create: `packages/core/issue-drafts/queries.ts`
- Create: `packages/core/issue-drafts/mutations.ts`
- Create: `packages/core/issue-drafts/index.ts`
- Modify: `packages/core/index.ts`

- [ ] **Step 1: Add schema tests first**

In `packages/core/api/schemas.test.ts`, add tests:

```ts
import {
  IssueDraftBundleSchema,
  type IssueDraftBundle,
} from "./schemas";

describe("IssueDraftBundleSchema", () => {
  it("parses a complete draft bundle", () => {
    const parsed = IssueDraftBundleSchema.parse({
      session: {
        id: "draft-1",
        workspace_id: "ws-1",
        project_id: "project-1",
        squad_id: "squad-1",
        leader_agent_id: "agent-1",
        primary_project_resource_id: "resource-1",
        primary_local_path_snapshot: "/repo",
        status: "clarifying",
        created_by: "user-1",
        created_issue_id: null,
        remote_issue_url: "",
        spec_file_path: "",
        git_commit_sha: "",
        last_error: "",
        created_at: "2026-07-03T00:00:00Z",
        updated_at: "2026-07-03T00:00:00Z",
      },
      messages: [],
      member_tasks: [],
      artifacts: [],
      steps: [],
    });
    expect(parsed.session.status).toBe("clarifying");
  });

  it("falls back when status has an unknown future value", () => {
    const result = IssueDraftBundleSchema.safeParse({
      session: { status: "future" },
      messages: [],
      member_tasks: [],
      artifacts: [],
      steps: [],
    });
    expect(result.success).toBe(false);
  });
});
```

- [ ] **Step 2: Add zod schemas**

In `packages/core/api/schemas.ts`, add:

```ts
export const IssueDraftStatusSchema = z.enum([
  "clarifying",
  "delegating",
  "drafting",
  "ready_for_review",
  "creating",
  "created",
  "failed",
  "cancelled",
]);

export const IssueDraftSessionSchema = z.object({
  id: z.string(),
  workspace_id: z.string(),
  project_id: z.string(),
  squad_id: z.string(),
  leader_agent_id: z.string(),
  primary_project_resource_id: z.string(),
  primary_local_path_snapshot: z.string(),
  status: IssueDraftStatusSchema,
  created_by: z.string(),
  created_issue_id: z.string().nullable().optional(),
  remote_issue_url: z.string().default(""),
  spec_file_path: z.string().default(""),
  git_commit_sha: z.string().default(""),
  last_error: z.string().default(""),
  created_at: z.string(),
  updated_at: z.string(),
});

export const IssueDraftMessageSchema = z.object({
  id: z.string(),
  session_id: z.string(),
  workspace_id: z.string(),
  author_type: z.enum(["member", "agent", "system"]),
  author_id: z.string().nullable().optional(),
  message_type: z.enum(["user_message", "question", "answer", "finding", "draft_preview", "status", "error"]),
  content: z.string(),
  metadata: z.record(z.unknown()).default({}),
  created_at: z.string(),
});

export const IssueDraftMemberTaskSchema = z.object({
  id: z.string(),
  session_id: z.string(),
  workspace_id: z.string(),
  agent_id: z.string(),
  task_id: z.string().nullable().optional(),
  status: z.enum(["queued", "running", "completed", "failed", "cancelled"]),
  skill_basis: z.string().default(""),
  read_scope: z.record(z.unknown()).default({}),
  findings: z.string().default(""),
  error: z.string().default(""),
  created_at: z.string(),
  updated_at: z.string(),
});

export const IssueDraftArtifactSchema = z.object({
  id: z.string(),
  session_id: z.string(),
  workspace_id: z.string(),
  artifact_type: z.enum(["detailed_spec", "multica_issue", "remote_issue"]),
  revision: z.number(),
  content: z.string(),
  generated_by_agent_id: z.string().nullable().optional(),
  created_at: z.string(),
});

export const IssueDraftConfirmStepSchema = z.object({
  session_id: z.string(),
  workspace_id: z.string(),
  step: z.enum(["create_multica_issue", "write_spec", "commit_and_push", "create_remote_issue", "link_outputs"]),
  status: z.enum(["pending", "running", "succeeded", "failed"]),
  external_id: z.string().default(""),
  result_metadata: z.record(z.unknown()).default({}),
  error: z.string().default(""),
  created_at: z.string(),
  updated_at: z.string(),
});

export const IssueDraftBundleSchema = z.object({
  session: IssueDraftSessionSchema,
  messages: z.array(IssueDraftMessageSchema).default([]),
  member_tasks: z.array(IssueDraftMemberTaskSchema).default([]),
  artifacts: z.array(IssueDraftArtifactSchema).default([]),
  steps: z.array(IssueDraftConfirmStepSchema).default([]),
});

export type IssueDraftBundle = z.infer<typeof IssueDraftBundleSchema>;
```

- [ ] **Step 3: Add API client methods**

In `packages/core/api/client.ts`, add methods:

```ts
async createIssueDraft(data: {
  project_id: string;
  squad_id: string;
  initial_prompt?: string;
}): Promise<IssueDraftBundle> {
  const raw = await this.fetch<unknown>("/api/issue-drafts", {
    method: "POST",
    body: JSON.stringify(data),
  });
  return parseWithFallback(raw, IssueDraftBundleSchema, {
    session: {
      id: "",
      workspace_id: "",
      project_id: data.project_id,
      squad_id: data.squad_id,
      leader_agent_id: "",
      primary_project_resource_id: "",
      primary_local_path_snapshot: "",
      status: "failed",
      created_by: "",
      created_issue_id: null,
      remote_issue_url: "",
      spec_file_path: "",
      git_commit_sha: "",
      last_error: "Malformed issue draft response",
      created_at: "",
      updated_at: "",
    },
    messages: [],
    member_tasks: [],
    artifacts: [],
    steps: [],
  }, { endpoint: "POST /api/issue-drafts" });
}

async getIssueDraft(id: string): Promise<IssueDraftBundle> {
  const raw = await this.fetch<unknown>(`/api/issue-drafts/${id}`);
  return parseWithFallback(raw, IssueDraftBundleSchema, null, {
    endpoint: "GET /api/issue-drafts/:id",
  });
}
```

Adjust fallback shape if `parseWithFallback` requires non-null fallback in this repo.

- [ ] **Step 4: Add query/mutation hooks**

Create `packages/core/issue-drafts/queries.ts`:

```ts
import { queryOptions } from "@tanstack/react-query";
import { api } from "../api";

export const issueDraftKeys = {
  all: (wsId: string) => ["issue-drafts", wsId] as const,
  detail: (wsId: string, id: string) => [...issueDraftKeys.all(wsId), "detail", id] as const,
};

export function issueDraftOptions(wsId: string, id: string) {
  return queryOptions({
    queryKey: issueDraftKeys.detail(wsId, id),
    queryFn: () => api.getIssueDraft(id),
    enabled: !!id,
  });
}
```

Create `packages/core/issue-drafts/mutations.ts`:

```ts
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { api } from "../api";
import { useWorkspaceId } from "../hooks";
import { issueDraftKeys } from "./queries";

export function useCreateIssueDraft() {
  const wsId = useWorkspaceId();
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (data: { project_id: string; squad_id: string; initial_prompt?: string }) =>
      api.createIssueDraft(data),
    onSuccess: (bundle) => {
      qc.setQueryData(issueDraftKeys.detail(wsId, bundle.session.id), bundle);
    },
  });
}
```

Create `packages/core/issue-drafts/types.ts` that re-exports `IssueDraftBundle` and related inferred types from schemas, or defines aliases from `packages/core/api/schemas.ts`.

- [ ] **Step 5: Run package tests**

```bash
pnpm --filter @multica/core test -- api/schemas.test.ts
```

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add packages/core/api/client.ts packages/core/api/schemas.ts packages/core/api/schemas.test.ts packages/core/issue-drafts packages/core/index.ts
git commit -m "feat(core): add issue draft API"
```

## Task 5: Leader and Member Draft Task Contexts

**Files:**
- Modify: `server/internal/service/task.go`
- Modify: `server/internal/daemon/types.go`
- Modify: `server/internal/daemon/prompt.go`
- Modify: `server/internal/handler/agent.go`
- Test: `server/internal/daemon/daemon_test.go` or `server/internal/daemon/prompt_test.go`
- Test: `server/internal/service/task_test.go`

- [ ] **Step 1: Add prompt tests**

Add tests asserting:

```go
func TestBuildIssueDraftMemberPromptIsReadOnly(t *testing.T) {
    prompt := BuildPrompt(Task{
        IssueDraftMemberTaskID: "member-task-1",
        IssueDraftReadScope: `{"files":["packages/core/api/client.ts"]}`,
        ProjectLocalPath: "/repo/lms-mini",
    })
    for _, want := range []string{
        "read-only",
        "Do not modify files",
        "Do not create commits",
        "Return structured findings",
    } {
        if !strings.Contains(prompt, want) { t.Fatalf("missing %q", want) }
    }
}
```

- [ ] **Step 2: Add task context types**

In `server/internal/service/task.go`, add:

```go
const IssueDraftLeaderContextType = "issue_draft_leader"
const IssueDraftMemberContextType = "issue_draft_member"

type IssueDraftLeaderContext struct {
    Type        string `json:"type"`
    WorkspaceID string `json:"workspace_id"`
    SessionID   string `json:"session_id"`
    ProjectID   string `json:"project_id"`
    SquadID     string `json:"squad_id"`
    LocalPath   string `json:"local_path"`
}

type IssueDraftMemberContext struct {
    Type         string          `json:"type"`
    WorkspaceID  string          `json:"workspace_id"`
    SessionID    string          `json:"session_id"`
    MemberTaskID string          `json:"member_task_id"`
    ReadScope    json.RawMessage `json:"read_scope"`
    LocalPath     string         `json:"local_path"`
}
```

- [ ] **Step 3: Add enqueue helpers**

Add helpers:

```go
func (s *TaskService) EnqueueIssueDraftLeaderTask(ctx context.Context, agentID, runtimeID pgtype.UUID, payload IssueDraftLeaderContext) (db.AgentTaskQueue, error) {
    payload.Type = IssueDraftLeaderContextType
    contextJSON, err := json.Marshal(payload)
    if err != nil { return db.AgentTaskQueue{}, err }
    task, err := s.Queries.CreateQuickCreateTask(ctx, db.CreateQuickCreateTaskParams{
        AgentID: agentID,
        RuntimeID: runtimeID,
        Priority: priorityToInt("high"),
        Context: contextJSON,
    })
    if err != nil { return db.AgentTaskQueue{}, err }
    s.NotifyTaskEnqueued(ctx, task)
    return task, nil
}
```

Use a dedicated sqlc insert later if `CreateQuickCreateTask` naming becomes misleading. For the first vertical slice, this reuses "no issue/chat/autopilot link" task shape.

- [ ] **Step 4: Add daemon task fields**

In `server/internal/daemon/types.go`, add fields:

```go
IssueDraftSessionID string `json:"issue_draft_session_id,omitempty"`
IssueDraftMemberTaskID string `json:"issue_draft_member_task_id,omitempty"`
IssueDraftReadScope string `json:"issue_draft_read_scope,omitempty"`
IssueDraftLocalPath string `json:"issue_draft_local_path,omitempty"`
```

- [ ] **Step 5: Add prompt builders**

In `server/internal/daemon/prompt.go`, route before quick-create:

```go
if task.IssueDraftMemberTaskID != "" {
    return buildIssueDraftMemberPrompt(task)
}
if task.IssueDraftSessionID != "" {
    return buildIssueDraftLeaderPrompt(task)
}
```

Add:

```go
func buildIssueDraftMemberPrompt(task Task) string {
    return fmt.Sprintf(`You are assisting an Issue Draft Session as a squad member.

Repository: %s
Read scope: %s

Rules:
- This is a read-only analysis task.
- Do not modify files.
- Do not create commits.
- Do not push branches.
- Do not create issues.
- Inspect only what is needed for the read scope.

Return structured findings with:
- Summary
- Relevant files and line references
- Risks
- Open questions
`, task.IssueDraftLocalPath, task.IssueDraftReadScope)
}
```

- [ ] **Step 6: Run prompt tests**

```bash
go test ./server/internal/daemon -run IssueDraft
```

Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add server/internal/service/task.go server/internal/daemon/types.go server/internal/daemon/prompt.go server/internal/handler/agent.go server/internal/daemon/*test.go
git commit -m "feat(issue-drafts): add draft task prompts"
```

## Task 6: Artifact Generation and Confirmation Service

**Files:**
- Modify: `server/internal/service/issuedraft/service.go`
- Create: `server/internal/service/issuedraft/confirm.go`
- Test: `server/internal/service/issuedraft/confirm_test.go`

- [ ] **Step 1: Add idempotency tests**

Add tests for:

```go
func TestConfirmDoesNotRepeatCreatedIssue(t *testing.T) {
    // Existing confirm step create_multica_issue is succeeded with external_id issue-1.
    // Confirm resumes at write_spec.
    // Assert IssueService.Create fake was not called.
}

func TestConfirmUsesRemoteIssueArtifactForRemoteProvider(t *testing.T) {
    // detailed_spec contains "Implementation Task Breakdown".
    // remote_issue omits it.
    // Confirm creates remote issue with remote_issue content only.
}
```

- [ ] **Step 2: Define collaborator interfaces**

In `confirm.go`:

```go
type IssueCreator interface {
    Create(context.Context, service.IssueCreateParams, service.IssueCreateOpts) (service.IssueCreateResult, error)
}

type SpecWriter interface {
    WriteCommitPush(ctx context.Context, in SpecWriteInput) (SpecWriteResult, error)
}

type RemoteIssueCreator interface {
    CreateRemoteIssue(ctx context.Context, in RemoteIssueInput) (RemoteIssueResult, error)
}

type SpecWriteInput struct {
    WorkspaceID pgtype.UUID
    ResourceID pgtype.UUID
    RelativePath string
    Content string
    CommitMessage string
}

type SpecWriteResult struct {
    Path string
    CommitSHA string
}

type RemoteIssueInput struct {
    WorkspaceID pgtype.UUID
    ProjectID pgtype.UUID
    Title string
    Body string
}

type RemoteIssueResult struct {
    URL string
    ExternalID string
}
```

- [ ] **Step 3: Implement confirm step runner**

Implement `Confirm(ctx, workspaceID, sessionID)` so it:

1. Locks/loads session and artifacts.
2. Upserts `create_multica_issue` as running, calls `IssueCreator.Create` if not already succeeded.
3. Upserts `write_spec` as running, writes `.spec/issues/<identifier>.md`.
4. Upserts `commit_and_push` as succeeded from `SpecWriteResult`.
5. Upserts `create_remote_issue` as running, passes only remote artifact content.
6. Marks session created.

On error, mark current step failed and session failed with `last_error`.

- [ ] **Step 4: Run confirm tests**

```bash
go test ./server/internal/service/issuedraft -run Confirm
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add server/internal/service/issuedraft
git commit -m "feat(issue-drafts): add idempotent confirmation"
```

## Task 7: Shared Issue Draft UI

**Files:**
- Create: `packages/views/issue-drafts/issue-draft-page.tsx`
- Create: `packages/views/issue-drafts/components/draft-preview-tabs.tsx`
- Create: `packages/views/issue-drafts/components/member-findings-panel.tsx`
- Create: `packages/views/issue-drafts/components/draft-composer.tsx`
- Create: `packages/views/issue-drafts/index.ts`
- Test: `packages/views/issue-drafts/issue-draft-page.test.tsx`

- [ ] **Step 1: Write rendering test**

Test that the page renders:

```tsx
it("renders draft status, member findings, and artifact tabs", async () => {
  render(<IssueDraftPage draftId="draft-1" />);
  expect(await screen.findByText(/clarifying/i)).toBeInTheDocument();
  expect(screen.getByRole("tab", { name: /Detailed .spec/i })).toBeInTheDocument();
  expect(screen.getByRole("tab", { name: /Remote issue/i })).toBeInTheDocument();
});
```

Mock `@multica/core/issue-drafts` hooks in the same style as existing `packages/views` tests.

- [ ] **Step 2: Implement preview tabs**

`draft-preview-tabs.tsx`:

```tsx
export function DraftPreviewTabs({ artifacts }: { artifacts: IssueDraftArtifact[] }) {
  const detailed = artifacts.find((a) => a.artifact_type === "detailed_spec");
  const multica = artifacts.find((a) => a.artifact_type === "multica_issue");
  const remote = artifacts.find((a) => a.artifact_type === "remote_issue");
  return (
    <Tabs defaultValue="detailed">
      <TabsList>
        <TabsTrigger value="detailed">Detailed .spec</TabsTrigger>
        <TabsTrigger value="multica">Multica issue</TabsTrigger>
        <TabsTrigger value="remote">Remote issue</TabsTrigger>
      </TabsList>
      <TabsContent value="detailed"><Markdown>{detailed?.content ?? ""}</Markdown></TabsContent>
      <TabsContent value="multica"><Markdown>{multica?.content ?? ""}</Markdown></TabsContent>
      <TabsContent value="remote"><Markdown>{remote?.content ?? ""}</Markdown></TabsContent>
    </Tabs>
  );
}
```

Use the repo's existing markdown component import path.

- [ ] **Step 3: Implement findings panel**

Render each member task with status, agent avatar/name, `skill_basis`, and findings or error. Use existing `ActorAvatar`.

- [ ] **Step 4: Implement page**

`issue-draft-page.tsx` should:

- Query `issueDraftOptions(wsId, draftId)`.
- Show header with status/project/squad/local path snapshot.
- Show messages timeline.
- Show composer.
- Show preview tabs and findings panel.
- Disable confirm if status is `creating` or required artifacts are missing.

- [ ] **Step 5: Run views test**

```bash
pnpm --filter @multica/views test -- issue-draft-page.test.tsx
```

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add packages/views/issue-drafts packages/views/package.json
git commit -m "feat(views): add issue draft page"
```

## Task 8: Route Wiring and Create Entry

**Files:**
- Modify: `packages/core/paths/paths.ts`
- Test: `packages/core/paths/paths.test.ts`
- Create: `apps/web/app/[workspaceSlug]/(dashboard)/issue-drafts/[id]/page.tsx`
- Modify: `apps/desktop/src/renderer/src/routes.tsx`
- Modify: `packages/views/modals/create-issue-dialog.tsx`
- Modify: `packages/views/modals/create-issue.tsx` or create modal shell component

- [ ] **Step 1: Add shared navigation target**

Add route helper in the workspace paths module:

```ts
issueDraft: (id: string) => `${ws}/issue-drafts/${encode(id)}`,
```

In `packages/core/paths/paths.test.ts`, add:

```ts
expect(paths.workspace("acme").issueDraft("draft 1")).toBe("/acme/issue-drafts/draft%201");
```

- [ ] **Step 2: Add web route**

Create `apps/web/app/[workspaceSlug]/(dashboard)/issue-drafts/[id]/page.tsx`:

```tsx
import { DashboardGuard } from "@multica/views/layout";
import { IssueDraftPage } from "@multica/views/issue-drafts";

export default function Page({ params }: { params: { id: string } }) {
  return (
    <DashboardGuard>
      <IssueDraftPage draftId={params.id} />
    </DashboardGuard>
  );
}
```

- [ ] **Step 3: Add desktop route**

In `apps/desktop/src/renderer/src/routes.tsx`, import the shared page:

```tsx
import { IssueDraftPage } from "@multica/views/issue-drafts";
```

Add a route wrapper near the other desktop detail wrappers:

```tsx
function DesktopIssueDraftRoute() {
  const { id = "" } = useParams();
  return <IssueDraftPage draftId={id} />;
}
```

Add this child route under `:workspaceSlug`:

```tsx
{
  path: "issue-drafts/:id",
  element: <DesktopIssueDraftRoute />,
  handle: { title: "Issue Draft" },
}
```

- [ ] **Step 4: Add modal entry action**

In create issue modal, add a "Clarify with squad" action that requires project and squad selection, calls `useCreateIssueDraft`, closes modal, and navigates to `p.issueDraft(bundle.session.id)`.

Use this handler shape in the manual create panel:

```tsx
const createIssueDraft = useCreateIssueDraft();
const navigation = useNavigation();
const paths = useWorkspacePaths();

async function startDraftSession() {
  if (!projectId || assigneeType !== "squad" || !assigneeId) return;
  const description = descEditorRef.current?.getMarkdown()?.trim() ?? "";
  const bundle = await createIssueDraft.mutateAsync({
    project_id: projectId,
    squad_id: assigneeId,
    initial_prompt: [title.trim(), description].filter(Boolean).join("\n\n"),
  });
  onClose();
  navigation.push(paths.issueDraft(bundle.session.id));
}
```

- [ ] **Step 5: Run typecheck**

```bash
pnpm typecheck
```

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add apps/web apps/desktop packages/core packages/views
git commit -m "feat(issue-drafts): wire draft routes"
```

## Task 9: Verification and Release Readiness

**Files:**
- Modify only files changed by Tasks 1-8 when a verification failure identifies a concrete defect.
- Do not add new product scope in this task.

- [ ] **Step 1: Run focused backend tests**

```bash
go test ./server/internal/service/issuedraft ./server/internal/handler -run 'IssueDraft'
```

Expected: PASS.

- [ ] **Step 2: Run focused frontend tests**

```bash
pnpm --filter @multica/core test -- api/schemas.test.ts
pnpm --filter @multica/views test -- issue-draft-page.test.tsx
```

Expected: PASS.

- [ ] **Step 3: Run broader checks**

```bash
pnpm typecheck
make test
```

Expected: PASS.

- [ ] **Step 4: Manual smoke check**

Start the app and verify:

1. Create issue modal exposes "Clarify with squad".
2. Selecting project without primary local repo returns a clear setup error.
3. Selecting project with primary local repo creates a draft and navigates to it.
4. Draft page shows conversation, findings panel, and preview tabs.

- [ ] **Step 5: Commit verification fixes**

```bash
git add .
git commit -m "test(issue-drafts): verify draft flow"
```

Only commit if verification required code/test fixes.

## Self-Review

Spec coverage:

- Dedicated draft session: Tasks 1-4, 7-8.
- Project primary local repo: Tasks 1-3.
- Real squad member delegation: Task 5 and Task 7 findings panel.
- Read-only member policy: Task 5 prompt test and prompt implementation.
- Detailed `.spec` and concise remote issue split: Task 6 confirmation tests.
- Idempotent confirmation: Task 6.
- UI and routing: Tasks 7-8.
- Testing: Task 9.

Known implementation decisions deferred to execution:

- Whether to add an explicit `is_primary` field to `project_resource` or keep "first positioned local_directory" for the first iteration. The plan implements first positioned local directory because existing schema already supports position.
- Whether GitHub remote issue creation ships in the first execution pass. The provider boundary in Task 6 supports both; first pass can wire GitLab through the existing issue bridge and leave GitHub behind the same interface.
