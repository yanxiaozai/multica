# Issue Draft Session Design

## Summary

Add a dedicated Issue Draft Session flow for creating high-quality issues before
they enter the execution queue. The flow lets a user select a Multica project
and squad, discuss the requirement with the squad, delegate read-only code
inspection to real squad member agents, review a structured draft, and then
confirm creation.

On confirmation the system produces three outputs:

1. A Multica issue assigned to the selected squad.
2. A detailed `.spec` document committed and pushed to the selected project's
   primary local repository.
3. A concise GitLab/GitHub issue that omits detailed implementation planning and
   links back to the `.spec` document.

This is intentionally separate from quick-create. Quick-create remains the
single-prompt "create an issue now" path. Issue Draft Sessions are for
collaborative clarification, code inspection, and explicit user confirmation.

## Goals

- Let users create issues through a structured pre-creation conversation.
- Support selecting a squad during issue creation.
- Use real squad member agent skills for requirement analysis and code
  inspection, not only the squad leader's summary.
- Keep draft-stage code inspection read-only until the user confirms creation.
- Write detailed requirement and technical context to the target business
  project's primary local repository under `.spec/`.
- Push a concise external Git issue and concise Git/PR metadata without exposing
  detailed implementation plans.
- Make confirmation idempotent so retries do not duplicate issues, commits, or
  remote Git issues.

## Non-Goals

- Replacing quick-create.
- Letting draft-stage agents modify business code before confirmation.
- Building a general multi-agent project planning product in this first phase.
- Supporting arbitrary remote providers beyond the existing GitLab bridge and a
  GitHub-compatible extension point.
- Writing draft details into the Multica repository's `.spec/`. The target is
  the selected project resource, for example `mini -> lms-mini/.spec/`.

## Confirmed Product Decisions

- Use a dedicated `issue_draft_session`, not regular chat.
- The selected Multica project has a primary local repository resource. The
  detailed draft is written under that repository's `.spec/`.
- If a project has multiple local repositories, the primary repository is used.
- Squad member agents participate through real delegated tasks based on skills.
- Member agents are read-only during the draft stage.
- The final external surfaces are concise: remote Git issue, commit message, and
  PR text must not include detailed implementation plans.
- The detailed plan lives only in the target repository `.spec` file.

## Current System Fit

Existing pieces to reuse:

- `IssueService.Create` already centralizes Multica issue creation, duplicate
  checks, numbering, attachment linking, events, analytics, and agent/squad task
  enqueue.
- Quick-create already supports selecting a squad and routing work to the squad
  leader, but its prompt contract is single-shot and should not be stretched for
  draft sessions.
- Project resources already support `local_directory` with `local_path`,
  `daemon_id`, label, and position.
- Issue bridge already models GitLab integration and issue mapping. The new flow
  should reuse or extend this service boundary rather than creating a parallel
  Git integration stack.
- The spec-memory protocol defines `.spec/issues/<issue-id>.md` and module docs
  conventions. This flow writes into the target project's repository, not the
  Multica repo.

Needed additions:

- Draft session tables and queries.
- Draft-specific API routes.
- Draft-specific task context and prompts.
- Read-only member delegation task type.
- Confirmation orchestration with idempotent step tracking.
- UI for draft conversation, findings, preview, and confirmation.

## Detailed `.spec` Template

The target repository detailed document uses this standard schema. PM sections
are filled through user conversation and leader synthesis. ARCH sections are
filled from read-only code inspection and member findings.

| Section | Owner | Required content |
| --- | --- | --- |
| 1. Background | PM | Why this is needed, current problem, business impact |
| 2. History Query Record | PM | Duplicate-check sources and conclusion |
| 2.1 Split Decision | PM | Granularity judgment and independence check |
| 3. Goals | PM | Outcome-oriented goals |
| 4. Non-Goals | PM | Explicitly out of scope |
| 5. Impact Scope | PM + ARCH | Module checklist and file list |
| 6. User Scenarios | PM | Who, when, expected behavior |
| 7. Acceptance Criteria | PM | 3-7 verifiable checkboxes |
| 8. Technical Constraints | ARCH | Core primitive reuse, architecture boundaries, API constraints, interface alignment |
| 9. Design and Decision Record | ARCH | Implementation approach, key decisions, interface field mapping |
| 10. Implementation Task Breakdown | ARCH | Ordered tasks with dependencies, file paths, line references, complexity |
| 11. Verification Plan | ARCH | Type-check, lint, test, and scope-check commands |
| 12. Risks and Rollback | ARCH | Backend gaps and rollback path |
| 13. Related Information | PM | Related Multica issues, remote issues, MRs/PRs |
| 14. Agent Work Record | PM + ARCH | PM creation time, ARCH alignment time, contributing agents |

Enhancement to section 2: record the exact sources checked, not only the final
duplicate conclusion. Sources should include Multica issues, remote Git issues,
project `.spec`, and relevant code paths.

