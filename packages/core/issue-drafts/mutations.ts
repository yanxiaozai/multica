import { useMutation, useQueryClient } from "@tanstack/react-query";
import { api } from "../api";
import { useWorkspaceId } from "../hooks";
import type {
  AppendIssueDraftMessageRequest,
  CreateIssueDraftRequest,
} from "../types";
import { issueDraftKeys } from "./queries";

export function useCreateIssueDraft() {
  const qc = useQueryClient();
  const wsId = useWorkspaceId();
  return useMutation({
    mutationFn: (data: CreateIssueDraftRequest) => api.createIssueDraft(data),
    onSuccess: (bundle) => {
      qc.setQueryData(
        issueDraftKeys.detail(wsId, bundle.session.id),
        bundle,
      );
      qc.invalidateQueries({ queryKey: issueDraftKeys.all(wsId) });
    },
  });
}

export function useAppendIssueDraftMessage(id: string) {
  const qc = useQueryClient();
  const wsId = useWorkspaceId();
  return useMutation({
    mutationFn: (data: AppendIssueDraftMessageRequest) =>
      api.appendIssueDraftMessage(id, data),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: issueDraftKeys.detail(wsId, id) });
    },
  });
}

export function useDelegateIssueDraft(id: string) {
  return useIssueDraftAction(id, () => api.delegateIssueDraft(id));
}

export function useGenerateIssueDraft(id: string) {
  return useIssueDraftAction(id, () => api.generateIssueDraft(id));
}

export function useConfirmIssueDraft(id: string) {
  return useIssueDraftAction(id, () => api.confirmIssueDraft(id));
}

export function useCancelIssueDraft(id: string) {
  return useIssueDraftAction(id, () => api.cancelIssueDraft(id));
}

function useIssueDraftAction(id: string, action: () => Promise<void>) {
  const qc = useQueryClient();
  const wsId = useWorkspaceId();
  return useMutation({
    mutationFn: action,
    onSettled: () => {
      qc.invalidateQueries({ queryKey: issueDraftKeys.detail(wsId, id) });
    },
  });
}
