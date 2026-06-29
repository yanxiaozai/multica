# Spec Memory CLI Design

## Summary

Add a first-stage Spec Memory layer to Multica as a local-file CLI workflow. The goal is to move the `.spec/` memory protocol out of individual agent definitions and into the tool surface that agents already use: `multica spec ...`.

This stage keeps storage in the project repository under `.spec/`. It does not introduce server-side tables or UI yet, but it shapes the commands and document metadata so a later productized UI/backend model can adopt the same concepts.

Revision note, 2026-06-29: `.spec` memory now has three separate layers:

- issue metadata for tiny routing facts
- `.spec/issues/<issue-id>.md` for issue/run execution state
- epic/module docs for durable project knowledge

Issue execution state must not be stored in module `00-index.md`.

## Problem

The current `.spec/` protocol lives inside user-level agent definitions. That works, but it creates drift:

- Every agent repeats the same memory startup, write, and recovery rules.
- Updating the protocol requires editing each agent definition.
- Agents can forget paths, write inconsistent module status, or skip handoff fields.
- The platform cannot reliably surface status because `.spec/` has no stable command contract.

Multica already injects runtime instructions and exposes a task-scoped CLI token. Spec memory should follow the same pattern: the tool provides the protocol, while agents keep only role-specific behavior.

## Goals

- Provide a stable `multica spec` CLI for initializing, reading, updating, handing off, and checking `.spec/` state.
- Keep `.spec/` file-based in stage one so it works offline, stays versionable, and avoids backend schema work.
- Make audit/review gates enabled by default, while allowing explicit skip with a reason.
- Inject a concise `## Spec Memory` section into agent runtime briefs so every provider gets the same recovery and write rules.
- Treat the `spec-memory` skill as the protocol source for agents; role files should only keep domain ownership rules.
- Prevent concurrent issues from clobbering each other by keeping issue state under `.spec/issues/` and durable module knowledge under module documents.
- Preserve a clean migration path to a future backend + UI model.

## Non-Goals

- No database schema or API endpoints in this stage.
- No web/desktop UI in this stage.
- No automatic LLM summarization of chat history into `.spec/`.
- No hard coupling to the user-level `seven`, `han`, `mini`, `alex`, or similar agent names. The CLI should support arbitrary actor names.
- No mandatory design audit for every tiny task. Audit is default, but skip is supported and recorded.
- No issue tracker integration in stage one. Issue metadata can be represented in files first and moved to backend key-value storage later.

## Directory Contract

`multica spec init` creates the following structure when missing:

```text
.spec/
  index.md
  glossary.md
  issues/
  decisions/
  standards/
  epics/
```

Epic/module commands create:

```text
.spec/epics/<epic>/
  00-index.md
  plan.md
  risks.md
  <module>/
    00-index.md
    requirements.md
    design.md
    architecture.md
    backend.md
    frontend.md
    testing.md
    review.md
    acceptance.md
```

Documents may be absent until needed. The CLI creates missing parents and index files, but it should not create empty role documents unless a command writes them. This keeps simple tasks light.

## Memory Layers

Spec memory has three layers with different lifetimes.

### Issue Metadata

Issue metadata is an eventual backend issue-level key-value layer. In stage one, the same routing facts can be mirrored in `.spec/issues/<issue-id>.md`.

Recommended keys:

- `spec_primary`: the primary `<epic>/<module>` affected by the issue
- `spec_related`: related `<epic>/<module>` mappings and why they matter
- `spec_issue_file`: `.spec/issues/<issue-id>.md`
- `audit_mode`: `required` or `skipped`
- `blocked_reason` and `waiting_on` when the issue is blocked

Keep metadata small. It is for routing, not for full requirements, handoffs, or review notes.

### Issue Execution State

`.spec/issues/<issue-id>.md` is the source of truth for issue/run execution state. The CLI manages a predictable markdown shape and can update small structured fields without rewriting durable module documents.

```yaml
---
issue: "40"
title: "预约管理（二）：修改/取消 + 立即预约创建流程"
primary: "lms-core-prototype/appointment-management"
related:
  - "lms-core-prototype/customer-profile"
status: in_progress
current_stage: frontend
owner: mini
audit:
  mode: required
  skipped: false
  skip_reason: ""
  skipped_by: ""
  skipped_at: ""
loop:
  current: implementation
open_questions: []
blockers: []
updated_at: "2026-06-27T00:00:00Z"
---
```

