# GitLab Issue Bridge Settings UI Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build the shared Settings -> Integrations UI for configuring GitLab issue bridge connections and sync rules.

**Architecture:** Add a small `packages/core/issue-bridge` React Query layer over the existing API client, then render a shared `GitLabIssuesTab` inside the existing Integrations settings tab. Keep server data in TanStack Query, keep only form drafts in component state, and reuse existing workspace/project/resource/agent/squad/skill query helpers.

**Tech Stack:** React, TypeScript, TanStack Query, shadcn/Base UI components from `@multica/ui`, Vitest, existing `@multica/core` API client.

---

## Scope

This plan implements:

- Core issue bridge query keys/options/mutation hooks.
- Settings -> Integrations -> GitLab Issues UI.
- GitLab integration create/edit/test/delete.
- Sync rule create/edit/delete for project and repo resource scopes.
- Auto-assignment configuration for agent or squad.
- English and mirrored locale keys for all supported locales.
- Focused component tests and type verification.

This plan does not implement:

- GitLab polling worker.
- Local issue create -> GitLab issue push.
- Local done -> GitLab close.
- Runtime status monitoring for sync jobs.

## File Map

- Create `packages/core/issue-bridge/queries.ts`: query keys/options and mutation hooks for issue bridge APIs.
- Create `packages/core/issue-bridge/index.ts`: public exports for the issue bridge core module.
- Modify `packages/core/package.json`: export `./issue-bridge`.
- Create `packages/views/settings/components/gitlab-issues-tab.tsx`: settings UI for GitLab connection and sync rules.
- Create `packages/views/settings/components/gitlab-issues-tab.test.tsx`: component tests.
- Modify `packages/views/settings/components/integrations-tab.tsx`: render the GitLab Issues section above Lark.
- Modify `packages/views/locales/en/settings.json`: add `issue_bridge` copy.
- Modify `packages/views/locales/zh-Hans/settings.json`: mirrored Chinese copy.
- Modify `packages/views/locales/ko/settings.json`: mirrored Korean copy.
- Modify `packages/views/locales/ja/settings.json`: mirrored Japanese copy.

---

### Task 1: Core Issue Bridge Query Layer

**Files:**
- Create: `packages/core/issue-bridge/queries.ts`
- Create: `packages/core/issue-bridge/index.ts`
- Modify: `packages/core/package.json`

- [ ] **Step 1: Add query keys and options**

Create `packages/core/issue-bridge/queries.ts` with this structure:

```ts
import { queryOptions, useMutation, useQueryClient } from "@tanstack/react-query";
import { api } from "../api";
import type {
  CreateGitLabIssueIntegrationRequest,
  UpdateIssueIntegrationRequest,
  UpsertIssueSyncConfigRequest,
} from "../types";

export const issueBridgeKeys = {
  all: (wsId: string) => ["workspaces", wsId, "issue-bridge"] as const,
  integrations: (wsId: string) =>
    [...issueBridgeKeys.all(wsId), "integrations"] as const,
  syncConfigs: (wsId: string) =>
    [...issueBridgeKeys.all(wsId), "sync-configs"] as const,
};

export function issueIntegrationsOptions(wsId: string) {
  return queryOptions({
    queryKey: issueBridgeKeys.integrations(wsId),
    queryFn: () => api.listIssueIntegrations(),
    enabled: !!wsId,
  });
}

export function issueSyncConfigsOptions(wsId: string) {
  return queryOptions({
    queryKey: issueBridgeKeys.syncConfigs(wsId),
    queryFn: () => api.listIssueSyncConfigs(),
    enabled: !!wsId,
  });
}
```

- [ ] **Step 2: Add integration mutation hooks**

Add these hooks to the same file:

```ts
export function useCreateGitLabIssueIntegration(wsId: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (data: CreateGitLabIssueIntegrationRequest) =>
      api.createGitLabIssueIntegration(data),
    onSettled: () => {
      qc.invalidateQueries({ queryKey: issueBridgeKeys.integrations(wsId) });
    },
  });
}

export function useUpdateIssueIntegration(wsId: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({
      id,
      data,
    }: {
      id: string;
      data: UpdateIssueIntegrationRequest;
    }) => api.updateIssueIntegration(id, data),
    onSettled: () => {
      qc.invalidateQueries({ queryKey: issueBridgeKeys.integrations(wsId) });
      qc.invalidateQueries({ queryKey: issueBridgeKeys.syncConfigs(wsId) });
    },
  });
}

export function useDeleteIssueIntegration(wsId: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (id: string) => api.deleteIssueIntegration(id),
    onSettled: () => {
      qc.invalidateQueries({ queryKey: issueBridgeKeys.integrations(wsId) });
      qc.invalidateQueries({ queryKey: issueBridgeKeys.syncConfigs(wsId) });
    },
  });
}

export function useTestIssueIntegration() {
  return useMutation({
    mutationFn: (id: string) => api.testIssueIntegration(id),
  });
}
```

