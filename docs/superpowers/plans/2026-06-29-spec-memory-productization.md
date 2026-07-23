# Spec Memory Productization Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use `spec-memory` first. Use `superpowers:subagent-driven-development` or `superpowers:executing-plans` to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Productize Spec Memory beyond the stage 1 file-based CLI by adding backend-backed spec resources, issue-to-spec mappings, and a focused web/desktop UI that lets users inspect and manage `.spec` memory without losing the file import/export path.

**Architecture:** Keep `.spec` files as the offline/import-export format. Add backend tables for durable spec documents and issue execution state. Reuse the existing issue `metadata` JSONB for small routing facts such as `spec_primary`, `spec_related`, `spec_issue_file`, and `audit_mode`. The UI reads backend state first and can later sync to/from `.spec` files through the CLI/import path.

**Tech Stack:** Go/Chi/sqlc/PostgreSQL backend, existing issue metadata JSONB, TypeScript/Zod core API client, React/TanStack Query shared views, shadcn/Base UI components.

---

## Protocol Boundary

Preserve the latest Spec Memory layering:

- **Issue metadata:** tiny routing facts only.
- **Issue state:** current owner, stage, LOOP Chain, open questions, blockers, next handoff, and audit state.
- **Epic/module docs:** durable requirements, design, architecture, implementation notes, testing, review, acceptance, decisions, and standards.

Do not add owner, current stage, blockers, open questions, LOOP Chain, or Next Handoff to module index records or module `00-index.md`.

---

### Task 1: Backend Data Model

**Files:**
- Create next available migration, for example `server/migrations/130_spec_memory_productization.up.sql`
- Create matching down migration
- Create `server/pkg/db/queries/spec_memory.sql`
- Generated after implementation: `server/pkg/db/generated/spec_memory.sql.go`
- Generated after implementation: `server/pkg/db/generated/models.go`

- [x] **Step 1: Add tables**

Create backend tables that mirror the three durable product concepts:

- `spec_epic`
- `spec_module`
- `spec_document`
- `spec_issue_state`
- `spec_issue_mapping`
- `spec_decision`

Recommended constraints:

- all rows are workspace-scoped
- `spec_module` belongs to `spec_epic`
- `spec_document` belongs to either an epic or module, with `doc_kind` constrained to known document kinds
- `spec_issue_state` references `issue(id)` and stores execution state only
- `spec_issue_mapping` references `issue(id)` and optionally `spec_epic/spec_module`
- `spec_issue_mapping` supports one primary mapping and multiple related mappings

- [x] **Step 2: Keep issue metadata as routing cache**

Do not create a separate issue metadata KV table. Reuse existing `issue.metadata` with these keys:

- `spec_primary`
- `spec_related`
- `spec_issue_file`
- `audit_mode`
- `blocked_reason`
- `waiting_on`

Backend writes should keep metadata in sync when mappings change, but full document bodies and execution notes belong in spec tables.

- [x] **Step 3: Add sqlc queries**

Add CRUD/list queries for:

- list epics/modules by workspace
- upsert issue mapping
- get issue spec context by issue id
- upsert issue execution state
- list documents for epic/module
- upsert document by `doc_kind`
- append decisions

- [x] **Step 4: Regenerate sqlc**

Run:

```bash
make sqlc
```

### Task 2: Backend API

**Files:**
- Create `server/internal/handler/spec_memory.go`
- Test: `server/internal/handler/spec_memory_test.go`
- Modify `server/cmd/server/router.go`

- [x] **Step 1: Add read APIs**

Add endpoints:

- `GET /api/spec/epics`
- `GET /api/spec/epics/{epic_id}/modules`
- `GET /api/spec/modules/{module_id}/documents`
- `GET /api/issues/{issue_id}/spec`

`GET /api/issues/{issue_id}/spec` should return the issue state, primary mapping, related mappings, and linked durable documents.

- [x] **Step 2: Add write APIs**

Add endpoints:

- `PUT /api/issues/{issue_id}/spec/mapping`
- `PUT /api/issues/{issue_id}/spec/state`
- `PUT /api/spec/modules/{module_id}/documents/{doc_kind}`
- `POST /api/spec/decisions`

All write APIs must enforce workspace membership and reuse the same issue-loading patterns as existing issue handlers.

- [x] **Step 3: Enforce protocol validation**

Reject attempts to write issue execution fields into module documents through structured fields. Freeform markdown can mention issue links, but structured `owner/current_stage/next_handoff/open_questions/blockers` fields belong only to issue state.

- [x] **Step 4: Sync issue metadata**

When mapping/state APIs update routing fields, update `issue.metadata` atomically through existing metadata helpers or sqlc queries. Publish the existing `issue_metadata:changed` event so list/detail caches remain consistent.

