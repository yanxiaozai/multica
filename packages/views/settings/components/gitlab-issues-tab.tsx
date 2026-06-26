"use client";

import { useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { GitBranch, GitPullRequestArrow, ShieldCheck } from "lucide-react";
import { toast } from "sonner";
import { useAuthStore } from "@multica/core/auth";
import { useWorkspaceId } from "@multica/core/hooks";
import {
  issueIntegrationsOptions,
  issueSyncConfigsOptions,
  useCreateGitLabIssueIntegration,
  useDeleteIssueIntegration,
  useTestIssueIntegration,
  useUpdateIssueIntegration,
} from "@multica/core/issue-bridge";
import {
  memberListOptions,
  skillListOptions,
} from "@multica/core/workspace/queries";
import type { IssueIntegration, SkillSummary } from "@multica/core/types";
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

export function GitLabIssuesTab() {
  const { t } = useT("settings");
  const wsId = useWorkspaceId();
  const user = useAuthStore((s) => s.user);

  const {
    data: members = [],
    isError: membersError,
    isLoading: membersLoading,
  } = useQuery(memberListOptions(wsId));
  const currentMember = members.find((member) => member.user_id === user?.id) ?? null;
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

  const createIntegration = useCreateGitLabIssueIntegration(wsId);
  const updateIntegration = useUpdateIssueIntegration(wsId);
  const deleteIntegration = useDeleteIssueIntegration(wsId);
  const testIntegration = useTestIssueIntegration(wsId);

  const [editingIntegrationId, setEditingIntegrationId] = useState<string | null>(null);
  const [integrationDraft, setIntegrationDraft] =
    useState<IntegrationDraft | null>(null);
  const [deleteTarget, setDeleteTarget] = useState<IssueIntegration | null>(null);

  const integrations = integrationData?.integrations ?? [];
  const syncConfigs = syncData?.sync_configs ?? [];
  const primaryIntegration = integrations[0] ?? null;
  const loading = integrationsLoading || syncLoading || membersLoading;
  const hasError = integrationsError || syncError || membersError;
  const issueCreatorSkill =
    skills.find((skill) => skill.name === "issue-creator") ?? null;
  const integrationNames = new Map(
    integrations.map((integration) => [
      integration.id,
      integration.name || integration.base_url,
    ]),
  );
  const busy =
    createIntegration.isPending ||
    updateIntegration.isPending ||
    deleteIntegration.isPending ||
    testIntegration.isPending;

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

  async function saveIntegration() {
    if (!integrationDraft) return;
    const isCreate = !editingIntegrationId;
    const validation = validateIntegrationDraft(integrationDraft, isCreate);
    if (validation) {
      toast.error(validation);
      return;
    }

    const currentIntegration =
      editingIntegrationId
        ? integrations.find((integration) => integration.id === editingIntegrationId)
        : null;
    const payload = {
      name: integrationDraft.name.trim() || "GitLab",
      base_url: integrationDraft.base_url.trim(),
      ...(integrationDraft.token.trim()
        ? { token: integrationDraft.token.trim() }
        : {}),
      default_issue_skill_id:
        integrationDraft.default_issue_skill_id || null,
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
      } else {
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
            <Button size="sm" disabled>
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
                  <div
                    key={rule.id}
                    className="flex items-start justify-between gap-4 py-3 first:pt-0 last:pb-0"
                  >
                    <div className="min-w-0 space-y-1">
                      <p className="truncate text-sm font-medium">
                        {rule.remote_project_ref}
                      </p>
                      <p className="text-xs text-muted-foreground">
                        {integrationNames.get(rule.integration_id) ??
                          t(($) => $.issue_bridge.unknown_connection)}
                        {" · "}
                        {rule.scope_type} ·{" "}
                        {rule.sync_enabled
                          ? t(($) => $.issue_bridge.sync_on)
                          : t(($) => $.issue_bridge.sync_off)}
                      </p>
                    </div>
                    {canManage && (
                      <div className="flex shrink-0 items-center gap-2">
                        <Button variant="outline" size="sm" disabled>
                          {t(($) => $.issue_bridge.edit)}
                        </Button>
                        <Button variant="outline" size="sm" disabled>
                          {t(($) => $.issue_bridge.delete)}
                        </Button>
                      </div>
                    )}
                  </div>
                ))}
              </div>
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