## Concise External Issue Template

The remote Git issue uses a smaller template:

```markdown
## Background

## Goal

## Non-Goals

## Scope

## User Scenarios

## Acceptance Criteria
- [ ]

## References
- Multica issue:
- Detailed spec:
```

It must not include:

- Detailed implementation task breakdown.
- Extensive code inspection notes.
- Internal agent findings.
- Long technical plan text.

## Data Model

### `issue_draft_session`

Stores the durable state of the draft flow.

- `id`
- `workspace_id`
- `project_id`
- `squad_id`
- `leader_agent_id`
- `primary_project_resource_id`
- `primary_local_path_snapshot`
- `status`: `clarifying`, `delegating`, `drafting`, `ready_for_review`,
  `creating`, `created`, `failed`, `cancelled`
- `created_by`
- `created_issue_id`
- `remote_issue_url`
- `spec_file_path`
- `git_commit_sha`
- `last_error`
- timestamps

The local path is snapshotted at draft creation so later resource edits do not
silently change the target repository for an in-flight draft.

### `issue_draft_message`

Stores the conversation and system timeline.

- `id`
- `session_id`
- `author_type`: `member`, `agent`, `system`
- `author_id`
- `message_type`: `user_message`, `question`, `answer`, `finding`,
  `draft_preview`, `status`, `error`
- `content`
- `metadata`
- timestamps

### `issue_draft_member_task`

Stores delegated read-only analysis tasks.

- `id`
- `session_id`
- `agent_id`
- `status`: `queued`, `running`, `completed`, `failed`, `cancelled`
- `skill_basis`: why this member was selected
- `read_scope`: files, modules, commands, or questions to inspect
- `findings`
- `error`
- timestamps

### `issue_draft_artifact`

Stores generated outputs and revisions.

- `id`
- `session_id`
- `artifact_type`: `detailed_spec`, `multica_issue`, `remote_issue`
- `revision`
- `content`
- `generated_by_agent_id`
- timestamps

### `issue_draft_confirm_step`

Stores idempotent confirmation progress.

- `session_id`
- `step`: `create_multica_issue`, `write_spec`, `commit_and_push`,
  `create_remote_issue`, `link_outputs`
- `status`: `pending`, `running`, `succeeded`, `failed`
- `external_id`
- `result_metadata`
- `error`
- timestamps

This table is the retry guard. Confirmation resumes at the first failed or
pending step and never repeats succeeded side effects.

## API Design

- `POST /api/issue-drafts`
  - Creates a session from `project_id`, `squad_id`, and initial prompt.
  - Resolves the project's primary local repository.
  - Resolves the squad leader and visible member agents.

- `GET /api/issue-drafts/:id`
  - Returns session state, messages, member tasks, and current artifacts.

- `POST /api/issue-drafts/:id/messages`
  - Adds user input and queues leader synthesis.

- `POST /api/issue-drafts/:id/delegate`
  - Starts read-only member analysis tasks. May be called by the leader
    orchestration service, not only directly by UI.

- `POST /api/issue-drafts/:id/generate`
  - Generates or refreshes detailed, Multica, and remote issue artifacts.

- `POST /api/issue-drafts/:id/confirm`
  - Runs idempotent confirmation steps.

- `POST /api/issue-drafts/:id/cancel`
  - Cancels the draft if it has not entered `creating` or `created`.

All responses consumed by TypeScript UI must be parsed through `parseWithFallback`
and zod schemas in `packages/core/api/schema.ts`.

## Agent Orchestration

### Leader Task

The leader owns the session. Its prompt includes:

- User's initial request.
- Project context.
- Primary local repository path.
- Squad roster and member skills.
- Read-only draft-stage policy.
- Required output schema.

Leader responsibilities:

- Ask clarifying questions.
- Decide which member agents should inspect which scope.
- Merge member findings.
- Produce detailed and concise artifacts.
- Surface unresolved questions to the user.

### Member Read-Only Task

Member tasks are linked to `issue_draft_member_task`, not to a Multica issue.
Their prompt includes:

- The specific question or module to inspect.
- The selected target repository path.
- A strict read-only policy.
- Required finding shape.

Allowed:

- Read files.
- Search code.
- Run safe read-only checks.
- Summarize findings with file and line references.

Forbidden:

- Modify files.
- Create commits.
- Push branches.
- Create issues.
- Change `.spec`.

### Draft Generation

The leader creates three artifacts:

1. `detailed_spec`: full 14-section template for target repo `.spec`.
2. `multica_issue`: execution-focused issue body with spec and remote links.
3. `remote_issue`: concise external issue body.

The user can ask follow-up questions, edit the draft, regenerate, or confirm.

## Confirmation Flow

Confirmation steps run in this order:

1. Create Multica issue through `IssueService.Create`.
2. Write detailed spec into the primary local repository `.spec/`.
3. Commit and push the spec file using concise commit metadata.
4. Create remote Git issue with concise content.
5. Link all outputs back to the draft and Multica issue metadata.

