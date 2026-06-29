"use client";

import { useEffect, useMemo, useState } from "react";
import { BookOpen, Layers3 } from "lucide-react";
import { useQuery } from "@tanstack/react-query";
import {
  specEpicListOptions,
  specModuleListOptions,
} from "@multica/core/spec-memory";
import { useWorkspaceId } from "@multica/core/hooks";
import { Badge } from "@multica/ui/components/ui/badge";
import { Skeleton } from "@multica/ui/components/ui/skeleton";
import { cn } from "@multica/ui/lib/utils";
import { PageHeader } from "../../layout/page-header";
import { ModuleDocumentsPanel } from "./module-documents-panel";

export function SpecMemoryPage() {
  const wsId = useWorkspaceId();
  const epics = useQuery(specEpicListOptions(wsId));
  const [selectedEpicId, setSelectedEpicId] = useState("");
  const modules = useQuery(specModuleListOptions(wsId, selectedEpicId));
  const [selectedModuleId, setSelectedModuleId] = useState("");

  useEffect(() => {
    if (!selectedEpicId && epics.data?.[0]) {
      setSelectedEpicId(epics.data[0].id);
    }
  }, [epics.data, selectedEpicId]);

  useEffect(() => {
    if (!selectedModuleId && modules.data?.[0]) {
      setSelectedModuleId(modules.data[0].id);
    }
  }, [modules.data, selectedModuleId]);

  const selectedModule = useMemo(
    () => modules.data?.find((module) => module.id === selectedModuleId) ?? null,
    [modules.data, selectedModuleId],
  );

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
        <ModuleDocumentsPanel module={selectedModule} />
      </div>
    </div>
  );
}
