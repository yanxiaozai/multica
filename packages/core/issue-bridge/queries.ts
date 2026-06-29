import { queryOptions, useMutation, useQueryClient } from "@tanstack/react-query";
import { api } from "../api";
import { issueKeys } from "../issues/queries";
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
    queryFn: () => api.listIssueIntegrations({ workspace_id: wsId }),
    enabled: !!wsId,
  });
}

export function issueSyncConfigsOptions(wsId: string) {
  return queryOptions({
    queryKey: issueBridgeKeys.syncConfigs(wsId),
    queryFn: () => api.listIssueSyncConfigs({ workspace_id: wsId }),
    enabled: !!wsId,
  });
}

export function useCreateGitLabIssueIntegration(wsId: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (data: CreateGitLabIssueIntegrationRequest) =>
      api.createGitLabIssueIntegration(data, { workspace_id: wsId }),
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
    }) => api.updateIssueIntegration(id, data, { workspace_id: wsId }),
    onSettled: () => {
      qc.invalidateQueries({ queryKey: issueBridgeKeys.integrations(wsId) });
      qc.invalidateQueries({ queryKey: issueBridgeKeys.syncConfigs(wsId) });
    },
  });
}

export function useDeleteIssueIntegration(wsId: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (id: string) =>
      api.deleteIssueIntegration(id, { workspace_id: wsId }),
    onSettled: () => {
      qc.invalidateQueries({ queryKey: issueBridgeKeys.integrations(wsId) });
      qc.invalidateQueries({ queryKey: issueBridgeKeys.syncConfigs(wsId) });
    },
  });
}

export function useTestIssueIntegration(wsId: string) {
  return useMutation({
    mutationFn: (id: string) =>
      api.testIssueIntegration(id, { workspace_id: wsId }),
  });
}

export function useCreateIssueSyncConfig(wsId: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (data: UpsertIssueSyncConfigRequest) =>
      api.createIssueSyncConfig(data, { workspace_id: wsId }),
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
    }) => api.updateIssueSyncConfig(id, data, { workspace_id: wsId }),
    onSettled: () => {
      qc.invalidateQueries({ queryKey: issueBridgeKeys.syncConfigs(wsId) });
    },
  });
}

export function useDeleteIssueSyncConfig(wsId: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (id: string) =>
      api.deleteIssueSyncConfig(id, { workspace_id: wsId }),
    onSettled: () => {
      qc.invalidateQueries({ queryKey: issueBridgeKeys.syncConfigs(wsId) });
    },
  });
}

/** One-shot import of GitLab issues (assigned to the connection owner) into a
 *  project. Invalidates the issue list caches on settle so the newly created
 *  issues show up in the importing user's project board / My Issues / Gantt
 *  without a manual refresh. We can't rely on the issue:created WS event here
 *  because the server-side import emits a minimal `{issue_id}` payload (no
 *  full issue object), which the WS dispatcher drops before it can invalidate
 *  caches — so the mutation does the invalidation itself, mirroring
 *  useCreateIssue. This only covers the importing client; OTHER clients/tabs
 *  still won't see imported or polled issues live until they refresh. See the
 *  TODO(issubreidge-ws) at the IssueService.Create call in
 *  server/internal/service/issuebridge/sync.go for the proper fix. */
export function useImportProjectGitLabIssues(wsId: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (projectId: string) =>
      api.importProjectGitLabIssues(projectId, { workspace_id: wsId }),
    onSettled: () => {
      // myAll covers BOTH the project board (scope=project:ID) and My Issues.
      qc.invalidateQueries({ queryKey: issueKeys.myAll(wsId) });
      qc.invalidateQueries({ queryKey: issueKeys.list(wsId) });
      qc.invalidateQueries({ queryKey: issueKeys.assigneeGroupsAll(wsId) });
      qc.invalidateQueries({ queryKey: issueKeys.myAssigneeGroupsAll(wsId) });
      qc.invalidateQueries({ queryKey: issueKeys.projectGanttAll(wsId) });
      qc.invalidateQueries({ queryKey: issueBridgeKeys.syncConfigs(wsId) });
    },
  });
}