- [ ] **Step 3: Add sync config mutation hooks**

Add these hooks to the same file:

```ts
export function useCreateIssueSyncConfig(wsId: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (data: UpsertIssueSyncConfigRequest) =>
      api.createIssueSyncConfig(data),
    onSettled: () => {
      qc.invalidateQueries({ queryKey: issueBridgeKeys.syncConfigs(wsId) });
    },
  });
}

export function useUpdateIssueSyncConfig(wsId: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({
      id,
      data,
    }: {
      id: string;
      data: UpsertIssueSyncConfigRequest;
    }) => api.updateIssueSyncConfig(id, data),
    onSettled: () => {
      qc.invalidateQueries({ queryKey: issueBridgeKeys.syncConfigs(wsId) });
    },
  });
}

export function useDeleteIssueSyncConfig(wsId: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (id: string) => api.deleteIssueSyncConfig(id),
    onSettled: () => {
      qc.invalidateQueries({ queryKey: issueBridgeKeys.syncConfigs(wsId) });
    },
  });
}
```

- [ ] **Step 4: Export the module**

Create `packages/core/issue-bridge/index.ts`:

```ts
export * from "./queries";
```

Add this export to `packages/core/package.json` inside `exports`:

```json
"./issue-bridge": "./issue-bridge/index.ts",
```

- [ ] **Step 5: Verify core typecheck**

Run:

```bash
./node_modules/.bin/tsc -p packages/core/tsconfig.json --noEmit
```

Expected: exits 0.

- [ ] **Step 6: Commit**

```bash
git add packages/core/issue-bridge packages/core/package.json
git commit -m "feat(issue-bridge): add query hooks"
```

---

### Task 2: GitLab Issues Settings UI Skeleton

**Files:**
- Create: `packages/views/settings/components/gitlab-issues-tab.tsx`
- Modify: `packages/views/settings/components/integrations-tab.tsx`
- Modify: `packages/views/locales/*/settings.json`

- [ ] **Step 1: Add locale keys**

Add an `issue_bridge` object to every `packages/views/locales/*/settings.json`. English source copy:

```json
"issue_bridge": {
  "section_title": "GitLab Issues",
  "page_description": "Connect GitLab issue projects to Multica issues, choose the issue creation skill, and configure how pulled issues are assigned.",
  "read_only_hint": "Read-only view. Only workspace admins and owners can update GitLab issue bridge settings.",
  "connection_title": "GitLab connection",
  "connection_empty_title": "No GitLab connection",
  "connection_empty_description": "Add a GitLab access token before creating sync rules.",
  "connection_saved_title": "Connected",
  "connection_extra_count": "{{count}} additional connection configured",
  "connection_extra_count_plural": "{{count}} additional connections configured",
  "operator_hint": "Token storage requires MULTICA_ISSUE_BRIDGE_SECRET_KEY on the server.",
  "sync_rules_title": "Sync rules",
  "sync_rules_description": "Map Multica projects or repository resources to GitLab projects. Polling starts when the sync worker is enabled.",
  "sync_rules_disabled": "Save a GitLab connection before adding sync rules.",
  "empty_rules_title": "No sync rules",
  "empty_rules_description": "Create a rule to pull GitLab issues into a project or repository resource.",
  "loading": "Loading GitLab issue bridge settings…",
  "add_connection": "Add connection",
  "edit": "Edit",
  "delete": "Delete",
  "cancel": "Cancel",
  "save": "Save",
  "saving": "Saving…",
  "testing": "Testing…",
  "test_connection": "Test connection",
  "add_rule": "Add rule"
}
```

Use mirrored keys in zh-Hans, ko, and ja. Translation quality can be simple but must preserve meaning.

- [ ] **Step 2: Create the skeleton component**

