# GitLab Issue Bridge Design

## Purpose

Adapt Multica into a local workflow hub where Multica issues can be created through the workspace `issue-creator` skill, pushed to GitLab, and kept in sync with remote GitLab issues.

The first implementation is GitLab-first, with provider-shaped storage and service interfaces so GitHub or another issue provider can be added later without replacing the feature.

## Goals

- Let each workspace configure a GitLab integration with one workspace-level token.
- Let each synced project or repository enable or disable GitLab issue sync.
- Use a workspace Skill named `issue-creator` as the default issue creation template.
- Create the `issue-creator` skill automatically when enabling the integration if the workspace does not already have it.
- Push newly created Multica issues to GitLab when their project or repository has the bridge enabled.
- Poll GitLab issues into Multica on a configurable interval.
- Sync GitLab labels into Multica labels.
- Optionally assign pulled issues to a configured agent or squad.
- Close the linked GitLab issue when a local Multica issue reaches `done`.

## Non-Goals

- Full bidirectional editing sync for title, description, and labels.
- Using local `glab` auth or daemon-owned CLI state for the server-side integration.
- Supporting GitHub in the first implementation.
- Building a complex rule engine for assignee routing from labels, milestones, or GitLab assignees.
- Syncing GitLab comments, milestones, or assignees in the first implementation.

## Product Behavior

### Workspace Integration

Workspace settings gain an Issue Integrations section with a GitLab provider.

The workspace-level GitLab configuration includes:

- Provider: `gitlab`
- GitLab base URL, defaulting to `https://gitlab.com`
- Workspace GitLab token
- Default issue skill ID
- Global polling enabled flag
- Default polling interval

When GitLab integration is enabled, the backend looks for a workspace skill named `issue-creator`. If it exists, that skill becomes the default issue skill. If it does not exist, the backend creates a default skill from the bundled `issue-creator` template and stores that skill ID in the integration config.

Users can edit the skill through the existing Skills UI. The integration stores the skill ID, not a copy of the skill body.

### Project Or Repository Sync

Projects or repository resources can enable GitLab issue sync.

Per project or repository configuration includes:

- Sync enabled flag
- GitLab project path or project ID
- Polling interval override, nullable when inheriting the workspace default
- State mapping from GitLab states to Multica issue statuses
- Auto-assign enabled flag
- Default assignee type: `agent` or `squad`
- Default assignee ID

Default state mapping:

- GitLab `opened` maps to Multica `backlog`
- GitLab `closed` maps to Multica `done`

The user can change the mapping. If auto-assignment is enabled and the desired outcome is immediate agent execution, the UI warns when the selected target status is `backlog` because backlog issues do not start agent work.

### Local Create To GitLab

When a user creates a Multica issue in a project or repository with GitLab bridge enabled:

1. The normal Multica issue creation path still creates the local issue.
2. The backend renders or applies the configured `issue-creator` skill to derive the remote title and description.
3. The backend calls the GitLab API to create the remote issue.
4. The backend writes remote linkage into the local issue metadata.
5. If GitLab creation fails, the local issue remains created and records sync error metadata so the user can retry.

The first implementation treats the skill as a content template and instruction source used by server code. It does not depend on spawning an agent or invoking `glab`.

### GitLab Poll To Multica

The backend scheduler polls enabled GitLab sync configs at their effective interval.

For each remote GitLab issue:

1. Match an existing Multica issue by metadata provider, project, and remote issue ID.
2. If matched, update local title, description, mapped status, remote metadata, and labels.
3. If unmatched, create a new Multica issue with title, description, mapped status, labels, remote metadata, and the configured default assignee when auto-assignment is enabled.
4. If auto-assignment is enabled and an assignee is configured, create the issue with that assignee.

Auto-assigned pulled issues can trigger existing Multica agent execution if the target status is not `backlog`, because `IssueService.Create` already enqueues work for assigned agent-like actors when creation is runnable.

### Local Done Closes GitLab

When a linked local Multica issue transitions to `done`, the backend closes the corresponding GitLab issue through the GitLab API.

This is the only first-version local-to-remote update after creation. Title, description, and label edits in Multica do not update GitLab in v1.

## Data Model

Add a workspace-scoped integration table for issue providers.

Table: `issue_integration`

- `id`
- `workspace_id`
- `provider`
- `name`
- `base_url`
- `encrypted_token`
- `default_issue_skill_id`
- `polling_enabled`
- `default_poll_interval_seconds`
- `config`
- `created_at`
- `updated_at`

Add a project or repository sync config table.

Table: `issue_sync_config`

- `id`
- `workspace_id`
- `integration_id`
- `scope_type`: `project` or `repo_resource`
- `scope_id`
- `remote_project_ref`
- `sync_enabled`
- `poll_interval_seconds`
- `state_mapping`
- `auto_assign_enabled`
- `default_assignee_type`
- `default_assignee_id`
- `last_poll_at`
- `last_successful_poll_at`
- `last_error`
- `created_at`
- `updated_at`

