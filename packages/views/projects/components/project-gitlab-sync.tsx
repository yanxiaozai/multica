"use client";

import { useEffect, useMemo, useState } from "react";
import { ChevronRight, GitBranch, Zap } from "lucide-react";
import { toast } from "sonner";
import { useQuery } from "@tanstack/react-query";
import { useWorkspaceId } from "@multica/core/hooks";
import { agentListOptions, squadListOptions } from "@multica/core/workspace/queries";
import { projectResourcesOptions } from "@multica/core/projects";
import {
  issueIntegrationsOptions,
  issueSyncConfigsOptions,
  useCreateIssueSyncConfig,
  useUpdateIssueSyncConfig,
  useImportProjectGitLabIssues,
} from "@multica/core/issue-bridge";
import { Button } from "@multica/ui/components/ui/button";
import { Input } from "@multica/ui/components/ui/input";
import { Label } from "@multica/ui/components/ui/label";
import { Checkbox } from "@multica/ui/components/ui/checkbox";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@multica/ui/components/ui/select";
import {
  detectGitRemote,
  parseGitRemote,
  isDesktopShell,
} from "../../platform";
import { useT } from "../../i18n";

// One auto-detected link: a workspace integration whose host matches the
// local repo's origin, plus the parsed project ref. null until detection
// runs (or when there's no match).
interface AutoDetect {
  integrationId: string;
  integrationName: string;
  ref: string;
}

type AssigneeType = "agent" | "squad";

