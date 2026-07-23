import { useMutation, useQueryClient } from "@tanstack/react-query";
import { api } from "../api";
import { useWorkspaceId } from "../hooks";
import type {
  CreateSpecDecisionRequest,
  SyncSpecFromFilesRequest,
  UpdateIssueSpecMappingRequest,
  UpdateIssueSpecStateRequest,
  UpdateSpecDocumentRequest,
} from "../types";
import { specMemoryKeys } from "./queries";

export function useUpdateIssueSpecMapping(issueId: string) {
  const qc = useQueryClient();
  const wsId = useWorkspaceId();
  return useMutation({
    mutationFn: (data: UpdateIssueSpecMappingRequest) =>
      api.updateIssueSpecMapping(issueId, data),
    onSettled: () => {
      qc.invalidateQueries({ queryKey: specMemoryKeys.issue(wsId, issueId) });
    },
  });
}

export function useUpdateIssueSpecState(issueId: string) {
  const qc = useQueryClient();
  const wsId = useWorkspaceId();
  return useMutation({
    mutationFn: (data: UpdateIssueSpecStateRequest) =>
      api.updateIssueSpecState(issueId, data),
    onSettled: () => {
      qc.invalidateQueries({ queryKey: specMemoryKeys.issue(wsId, issueId) });
    },
  });
}

export function useUpdateSpecDocument(moduleId: string, docKind: string) {
  const qc = useQueryClient();
  const wsId = useWorkspaceId();
  return useMutation({
    mutationFn: (data: UpdateSpecDocumentRequest) =>
      api.updateSpecDocument(moduleId, docKind, data),
    onSettled: () => {
      qc.invalidateQueries({
        queryKey: specMemoryKeys.documents(wsId, moduleId),
      });
    },
  });
}

export function useCreateSpecDecision() {
  const qc = useQueryClient();
  const wsId = useWorkspaceId();
  return useMutation({
    mutationFn: (data: CreateSpecDecisionRequest) =>
      api.createSpecDecision(data),
    onSettled: (_data, _error, variables) => {
      if (variables.module_id) {
        qc.invalidateQueries({
          queryKey: specMemoryKeys.documents(wsId, variables.module_id),
        });
      }
      qc.invalidateQueries({ queryKey: specMemoryKeys.epics(wsId) });
    },
  });
}

export function useSyncSpecFromFiles() {
  const qc = useQueryClient();
  const wsId = useWorkspaceId();
  return useMutation({
    mutationFn: (data: SyncSpecFromFilesRequest) =>
      api.syncSpecFromFiles(data),
    onSettled: () => {
      qc.invalidateQueries({ queryKey: specMemoryKeys.epics(wsId) });
      qc.invalidateQueries({ queryKey: specMemoryKeys.all(wsId) });
    },
  });
}
