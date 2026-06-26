"use client";

import { useMemo, useState } from "react";
import { useQueries, useQuery } from "@tanstack/react-query";
import { GitBranch, GitPullRequestArrow, ShieldCheck } from "lucide-react";
import { toast } from "sonner";
import { useAuthStore } from "@multica/core/auth";
import { useWorkspaceId } from "@multica/core/hooks";
import {
  issueIntegrationsOptions,
  issueSyncConfigsOptions,
  useCreateGitLabIssueIntegration,
  useCreateIssueSyncConfig,
  useDeleteIssueIntegration,
  useDeleteIssueSyncConfig,
  useTestIssueIntegration,
  useUpdateIssueIntegration,
  useUpdateIssueSyncConfig,
} from "@multica/core/issue-bridge";
import { projectResourcesOptions } from "@multica/core/projects";
import { projectListOptions } from "@multica/core/projects/queries";
import {
  agentListOptions,
  memberListOptions,
  skillListOptions,
  squadListOptions,
} from "@multica/core/workspace/queries";
import type {
  Agent,
  IssueIntegration,
  IssueSyncAssigneeType,
  IssueSyncConfig,
  IssueSyncScopeType,
  Project,
  ProjectResource,
  SkillSummary,
  Squad,
} from "@multica/core/types";
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from "@multica/ui/components/ui/alert-dialog";
import { Button } from "@multica/ui/components/ui/button";
import { Card, CardContent } from "@multica/ui/components/ui/card";
import { Input } from "@multica/ui/components/ui/input";
import { Label } from "@multica/ui/components/ui/label";
import { Skeleton } from "@multica/ui/components/ui/skeleton";
import { Switch } from "@multica/ui/components/ui/switch";
import { useT } from "../../i18n";

type ScopeType = IssueSyncScopeType;
type AssigneeType = IssueSyncAssigneeType;

type IntegrationDraft = {
  name: string;
  base_url: string;
  token: string;
  default_issue_skill_id: string;
  polling_enabled: boolean;
  default_poll_interval_seconds: string;
};

type SyncRuleDraft = {
  integration_id: string;
  scope_type: ScopeType;
  scope_id: string;
  remote_project_ref: string;
  sync_enabled: boolean;
  poll_interval_seconds: string;
  state_mapping: Record<string, unknown>;
  auto_assign_enabled: boolean;
  default_assignee_type: AssigneeType;
  default_assignee_id: string;
};

type ResourceOption = ProjectResource & { projectTitle: string };

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

function draftFromIntegration(integration: IssueIntegration): IntegrationDraft {
  return {
    name: integration.name || "GitLab",
    base_url: integration.base_url,
    token: "",
    default_issue_skill_id: integration.default_issue_skill_id ?? "",
    polling_enabled: integration.polling_enabled,
    default_poll_interval_seconds: String(
      integration.default_poll_interval_seconds || 300,
    ),
  };
}

function defaultSyncRuleDraft(integrationId: string): SyncRuleDraft {
  return {
    integration_id: integrationId,
    scope_type: "project",
    scope_id: "",
    remote_project_ref: "",
    sync_enabled: false,
    poll_interval_seconds: "",
    state_mapping: { opened: "backlog", closed: "done" },
    auto_assign_enabled: false,
    default_assignee_type: "agent",
    default_assignee_id: "",
  };
}

function draftFromSyncRule(rule: IssueSyncConfig): SyncRuleDraft {
  return {
    integration_id: rule.integration_id,
    scope_type:
      rule.scope_type === "repo_resource" ? "repo_resource" : "project",
    scope_id: rule.scope_id,
    remote_project_ref: rule.remote_project_ref,
    sync_enabled: rule.sync_enabled,
    poll_interval_seconds: rule.poll_interval_seconds
      ? String(rule.poll_interval_seconds)
      : "",
    state_mapping: rule.state_mapping,
    auto_assign_enabled: rule.auto_assign_enabled,
    default_assignee_type:
      rule.default_assignee_type === "squad" ? "squad" : "agent",
    default_assignee_id: rule.default_assignee_id ?? "",
  };
}

function ruleScopeLabel(
  rule: Pick<IssueSyncConfig, "scope_type" | "scope_id">,
  projectsById: Map<string, Project>,
  resourcesById: Map<string, ResourceOption>,
): string {
  if (rule.scope_type === "repo_resource") {
    const resource = resourcesById.get(rule.scope_id);
    return resource
      ? resource.label || resource.projectTitle
      : rule.scope_id;
  }
  return projectsById.get(rule.scope_id)?.title || rule.scope_id;
}

