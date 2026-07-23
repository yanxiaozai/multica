# Agent Evolution Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build the v1 backend/API loop for storing agent Learning Reports and manually applying safe evolution suggestions to agent instructions or workspace skills.

**Architecture:** Add Postgres tables for reports, suggestions, and application audit records, expose workspace-scoped HTTP endpoints, and reuse existing `agent` / `skill` update paths for the write-back. v1 intentionally does not auto-generate reports with an LLM and does not auto-apply suggestions; those plug into the API later.

**Tech Stack:** Go, Chi handlers, pgx/sqlc, Postgres migrations, existing Multica auth/workspace guards, TypeScript API client and zod schemas.

---

### Task 1: Database Model

**Files:**
- Create: `server/migrations/134_agent_evolution.up.sql`
- Create: `server/migrations/134_agent_evolution.down.sql`
- Create: `server/pkg/db/queries/agent_learning.sql`

- [ ] **Step 1: Add migration tables**

Create `agent_learning_report` with `workspace_id`, `issue_id`, `task_id`, `agent_id`, `summary`, `metadata`, timestamps.

Create `agent_evolution_suggestion` with `report_id`, `workspace_id`, `scope`, `risk`, `status`, target fields, `title`, `rationale`, `proposed_content`, `metadata`, timestamps.

Create `agent_evolution_application` with `suggestion_id`, `workspace_id`, `applied_by`, target fields, before/after snapshots, and timestamps.

- [ ] **Step 2: Add sqlc queries**

Add create/list/get/update/application queries in `agent_learning.sql`, keeping every read/write workspace-scoped.

- [ ] **Step 3: Regenerate db code**

Run: `make sqlc`

Expected: generated Go models and query methods for the new tables.

### Task 2: Handler API

**Files:**
- Create: `server/internal/handler/agent_learning.go`
- Modify: `server/internal/handler/handler.go`
- Modify: route registration file identified during implementation

- [ ] **Step 1: Add response/request types**

Define Learning Report, Suggestion, and Application response structs matching API JSON conventions.

- [ ] **Step 2: Implement create/list endpoints**

Implement:

```text
POST /api/agent-learning-reports
GET /api/issues/{id}/learning-reports
GET /api/agents/{id}/learning-reports
GET /api/agent-learning-reports/{id}
```

- [ ] **Step 3: Implement apply endpoint**

Implement:

```text
POST /api/agent-evolution-suggestions/{id}/apply
```

Rules:
- `personal_agent` appends to target agent `instructions`.
- `workspace_skill` appends to target skill `content`.
- `builtin_skill_candidate` is recorded only and cannot apply in v1.
- `manual` risk cannot apply in v1.
- Applied/dismissed suggestions cannot apply again.
- Cross-workspace targets are rejected.

### Task 3: Server Tests

**Files:**
- Create: `server/internal/handler/agent_learning_test.go`

- [ ] **Step 1: Test report creation and listing**

Create a report with personal and workspace suggestions, then verify issue/agent lists return it in workspace scope.

- [ ] **Step 2: Test personal apply**

Apply a personal suggestion and verify agent instructions gain a marked Agent Evolution section plus an application row.

- [ ] **Step 3: Test workspace skill apply**

Apply a workspace suggestion and verify skill content gains a marked Agent Evolution section plus an application row.

- [ ] **Step 4: Test blocked cases**

Verify manual/builtin/already-applied/cross-workspace cases fail without mutating targets.

### Task 4: TypeScript Client Surface

**Files:**
- Modify: `packages/core/api/schemas.ts`
- Modify: `packages/core/api/client.ts`
- Modify: `packages/core/api/schema.test.ts`
- Create or modify: `packages/core/agent-learning/*`

- [ ] **Step 1: Add zod schemas**

Define defensive schemas for reports, suggestions, and application responses.

- [ ] **Step 2: Add API client methods**

Add methods for create/list/get/apply.

- [ ] **Step 3: Add schema tests**

Verify missing arrays default safely and unknown future fields do not break parsing.

### Task 5: Verification

**Files:**
- No new files unless generated output changes.

- [ ] **Step 1: Run narrow Go tests**

Run: `go test ./internal/handler -run 'AgentLearning|AgentEvolution'`

- [ ] **Step 2: Run TypeScript checks**

Run: `pnpm typecheck`

- [ ] **Step 3: Inspect git diff**

Run: `git status --short` and review only this issue's files for staging.
