"use client";

import { useEffect, useMemo, useRef, useState } from "react";
import { BookOpen, FileText, FolderSync, Layers3, Terminal } from "lucide-react";
import { useQuery } from "@tanstack/react-query";
import { projectResourcesOptions } from "@multica/core/projects";
import {
  specEpicDocumentListOptions,
  specEpicListOptions,
  specModuleListOptions,
  useSyncSpecFromFiles,
} from "@multica/core/spec-memory";
import { useWorkspaceId } from "@multica/core/hooks";
import type {
  LocalDirectoryResourceRef,
  ProjectResource,
  SpecDocument,
  SpecEpic,
} from "@multica/core/types";
import { Badge } from "@multica/ui/components/ui/badge";
import { Skeleton } from "@multica/ui/components/ui/skeleton";
import { cn } from "@multica/ui/lib/utils";
import { toast } from "sonner";
import { PageHeader } from "../../layout/page-header";
import { isDesktopShell, readSpecSnapshot, useLocalDaemonStatus } from "../../platform";
import { ModuleDocumentsPanel } from "./module-documents-panel";

function isLocalDirectoryRef(r: ProjectResource): r is ProjectResource & {
  resource_ref: LocalDirectoryResourceRef;
} {
  return r.resource_type === "local_directory";
}

function snapshotHasContent(snapshot: {
  epics: unknown[];
  issues: unknown[];
  decisions: unknown[];
}) {
  return snapshot.epics.length > 0 || snapshot.issues.length > 0 || snapshot.decisions.length > 0;
}