Create `packages/views/settings/components/gitlab-issues-tab.tsx`:

```tsx
"use client";

import { useMemo } from "react";
import { useQuery } from "@tanstack/react-query";
import { GitBranch, GitPullRequestArrow, ShieldCheck } from "lucide-react";
import { Card, CardContent } from "@multica/ui/components/ui/card";
import { Button } from "@multica/ui/components/ui/button";
import { useAuthStore } from "@multica/core/auth";
import { useWorkspaceId } from "@multica/core/hooks";
import { issueIntegrationsOptions, issueSyncConfigsOptions } from "@multica/core/issue-bridge";
import { memberListOptions } from "@multica/core/workspace/queries";
import { useT } from "../../i18n";

export function GitLabIssuesTab() {
  const { t } = useT("settings");
  const wsId = useWorkspaceId();
  const user = useAuthStore((s) => s.user);

  const { data: members = [] } = useQuery(memberListOptions(wsId));
  const currentMember = members.find((m) => m.user_id === user?.id) ?? null;
  const canManage = currentMember?.role === "owner" || currentMember?.role === "admin";

  const { data: integrationData, isLoading: integrationsLoading } = useQuery(
    issueIntegrationsOptions(wsId),
  );
  const { data: syncData, isLoading: syncLoading } = useQuery(
    issueSyncConfigsOptions(wsId),
  );

  const integrations = integrationData?.integrations ?? [];
  const syncConfigs = syncData?.sync_configs ?? [];
  const primaryIntegration = integrations[0] ?? null;
  const loading = integrationsLoading || syncLoading;

  const extraCount = Math.max(0, integrations.length - 1);
  const extraCopy = useMemo(() => {
    if (extraCount === 0) return null;
    return t(($) => $.issue_bridge.connection_extra_count, { count: extraCount });
  }, [extraCount, t]);

  if (loading) {
    return (
      <Card>
        <CardContent>
          <p className="text-sm text-muted-foreground">
            {t(($) => $.issue_bridge.loading)}
          </p>
        </CardContent>
      </Card>
    );
  }

  return (
    <div className="space-y-8">
      <section className="space-y-1">
        <p className="text-sm text-muted-foreground">
          {t(($) => $.issue_bridge.page_description)}
        </p>
        {!canManage && (
          <p className="text-xs text-muted-foreground">
            {t(($) => $.issue_bridge.read_only_hint)}
          </p>
        )}
      </section>

      <section className="space-y-3">
        <h3 className="text-sm font-semibold">
          {t(($) => $.issue_bridge.connection_title)}
        </h3>
        <Card>
          <CardContent className="space-y-4">
            {primaryIntegration ? (
              <div className="flex items-start justify-between gap-4">
                <div className="flex items-start gap-3">
                  <div className="rounded-md border bg-muted/50 p-2 text-muted-foreground">
                    <GitBranch className="h-4 w-4" />
                  </div>
                  <div className="space-y-1">
                    <p className="text-sm font-medium">{primaryIntegration.name}</p>
                    <p className="text-xs text-muted-foreground">
                      {primaryIntegration.base_url}
                    </p>
                    {extraCopy && (
                      <p className="text-xs text-muted-foreground">{extraCopy}</p>
                    )}
                  </div>
                </div>
                {canManage && (
                  <div className="flex items-center gap-2">
                    <Button variant="outline" size="sm">
                      {t(($) => $.issue_bridge.test_connection)}
                    </Button>
                    <Button variant="outline" size="sm">
                      {t(($) => $.issue_bridge.edit)}
                    </Button>
                  </div>
                )}
              </div>
            ) : (
              <div className="flex items-start justify-between gap-4">
                <div className="flex items-start gap-3">
                  <div className="rounded-md border bg-muted/50 p-2 text-muted-foreground">
                    <ShieldCheck className="h-4 w-4" />
                  </div>
                  <div className="space-y-1">
                    <p className="text-sm font-medium">
                      {t(($) => $.issue_bridge.connection_empty_title)}
                    </p>
                    <p className="text-xs text-muted-foreground">
                      {t(($) => $.issue_bridge.connection_empty_description)}
                    </p>
                  </div>
                </div>
                {canManage && (
                  <Button size="sm">{t(($) => $.issue_bridge.add_connection)}</Button>
                )}
              </div>
            )}
          </CardContent>
        </Card>
      </section>

      <section className="space-y-3">
        <div className="flex items-center justify-between gap-4">
          <div>
            <h3 className="text-sm font-semibold">
              {t(($) => $.issue_bridge.sync_rules_title)}
            </h3>
            <p className="text-xs text-muted-foreground">
              {t(($) => $.issue_bridge.sync_rules_description)}
            </p>
          </div>
          {canManage && (
            <Button size="sm" disabled={!primaryIntegration}>
              {t(($) => $.issue_bridge.add_rule)}
            </Button>
          )}
        </div>
        <Card>
          <CardContent className="space-y-3">
            {!primaryIntegration ? (
              <p className="text-sm text-muted-foreground">
                {t(($) => $.issue_bridge.sync_rules_disabled)}
              </p>
            ) : syncConfigs.length === 0 ? (
              <div className="flex items-start gap-3">
                <GitPullRequestArrow className="mt-0.5 h-4 w-4 text-muted-foreground" />
                <div className="space-y-1">
                  <p className="text-sm font-medium">
                    {t(($) => $.issue_bridge.empty_rules_title)}
                  </p>
                  <p className="text-xs text-muted-foreground">
                    {t(($) => $.issue_bridge.empty_rules_description)}
                  </p>
                </div>
              </div>
            ) : (
              <div className="divide-y">
                {syncConfigs.map((rule) => (
                  <div key={rule.id} className="py-3 first:pt-0 last:pb-0">
                    <p className="text-sm font-medium">{rule.remote_project_ref}</p>
                    <p className="text-xs text-muted-foreground">
                      {rule.scope_type} · {rule.sync_enabled ? "sync on" : "sync off"}
                    </p>
                  </div>
                ))}
              </div>
            )}
          </CardContent>
        </Card>
      </section>
    </div>
  );
}
```