function ruleAssigneeLabel(
  rule: Pick<
    IssueSyncConfig,
    "auto_assign_enabled" | "default_assignee_type" | "default_assignee_id"
  >,
  agentsById: Map<string, Agent>,
  squadsById: Map<string, Squad>,
  unknownAssigneeLabel: string,
): string | null {
  if (!rule.auto_assign_enabled || !rule.default_assignee_id) return null;
  if (rule.default_assignee_type === "squad") {
    return squadsById.get(rule.default_assignee_id)?.name || unknownAssigneeLabel;
  }
  return agentsById.get(rule.default_assignee_id)?.name || unknownAssigneeLabel;
}

export function GitLabIssuesTab() {
  const { t } = useT("settings");
  const wsId = useWorkspaceId();
  const user = useAuthStore((s) => s.user);

  const {
    data: members = [],
    isError: membersError,
    isLoading: membersLoading,
  } = useQuery(memberListOptions(wsId));
  const currentMember =
    members.find((member) => member.user_id === user?.id) ?? null;
  const canManage =
    currentMember?.role === "owner" || currentMember?.role === "admin";

  const {
    data: integrationData,
    isError: integrationsError,
    isLoading: integrationsLoading,
  } = useQuery(issueIntegrationsOptions(wsId));
  const {
    data: syncData,
    isError: syncError,
    isLoading: syncLoading,
  } = useQuery(issueSyncConfigsOptions(wsId));
  const {
    data: skills = [],
    isError: skillsError,
  } = useQuery(skillListOptions(wsId));
  const { data: projects = [], isError: projectsError } = useQuery(
    projectListOptions(wsId),
  );
  const resourceQueries = useQueries({
    queries: projects.map((project) => projectResourcesOptions(wsId, project.id)),
  });
  const { data: agents = [], isError: agentsError } = useQuery(
    agentListOptions(wsId),
  );
  const { data: squads = [], isError: squadsError } = useQuery(
    squadListOptions(wsId),
  );

  const createIntegration = useCreateGitLabIssueIntegration(wsId);
  const updateIntegration = useUpdateIssueIntegration(wsId);
  const deleteIntegration = useDeleteIssueIntegration(wsId);
  const testIntegration = useTestIssueIntegration(wsId);
  const createRule = useCreateIssueSyncConfig(wsId);
  const updateRule = useUpdateIssueSyncConfig(wsId);
  const deleteRule = useDeleteIssueSyncConfig(wsId);

  const [editingIntegrationId, setEditingIntegrationId] = useState<string | null>(
    null,
  );
  const [integrationDraft, setIntegrationDraft] =
    useState<IntegrationDraft | null>(null);
  const [deleteTarget, setDeleteTarget] = useState<IssueIntegration | null>(null);
  const [editingRuleId, setEditingRuleId] = useState<string | null>(null);
  const [ruleDraft, setRuleDraft] = useState<SyncRuleDraft | null>(null);
  const [deleteRuleTarget, setDeleteRuleTarget] = useState<IssueSyncConfig | null>(
    null,
  );

  const integrations = integrationData?.integrations ?? [];
  const syncConfigs = syncData?.sync_configs ?? [];
  const primaryIntegration = integrations[0] ?? null;
  const loading = integrationsLoading || syncLoading || membersLoading;
  const hasError = integrationsError || syncError || membersError;
  const ruleOptionsError =
    projectsError ||
    resourceQueries.some((query) => query.isError) ||
    agentsError ||
    squadsError;
  const issueCreatorSkill =
    skills.find((skill) => skill.name === "issue-creator") ?? null;

  const integrationNames = useMemo(
    () =>
      new Map(
        integrations.map((integration) => [
          integration.id,
          integration.name || integration.base_url,
        ]),
      ),
    [integrations],
  );

  const repoResources = useMemo(
    () =>
      resourceQueries
        .flatMap((query, index) =>
          (query.data ?? []).map((resource) => ({
            ...resource,
            projectTitle: projects[index]?.title ?? "",
          })),
        )
        .filter(
          (resource): resource is ResourceOption =>
            resource.resource_type === "github_repo" ||
            resource.resource_type === "local_directory",
        ),
    [projects, resourceQueries],
  );

  const projectsById = useMemo(
    () => new Map(projects.map((project) => [project.id, project])),
    [projects],
  );
  const resourcesById = useMemo(
    () => new Map(repoResources.map((resource) => [resource.id, resource])),
    [repoResources],
  );
  const agentsById = useMemo(
    () => new Map(agents.map((agent) => [agent.id, agent])),
    [agents],
  );
  const squadsById = useMemo(
    () => new Map(squads.map((squad) => [squad.id, squad])),
    [squads],
  );

  const busy =
    createIntegration.isPending ||
    updateIntegration.isPending ||
    deleteIntegration.isPending ||
    testIntegration.isPending ||
    createRule.isPending ||
    updateRule.isPending ||
    deleteRule.isPending;

  function beginCreateIntegration() {
    setEditingIntegrationId(null);
    setIntegrationDraft(defaultIntegrationDraft(issueCreatorSkill?.id ?? ""));
  }

  function beginEditIntegration(integration: IssueIntegration) {
    setEditingIntegrationId(integration.id);
    setIntegrationDraft(draftFromIntegration(integration));
  }

  function cancelIntegrationEdit() {
    setEditingIntegrationId(null);
    setIntegrationDraft(null);
  }

  function beginCreateRule() {
    if (!primaryIntegration || ruleOptionsError) return;
    setEditingRuleId(null);
    setRuleDraft(defaultSyncRuleDraft(primaryIntegration.id));
  }

  function beginEditRule(rule: IssueSyncConfig) {
    if (ruleOptionsError) return;
    setEditingRuleId(rule.id);
    setRuleDraft(draftFromSyncRule(rule));
  }

  function cancelRuleEdit() {
    setEditingRuleId(null);
    setRuleDraft(null);
  }

  function validateIntegrationDraft(
    draft: IntegrationDraft,
    isCreate: boolean,
  ): string | null {
    if (!draft.base_url.trim()) {
      return t(($) => $.issue_bridge.validation_base_url);
    }
    if (isCreate && !draft.token.trim()) {
      return t(($) => $.issue_bridge.validation_token);
    }
    const interval = Number(draft.default_poll_interval_seconds);
    if (!Number.isFinite(interval) || interval < 60) {
      return t(($) => $.issue_bridge.validation_interval);
    }
    return null;
  }

  function validateSyncRuleDraft(draft: SyncRuleDraft): string | null {
    if (!draft.integration_id) {
      return t(($) => $.issue_bridge.validation_integration);
    }
    if (!draft.scope_id) {
      return t(($) => $.issue_bridge.validation_scope);
    }
    if (!draft.remote_project_ref.trim()) {
      return t(($) => $.issue_bridge.validation_remote_project);
    }
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

  async function saveIntegration() {
    if (!integrationDraft) return;
    const isCreate = !editingIntegrationId;
    const validation = validateIntegrationDraft(integrationDraft, isCreate);
    if (validation) {
      toast.error(validation);
      return;
    }

    const currentIntegration = editingIntegrationId
      ? integrations.find((integration) => integration.id === editingIntegrationId)
      : null;
    const payload = {
      name: integrationDraft.name.trim() || "GitLab",
      base_url: integrationDraft.base_url.trim(),
      ...(integrationDraft.token.trim()
        ? { token: integrationDraft.token.trim() }
        : {}),
      default_issue_skill_id: integrationDraft.default_issue_skill_id || null,
      polling_enabled: integrationDraft.polling_enabled,
      default_poll_interval_seconds: Number(
        integrationDraft.default_poll_interval_seconds,
      ),
      config: currentIntegration?.config ?? {},
    };

    try {
      if (isCreate) {
        await createIntegration.mutateAsync({
          ...payload,
          token: integrationDraft.token.trim(),
        });
      } else if (editingIntegrationId) {
        await updateIntegration.mutateAsync({
          id: editingIntegrationId,
          data: payload,
        });
      }
      toast.success(t(($) => $.issue_bridge.toast_connection_saved));
      cancelIntegrationEdit();
    } catch (e) {
      toast.error(
        e instanceof Error ? e.message : t(($) => $.issue_bridge.toast_failed),
      );
    }
  }

  async function saveRule() {
    if (!ruleDraft) return;
    const validation = validateSyncRuleDraft(ruleDraft);
    if (validation) {
      toast.error(validation);
      return;
    }

    const payload = {
      integration_id: ruleDraft.integration_id,
      scope_type: ruleDraft.scope_type,
      scope_id: ruleDraft.scope_id,
      remote_project_ref: ruleDraft.remote_project_ref.trim(),
      sync_enabled: ruleDraft.sync_enabled,
      poll_interval_seconds: ruleDraft.poll_interval_seconds.trim()
        ? Number(ruleDraft.poll_interval_seconds)
        : null,
      state_mapping: ruleDraft.state_mapping,
      auto_assign_enabled: ruleDraft.auto_assign_enabled,
      default_assignee_type: ruleDraft.auto_assign_enabled
        ? ruleDraft.default_assignee_type
        : null,
      default_assignee_id: ruleDraft.auto_assign_enabled
        ? ruleDraft.default_assignee_id
        : null,
    };

    try {
      if (editingRuleId) {
        await updateRule.mutateAsync({
          id: editingRuleId,
          data: payload,
        });
      } else {
        await createRule.mutateAsync(payload);
      }
      toast.success(t(($) => $.issue_bridge.toast_rule_saved));
      cancelRuleEdit();
    } catch (e) {
      toast.error(
        e instanceof Error ? e.message : t(($) => $.issue_bridge.toast_failed),
      );
    }
  }

  async function testConnection(integration: IssueIntegration) {
    try {
      const result = await testIntegration.mutateAsync(integration.id);
      toast.success(
        t(($) => $.issue_bridge.toast_connection_tested, {
          name: result.name || result.username,
        }),
      );
    } catch (e) {
      toast.error(
        e instanceof Error ? e.message : t(($) => $.issue_bridge.toast_failed),
      );
    }
  }

  async function confirmDeleteIntegration() {
    if (!deleteTarget) return;
    try {
      await deleteIntegration.mutateAsync(deleteTarget.id);
      toast.success(t(($) => $.issue_bridge.toast_connection_deleted));
      setDeleteTarget(null);
    } catch (e) {
      toast.error(
        e instanceof Error ? e.message : t(($) => $.issue_bridge.toast_failed),
      );
    }
  }

  async function confirmDeleteRule() {
    if (!deleteRuleTarget) return;
    try {
      await deleteRule.mutateAsync(deleteRuleTarget.id);
      toast.success(t(($) => $.issue_bridge.toast_rule_deleted));
      setDeleteRuleTarget(null);
    } catch (e) {
      toast.error(
        e instanceof Error ? e.message : t(($) => $.issue_bridge.toast_failed),
      );
    }
  }

  if (loading) {
    return (
      <Card>
        <CardContent className="space-y-2">
          <p className="text-sm text-muted-foreground">
            {t(($) => $.issue_bridge.loading)}
          </p>
          <div className="space-y-3">
            <Skeleton className="h-16 w-full rounded-lg" />
            <Skeleton className="h-20 w-full rounded-lg" />
          </div>
        </CardContent>
      </Card>
    );
  }

  if (hasError) {
    return (
      <Card>
        <CardContent className="space-y-2">
          <p className="text-sm font-medium">
            {t(($) => $.issue_bridge.load_failed_title)}
          </p>
          <p className="text-xs text-muted-foreground">
            {t(($) => $.issue_bridge.load_failed_description)}
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
            {integrationDraft ? (
              <IntegrationForm
                busy={busy}
                draft={integrationDraft}
                editing={!!editingIntegrationId}
                onCancel={cancelIntegrationEdit}
                onChange={setIntegrationDraft}
                onSave={saveIntegration}
                skills={skills}
                skillsError={skillsError}
              />
            ) : integrations.length > 0 ? (
              <div className="divide-y">
                {integrations.map((integration) => (
                  <div
                    key={integration.id}
                    className="flex items-start justify-between gap-4 py-3 first:pt-0 last:pb-0"
                  >
                    <div className="flex min-w-0 items-start gap-3">
                      <div className="rounded-md border bg-muted/50 p-2 text-muted-foreground">
                        <GitBranch className="h-4 w-4" />
                      </div>
                      <div className="min-w-0 space-y-1">
                        <p className="text-sm font-medium">
                          {integration.name ||
                            t(($) => $.issue_bridge.connection_saved_title)}
                        </p>
                        <p className="truncate text-xs text-muted-foreground">
                          {integration.base_url}
                        </p>
                      </div>
                    </div>
                    {canManage && (
                      <div className="flex shrink-0 items-center gap-2">
                        <Button
                          variant="outline"
                          size="sm"
                          disabled={busy}
                          onClick={() => testConnection(integration)}
                        >
                          {testIntegration.isPending
                            ? t(($) => $.issue_bridge.testing)
                            : t(($) => $.issue_bridge.test_connection)}
                        </Button>
                        <Button
                          variant="outline"
                          size="sm"
                          disabled={busy}
                          onClick={() => beginEditIntegration(integration)}
                        >
                          {t(($) => $.issue_bridge.edit)}
                        </Button>
                        <Button
                          variant="outline"
                          size="sm"
                          disabled={busy}
                          onClick={() => setDeleteTarget(integration)}
                        >
                          {t(($) => $.issue_bridge.delete)}
                        </Button>
                      </div>
                    )}
                  </div>
                ))}
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
                  <Button size="sm" disabled={busy} onClick={beginCreateIntegration}>
                    {t(($) => $.issue_bridge.add_connection)}
                  </Button>
                )}
              </div>
            )}

            <p className="text-xs text-muted-foreground">
              {t(($) => $.issue_bridge.operator_hint)}
            </p>
          </CardContent>
        </Card>
      </section>

      <section className="space-y-3">
        <div className="flex items-center justify-between gap-4">
          <div className="space-y-1">
            <h3 className="text-sm font-semibold">
              {t(($) => $.issue_bridge.sync_rules_title)}
            </h3>
            <p className="text-xs text-muted-foreground">
              {t(($) => $.issue_bridge.sync_rules_description)}
            </p>
          </div>
          {canManage && (
            <Button
              size="sm"
              disabled={!primaryIntegration || busy || !!ruleDraft || ruleOptionsError}
              onClick={beginCreateRule}
            >
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
            ) : (
              <>
                {ruleOptionsError && (
                  <p className="text-sm text-muted-foreground">
                    {t(($) => $.issue_bridge.rule_options_load_failed)}
                  </p>
                )}
                {ruleDraft && (
                  <SyncRuleForm
                    agents={agents}
                    busy={busy}
                    draft={ruleDraft}
                    integrations={integrations}
                    editing={!!editingRuleId}
                    projects={projects}
                    repoResources={repoResources}
                    squads={squads}
                    onCancel={cancelRuleEdit}
                    onChange={setRuleDraft}
                    onSave={saveRule}
                  />
                )}

                {syncConfigs.length === 0 && !ruleDraft ? (
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
                ) : syncConfigs.length > 0 ? (
                  <div className="divide-y">
                    {syncConfigs.map((rule) => {
                      const assignee = ruleAssigneeLabel(
                        rule,
                        agentsById,
                        squadsById,
                        t(($) => $.issue_bridge.unknown_assignee),
                      );
                      return (
                        <div
                          key={rule.id}
                          className="flex items-start justify-between gap-4 py-3 first:pt-0 last:pb-0"
                        >
                          <div className="min-w-0 space-y-1">
                            <p className="truncate text-sm font-medium">
                              {ruleScopeLabel(rule, projectsById, resourcesById)}
                            </p>
                            <p className="truncate text-xs text-muted-foreground">
                              {rule.remote_project_ref}
                            </p>
                            <p className="text-xs text-muted-foreground">
                              {integrationNames.get(rule.integration_id) ??
                                t(($) => $.issue_bridge.unknown_connection)}
                              {" · "}
                              {rule.scope_type === "repo_resource"
                                ? t(($) => $.issue_bridge.scope_repo_resource)
                                : t(($) => $.issue_bridge.scope_project)}
                              {" · "}
                              {rule.sync_enabled
                                ? t(($) => $.issue_bridge.sync_on)
                                : t(($) => $.issue_bridge.sync_off)}
                              {rule.poll_interval_seconds
                                ? ` · ${rule.poll_interval_seconds}s`
                                : ""}
                              {assignee
                                ? ` · ${t(($) => $.issue_bridge.auto_assign_short)}: ${assignee}`
                                : ""}
                            </p>
                          </div>
                          {canManage && (
                            <div className="flex shrink-0 items-center gap-2">
                              <Button
                                variant="outline"
                                size="sm"
                                disabled={busy || !!ruleDraft || ruleOptionsError}
                                onClick={() => beginEditRule(rule)}
                              >
                                {t(($) => $.issue_bridge.edit)}
                              </Button>
                              <Button
                                variant="outline"
                                size="sm"
                                disabled={busy || !!ruleDraft}
                                onClick={() => setDeleteRuleTarget(rule)}
                              >
                                {t(($) => $.issue_bridge.delete)}
                              </Button>
                            </div>
                          )}
                        </div>
                      );
                    })}
                  </div>
                ) : null}
              </>
            )}
          </CardContent>
        </Card>
      </section>

      <AlertDialog
        open={!!deleteTarget}
        onOpenChange={(open) => {
          if (!open && !deleteIntegration.isPending) setDeleteTarget(null);
        }}
      >
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>
              {t(($) => $.issue_bridge.delete_connection_title)}
            </AlertDialogTitle>
            <AlertDialogDescription>
              {t(($) => $.issue_bridge.delete_connection_description)}
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel disabled={deleteIntegration.isPending}>
              {t(($) => $.issue_bridge.cancel)}
            </AlertDialogCancel>
            <AlertDialogAction
              disabled={deleteIntegration.isPending}
              onClick={confirmDeleteIntegration}
            >
              {t(($) => $.issue_bridge.delete)}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>

      <AlertDialog
        open={!!deleteRuleTarget}
        onOpenChange={(open) => {
          if (!open && !deleteRule.isPending) setDeleteRuleTarget(null);
        }}
      >
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>
              {t(($) => $.issue_bridge.delete_rule_title)}
            </AlertDialogTitle>
            <AlertDialogDescription>
              {t(($) => $.issue_bridge.delete_rule_description)}
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel disabled={deleteRule.isPending}>
              {t(($) => $.issue_bridge.cancel)}
            </AlertDialogCancel>
            <AlertDialogAction
              disabled={deleteRule.isPending}
              onClick={confirmDeleteRule}
            >
              {t(($) => $.issue_bridge.confirm_delete_rule)}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </div>
  );
}