// Project sidebar section: link a GitLab project and one-shot import the
// issues assigned to the connection owner. The sync config (integration +
// remote_project_ref) is created here so the import has a source to pull
// from. Phase B will add polling/auto-assign toggles on top of the same
// config row.
export function ProjectGitLabSyncSection({ projectId }: { projectId: string }) {
  const { t } = useT("projects");
  const wsId = useWorkspaceId();

  const { data: integrationsResp } = useQuery(
    issueIntegrationsOptions(wsId),
  );
  const { data: syncConfigsResp } = useQuery(issueSyncConfigsOptions(wsId));
  const integrations = integrationsResp?.integrations ?? [];
  const syncConfigs = syncConfigsResp?.sync_configs ?? [];

  // The project's own sync config — at most one per (scope_type=project,
  // scope_id=projectId) by the UNIQUE constraint on the table.
  const config = syncConfigs.find(
    (c) => c.scope_type === "project" && c.scope_id === projectId,
  );

  const createConfig = useCreateIssueSyncConfig(wsId);
  const updateConfig = useUpdateIssueSyncConfig(wsId);
  const importIssues = useImportProjectGitLabIssues(wsId);
  const { data: agents = [] } = useQuery(agentListOptions(wsId));
  const { data: squads = [] } = useQuery(squadListOptions(wsId));

  // Project resources — only `local_directory` ones point at a local git
  // working tree we can auto-detect from. github_repo resources have no
  // local path; they carry the URL directly and don't need detection.
  const { data: resources } = useQuery(projectResourcesOptions(wsId, projectId));
  const localPath = useMemo(() => {
    const local = (resources ?? []).find((r) => r.resource_type === "local_directory");
    const ref = local?.resource_ref as { local_path?: string } | undefined;
    return ref?.local_path;
  }, [resources]);

  const [open, setOpen] = useState(false);
  const [integrationId, setIntegrationId] = useState("");
  const [remoteRef, setRemoteRef] = useState("");

  // Auto-detection result: a matching integration + parsed ref, or null once
  // detection has run and found nothing. `undefined` = not yet attempted.
  const [autoDetect, setAutoDetect] = useState<AutoDetect | null | undefined>(undefined);
  const [autoConnecting, setAutoConnecting] = useState(false);

  // Editable polling/auto-assign fields, seeded from the config row. Tracked
  // separately so the user can stage changes and Save in one round-trip.
  const [pollEnabled, setPollEnabled] = useState(false);
  const [pollInterval, setPollInterval] = useState(300);
  const [syncMode, setSyncMode] = useState<"assigned_to_me" | "auto_accept">(
    "assigned_to_me",
  );
  const [autoAcceptLabel, setAutoAcceptLabel] = useState("ai-auto");
  const [autoAssign, setAutoAssign] = useState(false);
  const [assigneeType, setAssigneeType] = useState<AssigneeType>("agent");
  const [agentId, setAgentId] = useState("");
  const syncModeLabel =
    syncMode === "auto_accept"
      ? t(($) => $.gitlab_sync.sync_mode_auto_accept)
      : t(($) => $.gitlab_sync.sync_mode_assigned_to_me);
  const assigneeTypeLabel =
    assigneeType === "squad"
      ? t(($) => $.gitlab_sync.assignee_type_squad)
      : t(($) => $.gitlab_sync.assignee_type_agent);
  const visibleAssignees =
    assigneeType === "squad"
      ? squads.filter((squad) => !squad.archived_at)
      : agents.filter((agent) => !agent.archived_at);
  const selectedAssigneeName =
    visibleAssignees.find((assignee) => assignee.id === agentId)?.name ?? "";
  const selectedIntegrationName = integrationId
    ? integrations.find((integration) => integration.id === integrationId)?.name ??
      integrations.find((integration) => integration.id === integrationId)?.base_url ??
      ""
    : "";

  // Re-seed the form whenever the config row loads / changes. Without this a
  // save would clobber server state the user never saw.
  useEffect(() => {
    if (!config) return;
    setPollEnabled(config.sync_enabled);
    setPollInterval(config.poll_interval_seconds ?? 300);
    setSyncMode(config.sync_mode ?? "assigned_to_me");
    setAutoAcceptLabel(config.auto_accept_label || "ai-auto");
    setAutoAssign(config.auto_assign_enabled);
    setAssigneeType(config.default_assignee_type === "squad" ? "squad" : "agent");
    setAgentId(config.default_assignee_id ?? "");
  }, [config]);

  // Auto-detect: only when there's no config yet, the panel is open, we're on
  // desktop, and the project has a local_directory resource. Read the local
  // repo's origin, parse host+ref, and match the host against the workspace's
  // GitLab integrations (scheme/port-agnostic, so an SSH remote matches an
  // HTTPS integration). Runs once per open.
  useEffect(() => {
    if (!open || config || !localPath || !isDesktopShell()) {
      setAutoDetect(undefined);
      return;
    }
    let cancelled = false;
    setAutoDetect(undefined);
    detectGitRemote(localPath)
      .then((res) => {
        if (cancelled || !res.ok || !res.remote_url) return;
        const parsed = parseGitRemote(res.remote_url);
        if (!parsed) return;
        const match = integrations.find(
          (itg) => hostOf(itg.base_url) === parsed.host,
        );
        if (!match) return;
        if (cancelled) return;
        setAutoDetect({
          integrationId: match.id,
          integrationName: match.name || match.base_url,
          ref: parsed.ref,
        });
      })
      .catch(() => {
        // Detection is best-effort; any failure just falls back to manual.
      });
    return () => {
      cancelled = true;
    };
  }, [open, config, localPath, integrations]);

  const noIntegration = integrations.length === 0;

  const handleConnect = async () => {
    if (!integrationId || !remoteRef.trim()) return;
    try {
      await createConfig.mutateAsync({
        integration_id: integrationId,
        scope_type: "project",
        scope_id: projectId,
        remote_project_ref: remoteRef.trim(),
        sync_enabled: false,
      });
      setRemoteRef("");
      setIntegrationId("");
      setOpen(false);
    } catch (err) {
      toast.error(
        t(($) => $.gitlab_sync.toast_connect_failed, {
          error: err instanceof Error ? err.message : "unknown error",
        }),
      );
    }
  };

  const handleImport = async () => {
    try {
      const result = await importIssues.mutateAsync(projectId);
      if (result.failed > 0 || result.skipped > 0 || result.updated > 0) {
        toast.message(
          t(($) => $.gitlab_sync.toast_import_partial, {
            imported: result.imported,
            updated: result.updated,
            skipped: result.skipped,
            failed: result.failed,
          }),
        );
      } else {
        toast.success(
          t(($) => $.gitlab_sync.toast_imported, { count: result.imported }),
        );
      }
    } catch (err) {
      toast.error(
        t(($) => $.gitlab_sync.toast_import_failed, {
          error: err instanceof Error ? err.message : "unknown error",
        }),
      );
    }
  };

  const handleSaveSettings = async () => {
    if (!config) return;
    try {
      await updateConfig.mutateAsync({
        id: config.id,
        data: {
          // remote_project_ref is required by the upsert validator and
          // immutable here — forward the current value unchanged.
          remote_project_ref: config.remote_project_ref,
          sync_enabled: pollEnabled,
          poll_interval_seconds: pollEnabled ? pollInterval : null,
          sync_mode: syncMode,
          auto_accept_label: autoAcceptLabel.trim() || "ai-auto",
          auto_assign_enabled: autoAssign,
          default_assignee_type: autoAssign ? assigneeType : null,
          default_assignee_id: autoAssign ? agentId : null,
        },
      });
    } catch (err) {
      toast.error(
        t(($) => $.gitlab_sync.toast_connect_failed, {
          error: err instanceof Error ? err.message : "unknown error",
        }),
      );
    }
  };

  // Whether the staged form differs from the persisted config — gates the
  // Save button so the user can tell when there's nothing to write.
  const dirty =
    !!config &&
    (pollEnabled !== config.sync_enabled ||
      (config.poll_interval_seconds ?? 300) !== pollInterval ||
      (config.sync_mode ?? "assigned_to_me") !== syncMode ||
      (config.auto_accept_label || "ai-auto") !== autoAcceptLabel ||
      autoAssign !== config.auto_assign_enabled ||
      (config.default_assignee_type ?? "agent") !== assigneeType ||
      (config.default_assignee_id ?? "") !== agentId);

  // One-click connect from an auto-detected match — skips the manual form.
  const handleAutoConnect = async () => {
    if (!autoDetect) return;
    setAutoConnecting(true);
    try {
      await createConfig.mutateAsync({
        integration_id: autoDetect.integrationId,
        scope_type: "project",
        scope_id: projectId,
        remote_project_ref: autoDetect.ref,
        sync_enabled: false,
      });
    } catch (err) {
      toast.error(
        t(($) => $.gitlab_sync.toast_connect_failed, {
          error: err instanceof Error ? err.message : "unknown error",
        }),
      );
    } finally {
      setAutoConnecting(false);
    }
  };

  return (
    <div>
      <button
        type="button"
        className={`flex w-full items-center gap-1 rounded-md px-2 py-1 text-xs font-medium transition-colors mb-2 hover:bg-accent/70 ${
          open ? "" : "text-muted-foreground hover:text-foreground"
        }`}
        onClick={() => setOpen(!open)}
      >
        <GitBranch className="!size-3 shrink-0 text-muted-foreground" />
        {t(($) => $.gitlab_sync.section_header)}
        <ChevronRight
          className={`!size-3 shrink-0 stroke-[2.5] text-muted-foreground transition-transform ${
            open ? "rotate-90" : ""
          }`}
        />
      </button>
      {open && (
        <div className="pl-2 space-y-2">
          {noIntegration && (
            <p className="text-xs text-muted-foreground">
              {t(($) => $.gitlab_sync.no_integration)}
            </p>
          )}

          {/* Auto-detected match: one-click connect, skips the manual form.
              Only relevant before a config exists and when detection ran. */}
          {!config && !noIntegration && autoDetect && (
            <div className="space-y-2 rounded-md border border-primary/40 bg-primary/5 p-2">
              <p className="flex items-start gap-1.5 text-xs">
                <Zap className="mt-0.5 h-3.5 w-3.5 shrink-0 text-primary" />
                <span>
                  {t(($) => $.gitlab_sync.autodetect_label, {
                    integration: autoDetect.integrationName,
                    ref: autoDetect.ref,
                  })}
                </span>
              </p>
              <Button
                size="sm"
                className="h-7 w-full text-xs"
                onClick={handleAutoConnect}
                disabled={autoConnecting || createConfig.isPending}
              >
                {autoConnecting || createConfig.isPending
                  ? t(($) => $.gitlab_sync.autodetect_connecting)
                  : t(($) => $.gitlab_sync.autodetect_connect)}
              </Button>
            </div>
          )}

          {config ? (
            <div className="space-y-3">
              <p className="text-xs text-muted-foreground">
                {t(($) => $.gitlab_sync.connected_as, {
                  ref: config.remote_project_ref,
                })}
              </p>
              <Button
                size="sm"
                variant="outline"
                className="h-7 w-full text-xs"
                onClick={handleImport}
                disabled={importIssues.isPending}
              >
                {importIssues.isPending
                  ? t(($) => $.gitlab_sync.importing)
                  : t(($) => $.gitlab_sync.import_button)}
              </Button>

              {/* Polling + auto-assign settings (Phase B). */}
              <label className="flex items-center gap-2 text-xs">
                <Checkbox
                  checked={pollEnabled}
                  onCheckedChange={(v) => setPollEnabled(v === true)}
                />
                {t(($) => $.gitlab_sync.poll_toggle)}
              </label>
              {pollEnabled && (
                <div className="space-y-3">
                  <div>
                    <Label className="text-[11px] text-muted-foreground">
                      {t(($) => $.gitlab_sync.poll_interval_label)}
                    </Label>
                    <Input
                      type="number"
                      min={60}
                      className="mt-1 h-8 text-xs"
                      value={pollInterval}
                      onChange={(e) =>
                        setPollInterval(Number.parseInt(e.target.value, 10) || 60)
                      }
                    />
                    <p className="mt-1 text-[11px] text-muted-foreground/70">
                      {t(($) => $.gitlab_sync.poll_interval_hint)}
                    </p>
                  </div>

                  <div>
                    <Label className="text-[11px] text-muted-foreground">
                      {t(($) => $.gitlab_sync.sync_mode_label)}
                    </Label>
                    <Select
                      value={syncMode}
                      onValueChange={(v) => {
                        const next = v as "assigned_to_me" | "auto_accept";
                        setSyncMode(next);
                        if (next === "auto_accept") setAutoAssign(true);
                      }}
                    >
                      <SelectTrigger className="mt-1 h-8 text-xs">
                        <SelectValue>{syncModeLabel}</SelectValue>
                      </SelectTrigger>
                      <SelectContent>
                        <SelectItem value="assigned_to_me">
                          {t(($) => $.gitlab_sync.sync_mode_assigned_to_me)}
                        </SelectItem>
                        <SelectItem value="auto_accept">
                          {t(($) => $.gitlab_sync.sync_mode_auto_accept)}
                        </SelectItem>
                      </SelectContent>
                    </Select>
                    <p className="mt-1 text-[11px] text-muted-foreground/70">
                      {syncMode === "auto_accept"
                        ? t(($) => $.gitlab_sync.sync_mode_auto_accept_hint)
                        : t(($) => $.gitlab_sync.poll_interval_hint)}
                    </p>
                  </div>

                  {syncMode === "auto_accept" && (
                    <div>
                      <Label className="text-[11px] text-muted-foreground">
                        {t(($) => $.gitlab_sync.auto_accept_label)}
                      </Label>
                      <Input
                        className="mt-1 h-8 text-xs"
                        value={autoAcceptLabel}
                        onChange={(e) => setAutoAcceptLabel(e.target.value)}
                      />
                    </div>
                  )}
                </div>
              )}

              <label className="flex items-center gap-2 text-xs">
                <Checkbox
                  checked={autoAssign}
                  disabled={syncMode === "auto_accept"}
                  onCheckedChange={(v) => setAutoAssign(v === true)}
                />
                {t(($) => $.gitlab_sync.auto_assign_toggle)}
              </label>
              {autoAssign && (
                <div className="grid gap-2 sm:grid-cols-2">
                  <div>
                    <Label className="text-[11px] text-muted-foreground">
                      {t(($) => $.gitlab_sync.assignee_type_label)}
                    </Label>
                    <Select
                      value={assigneeType}
                      onValueChange={(v) => {
                        setAssigneeType(v === "squad" ? "squad" : "agent");
                        setAgentId("");
                      }}
                    >
                      <SelectTrigger className="mt-1 h-8 text-xs">
                        <SelectValue>{assigneeTypeLabel}</SelectValue>
                      </SelectTrigger>
                      <SelectContent>
                        <SelectItem value="agent">
                          {t(($) => $.gitlab_sync.assignee_type_agent)}
                        </SelectItem>
                        <SelectItem value="squad">
                          {t(($) => $.gitlab_sync.assignee_type_squad)}
                        </SelectItem>
                      </SelectContent>
                    </Select>
                  </div>

                  <div>
                    <Label className="text-[11px] text-muted-foreground">
                      {t(($) => $.gitlab_sync.assignee_label)}
                    </Label>
                    <Select
                      value={agentId}
                      onValueChange={(v) => setAgentId(v ?? "")}
                    >
                      <SelectTrigger className="mt-1 h-8 text-xs">
                        <SelectValue placeholder="—">
                          {selectedAssigneeName || "—"}
                        </SelectValue>
                      </SelectTrigger>
                      <SelectContent>
                        {visibleAssignees.map((assignee) => (
                          <SelectItem key={assignee.id} value={assignee.id}>
                            {assignee.name}
                          </SelectItem>
                        ))}
                      </SelectContent>
                    </Select>
                  </div>
                </div>
              )}

              <Button
                size="sm"
                className="h-7 w-full text-xs"
                onClick={handleSaveSettings}
                disabled={
                  !dirty ||
                  updateConfig.isPending ||
                  (syncMode === "auto_accept" && !agentId)
                }
              >
                {updateConfig.isPending
                  ? t(($) => $.gitlab_sync.saving)
                  : t(($) => $.gitlab_sync.save)}
              </Button>

              {/* Sync status — populated by the scheduler's watermark writes. */}
              {config.last_poll_at && (
                <p className="text-[11px] text-muted-foreground/70">
                  {config.last_error
                    ? t(($) => $.gitlab_sync.status_error, {
                        error: config.last_error,
                      })
                    : t(($) => $.gitlab_sync.status_ok, {
                        at: new Date(config.last_poll_at).toLocaleString(),
                      })}
                </p>
              )}
            </div>
          ) : (
            !noIntegration && !autoDetect && (
              <div className="space-y-2">
                <p className="text-xs text-muted-foreground">
                  {t(($) => $.gitlab_sync.configure_hint)}
                </p>
                <div>
                  <Label className="text-[11px] text-muted-foreground">
                    {t(($) => $.gitlab_sync.integration_label)}
                  </Label>
                  <Select
                    value={integrationId}
                    onValueChange={(v) => setIntegrationId(v ?? "")}
                  >
                    <SelectTrigger className="mt-1 h-8 text-xs">
                      <SelectValue placeholder="—">
                        {selectedIntegrationName || "—"}
                      </SelectValue>
                    </SelectTrigger>
                    <SelectContent>
                      {integrations.map((itg) => (
                        <SelectItem key={itg.id} value={itg.id}>
                          {itg.name || itg.base_url}
                        </SelectItem>
                      ))}
                    </SelectContent>
                  </Select>
                </div>
                <div>
                  <Label className="text-[11px] text-muted-foreground">
                    {t(($) => $.gitlab_sync.remote_ref_label)}
                  </Label>
                  <Input
                    className="mt-1 h-8 text-xs"
                    value={remoteRef}
                    onChange={(e) => setRemoteRef(e.target.value)}
                    placeholder={t(($) => $.gitlab_sync.remote_ref_placeholder)}
                  />
                </div>
                <Button
                  size="sm"
                  className="h-7 w-full text-xs"
                  onClick={handleConnect}
                  disabled={
                    createConfig.isPending ||
                    !integrationId ||
                    !remoteRef.trim()
                  }
                >
                  {createConfig.isPending
                    ? t(($) => $.gitlab_sync.connecting)
                    : t(($) => $.gitlab_sync.connect)}
                </Button>
              </div>
            )
          )}
        </div>
      )}
    </div>
  );
}

// Extract the bare lowercase hostname from an integration base_url
// (`https://gitlab.example.com` → `gitlab.example.com`). Used to match against
// a parsed git remote's host, scheme/port-agnostic so SSH remotes match HTTPS
// integrations. Returns "" on any parse failure.
function hostOf(baseURL: string): string {
  try {
    return new URL(baseURL).hostname.toLowerCase();
  } catch {
    return "";
  }
}