export function SpecMemoryPage({ projectId }: { projectId?: string }) {
  const wsId = useWorkspaceId();
  const epics = useQuery(specEpicListOptions(wsId));
  const [selectedEpicId, setSelectedEpicId] = useState("");
  const modules = useQuery(specModuleListOptions(wsId, selectedEpicId));
  const epicDocuments = useQuery(specEpicDocumentListOptions(wsId, selectedEpicId));
  const [selectedModuleId, setSelectedModuleId] = useState("");
  const daemonStatus = useLocalDaemonStatus();
  const syncSpec = useSyncSpecFromFiles();
  const autoSyncKeys = useRef(new Set<string>());
  const resources = useQuery({
    ...projectResourcesOptions(wsId, projectId ?? ""),
    enabled: !!projectId,
  });

  const localSpecRoot = useMemo(() => {
    if (!projectId || !isDesktopShell()) return "";
    const localDirectories = (resources.data ?? []).filter(isLocalDirectoryRef);
    if (daemonStatus.daemonId) {
      const matching = localDirectories.find(
        (item) => item.resource_ref.daemon_id === daemonStatus.daemonId,
      );
      if (matching) return matching.resource_ref.local_path;
    }
    // Desktop can read files without the daemon process itself being connected.
    // If there is only one local directory, use it as the project-local sync
    // root; with multiple machine-bound directories we wait for daemonId so we
    // do not guess the wrong teammate's path.
    return localDirectories.length === 1
      ? localDirectories[0]?.resource_ref.local_path ?? ""
      : "";
  }, [daemonStatus.daemonId, projectId, resources.data]);

  useEffect(() => {
    if (!projectId || !localSpecRoot || syncSpec.isPending) return;
    const syncKey = `${wsId}:${projectId}:${localSpecRoot}`;
    if (autoSyncKeys.current.has(syncKey)) return;
    autoSyncKeys.current.add(syncKey);

    let cancelled = false;
    void (async () => {
      const result = await readSpecSnapshot(localSpecRoot);
      if (cancelled) return;
      if (!result.ok) {
        if (result.reason !== "no_spec" && result.reason !== "unsupported") {
          toast.error(result.error ?? "Failed to read local .spec files");
        }
        return;
      }
      if (!snapshotHasContent(result.snapshot)) return;
      try {
        const synced = await syncSpec.mutateAsync(result.snapshot);
        if (cancelled) return;
        toast.success(
          `Synced ${synced.epics} epics, ${synced.modules} modules, ${synced.documents} docs from local .spec`,
        );
      } catch (err) {
        if (!cancelled) {
          toast.error(err instanceof Error ? err.message : "Failed to sync local .spec files");
        }
      }
    })();

    return () => {
      cancelled = true;
    };
  }, [localSpecRoot, projectId, syncSpec, wsId]);

  useEffect(() => {
    if (!selectedEpicId && epics.data?.[0]) {
      setSelectedEpicId(epics.data[0].id);
    }
  }, [epics.data, selectedEpicId]);

  useEffect(() => {
    if (
      selectedModuleId &&
      modules.data &&
      !modules.data.some((module) => module.id === selectedModuleId)
    ) {
      setSelectedModuleId("");
      return;
    }
    if (!selectedModuleId && modules.data?.[0]) {
      setSelectedModuleId(modules.data[0].id);
    }
  }, [modules.data, selectedModuleId]);

  const selectedEpic = useMemo(
    () => epics.data?.find((epic) => epic.id === selectedEpicId) ?? null,
    [epics.data, selectedEpicId],
  );
  const selectedModule = useMemo(
    () => modules.data?.find((module) => module.id === selectedModuleId) ?? null,
    [modules.data, selectedModuleId],
  );
  const showEpicDocuments =
    !!selectedEpic && !modules.isLoading && (modules.data ?? []).length === 0;
  const hasNoSyncedMemory =
    !epics.isLoading && !epics.isError && (epics.data ?? []).length === 0;

  return (
    <div className="flex h-full min-h-0 flex-col">
      <PageHeader>
        <div className="min-w-0">
          <h1 className="truncate text-sm font-semibold">Spec Memory</h1>
          <p className="truncate text-xs text-muted-foreground">
            Durable epics, module knowledge, and issue-scoped execution state.
          </p>
        </div>
      </PageHeader>
      {hasNoSyncedMemory ? (
        <div className="flex min-h-0 flex-1 items-center justify-center p-6">
          <section className="grid w-full max-w-2xl gap-4 rounded-lg border border-dashed bg-background p-6">
            <div className="flex items-start gap-3">
              <div className="flex size-10 shrink-0 items-center justify-center rounded-md border bg-muted">
                <FolderSync className="size-5 text-muted-foreground" />
              </div>
              <div className="min-w-0">
                <h2 className="text-sm font-semibold">No synced spec memory yet</h2>
                <p className="mt-1 text-sm text-muted-foreground">
                  {localSpecRoot
                    ? "Reading and syncing local .spec files from the project's local directory."
                    : "Local .spec files are not read directly by the web app. Import them into the workspace first, then reopen Spec Memory."}
                </p>
              </div>
            </div>
            <div className="flex items-center gap-2 rounded-md border bg-muted/40 px-3 py-2 font-mono text-xs text-muted-foreground">
              <Terminal className="size-3.5 shrink-0" />
              <span className="truncate">multica spec sync --from-files</span>
            </div>
          </section>
        </div>
      ) : (
      <div className="grid min-h-0 flex-1 grid-cols-[280px_minmax(0,1fr)] gap-4 p-4">
        <aside className="min-h-0 overflow-hidden rounded-lg border bg-background">
          <div className="border-b p-3">
            <div className="flex items-center gap-2 text-sm font-medium">
              <BookOpen className="size-4" />
              Epics
            </div>
          </div>
          <div className="max-h-64 overflow-y-auto p-2">
            {epics.isLoading ? (
              <Skeleton className="h-24 w-full" />
            ) : (
              (epics.data ?? []).map((epic) => (
                <button
                  key={epic.id}
                  type="button"
                  onClick={() => {
                    setSelectedEpicId(epic.id);
                    setSelectedModuleId("");
                  }}
                  className={cn(
                    "mb-1 flex w-full items-start justify-between gap-2 rounded-md px-2 py-2 text-left text-sm hover:bg-muted",
                    epic.id === selectedEpicId && "bg-muted",
                  )}
                >
                  <span className="min-w-0">
                    <span className="block truncate font-medium">{epic.title}</span>
                    <span className="block truncate text-xs text-muted-foreground">
                      {epic.key}
                    </span>
                  </span>
                  <Badge variant="outline">{epic.stability}</Badge>
                </button>
              ))
            )}
          </div>
          <div className="border-t p-3">
            <div className="mb-2 flex items-center gap-2 text-sm font-medium">
              <Layers3 className="size-4" />
              Modules
            </div>
            <div className="max-h-[calc(100vh-24rem)] overflow-y-auto">
              {modules.isLoading ? (
                <Skeleton className="h-24 w-full" />
              ) : (modules.data ?? []).length === 0 ? (
                <p className="px-2 py-3 text-xs text-muted-foreground">
                  This epic has no modules.
                </p>
              ) : (
                (modules.data ?? []).map((module) => (
                  <button
                    key={module.id}
                    type="button"
                    onClick={() => setSelectedModuleId(module.id)}
                    className={cn(
                      "mb-1 flex w-full flex-col rounded-md px-2 py-2 text-left text-sm hover:bg-muted",
                      module.id === selectedModuleId && "bg-muted",
                    )}
                  >
                    <span className="truncate font-medium">{module.title}</span>
                    <span className="truncate text-xs text-muted-foreground">
                      {module.key}
                    </span>
                  </button>
                ))
              )}
            </div>
          </div>
        </aside>
        {showEpicDocuments ? (
          <EpicDocumentsPanel
            epic={selectedEpic}
            documents={epicDocuments.data ?? []}
            isLoading={epicDocuments.isLoading}
          />
        ) : (
          <ModuleDocumentsPanel module={selectedModule} />
        )}
      </div>
      )}
    </div>
  );
}