function IntegrationForm({
  busy,
  draft,
  editing,
  onCancel,
  onChange,
  onSave,
  skills,
  skillsError,
}: {
  busy: boolean;
  draft: IntegrationDraft;
  editing: boolean;
  onCancel: () => void;
  onChange: (draft: IntegrationDraft) => void;
  onSave: () => void;
  skills: SkillSummary[];
  skillsError: boolean;
}) {
  const { t } = useT("settings");

  return (
    <div className="space-y-4">
      <div className="grid gap-4 md:grid-cols-2">
        <div className="space-y-2">
          <Label htmlFor="issue-bridge-name" className="text-xs">
            {t(($) => $.issue_bridge.name_label)}
          </Label>
          <Input
            id="issue-bridge-name"
            value={draft.name}
            onChange={(e) => onChange({ ...draft, name: e.target.value })}
          />
        </div>
        <div className="space-y-2">
          <Label htmlFor="issue-bridge-base-url" className="text-xs">
            {t(($) => $.issue_bridge.base_url_label)}
          </Label>
          <Input
            id="issue-bridge-base-url"
            value={draft.base_url}
            onChange={(e) => onChange({ ...draft, base_url: e.target.value })}
          />
        </div>
        <div className="space-y-2">
          <Label htmlFor="issue-bridge-token" className="text-xs">
            {t(($) => $.issue_bridge.token_label)}
          </Label>
          <Input
            id="issue-bridge-token"
            type="password"
            value={draft.token}
            placeholder={
              editing
                ? t(($) => $.issue_bridge.token_keep_placeholder)
                : undefined
            }
            onChange={(e) => onChange({ ...draft, token: e.target.value })}
          />
        </div>
        <div className="space-y-2">
          <Label htmlFor="issue-bridge-default-skill" className="text-xs">
            {t(($) => $.issue_bridge.default_skill_label)}
          </Label>
          <select
            id="issue-bridge-default-skill"
            className="h-9 w-full rounded-md border border-input bg-background px-3 text-sm"
            disabled={skillsError}
            value={draft.default_issue_skill_id}
            onChange={(e) =>
              onChange({ ...draft, default_issue_skill_id: e.target.value })
            }
          >
            <option value="">
              {t(($) => $.issue_bridge.default_skill_auto)}
            </option>
            {skills.map((skill) => (
              <option key={skill.id} value={skill.id}>
                {skill.name}
              </option>
            ))}
          </select>
          {skillsError && (
            <p className="text-xs text-muted-foreground">
              {t(($) => $.issue_bridge.skills_unavailable_hint)}
            </p>
          )}
        </div>
      </div>

      <div className="flex flex-col gap-3 rounded-md border bg-muted/30 p-3 sm:flex-row sm:items-center sm:justify-between">
        <div className="space-y-1">
          <Label htmlFor="issue-bridge-polling" className="text-sm font-medium">
            {t(($) => $.issue_bridge.polling_enabled_label)}
          </Label>
          <p className="text-xs text-muted-foreground">
            {t(($) => $.issue_bridge.polling_enabled_hint)}
          </p>
        </div>
        <Switch
          id="issue-bridge-polling"
          checked={draft.polling_enabled}
          onCheckedChange={(checked) =>
            onChange({ ...draft, polling_enabled: checked })
          }
        />
      </div>

      <div className="space-y-2">
        <Label htmlFor="issue-bridge-interval" className="text-xs">
          {t(($) => $.issue_bridge.poll_interval_label)}
        </Label>
        <Input
          id="issue-bridge-interval"
          inputMode="numeric"
          value={draft.default_poll_interval_seconds}
          onChange={(e) =>
            onChange({
              ...draft,
              default_poll_interval_seconds: e.target.value,
            })
          }
        />
      </div>

      <div className="flex items-center justify-end gap-2">
        <Button variant="outline" size="sm" disabled={busy} onClick={onCancel}>
          {t(($) => $.issue_bridge.cancel)}
        </Button>
        <Button size="sm" disabled={busy} onClick={onSave}>
          {busy
            ? t(($) => $.issue_bridge.saving)
            : t(($) => $.issue_bridge.save)}
        </Button>
      </div>
    </div>
  );
}

