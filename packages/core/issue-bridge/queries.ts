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