For skipped audit:

```yaml
audit:
  mode: skipped
  skipped: true
  skip_reason: "docs-only change, no behavior changed"
  skipped_by: "boss"
  skipped_at: "2026-06-27T00:00:00Z"
```

The markdown body can contain richer sections such as Mapping, LOOP Chain, Open Questions, Blockers, Next Handoff, and Audit. The CLI should update frontmatter deterministically and append short handoff entries instead of rewriting the whole body.

### Epic And Module Docs

Epic and module documents are durable project memory. They store long-lived requirements, design, architecture, frontend/backend plans, tests, reviews, acceptance evidence, decisions, standards, and stable module notes.

Module `00-index.md` is only a long-term module index. It may include:

- module goal and scope
- stability: `draft`, `active`, `stable`, or `deprecated`
- linked spec documents
- durable progress by document area
- key long-lived decisions
- related issues as links
- module notes that will still matter after the current issue closes

It must not include current owner, current issue, current stage, Next Handoff, issue-specific blockers, issue-specific open questions, or LOOP Chain current state.

## CLI Commands

### `multica spec init`

Initializes `.spec/` in the current repository.

Flags:

- `--epic <id>` optional, also creates an epic index.
- `--module <id>` optional with `--epic`, also creates a module index.
- `--issue <id>` optional, also creates `.spec/issues/<issue-id>.md`.
- `--force` refreshes missing template sections but does not overwrite existing user content.

### `multica spec status`

Shows the current spec state.

Flags:

- `--issue <id>`
- `--epic <id>`
- `--module <id>`
- `--json`
- `--skip-audit` displays status as if audit were not required, without mutating files.

Default behavior separates durable module completeness from issue execution state:

- with `--issue`, report mapping, owner, current stage, LOOP Chain, audit state, open questions, blockers, next handoff, and linked module docs
- with `--epic/--module`, report durable document coverage and linked issues without inventing current owner or current stage

For issue state, the recommended gate chain is:

- requirements
- design or explicit design-not-needed note
- architecture or explicit architecture-not-needed note
- frontend and/or backend evidence when implementation exists
- testing
- review unless audit is skipped with a reason
- acceptance unless audit is skipped with a reason

This is advisory in stage one. It exits non-zero only for malformed `.spec` state, missing requested paths, invalid frontmatter, or protocol violations such as issue execution fields inside module `00-index.md`. It does not block other CLI commands.

### `multica spec read`

Reads high-signal context for an agent.

Flags:

- `--issue <id>`
- `--epic <id>`
- `--module <id>`
- `--doc <index|issue|requirements|design|architecture|backend|frontend|testing|review|acceptance|plan|risks|glossary>`
- `--json`

Without `--doc`, it returns the recovery bundle:

1. `.spec/index.md`
2. `.spec/issues/<issue-id>.md` when `--issue` is present
3. `.spec/epics/<epic>/00-index.md`
4. `.spec/epics/<epic>/<module>/00-index.md`
5. Links to existing module documents

### `multica spec update`

Writes a durable document or updates issue execution state.

Flags:

- `--issue <id>`
- `--epic <id>`
- `--module <id>`
- `--doc <doc>`
- `--content-file <path>`
- `--stage <stage>`
- `--status <status>`
- `--owner <actor>`
- `--next <actor>`
- `--open-question <text>` repeatable
- `--blocker <text>` repeatable
- `--skip-audit-reason <text>`
- `--actor <actor>`

`--content-file` replaces the target durable document body when `--doc` names a module or project document. State flags update `.spec/issues/<issue-id>.md`, not module `00-index.md`. If `--skip-audit-reason` is present, the command records the audit skip with `actor` and timestamp in the issue file or issue metadata.

The command should reject state flags such as `--stage`, `--owner`, `--next`, `--open-question`, and `--blocker` unless `--issue` is provided.

### `multica spec handoff`

Appends an issue handoff entry and updates next owner/state in issue memory.

Flags:

- `--issue <id>`
- `--epic <id>`
- `--module <id>`
- `--from <actor>`
- `--to <actor>`
- `--summary-file <path>`
- `--stage <stage>`
- `--status <status>`

