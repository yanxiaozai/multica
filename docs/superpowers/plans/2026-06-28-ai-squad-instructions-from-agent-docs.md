# AI Squad Instructions From Agent Docs Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add an AI-backed squad instructions generator that summarizes each squad member agent's saved instructions document and returns editable leader delegation instructions.

**Architecture:** Keep the existing synchronous template generator as the fast fallback. Add a durable asynchronous generation job linked to `agent_task_queue`; the squad leader agent runs the summarization through the existing runtime/daemon system, and task completion writes the result back to the job for UI polling. The UI starts the job, polls until terminal state, and fills the editor without auto-saving.

**Tech Stack:** Go/Chi/sqlc/PostgreSQL backend, existing daemon task queue/runtime flow, TypeScript/Zod core API client, React/TanStack Query shared views.

---

## File Structure

- `server/migrations/129_squad_instructions_generation_job.up.sql` and `.down.sql`: durable job table for polling AI generation results.
- `server/pkg/db/queries/squad_instructions_generation.sql`: sqlc queries for creating, reading, and updating generation jobs.
- `server/internal/handler/squad_instructions_generation.go`: HTTP handlers and response conversion for AI generation jobs.
- `server/internal/handler/squad_instructions_generation_prompt.go`: prompt assembly from squad, leader, members, skills, and bounded agent instructions documents.
- `server/internal/service/task.go`: enqueue and completion integration for `context.type == "squad_instructions_generation"`.
- `server/internal/daemon/execenv/runtime_config.go` and `server/internal/daemon/execenv/runtime_config_sections.go`: task-context prompt plumbing if the daemon needs a dedicated briefing section for the new context.
- `server/cmd/server/router.go`: routes for creating and polling generation jobs.
- `packages/core/types/squad.ts`, `packages/core/types/index.ts`, `packages/core/api/schemas.ts`, `packages/core/api/client.ts`: request/response types and parsed API methods.
- `packages/views/squads/components/squad-detail-page.tsx`: AI summarize action, job polling, editor replacement, status UI.
- `packages/views/locales/*/squads.json`: localized UI strings.

---

### Task 1: Database Job Model

**Files:**
- Create: `server/migrations/129_squad_instructions_generation_job.up.sql`
- Create: `server/migrations/129_squad_instructions_generation_job.down.sql`
- Create: `server/pkg/db/queries/squad_instructions_generation.sql`
- Generated after implementation: `server/pkg/db/generated/squad_instructions_generation.sql.go`
- Generated after implementation: `server/pkg/db/generated/models.go`

- [ ] **Step 1: Add the migration**

Create `server/migrations/129_squad_instructions_generation_job.up.sql`:

```sql
CREATE TABLE squad_instructions_generation_job (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id UUID NOT NULL REFERENCES workspace(id) ON DELETE CASCADE,
    squad_id UUID NOT NULL REFERENCES squad(id) ON DELETE CASCADE,
    task_id UUID REFERENCES agent_task_queue(id) ON DELETE SET NULL,
    created_by UUID NOT NULL REFERENCES "user"(id) ON DELETE CASCADE,
    status TEXT NOT NULL DEFAULT 'queued'
        CHECK (status IN ('queued', 'running', 'completed', 'failed', 'cancelled')),
    mode TEXT NOT NULL DEFAULT 'agent_docs',
    draft TEXT NOT NULL DEFAULT '',
    instructions TEXT NOT NULL DEFAULT '',
    error TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    completed_at TIMESTAMPTZ
);

CREATE INDEX idx_squad_instructions_generation_job_squad_created
    ON squad_instructions_generation_job(squad_id, created_at DESC);

CREATE INDEX idx_squad_instructions_generation_job_task
    ON squad_instructions_generation_job(task_id)
    WHERE task_id IS NOT NULL;
```

Create `server/migrations/129_squad_instructions_generation_job.down.sql`:

```sql
DROP TABLE IF EXISTS squad_instructions_generation_job;
```

- [ ] **Step 2: Add sqlc queries**

Create `server/pkg/db/queries/squad_instructions_generation.sql`:

```sql
-- name: CreateSquadInstructionsGenerationJob :one
INSERT INTO squad_instructions_generation_job (
    workspace_id, squad_id, created_by, status, mode, draft
)
VALUES ($1, $2, $3, 'queued', $4, $5)
RETURNING *;

-- name: AttachSquadInstructionsGenerationTask :one
UPDATE squad_instructions_generation_job
SET task_id = $2, updated_at = now()
WHERE id = $1
RETURNING *;

-- name: GetSquadInstructionsGenerationJobInWorkspace :one
SELECT *
FROM squad_instructions_generation_job
WHERE id = $1 AND squad_id = $2 AND workspace_id = $3;

-- name: GetSquadInstructionsGenerationJobByTask :one
SELECT *
FROM squad_instructions_generation_job
WHERE task_id = $1;

-- name: MarkSquadInstructionsGenerationRunning :one
UPDATE squad_instructions_generation_job
SET status = 'running', updated_at = now()
WHERE id = $1 AND status = 'queued'
RETURNING *;

-- name: CompleteSquadInstructionsGenerationJob :one
UPDATE squad_instructions_generation_job
SET status = 'completed',
    instructions = $2,
    error = NULL,
    completed_at = now(),
    updated_at = now()
WHERE id = $1
RETURNING *;

-- name: FailSquadInstructionsGenerationJob :one
UPDATE squad_instructions_generation_job
SET status = 'failed',
    error = $2,
    completed_at = now(),
    updated_at = now()
WHERE id = $1 AND status IN ('queued', 'running')
RETURNING *;
```

- [ ] **Step 3: Regenerate sqlc**

Run:

```bash
make sqlc
```

Expected: generated Go files update with `SquadInstructionsGenerationJob` model and query methods.

- [ ] **Step 4: Commit database model**

```bash
git add server/migrations/129_squad_instructions_generation_job.*.sql server/pkg/db/queries/squad_instructions_generation.sql server/pkg/db/generated
git commit -m "feat(squads): add instructions generation job model"
```

---

### Task 2: Prompt Builder From Agent Documents

**Files:**
- Create: `server/internal/handler/squad_instructions_generation_prompt.go`
- Test: `server/internal/handler/squad_instructions_generation_prompt_test.go`

- [ ] **Step 1: Write prompt builder tests**

Create tests that seed a squad with a leader, two agent members, one archived agent, and one human member. Assert that:

- leader and non-archived agent `instructions` appear
- archived agent instructions do not appear
- human member appears only as escalation context
- long agent documents are truncated with an explicit marker
- prompt asks for only markdown instructions

Test skeleton:

```go
func TestBuildSquadInstructionsGenerationPromptUsesAgentDocs(t *testing.T) {
    // Seed squad, agents, skills, and members.
    prompt, err := testHandler.buildSquadInstructionsGenerationPrompt(context.Background(), squad, "current draft")
    if err != nil {
        t.Fatalf("build prompt: %v", err)
    }
    for _, want := range []string{
        "## Required Output",
        "Return only markdown suitable for squad.instructions",
        "Leader Agent",
        "frontend specialist instructions",
        "backend specialist instructions",
        "Human escalation contacts",
    } {
        if !strings.Contains(prompt, want) {
            t.Fatalf("prompt missing %q\n%s", want, prompt)
        }
    }
    if strings.Contains(prompt, "archived specialist instructions") {
        t.Fatalf("prompt included archived agent\n%s", prompt)
    }
}
```

- [ ] **Step 2: Implement bounded document helpers**

In `server/internal/handler/squad_instructions_generation_prompt.go`, add:

```go
const squadInstructionsAgentDocMaxChars = 12000

func boundedAgentDoc(value string) string {
    value = strings.TrimSpace(value)
    if len(value) <= squadInstructionsAgentDocMaxChars {
        return value
    }
    return strings.TrimSpace(value[:squadInstructionsAgentDocMaxChars]) + "\n\n[truncated: agent instructions exceeded prompt budget]"
}
```

- [ ] **Step 3: Implement prompt assembly**

Implement `buildSquadInstructionsGenerationPrompt(ctx context.Context, squad db.Squad, draft string) (string, error)` on `Handler`. It should use existing queries and helpers:

- `ListSquadMembers`
- `GetAgent`
- `GetUser`
- `loadSquadMemberSkillNames`
- `util.UUIDToString`

Output shape:

```md
# Squad Instructions Generation

## Goal

Generate markdown for squad.instructions...

## Required Output

Return only markdown with these sections:
- ## Delegation Strategy
- ## Member Routing
- ## Coordination Rules
- ## Escalation

## Squad

Name: ...
Description: ...
Existing draft: ...

## Leader Agent

Name: ...
Description: ...
Skills: ...
Instructions:
...

## Runnable Agent Members

### Agent Name
Role: ...
Description: ...
Skills: ...
Instructions:
...

## Human Escalation Contacts

- User Name: role
```

- [ ] **Step 4: Run prompt tests**

Run:

```bash
go test ./server/internal/handler -run TestBuildSquadInstructionsGenerationPrompt
```

Expected: tests pass.

---

### Task 3: Backend Job Handlers And Enqueue

