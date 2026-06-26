# GitLab Issue Bridge Settings UI Design

## Goal

Add a shared Settings UI for configuring the GitLab issue bridge from web and desktop. The UI lives under Settings -> Integrations and lets workspace admins configure:

- GitLab connection settings.
- The default issue creation skill.
- Remote issue sync rules for projects or repository resources.
- Optional auto-assignment of pulled issues to an agent or squad.

This design builds on the existing issue bridge foundation:

- Backend integration and sync config APIs.
- Core API types, schemas, and client methods.
- Default `issue-creator` skill creation on GitLab integration create.

## Non-Goals

This UI does not implement remote polling, local issue creation push, or local done-to-close behavior. It only configures the records those workers will use later. When sync is enabled, copy should say that polling takes effect after the sync worker is enabled.

This UI does not add a multi-step onboarding wizard. It uses the existing Settings page pattern: simple sections, inline cards, and dialogs only for destructive confirmation.

## Placement

Add a new "GitLab Issues" section inside `packages/views/settings/components/integrations-tab.tsx`.

The order should be:

1. GitLab Issues
2. Lark

GitHub App remains its own top-level Settings tab. The new GitLab Issues section is separate because it configures GitLab issue synchronization through personal/project tokens, not the existing GitHub App workflow.

## Architecture

### Core Query Layer

Create `packages/core/issue-bridge/queries.ts` with:

- `issueBridgeKeys`
- `issueIntegrationsOptions(wsId)`
- `issueSyncConfigsOptions(wsId)`
- `useCreateGitLabIssueIntegration(wsId)`
- `useUpdateIssueIntegration(wsId)`
- `useDeleteIssueIntegration(wsId)`
- `useTestIssueIntegration()`
- `useCreateIssueSyncConfig(wsId)`
- `useUpdateIssueSyncConfig(wsId)`
- `useDeleteIssueSyncConfig(wsId)`

Mutations invalidate the relevant issue bridge query keys on settle. The feature uses React Query for server state; no Zustand store is needed.

### View Layer

Create `packages/views/settings/components/gitlab-issues-tab.tsx`.

The component owns only transient form state:

- Selected integration being edited.
- New integration draft.
- Selected sync rule being edited.
- New sync rule draft.
- Delete confirmation target.

All persisted data comes from React Query.

### Settings Integration

Modify `packages/views/settings/components/integrations-tab.tsx` to render:

- A section heading for GitLab Issues.
- The new `GitLabIssuesTab`.
- Existing Lark section unchanged.

## Data Dependencies

The UI reads:

- Issue bridge integrations via `issueIntegrationsOptions(wsId)`.
- Issue sync configs via `issueSyncConfigsOptions(wsId)`.
- Current workspace member list via `memberListOptions(wsId)` for admin/owner gating.
- Skills list via the existing skills API/query layer; the default selected skill should prefer a skill named `issue-creator` when present.
- Projects via existing project query helpers.
- Project resources via existing project resource query helpers.
- Agents via existing agent query helpers.
- Squads via existing squad query helpers.

If an existing query helper is missing for one of these lists, add it in `packages/core/` rather than calling `api.*` directly from the view.

## Permissions

All workspace members can view existing GitLab issue bridge settings.

Only workspace owners and admins can:

- Create, update, or delete integrations.
- Test a GitLab integration connection.
- Create, update, or delete sync rules.

The UI should hide or disable write actions for non-admin members and show a short read-only hint. The backend remains the authority and already rejects unauthorized writes.

## UI Structure

### Connection Card

The connection card shows one GitLab integration at a time. The first version supports multiple integrations in the data model but keeps the UI optimized for the common single-integration case:

- If no integration exists, show a compact setup form.
- If integrations exist, show the first integration as the primary connection and list any additional integrations below as compact rows.
- Editing opens inline form state for that integration.

Fields:

- Name, defaulting to `GitLab`.
- Base URL, defaulting to `https://gitlab.com`.
- Token.
- Default issue skill.
- Polling enabled.
- Default poll interval seconds.

Token behavior:

- Creation requires a token.
- Editing never shows the stored token.
- Leaving the token field blank on edit means "keep current token".
- Entering a token on edit rotates the stored token.

Actions:

- Save.
- Cancel edit.
- Test connection.
- Delete integration.

Test connection success shows a toast with the returned GitLab username/name. Failure shows the API error.

If token storage is not configured, create/test/update-token operations will fail with the backend error. The UI should surface that error directly and include a small operator hint mentioning `MULTICA_ISSUE_BRIDGE_SECRET_KEY`.

### Sync Rules Section

The sync rules section lists all `issue_sync_config` rows.

Each rule row shows:

- Scope type: Project or Repository resource.
- Scope name.
- GitLab project ref.
- Sync enabled state.
- Poll interval override or "uses integration default".
- Auto-assignment target, if enabled.
- Last poll metadata if present.
- Last error if present.

Rule form fields:

- Scope type: `project` or `repo_resource`.
- Scope selector: project list or repository resource list.
- GitLab project ref: path like `group/project` or numeric project id.
- Sync enabled.
- Poll interval seconds, optional.
- State mapping display: default opened -> backlog, closed -> done. Keep this read-only in the first UI slice.
- Auto-assign enabled.
- Default assignee type: agent or squad.
- Default assignee selector.

When auto-assign is off, hide assignee type and selector. When enabled, both are required before saving.

When poll interval is blank, save `null` so the backend uses the integration default. When set, the UI should require at least 60 seconds.

### Empty States

If no GitLab integration exists, sync rule creation is disabled with copy explaining that a GitLab connection must be saved first.

If no projects or repo resources exist, the relevant scope selector shows an empty message and disables save.

If no agents or squads exist, auto-assignment can still be disabled. Enabling it shows an empty selector state.

## Error Handling

Use toasts for mutation failures and successes.

Inline validation should catch:

- Missing base URL.
- Missing token on create.
- Missing remote project ref.
- Missing scope.
- Poll interval below 60 seconds.
- Missing assignee target when auto-assign is enabled.

API errors are displayed through toast messages without parsing brittle strings.

## I18n

Add settings copy under `issue_bridge` in `packages/views/locales/*/settings.json`.

Keep keys stable and mirrored across English, Simplified Chinese, Korean, and Japanese locale files. Use plain product language; do not mention implementation internals except the operator env var hint.

## Testing

Add tests in `packages/views/settings/components/gitlab-issues-tab.test.tsx` for:

- Rendering the empty state and setup form for admins.
- Rendering read-only state for non-admin members.
- Token field is blank when editing an existing integration.
- Saving a new integration calls the create mutation with token and default fields.
- Saving an existing integration with a blank token does not send a token.
- Test connection button calls the test mutation and shows success copy.
- Sync rule form saves project/repo scope, remote project ref, poll interval, and auto-assignment fields.
- Auto-assignment fields hide when disabled and are required when enabled.

Add or update locale parity tests if the repository's existing parity test does not already cover the new settings keys automatically.

Run:

- `./node_modules/.bin/vitest run packages/views/settings/components/gitlab-issues-tab.test.tsx`
- `./node_modules/.bin/vitest run packages/views/locales/parity.test.ts`
- `./node_modules/.bin/tsc -p packages/views/tsconfig.json --noEmit`

If pnpm command shims work in the local environment, equivalent package scripts are acceptable.

## Implementation Order

1. Add core issue bridge React Query helpers.
2. Add GitLab Issues settings component with read-only listing.
3. Add integration create/edit/test/delete form behavior.
4. Add sync rule create/edit/delete form behavior.
5. Add locale strings.
6. Add focused component tests and run verification.