- [ ] **Step 3: Mount the section**

Modify `packages/views/settings/components/integrations-tab.tsx`:

```tsx
import { GitLabIssuesTab } from "./gitlab-issues-tab";
import { LarkTab } from "./lark-tab";
import { useT } from "../../i18n";

export function IntegrationsTab() {
  const { t } = useT("settings");
  return (
    <div className="space-y-10">
      <section className="space-y-4">
        <h2 className="text-sm font-semibold">
          {t(($) => $.issue_bridge.section_title)}
        </h2>
        <GitLabIssuesTab />
      </section>

      <section className="space-y-4">
        <h2 className="text-sm font-semibold">{t(($) => $.lark.section_title)}</h2>
        <LarkTab />
      </section>
    </div>
  );
}
```

- [ ] **Step 4: Verify skeleton typecheck**

Run:

```bash
./node_modules/.bin/tsc -p packages/views/tsconfig.json --noEmit
```

Expected: exits 0.

- [ ] **Step 5: Commit**

```bash
git add packages/core/issue-bridge packages/core/package.json packages/views/settings/components/gitlab-issues-tab.tsx packages/views/settings/components/integrations-tab.tsx packages/views/locales
git commit -m "feat(settings): add gitlab issue bridge section"
```

---

### Task 3: Integration Form Behavior

**Files:**
- Modify: `packages/views/settings/components/gitlab-issues-tab.tsx`
- Test: `packages/views/settings/components/gitlab-issues-tab.test.tsx`

- [ ] **Step 1: Add integration draft types and defaults**

In `gitlab-issues-tab.tsx`, add:

```ts
type IntegrationDraft = {
  name: string;
  base_url: string;
  token: string;
  default_issue_skill_id: string;
  polling_enabled: boolean;
  default_poll_interval_seconds: string;
};

function defaultIntegrationDraft(issueCreatorSkillId: string): IntegrationDraft {
  return {
    name: "GitLab",
    base_url: "https://gitlab.com",
    token: "",
    default_issue_skill_id: issueCreatorSkillId,
    polling_enabled: false,
    default_poll_interval_seconds: "300",
  };
}
```

- [ ] **Step 2: Load skills and compute default skill**

Import `skillListOptions` from `@multica/core/workspace/queries`. In the component:

```ts
const { data: skills = [] } = useQuery(skillListOptions(wsId));
const issueCreatorSkill = skills.find((skill) => skill.name === "issue-creator") ?? null;
```

Use `issueCreatorSkill?.id ?? ""` when creating a new integration draft.

- [ ] **Step 3: Add form state and validation**

Add state:

```ts
const [editingIntegrationId, setEditingIntegrationId] = useState<string | null>(null);
const [integrationDraft, setIntegrationDraft] = useState<IntegrationDraft | null>(null);
```

Add validation:

```ts
function validateIntegrationDraft(draft: IntegrationDraft, isCreate: boolean): string | null {
  if (!draft.base_url.trim()) return t(($) => $.issue_bridge.validation_base_url);
  if (isCreate && !draft.token.trim()) return t(($) => $.issue_bridge.validation_token);
  const interval = Number(draft.default_poll_interval_seconds);
  if (!Number.isFinite(interval) || interval < 60) {
    return t(($) => $.issue_bridge.validation_interval);
  }
  return null;
}
```

Add locale keys:

```json
"validation_base_url": "Base URL is required.",
"validation_token": "Token is required when creating a connection.",
"validation_interval": "Polling interval must be at least 60 seconds.",
"toast_connection_saved": "GitLab connection saved",
"toast_connection_deleted": "GitLab connection deleted",
"toast_connection_tested": "Connected as {{name}}",
"toast_failed": "GitLab issue bridge update failed"
```

- [ ] **Step 4: Wire mutations and save/test/delete actions**

Use hooks from `@multica/core/issue-bridge`:

```ts
const createIntegration = useCreateGitLabIssueIntegration(wsId);
const updateIntegration = useUpdateIssueIntegration(wsId);
const deleteIntegration = useDeleteIssueIntegration(wsId);
const testIntegration = useTestIssueIntegration();
```

Save payload:

```ts
const payload = {
  name: draft.name.trim() || "GitLab",
  base_url: draft.base_url.trim(),
  ...(draft.token.trim() ? { token: draft.token.trim() } : {}),
  default_issue_skill_id: draft.default_issue_skill_id || null,
  polling_enabled: draft.polling_enabled,
  default_poll_interval_seconds: Number(draft.default_poll_interval_seconds),
  config: {},
};
```

For create, require and include `token`. For update, omit `token` when blank.

Test connection:

```ts
const result = await testIntegration.mutateAsync(integration.id);
toast.success(
  t(($) => $.issue_bridge.toast_connection_tested, {
    name: result.name || result.username,
  }),
);
```

- [ ] **Step 5: Render editable form**

Replace the skeleton connection card body with a form using existing `Input`, `Label`, `Switch`, and `Button` components. The token input must always be empty when editing an existing integration.

Important JSX fields:

```tsx
<Input
  value={integrationDraft.name}
  onChange={(e) => setIntegrationDraft({ ...integrationDraft, name: e.target.value })}
/>
<Input
  value={integrationDraft.base_url}
  onChange={(e) => setIntegrationDraft({ ...integrationDraft, base_url: e.target.value })}
/>
<Input
  type="password"
  value={integrationDraft.token}
  placeholder={editingIntegrationId ? t(($) => $.issue_bridge.token_keep_placeholder) : ""}
  onChange={(e) => setIntegrationDraft({ ...integrationDraft, token: e.target.value })}
/>
<select
  value={integrationDraft.default_issue_skill_id}
  onChange={(e) => setIntegrationDraft({ ...integrationDraft, default_issue_skill_id: e.target.value })}
>
  <option value="">{t(($) => $.issue_bridge.default_skill_auto)}</option>
  {skills.map((skill) => (
    <option key={skill.id} value={skill.id}>{skill.name}</option>
  ))}
</select>
```

- [ ] **Step 6: Add tests for integration behavior**

Create `packages/views/settings/components/gitlab-issues-tab.test.tsx`. Mock `@multica/core/api` or `@multica/core/issue-bridge` consistently with existing settings tests. Required tests:

```ts
it("renders setup form for admins when no integration exists", async () => {});
it("renders read-only hint for non-admin members", async () => {});
it("does not prefill token when editing an integration", async () => {});
it("creates a gitlab integration with token and default skill", async () => {});
it("updates an integration without sending a blank token", async () => {});
it("tests connection and shows the returned user", async () => {});
```

- [ ] **Step 7: Run focused tests and typecheck**

Run:

```bash
./node_modules/.bin/vitest run packages/views/settings/components/gitlab-issues-tab.test.tsx
./node_modules/.bin/tsc -p packages/views/tsconfig.json --noEmit
```

Expected: exits 0.

- [ ] **Step 8: Commit**