**Files:**
- Create: `server/internal/handler/squad_instructions_generation.go`
- Modify: `server/internal/handler/squad.go`
- Modify: `server/cmd/server/router.go`
- Modify: `server/internal/service/task.go`
- Test: `server/internal/handler/squad_instructions_generation_test.go`

- [ ] **Step 1: Add response/request structs**

Create response types:

```go
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
```

- [ ] **Step 2: Add enqueue method in task service**

In `server/internal/service/task.go`, add:

```go
const SquadInstructionsGenerationContextType = "squad_instructions_generation"

type SquadInstructionsGenerationContext struct {
    Type            string `json:"type"`
    WorkspaceID     string `json:"workspace_id"`
    SquadID         string `json:"squad_id"`
    GenerationJobID string `json:"generation_job_id"`
    Prompt          string `json:"prompt"`
}
```

Add `EnqueueSquadInstructionsGenerationTask(ctx, agent db.Agent, job db.SquadInstructionsGenerationJob, prompt string) (db.AgentTaskQueue, error)` that:

- rejects archived agents
- rejects agents without `RuntimeID`
- marshals the context above
- calls `CreateQuickCreateTask` with priority `2`
- broadcasts and notifies like other enqueue helpers

- [ ] **Step 3: Add create handler**

Implement `CreateSquadInstructionsGenerationJob`:

- require current user
- load squad with `loadSquadInWorkspace`
- require owner/admin with `requireWorkspaceRole`
- accept only `mode == "" || mode == "agent_docs"`
- load leader agent with `GetAgent`
- reject archived leader with `400 leader agent is archived`
- reject missing runtime with `400 leader agent has no runtime`
- build prompt with `buildSquadInstructionsGenerationPrompt`
- create job row
- enqueue task
- attach task id
- return `201` with job response

- [ ] **Step 4: Add get handler**

Implement `GetSquadInstructionsGenerationJob`:

- load squad with `loadSquadInWorkspace`
- require workspace member access
- parse `{jobId}`
- fetch job by id, squad id, workspace id
- return response

- [ ] **Step 5: Register routes**

In `server/cmd/server/router.go`, under squad routes:

```go
r.Post("/instructions/generation-jobs", h.CreateSquadInstructionsGenerationJob)
r.Get("/instructions/generation-jobs/{jobId}", h.GetSquadInstructionsGenerationJob)
```

- [ ] **Step 6: Write handler tests**

Cover:

- successful create returns `201`, `status: queued`, and task id
- invalid mode returns `400`
- leader without runtime returns `400`
- get returns completed/failed job response shape
- non-admin cannot create but can read if normal squad access allows it

- [ ] **Step 7: Run focused handler tests**

Run:

```bash
go test ./server/internal/handler -run 'TestSquadInstructionsGeneration'
```

Expected: tests pass.

---

### Task 4: Runtime Prompt Delivery

**Files:**
- Modify: `server/internal/daemon/types.go`
- Modify: `server/internal/handler/agent.go`
- Modify: `server/internal/daemon/execenv/runtime_config.go`
- Modify: `server/internal/daemon/execenv/runtime_config_sections.go`
- Test: existing daemon/execenv prompt tests plus new focused tests

- [ ] **Step 1: Surface prompt in daemon task payload**

Extend the task response struct used by daemon claim to include:

```go
SquadInstructionsGenerationPrompt string `json:"squad_instructions_generation_prompt,omitempty"`
```

When `task.context.type == "squad_instructions_generation"`, copy `context.prompt` into this field and set task kind to a distinct generation kind if the existing kind mapper supports it.

- [ ] **Step 2: Add runtime config section**

In daemon execenv prompt construction, when the task has `SquadInstructionsGenerationPrompt`, write:

```md
## Squad Instructions Generation

<prompt from server>

Return only the final markdown. Do not create issues, comments, files, commits, or chat messages.
```

- [ ] **Step 3: Add prompt tests**

Add a test that builds a daemon task with `SquadInstructionsGenerationPrompt` and asserts the runtime config contains:

- `## Squad Instructions Generation`
- `Return only the final markdown`
- the member agent document excerpt

- [ ] **Step 4: Run daemon prompt tests**

Run:

```bash
go test ./server/internal/daemon/execenv -run 'SquadInstructionsGeneration|RuntimeConfig'
```

Expected: tests pass.

---

### Task 5: Completion And Failure Backfill

**Files:**
- Modify: `server/internal/service/task.go`
- Test: `server/internal/service/task_test.go` or a new focused service test file

- [ ] **Step 1: Add context parser**

Add a helper:

```go
func (s *TaskService) parseSquadInstructionsGenerationContext(task db.AgentTaskQueue) (SquadInstructionsGenerationContext, bool) {
    if len(task.Context) == 0 {
        return SquadInstructionsGenerationContext{}, false
    }
    var ctx SquadInstructionsGenerationContext
    if err := json.Unmarshal(task.Context, &ctx); err != nil {
        return SquadInstructionsGenerationContext{}, false
    }
    return ctx, ctx.Type == SquadInstructionsGenerationContextType && ctx.GenerationJobID != ""
}
```

- [ ] **Step 2: Mark running on task start**

Where task status changes to running, detect this context and call `MarkSquadInstructionsGenerationRunning`. If the job is already terminal, do not fail task start.

- [ ] **Step 3: Complete job on task completion**

In `CompleteTask`, after `CompleteAgentTask` succeeds and before issue/comment/chat side effects, detect generation context:

```go
if genCtx, ok := s.parseSquadInstructionsGenerationContext(task); ok {
    body := outputFromTaskCompletedPayload(result)
    body = strings.TrimSpace(util.UnescapeBackslashEscapes(body))
    if body == "" {
        _, _ = s.Queries.FailSquadInstructionsGenerationJob(ctx, db.FailSquadInstructionsGenerationJobParams{
            ID: parseUUID(genCtx.GenerationJobID),
            Error: "AI generation completed without output",
        })
    } else {
        _, _ = s.Queries.CompleteSquadInstructionsGenerationJob(ctx, db.CompleteSquadInstructionsGenerationJobParams{
            ID: parseUUID(genCtx.GenerationJobID),
            Instructions: body,
        })
    }
    s.ReconcileAgentStatus(ctx, task.AgentID)
    s.broadcastTaskEvent(ctx, protocol.EventTaskCompleted, task)
    return &task, nil
}
```

Use the local UUID helper available in `service/task.go` rather than introducing handler-only parsing.

- [ ] **Step 4: Fail job on task failure**

In `FailTask`, detect the same context and call `FailSquadInstructionsGenerationJob` with the failure reason or error text.

- [ ] **Step 5: Add completion/failure tests**

Tests should create a generation job and a task with matching context, then assert:

- completed task writes `instructions`
- empty output marks job failed
- failed task marks job failed with error
- no issue comment or chat message is created for generation tasks

- [ ] **Step 6: Run service tests**

Run:

```bash
go test ./server/internal/service -run 'SquadInstructionsGeneration|CompleteTask|FailTask'
```

Expected: tests pass.

---

### Task 6: Core API Types And Client

**Files:**
- Modify: `packages/core/types/squad.ts`
- Modify: `packages/core/types/index.ts`
- Modify: `packages/core/api/schemas.ts`
- Modify: `packages/core/api/client.ts`
- Test: `packages/core/api/schemas.test.ts`

- [ ] **Step 1: Add TypeScript types**

Add:

```ts
export type SquadInstructionsGenerationStatus = "queued" | "running" | "completed" | "failed" | "cancelled";

export interface CreateSquadInstructionsGenerationRequest {
  mode?: "agent_docs";
  draft?: string;
}

export interface SquadInstructionsGenerationJob {
  id: string;
  job_id?: string;
  task_id: string | null;
  status: SquadInstructionsGenerationStatus;
  mode: string;
  instructions: string;
  error: string | null;
  created_at: string;
  updated_at: string;
  completed_at: string | null;
}
```

- [ ] **Step 2: Add zod schema and fallback**

In `packages/core/api/schemas.ts`, add a schema that defaults string fields safely and treats unknown statuses as `failed` only in UI helpers, not in the parser. Use `.catch("failed")` if the project pattern uses catch for enums.

- [ ] **Step 3: Add API client methods**

In `packages/core/api/client.ts`, add:

```ts
async createSquadInstructionsGenerationJob(
  id: string,
  data: CreateSquadInstructionsGenerationRequest = { mode: "agent_docs" },
): Promise<SquadInstructionsGenerationJob>

async getSquadInstructionsGenerationJob(
  squadId: string,
  jobId: string,
): Promise<SquadInstructionsGenerationJob>
```

Both methods must parse through `parseWithFallback`.

- [ ] **Step 4: Add schema tests**

Test malformed/missing optional fields and a normal completed response.

- [ ] **Step 5: Run focused TS tests**

Run:

```bash
pnpm vitest run packages/core/api/schemas.test.ts
```

Expected: schema tests pass.

---

### Task 7: Squad Instructions UI