function EpicDocumentsPanel({
  epic,
  documents,
  isLoading,
}: {
  epic: SpecEpic;
  documents: SpecDocument[];
  isLoading: boolean;
}) {
  const [selectedKind, setSelectedKind] = useState("index");
  const selectedDocument = useMemo(
    () =>
      documents.find((document) => document.doc_kind === selectedKind) ??
      documents[0] ??
      null,
    [documents, selectedKind],
  );

  useEffect(() => {
    if (selectedDocument) setSelectedKind(selectedDocument.doc_kind);
  }, [selectedDocument]);

  if (isLoading) {
    return (
      <section className="space-y-3">
        <Skeleton className="h-8 w-52" />
        <Skeleton className="h-96 w-full" />
      </section>
    );
  }

  if (documents.length === 0) {
    return (
      <section className="flex min-h-72 items-center justify-center rounded-lg border border-dashed">
        <p className="text-sm text-muted-foreground">
          No epic-level documents have been synced for this epic.
        </p>
      </section>
    );
  }

  return (
    <section className="grid min-h-0 grid-cols-[220px_minmax(0,1fr)] gap-4">
      <aside className="min-h-0 rounded-lg border bg-background">
        <div className="border-b p-3">
          <div className="flex items-center gap-2 text-sm font-medium">
            <FileText className="size-4" />
            {epic.key}
          </div>
          <p className="mt-1 line-clamp-2 text-xs text-muted-foreground">
            {epic.title}
          </p>
        </div>
        <div className="p-2">
          {documents.map((document) => (
            <button
              key={document.id}
              type="button"
              onClick={() => setSelectedKind(document.doc_kind)}
              className={cn(
                "flex w-full items-center justify-between gap-2 rounded-md px-2 py-2 text-left text-sm hover:bg-muted",
                document.doc_kind === selectedKind && "bg-muted",
              )}
            >
              <span className="truncate">{document.title || document.doc_kind}</span>
              <Badge variant="outline" className="max-w-24 truncate">
                {document.doc_kind}
              </Badge>
            </button>
          ))}
        </div>
      </aside>
      <article className="flex min-h-0 flex-col rounded-lg border bg-background">
        <div className="border-b p-3">
          <h2 className="truncate text-sm font-semibold">
            {selectedDocument?.title ?? selectedKind}
          </h2>
          <p className="mt-1 truncate text-xs text-muted-foreground">
            {selectedDocument?.source_path ?? ""}
          </p>
        </div>
        <pre className="min-h-[32rem] flex-1 overflow-auto whitespace-pre-wrap p-3 font-mono text-sm">
          {selectedDocument?.body ?? ""}
        </pre>
      </article>
    </section>
  );
}