```bash
git add packages/views/settings/components/gitlab-issues-tab.tsx packages/views/settings/components/gitlab-issues-tab.test.tsx packages/views/locales
git commit -m "feat(settings): configure gitlab issue connection"
```

---

### Task 4: Sync Rule Form Behavior

**Files:**
- Modify: `packages/views/settings/components/gitlab-issues-tab.tsx`
- Modify: `packages/views/settings/components/gitlab-issues-tab.test.tsx`

- [ ] **Step 1: Add rule draft types and defaults**

Add:

```ts
type ScopeType = "project" | "repo_resource";
type AssigneeType = "agent" | "squad";

type SyncRuleDraft = {
  integration_id: string;
  scope_type: ScopeType;
  scope_id: string;
  remote_project_ref: string;
  sync_enabled: boolean;
  poll_interval_seconds: string;
  auto_assign_enabled: boolean;
  default_assignee_type: AssigneeType;
  default_assignee_id: string;
};

function defaultSyncRuleDraft(integrationId: string): SyncRuleDraft {
  return {
    integration_id: integrationId,
    scope_type: "project",
    scope_id: "",
    remote_project_ref: "",
    sync_enabled: false,
    poll_interval_seconds: "",
    auto_assign_enabled: false,
    default_assignee_type: "agent",
    default_assignee_id: "",
  };
}
```

- [ ] **Step 2: Load projects, resources, agents, and squads**

Use existing helpers:

```ts
const { data: projects = [] } = useQuery(projectListOptions(wsId));
const resourceQueries = useQueries({
  queries: projects.map((project) => projectResourcesOptions(wsId, project.id)),
});
const repoResources = resourceQueries
  .flatMap((q, index) =>
    (q.data ?? []).map((resource) => ({
      ...resource,
      projectTitle: projects[index]?.title ?? "",
    })),
  )
  .filter((resource) => resource.type === "github_repo" || resource.type === "local_directory");
const { data: agents = [] } = useQuery(agentListOptions(wsId));
const { data: squads = [] } = useQuery(squadListOptions(wsId));
```

If TypeScript dislikes spreading resource objects after `flatMap`, define a small local type:

```ts
type ResourceOption = ProjectResource & { projectTitle: string };
```

- [ ] **Step 3: Add sync rule validation**

Add:

```ts
function validateSyncRuleDraft(draft: SyncRuleDraft): string | null {
  if (!draft.integration_id) return t(($) => $.issue_bridge.validation_integration);
  if (!draft.scope_id) return t(($) => $.issue_bridge.validation_scope);
  if (!draft.remote_project_ref.trim()) return t(($) => $.issue_bridge.validation_remote_project);
  if (draft.poll_interval_seconds.trim()) {
    const interval = Number(draft.poll_interval_seconds);
    if (!Number.isFinite(interval) || interval < 60) {
      return t(($) => $.issue_bridge.validation_interval);
    }
  }
  if (draft.auto_assign_enabled && !draft.default_assignee_id) {
    return t(($) => $.issue_bridge.validation_assignee);
  }
  return null;
}
```

Add locale keys:

```json
"validation_integration": "Choose a GitLab connection.",
"validation_scope": "Choose a Multica project or repository resource.",
"validation_remote_project": "GitLab project ref is required.",
"validation_assignee": "Choose an agent or squad for auto-assignment.",
"toast_rule_saved": "Sync rule saved",
"toast_rule_deleted": "Sync rule deleted"
```

- [ ] **Step 4: Wire sync rule mutations**

Use:

```ts
const createRule = useCreateIssueSyncConfig(wsId);
const updateRule = useUpdateIssueSyncConfig(wsId);
const deleteRule = useDeleteIssueSyncConfig(wsId);
```

Build payload:

```ts
const payload = {
  integration_id: draft.integration_id,
  scope_type: draft.scope_type,
  scope_id: draft.scope_id,
  remote_project_ref: draft.remote_project_ref.trim(),
  sync_enabled: draft.sync_enabled,
  poll_interval_seconds: draft.poll_interval_seconds.trim()
    ? Number(draft.poll_interval_seconds)
    : null,
  state_mapping: { opened: "backlog", closed: "done" },
  auto_assign_enabled: draft.auto_assign_enabled,
  default_assignee_type: draft.auto_assign_enabled
    ? draft.default_assignee_type
    : null,
  default_assignee_id: draft.auto_assign_enabled
    ? draft.default_assignee_id
    : null,
};
```