**Files:**
- Modify: `packages/views/squads/components/squad-detail-page.tsx`
- Modify: `packages/views/locales/en/squads.json`
- Modify: `packages/views/locales/zh-Hans/squads.json`
- Modify: `packages/views/locales/ja/squads.json`
- Modify: `packages/views/locales/ko/squads.json`
- Test: add or update `packages/views/squads/components/squad-detail-page.test.tsx` if a nearby test exists

- [ ] **Step 1: Add create-job mutation**

In `SquadDetailPage`, add a mutation:

```ts
const aiInstructionsMut = useMutation({
  mutationFn: (draft: string) =>
    api.createSquadInstructionsGenerationJob(squadId, {
      mode: "agent_docs",
      draft,
    }),
  onSuccess: (job) => {
    setInstructionsGenerationJobId(job.id);
  },
  onError: (error) => {
    const detail = error instanceof ApiError ? ` (${error.status}: ${error.message})` : "";
    toast.error(`${t(($) => $.instructions_tab.ai_failed_toast)}${detail}`);
  },
});
```

- [ ] **Step 2: Add polling query**

Poll while a job id exists and status is not terminal:

```ts
const generationJobQuery = useQuery({
  queryKey: ["squad-instructions-generation", wsId, squadId, instructionsGenerationJobId],
  queryFn: () => api.getSquadInstructionsGenerationJob(squadId, instructionsGenerationJobId!),
  enabled: Boolean(instructionsGenerationJobId),
  refetchInterval: (query) => {
    const status = query.state.data?.status;
    return status === "queued" || status === "running" ? 2000 : false;
  },
});
```

- [ ] **Step 3: Fill editor on completion**

When the job reaches `completed` with non-empty `instructions`, call the same editor replacement path used by template generation. Do not call `updateSquad`.

- [ ] **Step 4: Add AI button and status**

In `SquadInstructionsTab`, add:

- `onGenerateFromAgentDocs`
- `generatingFromAgentDocs`
- `generationStatus`

Render a secondary button labeled from locale key `ai_generate_button`. Show compact status text next to actions.

- [ ] **Step 5: Add locale strings**

Add keys under `instructions_tab`:

```json
{
  "ai_generate_button": "AI summarize agents",
  "ai_generating": "AI is summarizing agent documents...",
  "ai_generated_toast": "AI-generated squad instructions are ready. Review and save to apply them.",
  "ai_failed_toast": "Failed to summarize agent documents",
  "ai_status_queued": "Queued",
  "ai_status_running": "Running",
  "ai_status_completed": "Completed",
  "ai_status_failed": "Failed"
}
```

Translate equivalents for `zh-Hans`, `ja`, and `ko`.

- [ ] **Step 6: Add UI tests**

Mock the API client so:

- clicking AI button calls `createSquadInstructionsGenerationJob`
- polling completed job fills editor
- failed job shows error and leaves content unchanged

- [ ] **Step 7: Run focused UI tests**

Run:

```bash
pnpm vitest run packages/views/squads
```

Expected: squad view tests pass.

---

### Task 8: Verification And Commit

**Files:**
- All files touched in Tasks 1-7

- [ ] **Step 1: Run focused backend checks**

```bash
go test ./server/internal/handler -run 'SquadInstructionsGeneration|BuildSquadInstructionsGenerationPrompt|GenerateSquadInstructions'
go test ./server/internal/service -run 'SquadInstructionsGeneration|CompleteTask|FailTask'
go test ./server/internal/daemon/execenv -run 'SquadInstructionsGeneration|RuntimeConfig'
```

- [ ] **Step 2: Run focused frontend checks**

```bash
pnpm vitest run packages/core/api/schemas.test.ts packages/views/squads
```

- [ ] **Step 3: Run broader checks if dependency tooling works**

```bash
pnpm typecheck
make test
```

If pnpm still fails with the local signature/corepack issue, record that exact failure in the final response.

- [ ] **Step 4: Inspect diff**

```bash
git diff --check
git status --short
```

Expected: no whitespace errors. Only intended AI squad instructions files plus already-existing unrelated working tree files remain.

- [ ] **Step 5: Commit feature**

Stage only files for this AI generation feature:

```bash
git add server/migrations/129_squad_instructions_generation_job.*.sql server/pkg/db/queries/squad_instructions_generation.sql server/pkg/db/generated server/internal/handler/squad_instructions_generation*.go server/internal/service/task.go server/internal/daemon/types.go server/internal/daemon/execenv/runtime_config*.go server/cmd/server/router.go packages/core/types/squad.ts packages/core/types/index.ts packages/core/api/schemas.ts packages/core/api/client.ts packages/views/squads/components/squad-detail-page.tsx packages/views/locales/*/squads.json
git commit -m "feat(squads): generate instructions from agent docs"
```
