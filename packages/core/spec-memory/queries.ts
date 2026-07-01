import { queryOptions } from "@tanstack/react-query";
import { api } from "../api";

export const specMemoryKeys = {
  all: (wsId: string) => ["spec-memory", wsId] as const,
  epics: (wsId: string) => [...specMemoryKeys.all(wsId), "epics"] as const,
  modules: (wsId: string, epicId: string) =>
    [...specMemoryKeys.all(wsId), "epics", epicId, "modules"] as const,
  epicDocuments: (wsId: string, epicId: string) =>
    [...specMemoryKeys.all(wsId), "epics", epicId, "documents"] as const,
  documents: (wsId: string, moduleId: string) =>
    [...specMemoryKeys.all(wsId), "modules", moduleId, "documents"] as const,
  issue: (wsId: string, issueId: string) =>
    [...specMemoryKeys.all(wsId), "issues", issueId] as const,
};

export function specEpicListOptions(wsId: string) {
  return queryOptions({
    queryKey: specMemoryKeys.epics(wsId),
    queryFn: () => api.listSpecEpics(),
    select: (data) => data.epics,
  });
}

export function specModuleListOptions(wsId: string, epicId: string) {
  return queryOptions({
    queryKey: specMemoryKeys.modules(wsId, epicId),
    queryFn: () => api.listSpecModules(epicId),
    select: (data) => data.modules,
    enabled: !!epicId,
  });
}

export function specEpicDocumentListOptions(wsId: string, epicId: string) {
  return queryOptions({
    queryKey: specMemoryKeys.epicDocuments(wsId, epicId),
    queryFn: () => api.listSpecEpicDocuments(epicId),
    select: (data) => data.documents,
    enabled: !!epicId,
  });
}

export function specDocumentListOptions(wsId: string, moduleId: string) {
  return queryOptions({
    queryKey: specMemoryKeys.documents(wsId, moduleId),
    queryFn: () => api.listSpecDocuments(moduleId),
    select: (data) => data.documents,
    enabled: !!moduleId,
  });
}

export function issueSpecOptions(wsId: string, issueId: string) {
  return queryOptions({
    queryKey: specMemoryKeys.issue(wsId, issueId),
    queryFn: () => api.getIssueSpec(issueId),
    enabled: !!issueId,
  });
}