For create, include `integration_id`, `scope_type`, and `scope_id`. For update, keep those fields in the payload harmlessly; the backend ignores scope/integration changes on update.

- [ ] **Step 5: Render sync rule rows and form**

Replace the skeleton sync rows with:

- A row showing scope label, remote project ref, sync on/off, poll interval, and auto-assignee label.
- Edit/delete buttons for admins.
- An inline form when adding or editing.

Use native `select` controls if no local Select component exists in `@multica/ui`; this project already accepts plain controls in settings forms when needed.

Required select behavior:

```tsx
<select value={ruleDraft.scope_type} onChange={...}>
  <option value="project">{t(($) => $.issue_bridge.scope_project)}</option>
  <option value="repo_resource">{t(($) => $.issue_bridge.scope_repo_resource)}</option>
</select>
```

When `scope_type === "project"`, options come from `projects`. When `scope_type === "repo_resource"`, options come from `repoResources`.

When `auto_assign_enabled === true`, render assignee type and assignee selector. Use `agents` for type `agent`, `squads` for type `squad`.

- [ ] **Step 6: Add tests for sync rule behavior**

Add tests:

```ts
it("disables rule creation until a connection exists", async () => {});
it("creates a project sync rule with poll interval", async () => {});
it("creates a repo-resource sync rule", async () => {});
it("requires assignee when auto-assignment is enabled", async () => {});
it("saves auto-assignment to an agent", async () => {});
it("deletes a sync rule after confirmation", async () => {});
```

- [ ] **Step 7: Run verification**

Run:

```bash
./node_modules/.bin/vitest run packages/views/settings/components/gitlab-issues-tab.test.tsx
./node_modules/.bin/vitest run packages/views/locales/parity.test.ts
./node_modules/.bin/tsc -p packages/views/tsconfig.json --noEmit
```

Expected: exits 0.

- [ ] **Step 8: Commit**

```bash
git add packages/views/settings/components/gitlab-issues-tab.tsx packages/views/settings/components/gitlab-issues-tab.test.tsx packages/views/locales
git commit -m "feat(settings): configure gitlab issue sync rules"
```

---

### Task 5: Final UI Review And Verification

**Files:**
- Review all files touched by Tasks 1-4.

- [ ] **Step 1: Run targeted verification**

Run:

```bash
./node_modules/.bin/vitest run packages/views/settings/components/gitlab-issues-tab.test.tsx
./node_modules/.bin/vitest run packages/views/locales/parity.test.ts
./node_modules/.bin/vitest run packages/core/api/schema.test.ts
./node_modules/.bin/tsc -p packages/core/tsconfig.json --noEmit
./node_modules/.bin/tsc -p packages/views/tsconfig.json --noEmit
```

Expected: exits 0.

- [ ] **Step 2: Run broader frontend check if pnpm is usable**

Try:

```bash
pnpm typecheck
```

Expected: exits 0. If pnpm fails due the local registry signature/version switch issue already observed in this branch, record that and rely on the direct `tsc` commands above.

- [ ] **Step 3: Manual app smoke check**

Start or use the running app, then visit Settings -> Integrations:

```bash
pnpm dev:web
```

Expected:

- GitLab Issues section appears above Lark.
- Non-admin member sees read-only hint.
- Admin can open connection form.
- Token field is blank on edit.
- Sync rule add button is disabled until a connection exists.

If `pnpm dev:web` is blocked by the same pnpm registry signature issue, use the already running local app if available and record the limitation.

- [ ] **Step 4: Final commit only if needed**

If Step 1-3 required small fixes, commit them:

```bash
git add packages/core packages/views
git commit -m "fix(settings): polish gitlab issue bridge ui"
```

If no fixes were needed, do not create an empty commit.

---

## Self-Review Notes

- Spec coverage: all spec sections map to tasks. Core query layer is Task 1, settings placement is Task 2, connection form is Task 3, sync rules and auto-assignment are Task 4, verification is Task 5.
- Scope check: this plan does not implement polling workers or issue sync execution. It only configures existing backend records.
- Placeholder scan: no TBD/TODO placeholders remain. Each task includes exact files, commands, and expected outcomes.
- Type consistency: task payload names match existing `packages/core/types/api.ts` request fields and existing backend JSON field names.