Recommended spec path:

```text
.spec/issues/<multica-issue-identifier>.md
```

If the identifier is not available before issue creation, write after Multica
issue creation rather than using a temporary draft filename in the target repo.
Draft artifacts remain in the Multica database until confirmation.

Commit message example:

```text
docs(spec): add <ISSUE-ID> requirement draft
```

Remote issue references:

- Multica issue URL.
- Repository spec path and commit SHA.

## Git and Provider Integration

Local repository writes run through the daemon that owns the selected
`local_directory` resource. The server should not write arbitrary paths itself.

Remote issue creation should reuse the issue bridge boundary:

- GitLab support can reuse the existing bridge service.
- GitHub support should be added through the same provider abstraction rather
  than a one-off handler.
- The remote issue body is the concise artifact only.

If commit/push succeeds and remote issue creation fails, the session remains
`failed` with completed step metadata. Retrying confirmation creates only the
remote issue and link step.

## UI Design

Entry points:

- New Issue dialog gains a "Clarify with squad" path.
- Project pages may seed `project_id`.
- Squad pages may seed `squad_id`.

Draft session screen:

- Header: project, squad, primary repository, status.
- Left/main: conversation with user, leader, and system messages.
- Right panel: current draft preview with tabs:
  - Detailed `.spec`
  - Multica issue
  - Remote issue
- Findings panel: member tasks, status, and summarized findings.
- Footer actions:
  - Send reply.
  - Regenerate draft.
  - Confirm and create.
  - Cancel.

The confirm button is disabled until:

- Required artifacts exist.
- Required template sections are filled.
- No required member task is running.
- User has not unresolved required questions.

## State and Package Boundaries

- TanStack Query owns draft session server state.
- Zustand may hold local UI state only: selected draft tab, composer draft,
  expanded findings.
- Shared hooks and API client additions live in `packages/core`.
- Shared UI and pages live in `packages/views`.
- Web and desktop platform routes wire to the shared views.
- `packages/views` must use `NavigationAdapter` and must not import `next/*` or
  `react-router-dom`.

## Error Handling

- Missing primary local repository: block session creation with a clear project
  setup error.
- Offline leader or member runtime: allow user to pick another squad/member or
  retry later.
- Member task failure: show finding failure and let leader continue or retry.
- Spec write failure: keep Multica issue if already created, mark confirm step
  failed, and allow retry.
- Push failure: keep local commit metadata if available, report retry command or
  retry through daemon.
- Remote issue failure: do not duplicate Multica issue or spec commit on retry.
- Duplicate Multica issue: surface duplicate and let user open existing issue,
  revise title, or force create if supported by existing duplicate flow.

## Security and Safety

- Draft-stage member tasks are read-only.
- Server validates project, squad, member, and resource workspace ownership.
- The local path must come from a project resource owned by a registered daemon.
- Confirmation writes only to the selected primary local repository.
- Remote issue body must be derived from the concise artifact, not the detailed
  artifact.
- Sensitive local paths may be shown in desktop/dev contexts but should be
  truncated or labelled carefully in shared/web contexts.

## Testing Plan

Backend:

- Draft creation validates workspace/project/squad/resource boundaries.
- Draft creation rejects projects without a primary `local_directory`.
- Member task creation records read-only scope and selected agent.
- Confirm is idempotent across partial failures.
- Confirm uses `IssueService.Create`.
- Remote issue creation receives concise content only.

Frontend/core:

- API schemas parse draft/session/artifact responses with fallback.
- Create flow handles missing primary repo, offline squad, running member tasks,
  and failed confirmation steps.
- Draft preview tabs show detailed vs concise versions correctly.

E2E or integration:

- User creates draft, member finding appears, leader generates artifacts, user
  confirms, and all three outputs are linked.
- Retry after remote issue failure does not create a duplicate Multica issue or
  duplicate spec commit.

Useful commands:

```bash
pnpm typecheck
pnpm test
make test
```

Run narrower package tests while iterating, then broader checks when the feature
spans API, core, and views.

## Rollout Plan

1. Add database schema and backend service behind a feature flag.
2. Add API client, schemas, and focused backend tests.
3. Add draft session view and route wiring for web/desktop.
4. Add leader-only draft generation path.
5. Add member read-only delegation.
6. Add confirmation orchestration for Multica issue and `.spec` commit/push.
7. Add remote Git issue creation.
8. Enable for internal projects and iterate on templates.

## Open Implementation Notes

- Decide whether "primary local repository" is represented by the first
  positioned `local_directory` resource or by an explicit `is_primary` field.
  The product behavior is confirmed; the storage representation can be chosen
  during implementation.
- Decide whether GitHub remote issue creation is in the first implementation
  slice or follows GitLab after the provider abstraction is ready.
- Decide exact daemon API shape for writing, committing, and pushing spec files.

