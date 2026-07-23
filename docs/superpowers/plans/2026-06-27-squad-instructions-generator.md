# Squad Instructions Generator Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a squad Instructions tab action that generates editable delegation instructions from the squad's current roster, roles, descriptions, and skills.

**Architecture:** Implement a deterministic backend generator behind `POST /api/squads/{id}/instructions/generate`, expose it through the core API client with schema parsing, then wire a `Generate draft` button into the existing shared Squad Instructions tab. The generator returns a draft only; users still edit and save with the existing `updateSquad` mutation.

**Tech Stack:** Go/Chi/sqlc backend, TypeScript/Zod API client, React/TanStack Query shared views, shadcn UI components.

---

### Task 1: Backend Generator

**Files:**
- Create: `server/internal/handler/squad_instructions_generator.go`
- Test: `server/internal/handler/squad_instructions_generator_test.go`

- [ ] **Step 1: Write generator tests**

Cover agent members with roles and skills, archived agent exclusion, human member escalation text, and empty squad fallback.

- [ ] **Step 2: Implement deterministic generator**

Create a pure renderer that loads members, agent descriptions, and skill names, then renders markdown sections:

- `## Delegation Strategy`
- `## Member Routing`
- optional `## Human Escalation`

- [ ] **Step 3: Run backend generator tests**

Run: `go test ./server/internal/handler -run 'TestGenerateSquadInstructions'`

### Task 2: Backend Route

**Files:**
- Modify: `server/internal/handler/squad.go`
- Modify: `server/cmd/server/router.go`
- Test: `server/internal/handler/squad_instructions_generator_test.go`

- [ ] **Step 1: Add handler**

Add `GenerateSquadInstructions` with `mode: "template"` validation and admin/owner permission parity with `UpdateSquad`.

- [ ] **Step 2: Register route**

Register `POST /api/squads/{id}/instructions/generate`.

- [ ] **Step 3: Add handler tests**

Cover invalid mode and successful response shape.

### Task 3: Core API

**Files:**
- Modify: `packages/core/types/squad.ts`
- Modify: `packages/core/types/index.ts`
- Modify: `packages/core/api/schemas.ts`
- Modify: `packages/core/api/client.ts`
- Test: `packages/core/api/schemas.test.ts`

- [ ] **Step 1: Add request/response types**

Add `GenerateSquadInstructionsRequest` and `GenerateSquadInstructionsResponse`.

- [ ] **Step 2: Add Zod schema and empty fallback**

Parse `instructions`, `mode`, and `warnings`.

- [ ] **Step 3: Add API client method**

Add `api.generateSquadInstructions(id, { mode: "template" })`.

### Task 4: Squad Instructions UI

**Files:**
- Modify: `packages/views/squads/components/squad-detail-page.tsx`
- Locale: `packages/views/locales/*/squads.json`

- [ ] **Step 1: Add generate mutation**

Call the new API method from `SquadDetailPage` and pass it to `SquadInstructionsTab`.

- [ ] **Step 2: Add Generate draft button**

Show a loading spinner, replace editor content on success, and leave Save as the persistence action.

- [ ] **Step 3: Protect dirty drafts**

If local editor content differs from persisted instructions, confirm before replacing it.

### Task 5: Verification

- [ ] **Step 1: Run focused Go test**

Run: `go test ./server/internal/handler -run 'TestGenerateSquadInstructions|TestGenerateSquadInstructionsHandler'`

- [ ] **Step 2: Run focused TS checks if practical**

Run: `pnpm typecheck`

- [ ] **Step 3: Review git diff**

Ensure only squad generator files, core API/schema/types, locales, and the plan changed for this feature.