### Task 3: Core API Client

**Files:**
- Modify `packages/core/types/api.ts`
- Modify `packages/core/types/issue.ts`
- Add `packages/core/types/spec-memory.ts`
- Modify `packages/core/types/index.ts`
- Modify `packages/core/api/schemas.ts`
- Modify `packages/core/api/client.ts`
- Test: `packages/core/api/schemas.test.ts`

- [x] **Step 1: Add typed models**

Add TypeScript models for:

- `SpecEpic`
- `SpecModule`
- `SpecDocument`
- `SpecIssueMapping`
- `SpecIssueState`
- `SpecIssueContext`
- `SpecDecision`

- [x] **Step 2: Add Zod schemas**

Parse all API responses through schemas. Default missing optional arrays to `[]` and missing nullable text fields to `null` or `""` consistently with existing API patterns.

- [x] **Step 3: Add client methods**

Add methods:

- `api.listSpecEpics()`
- `api.listSpecModules(epicId)`
- `api.getIssueSpec(issueId)`
- `api.updateIssueSpecMapping(issueId, body)`
- `api.updateIssueSpecState(issueId, body)`
- `api.listSpecDocuments(moduleId)`
- `api.updateSpecDocument(moduleId, docKind, body)`
- `api.createSpecDecision(body)`

### Task 4: Shared UI

**Files:**
- Create `packages/views/spec-memory/components/spec-memory-page.tsx`
- Create `packages/views/spec-memory/components/issue-spec-panel.tsx`
- Create `packages/views/spec-memory/components/module-documents-panel.tsx`
- Create `packages/views/spec-memory/hooks.ts`
- Modify `packages/views/platform/index.ts`
- Add locale keys under `packages/views/locales/*/`

- [x] **Step 1: Build issue spec panel**

Show the issue's:

- primary mapping
- related mappings
- owner
- current stage
- LOOP state
- open questions
- blockers
- next handoff
- audit mode and skipped reason

This panel must make clear that these are issue-level fields.

- [x] **Step 2: Build module document panel**

Show module durable docs by document kind:

- requirements
- design
- architecture
- frontend/backend
- testing
- review
- acceptance

Do not show issue owner/current stage in this panel.

- [x] **Step 3: Add editing affordances**

Support focused edits:

- update issue mapping
- update issue state
- edit a document body
- append a decision
- mark audit skipped with required reason

Use confirmation or dirty-state protection before replacing unsaved text.

### Task 5: App Wiring

**Files:**
- Modify `apps/web/app/...` route files
- Modify desktop router files under `apps/desktop/src/renderer/src/`
- Add any navigation labels/locales needed by shared views

- [x] **Step 1: Add web route**

Add a workspace-scoped route for Spec Memory, likely under the project or issue context rather than a global landing page.

- [x] **Step 2: Add desktop route**

Wire the same shared view into desktop routing with the existing navigation adapter.

- [x] **Step 3: Add issue detail entry point**

Add an issue detail action or tab that opens the issue spec panel for that issue.

### Task 6: CLI/API Bridge

**Files:**
- Modify `server/internal/specmem`
- Modify `server/cmd/multica/cmd_spec.go`
- Add focused tests under `server/internal/specmem` and `server/cmd/multica`

- [x] **Step 1: Keep local file mode**

The existing stage 1 commands continue to work offline against `.spec`.

- [x] **Step 2: Add explicit backend mode**

Add an opt-in backend mode later, for example:

```bash
multica spec sync --from-files
multica spec sync --to-files
multica spec status --backend
```

Do not silently dual-write in stage 2. Backend sync must be explicit to avoid surprising local repos.

- [x] **Step 3: Preserve protocol validation**

Both local file mode and backend mode must reject issue execution state without an issue scope.

### Task 7: Verification

- [x] **Step 1: Backend tests**

Run focused handler/sqlc tests:

```bash
go test ./server/internal/handler -run 'SpecMemory|IssueSpec'
```

- [x] **Step 2: Core API tests**

Run schema/client checks:

```bash
pnpm test -- --run packages/core/api/schemas.test.ts
```

- [x] **Step 3: UI checks**

Run focused view tests if added, then typecheck:

```bash
pnpm typecheck
```

- [x] **Step 4: Protocol regression checks**

Search for forbidden module-index execution fields in new code and docs:

```bash
rg -n "current owner|current stage|Next Handoff|open_questions|blockers|LOOP Chain" packages server docs
```

Allowed hits must be issue-state code, validation text, or tests asserting the separation.

---

## Out Of Scope For Stage 2

- Automatic LLM summarization of chat history into Spec Memory.
- Silent bidirectional sync between backend and `.spec` files.
- Full visual workflow builder for LOOP Chain.
- Cross-repo spec graph search.
- Product analytics around spec usage.
