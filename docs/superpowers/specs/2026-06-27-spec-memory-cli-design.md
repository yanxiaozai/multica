# Spec Memory CLI Design

## Summary

Add a first-stage Spec Memory layer to Multica as a local-file CLI workflow. The goal is to move the `.spec/` memory protocol out of individual agent definitions and into the tool surface that agents already use: `multica spec ...`.

This stage keeps storage in the project repository under `.spec/`. It does not introduce server-side tables or UI yet, but it shapes the commands and document metadata so a later productized UI/backend model can adopt the same concepts.

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
- Preserve a clean migration path to a future backend + UI model.

## Non-Goals

- No database schema or API endpoints in this stage.
- No web/desktop UI in this stage.
- No automatic LLM summarization of chat history into `.spec/`.
- No hard coupling to the user-level `seven`, `han`, `mini`, `alex`, or similar agent names. The CLI should support arbitrary actor names.
- No mandatory design audit for every tiny task. Audit is default, but skip is supported and recorded.

## Directory Contract

`multica spec init` creates the following structure when missing:

```text
.spec/
  index.md
  glossary.md
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

## Module State Model

The module `00-index.md` is the source of truth for local status. The CLI manages a small YAML frontmatter block and leaves the markdown body human-editable.

```yaml
---
epic: checkout-redesign
module: cart-drawer
status: in_progress
current_stage: frontend
audit:
  mode: required
  skipped: false
  skip_reason: ""
  skipped_by: ""
  skipped_at: ""
loop:
  current: implementation
  owner: mini
  next_handoff: lch
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

The markdown body can contain richer sections such as Progress, Links, LOOP Chain notes, and Handoff History. The CLI should update frontmatter deterministically and append short handoff entries instead of rewriting the whole body.

## CLI Commands

### `multica spec init`

Initializes `.spec/` in the current repository.

Flags:

- `--epic <id>` optional, also creates an epic index.
- `--module <id>` optional with `--epic`, also creates a module index.
- `--force` refreshes missing template sections but does not overwrite existing user content.

### `multica spec status`

Shows the current spec state.

Flags:

- `--epic <id>`
- `--module <id>`
- `--json`
- `--skip-audit` displays status as if audit were not required, without mutating files.

Default behavior checks the recommended gate chain:

- requirements
- design or explicit design-not-needed note
- architecture or explicit architecture-not-needed note
- frontend and/or backend evidence when implementation exists
- testing
- review unless audit is skipped with a reason
- acceptance unless audit is skipped with a reason

This is advisory in stage one. It exits non-zero only for malformed `.spec` state, missing requested paths, or invalid frontmatter. It does not block other CLI commands.

### `multica spec read`

Reads high-signal context for an agent.

Flags:

- `--epic <id>`
- `--module <id>`
- `--doc <index|requirements|design|architecture|backend|frontend|testing|review|acceptance|plan|risks|glossary>`
- `--json`

Without `--doc`, it returns the recovery bundle:

1. `.spec/index.md`
2. `.spec/epics/<epic>/00-index.md`
3. `.spec/epics/<epic>/<module>/00-index.md`
4. Links to existing module documents

### `multica spec update`

Writes a document or updates module state.

Flags:

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

`--content-file` replaces the target document body. State flags update module `00-index.md` frontmatter. If `--skip-audit-reason` is present, the command records the audit skip with `actor` and timestamp.

### `multica spec handoff`

Appends a handoff entry and updates next owner/state.

Flags:

- `--epic <id>`
- `--module <id>`
- `--from <actor>`
- `--to <actor>`
- `--summary-file <path>`
- `--stage <stage>`
- `--status <status>`

The command appends a dated entry to the module index. The entry must capture completed work, remaining work, blockers, next expected output, and evidence links when included in the summary file.

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
- Prefer `multica spec read/status/update/handoff` over hand-editing paths.
- On recovery, read project index, epic index, then module index.
- Write long-lived project facts to `.spec/`; do not write them back into agent role files.
- Audit/review gates are default for medium or complex work; skip only with an explicit reason recorded through `multica spec update --skip-audit-reason`.

The brief should stay generic. It should not mention user-specific agent names unless task context already contains them.

## Implementation Shape

Add a small CLI package for local spec operations, likely under `server/internal/cli/spec` or the existing CLI command layout if one already exists. The package should avoid server calls and use only local filesystem operations.

Core helpers:

- repo root detection by walking upward for `.git`, `AGENTS.md`, or `CLAUDE.md`
- safe path joining that rejects `..` and absolute epic/module/doc names
- markdown frontmatter parse/update
- deterministic slug generation for decisions
- atomic-ish writes via temp file + rename where practical

The daemon runtime brief can call a pure helper that renders the Spec Memory section. It should not require `.spec/` to exist; the instructions can still be useful before initialization.

## Errors

- Missing `.spec` on read/status returns a helpful message: run `multica spec init`.
- Invalid epic/module/doc names return a validation error.
- Missing `--content-file` for write commands returns usage error.
- Malformed frontmatter returns non-zero and does not overwrite the file.
- Skip audit requires a non-empty reason.

## Testing

Backend/CLI unit tests:

- `spec init` creates expected files and is idempotent.
- path validation rejects traversal.
- frontmatter update preserves markdown body.
- `status` reports missing gate docs and respects recorded audit skip.
- `update --skip-audit-reason` records actor/reason/timestamp.
- `handoff` appends history and updates next owner.
- runtime brief includes Spec Memory instructions.

No Playwright or frontend tests are required for stage one.

## Future Productization

The file schema should map cleanly to backend resources later:

- `spec_project`
- `spec_epic`
- `spec_module`
- `spec_document`
- `spec_decision`
- `spec_handoff`

The UI can then show current stage, owner, audit state, open questions, blockers, document completeness, and handoff history. The file-based `.spec` format remains useful as import/export and offline mode.