The command appends a dated entry to `.spec/issues/<issue-id>.md`. The entry must capture completed work, remaining work, blockers, next expected output, and evidence links when included in the summary file. Module docs may link to handoff history only when the handoff contains durable knowledge.

### `multica spec issue bind`

Creates or updates an issue-to-spec mapping.

Flags:

- `--issue <id>`
- `--title <title>` optional
- `--primary <epic/module>`
- `--related <epic/module:reason>` repeatable
- `--actor <actor>`

This command creates `.spec/issues/<issue-id>.md` when missing and records the primary/related mappings. Later backend productization can mirror these fields into issue metadata.

### `multica spec decision`

Creates a durable decision document.

Flags:

- `--title <title>`
- `--content-file <path>`
- `--epic <id>` optional
- `--module <id>` optional

The file path is `.spec/decisions/YYYY-MM-DD-<slug>.md`. If an epic/module is provided, the command adds a link to the relevant index.

### `multica spec standard`

Creates or updates a cross-cutting standard under `.spec/standards/<name>.md`.

Flags:

- `--name <name>`
- `--content-file <path>`

## Runtime Brief Injection

The daemon should inject a concise `## Spec Memory` section into provider runtime config files.

Content:

- `.spec/` is durable project memory; chat and task messages are temporary working context.
- Prefer `multica spec read/status/update/handoff/issue bind` over hand-editing paths.
- Use the `spec-memory` skill whenever the task touches `.spec`, issue mapping, handoff, audit, LOOP Chain, or long-term agent memory.
- On recovery, read project index, issue state when an issue exists, epic index, then module index.
- Write issue execution state to `.spec/issues/<issue-id>.md` or issue metadata.
- Write long-lived project facts to epic/module docs; do not write them back into agent role files.
- Do not write current owner, current stage, issue blockers, open questions, LOOP Chain, or Next Handoff into module `00-index.md`.
- Audit/review gates are default for medium or complex work; skip only with an explicit reason recorded through `multica spec update --issue <id> --skip-audit-reason`.

The brief should stay generic. It should not mention user-specific agent names unless task context already contains them.

## Implementation Shape

Add a small CLI package for local spec operations, likely under `server/internal/cli/spec` or the existing CLI command layout if one already exists. The package should avoid server calls and use only local filesystem operations.

Core helpers:

- repo root detection by walking upward for `.git`, `AGENTS.md`, or `CLAUDE.md`
- safe path joining that rejects `..` and absolute epic/module/doc names
- markdown frontmatter parse/update
- issue mapping read/write helpers for `.spec/issues/<issue-id>.md`
- deterministic slug generation for decisions
- atomic-ish writes via temp file + rename where practical

The daemon runtime brief can call a pure helper that renders the Spec Memory section. It should not require `.spec/` to exist; the instructions can still be useful before initialization.

## Errors

- Missing `.spec` on read/status returns a helpful message: run `multica spec init`.
- Invalid epic/module/doc names return a validation error.
- Missing `--content-file` for write commands returns usage error.
- Malformed frontmatter returns non-zero and does not overwrite the file.
- Skip audit requires a non-empty reason.
- State update flags without `--issue` return usage error.
- Module `00-index.md` containing issue execution fields should be reported by `status` as a protocol violation and migrated manually or with a later repair command.

## Testing

Backend/CLI unit tests:

- `spec init` creates expected files and is idempotent.
- path validation rejects traversal.
- frontmatter update preserves markdown body.
- `status --issue` reports missing gate docs and respects recorded audit skip.
- `status --epic --module` reports durable document coverage without owner/stage/handoff fields.
- `update --issue --skip-audit-reason` records actor/reason/timestamp.
- `update` rejects issue-state flags when `--issue` is absent.
- `handoff --issue` appends history and updates next owner.
- `issue bind` creates primary/related mappings.
- runtime brief includes Spec Memory instructions.

No Playwright or frontend tests are required for stage one.

## Future Productization

The file schema should map cleanly to backend resources later:

- `spec_project`
- `spec_epic`
- `spec_module`
- `spec_document`
- `spec_decision`
- `spec_issue_mapping`
- `spec_issue_state`
- `spec_handoff`

The UI can then show current stage, owner, audit state, open questions, blockers, and handoff history from issue state, while showing document completeness and durable progress from epic/module docs. The file-based `.spec` format remains useful as import/export and offline mode.