Use existing issue metadata for remote linkage:

```json
{
  "issue_bridge": {
    "provider": "gitlab",
    "integration_id": "...",
    "sync_config_id": "...",
    "remote_project_ref": "group/project",
    "remote_issue_id": 123456,
    "remote_iid": 42,
    "remote_url": "https://gitlab.example.com/group/project/-/issues/42",
    "remote_state": "opened",
    "last_synced_at": "2026-06-26T00:00:00Z",
    "last_sync_error": ""
  }
}
```

Label sync reuses existing Multica labels by name when possible. If GitLab provides a valid color, use it. Otherwise use the existing default Multica label color behavior.

## Backend Architecture

Introduce an issue bridge service layer with provider interfaces.

Core interfaces:

- `IssueProviderClient`
- `ListIssues`
- `CreateIssue`
- `CloseIssue`
- `ListLabels`

First provider implementation:

- `GitLabIssueProvider`

The scheduler lives server-side and uses persisted sync configs. It does not require a browser window, desktop app, local daemon, or `glab` process.

`IssueService.Create` remains the single local issue creation path. Bridge behavior is integrated through a post-create service hook or a caller-level orchestration layer, while preserving duplicate guard, numbering, broadcasts, analytics, attachments, and task enqueue behavior.

For local `done` transitions, use the existing issue update path to trigger a bridge close operation after the local transaction succeeds. Failures to close GitLab do not roll back the local status update; they are recorded and surfaced as sync state.

## API Surface

Add authenticated workspace-scoped endpoints:

- `GET /api/issue-integrations`
- `POST /api/issue-integrations/gitlab`
- `PUT /api/issue-integrations/{id}`
- `DELETE /api/issue-integrations/{id}`
- `POST /api/issue-integrations/{id}/test`
- `GET /api/issue-sync-configs`
- `POST /api/issue-sync-configs`
- `PUT /api/issue-sync-configs/{id}`
- `POST /api/issue-sync-configs/{id}/sync-now`

Responses consumed by frontend code must use zod schemas in `packages/core/api/schemas.ts` and `parseWithFallback`, following the repository API compatibility rules.

## Frontend Architecture

Settings UI is shared in `packages/views`, not app-specific.

Add an Issue Integrations settings area where users can:

- Enable GitLab integration.
- Enter base URL and token.
- Test the connection.
- Choose the default `issue-creator` skill.
- See whether the default skill was auto-created.
- Configure workspace polling defaults.

Project or repository configuration UI lets users:

- Enable sync for the project or repo.
- Select or enter remote GitLab project reference.
- Configure state mapping.
- Configure polling interval.
- Enable or disable auto-assignment.
- Pick the default agent or squad.
- Trigger sync now.
- See last sync status.

The issue detail page shows remote issue metadata as a small linked strip when present, using the existing metadata section patterns.

## Error Handling

- Token test failures are explicit and do not save a broken token unless the user chooses to save anyway.
- Poll failures update `last_error` and keep the previous successful sync state.
- Remote create failures leave the local issue intact and record retryable metadata.
- Remote close failures leave the local issue as `done` and record retryable metadata.
- Unknown remote state values use the configured fallback mapping or leave the local status unchanged.
- Deleted or inaccessible remote issues are marked in metadata but do not delete the local issue.

## Security

The GitLab token must be encrypted at rest. It is never returned to the frontend after creation.

Server logs must not include token values. Error messages returned to the client include enough provider context to debug permissions or project references without exposing secrets.

Workspace membership checks apply to all integration and sync config endpoints.

## Testing Strategy

Backend tests:

- Integration config CRUD with workspace scoping.
- GitLab provider request shaping and response parsing.
- Poll creates missing local issues from remote issues.
- Poll updates existing linked issues.
- Poll syncs labels by name and color.
- Auto-assignment creates runnable issues only when configured.
- Local issue creation pushes to GitLab and records metadata.
- Local `done` closes the remote GitLab issue.
- Provider failures record errors without rolling back local state.

Frontend tests:

- Settings form validates GitLab base URL and token presence.
- Default skill selection and auto-created skill messaging.
- Sync config state mapping controls.
- Auto-assignment toggle and agent/squad picker behavior.
- Last sync status rendering.

Verification commands:

- `pnpm typecheck`
- `pnpm test`
- `make test`

Use narrower package tests while iterating, then broader checks before merging.

## Rollout Plan

1. Add database schema, sqlc queries, and backend models.
2. Add GitLab provider client and issue bridge service.
3. Add API endpoints and schemas.
4. Add scheduler polling.
5. Wire local create and local done close hooks.
6. Add shared settings UI.
7. Add remote metadata display on issue detail.
8. Add tests and verification.

## Fixed Implementation Choices

- The auto-created `issue-creator` skill is bundled from the local skill content and its direct workflow references, converted into a workspace skill during integration setup.
- Workspace-level integration controls live under workspace settings.
- Project-level sync controls live with the existing project/repository configuration UI, using the nearest existing settings surface rather than adding a new top-level route.