function SyncRuleForm({
  agents,
  busy,
  draft,
  integrations,
  editing,
  projects,
  repoResources,
  squads,
  onCancel,
  onChange,
  onSave,
}: {
  agents: Agent[];
  busy: boolean;
  draft: SyncRuleDraft;
  integrations: IssueIntegration[];
  editing: boolean;
  projects: Project[];
  repoResources: ResourceOption[];
  squads: Squad[];
  onCancel: () => void;
  onChange: (draft: SyncRuleDraft) => void;
  onSave: () => void;
}) {
  const { t } = useT("settings");

  const scopeOptions =
    draft.scope_type === "repo_resource" ? repoResources : projects;
  const assigneeOptions =
    draft.default_assignee_type === "squad" ? squads : agents;

  return (
    <div className="space-y-4 rounded-md border bg-muted/20 p-4">
      <div className="grid gap-4 md:grid-cols-2">
        <div className="space-y-2">
          <Label htmlFor="issue-bridge-rule-integration" className="text-xs">
            {t(($) => $.issue_bridge.integration_label)}
          </Label>
          <select
            id="issue-bridge-rule-integration"
            className="h-9 w-full rounded-md border border-input bg-background px-3 text-sm"
            disabled={editing}
            value={draft.integration_id}
            onChange={(e) =>
              onChange({ ...draft, integration_id: e.target.value })
            }
          >
            <option value="">
              {t(($) => $.issue_bridge.integration_placeholder)}
            </option>
            {integrations.map((integration) => (
              <option key={integration.id} value={integration.id}>
                {integration.name || integration.base_url}
              </option>
            ))}
          </select>
        </div>

        <div className="space-y-2">
          <Label htmlFor="issue-bridge-rule-scope" className="text-xs">
            {t(($) => $.issue_bridge.scope_label)}
          </Label>
          <select
            id="issue-bridge-rule-scope"
            className="h-9 w-full rounded-md border border-input bg-background px-3 text-sm"
            disabled={editing}
            value={draft.scope_type}
            onChange={(e) =>
              onChange({
                ...draft,
                scope_type: e.target.value as ScopeType,
                scope_id: "",
              })
            }
          >
            <option value="project">
              {t(($) => $.issue_bridge.scope_project)}
            </option>
            <option value="repo_resource">
              {t(($) => $.issue_bridge.scope_repo_resource)}
            </option>
          </select>
        </div>

        <div className="space-y-2">
          <Label htmlFor="issue-bridge-rule-scope-id" className="text-xs">
            {t(($) => $.issue_bridge.scope_target_label)}
          </Label>
          <select
            id="issue-bridge-rule-scope-id"
            className="h-9 w-full rounded-md border border-input bg-background px-3 text-sm"
            disabled={editing}
            value={draft.scope_id}
            onChange={(e) => onChange({ ...draft, scope_id: e.target.value })}
          >
            <option value="">
              {t(($) => $.issue_bridge.scope_target_placeholder)}
            </option>
            {scopeOptions.map((option) =>
              "resource_type" in option ? (
                <option key={option.id} value={option.id}>
                  {option.label || option.projectTitle}
                </option>
              ) : (
                <option key={option.id} value={option.id}>
                  {option.title}
                </option>
              ),
            )}
          </select>
        </div>

        <div className="space-y-2">
          <Label htmlFor="issue-bridge-rule-remote-project" className="text-xs">
            {t(($) => $.issue_bridge.remote_project_label)}
          </Label>
          <Input
            id="issue-bridge-rule-remote-project"
            value={draft.remote_project_ref}
            onChange={(e) =>
              onChange({ ...draft, remote_project_ref: e.target.value })
            }
          />
        </div>

        <div className="space-y-2">
          <Label htmlFor="issue-bridge-rule-interval" className="text-xs">
            {t(($) => $.issue_bridge.rule_interval_label)}
          </Label>
          <Input
            id="issue-bridge-rule-interval"
            inputMode="numeric"
            value={draft.poll_interval_seconds}
            onChange={(e) =>
              onChange({ ...draft, poll_interval_seconds: e.target.value })
            }
          />
        </div>
      </div>

      <div className="grid gap-3 md:grid-cols-2">
        <div className="flex flex-col gap-3 rounded-md border bg-background p-3">
          <Label htmlFor="issue-bridge-rule-sync" className="text-sm font-medium">
            {t(($) => $.issue_bridge.sync_enabled_label)}
          </Label>
          <div className="flex items-center justify-end">
            <Switch
              id="issue-bridge-rule-sync"
              checked={draft.sync_enabled}
              onCheckedChange={(checked) =>
                onChange({ ...draft, sync_enabled: checked })
              }
            />
          </div>
        </div>

        <div className="flex flex-col gap-3 rounded-md border bg-background p-3">
          <Label
            htmlFor="issue-bridge-rule-auto-assign"
            className="text-sm font-medium"
          >
            {t(($) => $.issue_bridge.auto_assign_label)}
          </Label>
          <div className="flex items-center justify-end">
            <Switch
              id="issue-bridge-rule-auto-assign"
              checked={draft.auto_assign_enabled}
              onCheckedChange={(checked) =>
                onChange({
                  ...draft,
                  auto_assign_enabled: checked,
                  default_assignee_id: checked ? draft.default_assignee_id : "",
                })
              }
            />
          </div>
        </div>
      </div>

      {draft.auto_assign_enabled && (
        <div className="grid gap-4 md:grid-cols-2">
          <div className="space-y-2">
            <Label htmlFor="issue-bridge-rule-assignee-type" className="text-xs">
              {t(($) => $.issue_bridge.assignee_type_label)}
            </Label>
            <select
              id="issue-bridge-rule-assignee-type"
              className="h-9 w-full rounded-md border border-input bg-background px-3 text-sm"
              value={draft.default_assignee_type}
              onChange={(e) =>
                onChange({
                  ...draft,
                  default_assignee_type: e.target.value as AssigneeType,
                  default_assignee_id: "",
                })
              }
            >
              <option value="agent">
                {t(($) => $.issue_bridge.assignee_type_agent)}
              </option>
              <option value="squad">
                {t(($) => $.issue_bridge.assignee_type_squad)}
              </option>
            </select>
          </div>

          <div className="space-y-2">
            <Label htmlFor="issue-bridge-rule-assignee" className="text-xs">
              {t(($) => $.issue_bridge.assignee_label)}
            </Label>
            <select
              id="issue-bridge-rule-assignee"
              className="h-9 w-full rounded-md border border-input bg-background px-3 text-sm"
              value={draft.default_assignee_id}
              onChange={(e) =>
                onChange({ ...draft, default_assignee_id: e.target.value })
              }
            >
              <option value="">
                {t(($) => $.issue_bridge.assignee_placeholder)}
              </option>
              {assigneeOptions.map((assignee) => (
                <option key={assignee.id} value={assignee.id}>
                  {assignee.name}
                </option>
              ))}
            </select>
          </div>
        </div>
      )}

      <div className="flex items-center justify-end gap-2">
        <Button variant="outline" size="sm" disabled={busy} onClick={onCancel}>
          {t(($) => $.issue_bridge.cancel)}
        </Button>
        <Button size="sm" disabled={busy} onClick={onSave}>
          {busy
            ? t(($) => $.issue_bridge.saving)
            : t(($) => $.issue_bridge.save)}
        </Button>
      </div>
    </div>
  );
}
