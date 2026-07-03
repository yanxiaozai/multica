import { queryOptions } from "@tanstack/react-query";
import { api } from "../api";

export const issueDraftKeys = {
  all: (wsId: string) => ["issue-drafts", wsId] as const,
  detail: (wsId: string, id: string) =>
    [...issueDraftKeys.all(wsId), "detail", id] as const,
};

export function issueDraftQueryOptions(wsId: string, id: string) {
  return queryOptions({
    queryKey: issueDraftKeys.detail(wsId, id),
    queryFn: () => api.getIssueDraft(id),
    enabled: Boolean(wsId && id),
  });
}
