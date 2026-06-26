"use client";

import { useQuery } from "@tanstack/react-query";
import { GitBranch, GitPullRequestArrow, ShieldCheck } from "lucide-react";
import { useAuthStore } from "@multica/core/auth";
import { useWorkspaceId } from "@multica/core/hooks";
import { issueIntegrationsOptions, issueSyncConfigsOptions } from "@multica/core/issue-bridge";
import { memberListOptions } from "@multica/core/workspace/queries";
import { Button } from "@multica/ui/components/ui/button";
import { Card, CardContent } from "@multica/ui/components/ui/card";
import { Skeleton } from "@multica/ui/components/ui/skeleton";
import { useT } from "../../i18n";

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
  } = useQuery(
    issueIntegrationsOptions(wsId),
  );
  const {
    data: syncData,
    isError: syncError,
    isLoading: syncLoading,
  } = useQuery(
    issueSyncConfigsOptions(wsId),
  );

  const integrations = integrationData?.integrations ?? [];
  const syncConfigs = syncData?.sync_configs ?? [];
  const primaryIntegration = integrations[0] ?? null;
  const loading = integrationsLoading || syncLoading || membersLoading;
  const hasError = integrationsError || syncError || membersError;
  const integrationNames = new Map(
    integrations.map((integration) => [
      integration.id,
      integration.name || integration.base_url,
    ]),
  );

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
            {integrations.length > 0 ? (
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
                          {integration.name || t(($) => $.issue_bridge.connection_saved_title)}
                        </p>
                        <p className="truncate text-xs text-muted-foreground">
                          {integration.base_url}
                        </p>
                      </div>
                    </div>
                    {canManage && (
                      <div className="flex shrink-0 items-center gap-2">
                        <Button variant="outline" size="sm" disabled>
                          {t(($) => $.issue_bridge.test_connection)}
                        </Button>
                        <Button variant="outline" size="sm" disabled>
                          {t(($) => $.issue_bridge.edit)}
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
                  <Button size="sm" disabled>
                    {t(($) => $.issue_bridge.add_connection)}
                  </Button>
                )}
              </div>
            )}

            <p className="text-xs text-muted-foreground">
              {t(($) => $.issue_bridge.operator_hint)}
            </p>
            {canManage && (
              <p className="text-xs text-muted-foreground">
                {t(($) => $.issue_bridge.actions_pending_hint)}
              </p>
            )}
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
    </div>
  );
}
