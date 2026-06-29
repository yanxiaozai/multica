"use client";

import { useEffect, useState } from "react";
import { Globe, Lock, Sparkles, Loader2 } from "lucide-react";
import { useQueryClient } from "@tanstack/react-query";
import { RuntimePicker, isRuntimeUsableForUser } from "./runtime-picker";
import {
  listClaudeAgentFiles,
  parseClaudeAgentFile,
  type ClaudeAgentDefinition,
} from "../../platform";
import { api } from "@multica/core/api";
import { useWorkspaceId } from "@multica/core/hooks";
import { workspaceKeys } from "@multica/core/workspace/queries";
import type {
  AgentVisibility,
  RuntimeDevice,
  MemberWithUser,
  CreateAgentRequest,
} from "@multica/core/types";
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogDescription,
} from "@multica/ui/components/ui/dialog";
import { Button } from "@multica/ui/components/ui/button";
import { Label } from "@multica/ui/components/ui/label";
import { Checkbox } from "@multica/ui/components/ui/checkbox";
import { ScrollArea } from "@multica/ui/components/ui/scroll-area";
import { toast } from "sonner";
import { VISIBILITY_LABEL, VISIBILITY_DESCRIPTION } from "@multica/core/agents";
import { useT } from "../../i18n";

// Batch-import dialog: read every Claude Code sub-agent from
// `~/.claude/agents/` (desktop only), let the user multi-select and pick a
// shared runtime + visibility, then create each as a workspace agent in one
// pass. Reuses RuntimePicker and the visibility selector from the single
// create flow so the affordances match. Web renders never mount this
// component — agents-page gates the entry button on
// isClaudeAgentImportSupported() — but listClaudeAgentFiles() still degrades
// to [] so a stray render is a harmless empty list.
export function ImportClaudeAgentsDialog({
  runtimes,
  runtimesLoading,
  members,
  currentUserId,
  onClose,
}: {
  runtimes: RuntimeDevice[];
  runtimesLoading?: boolean;
  members: MemberWithUser[];
  currentUserId: string | null;
  onClose: () => void;
}) {
  const { t } = useT("agents");
  const qc = useQueryClient();
  const wsId = useWorkspaceId();

  const [loading, setLoading] = useState(true);
  const [definitions, setDefinitions] = useState<ClaudeAgentDefinition[]>([]);
  const [selected, setSelected] = useState<Set<number>>(() => new Set());
  const [visibility, setVisibility] = useState<AgentVisibility>("workspace");
  const [selectedRuntimeId, setSelectedRuntimeId] = useState("");
  const [importing, setImporting] = useState(false);

  // Pull + parse the local agent files once on open. Parsing runs in the
  // renderer (reusing @multica/core/skills/frontmatter) so the main process
  // stays a dumb file reader.
  useEffect(() => {
    let cancelled = false;
    setLoading(true);
    listClaudeAgentFiles()
      .then((files) => {
        if (cancelled) return;
        const parsed = files
          .map(parseClaudeAgentFile)
          // Skip files that resolve to an empty name — they can't seed an
          // agent (the backend rejects name == "").
          .filter((d) => d.name.length > 0);
        setDefinitions(parsed);
      })
      .catch(() => {
        if (cancelled) return;
        setDefinitions([]);
      })
      .finally(() => {
        if (!cancelled) setLoading(false);
      });
    return () => {
      cancelled = true;
    };
  }, []);

  const selectedRuntime = runtimes.find((r) => r.id === selectedRuntimeId) ?? null;
  const selectedRuntimeLocked =
    selectedRuntime != null && !isRuntimeUsableForUser(selectedRuntime, currentUserId);

  const allSelected =
    definitions.length > 0 && selected.size === definitions.length;
  const toggleAll = () => {
    if (allSelected) {
      setSelected(new Set());
    } else {
      setSelected(new Set(definitions.map((_, i) => i)));
    }
  };
  const toggleOne = (index: number) => {
    setSelected((prev) => {
      const next = new Set(prev);
      if (next.has(index)) next.delete(index);
      else next.add(index);
      return next;
    });
  };

  const canImport =
    !importing &&
    selected.size > 0 &&
    !!selectedRuntime &&
    !selectedRuntimeLocked;

  const handleImport = async () => {
    if (!canImport || !selectedRuntime) return;
    setImporting(true);

    const targets = definitions
      .filter((_, i) => selected.has(i))
      // Dedupe by name within the batch so two local files sharing a name
      // don't both attempt a create (the second would 409 anyway).
      .filter(
        (def, _i, arr) =>
          arr.findIndex((other) => other.name === def.name) ===
          arr.indexOf(def),
      );

    let successCount = 0;
    const failures: { name: string; message: string }[] = [];

    for (const def of targets) {
      const data: CreateAgentRequest = {
        name: def.name,
        description: def.description || undefined,
        instructions: def.body || undefined,
        runtime_id: selectedRuntime.id,
        visibility,
        // model intentionally omitted — the user picks a runtime-native model
        // on the detail page after import (Claude's sonnet/opus don't map 1:1
        // to every runtime's model IDs).
      };
      try {
        await api.createAgent(data);
        successCount++;
      } catch (err) {
        const message = err instanceof Error ? err.message : "unknown error";
        failures.push({ name: def.name, message });
      }
    }

    // Refresh the agents list regardless of partial failures so the
    // successfully imported agents show up immediately.
    if (wsId) {
      qc.invalidateQueries({ queryKey: workspaceKeys.agents(wsId) });
    }

    if (successCount > 0) {
      toast.success(
        t(($) => $.import_dialog.toast_success, { count: successCount }),
      );
    }
    if (failures.length > 0) {
      // Surface up to a few names inline; the rest are implied by the count.
      // A 409 (name conflict) is the expected failure mode when re-importing.
      const preview = failures.slice(0, 3).map((f) => f.name).join(", ");
      toast.warning(
        t(($) => $.import_dialog.toast_partial_failure, {
          count: failures.length,
          names: preview,
        }),
      );
    }

    setImporting(false);
    onClose();
  };

  const empty = !loading && definitions.length === 0;

  return (
    <Dialog open onOpenChange={(v) => { if (!v && !importing) onClose(); }}>
      <DialogContent className="p-0 gap-0 flex flex-col overflow-hidden !top-1/2 !left-1/2 !-translate-x-1/2 !-translate-y-1/2 !w-full !max-w-2xl !h-[85vh]">
        <DialogHeader className="border-b px-5 py-3 space-y-0">
          <DialogTitle className="text-base font-semibold flex items-center gap-2">
            <Sparkles className="h-4 w-4 text-primary" />
            {t(($) => $.import_dialog.title)}
          </DialogTitle>
          <DialogDescription className="mt-1 text-xs">
            {t(($) => $.import_dialog.description)}
          </DialogDescription>
        </DialogHeader>

        <div className="flex-1 overflow-y-auto p-5">
          <div className="space-y-4 min-w-0">
            {/* Shared config: one runtime + visibility applies to every
                imported agent. They all originate from Claude Code, so a
                single runtime is the common case. */}
            <RuntimePicker
              runtimes={runtimes}
              runtimesLoading={runtimesLoading}
              members={members}
              currentUserId={currentUserId}
              selectedRuntimeId={selectedRuntimeId}
              onSelect={setSelectedRuntimeId}
            />

            <div>
              <Label className="text-xs text-muted-foreground">
                {t(($) => $.create_dialog.visibility_label)}
              </Label>
              <div className="mt-1.5 flex gap-2">
                <button
                  type="button"
                  onClick={() => setVisibility("workspace")}
                  className={`flex flex-1 items-center gap-2 rounded-lg border px-3 py-2.5 text-sm transition-colors ${
                    visibility === "workspace"
                      ? "border-primary bg-primary/5"
                      : "border-border hover:bg-muted"
                  }`}
                >
                  <Globe className="h-4 w-4 shrink-0 text-muted-foreground" />
                  <div className="text-left">
                    <div className="font-medium">{VISIBILITY_LABEL.workspace}</div>
                    <div className="text-xs text-muted-foreground">
                      {VISIBILITY_DESCRIPTION.workspace}
                    </div>
                  </div>
                </button>
                <button
                  type="button"
                  onClick={() => setVisibility("private")}
                  className={`flex flex-1 items-center gap-2 rounded-lg border px-3 py-2.5 text-sm transition-colors ${
                    visibility === "private"
                      ? "border-primary bg-primary/5"
                      : "border-border hover:bg-muted"
                  }`}
                >
                  <Lock className="h-4 w-4 shrink-0 text-muted-foreground" />
                  <div className="text-left">
                    <div className="font-medium">{VISIBILITY_LABEL.private}</div>
                    <div className="text-xs text-muted-foreground">
                      {VISIBILITY_DESCRIPTION.private}
                    </div>
                  </div>
                </button>
              </div>
            </div>

            {/* Agent list with multi-select. */}
            <div>
              <div className="flex items-center justify-between">
                <Label className="text-xs text-muted-foreground">
                  {t(($) => $.import_dialog.list_label)}
                </Label>
                {!loading && definitions.length > 0 && (
                  <button
                    type="button"
                    onClick={toggleAll}
                    className="text-xs text-primary hover:underline"
                  >
                    {allSelected
                      ? t(($) => $.import_dialog.deselect_all)
                      : t(($) => $.import_dialog.select_all)}
                  </button>
                )}
              </div>

              <div className="mt-1.5 rounded-lg border border-border">
                {loading && (
                  <div className="flex items-center gap-2 p-4 text-sm text-muted-foreground">
                    <Loader2 className="h-4 w-4 animate-spin" />
                    {t(($) => $.import_dialog.loading)}
                  </div>
                )}

                {empty && (
                  <div className="p-4 text-sm text-muted-foreground">
                    {t(($) => $.import_dialog.empty)}
                  </div>
                )}

                {!loading && definitions.length > 0 && (
                  <ScrollArea className="h-[280px]">
                    <ul className="divide-y divide-border">
                      {definitions.map((def, i) => (
                        <li key={`${def.name}-${i}`}>
                          <label className="flex cursor-pointer items-start gap-3 px-3 py-2.5 hover:bg-muted/50">
                            <Checkbox
                              checked={selected.has(i)}
                              onCheckedChange={() => toggleOne(i)}
                              className="mt-0.5"
                            />
                            <div className="min-w-0 flex-1">
                              <div className="truncate text-sm font-medium">
                                {def.name}
                              </div>
                              {def.description && (
                                <div className="mt-0.5 line-clamp-2 text-xs text-muted-foreground">
                                  {def.description}
                                </div>
                              )}
                              {def.body && (
                                <div className="mt-0.5 text-[11px] text-muted-foreground/70">
                                  {t(($) => $.import_dialog.body_chars, {
                                    count: def.body.length,
                                  })}
                                </div>
                              )}
                            </div>
                          </label>
                        </li>
                      ))}
                    </ul>
                  </ScrollArea>
                )}
              </div>
            </div>
          </div>
        </div>

        <div className="flex items-center justify-between gap-2 border-t bg-background px-5 py-3">
          <div className="text-xs text-muted-foreground">
            {selected.size > 0
              ? t(($) => $.import_dialog.selected_count, { count: selected.size })
              : ""}
          </div>
          <div className="flex items-center gap-2">
            <Button
              variant="ghost"
              onClick={onClose}
              disabled={importing}
            >
              {t(($) => $.create_dialog.cancel)}
            </Button>
            <Button
              onClick={handleImport}
              disabled={!canImport}
              title={
                selectedRuntimeLocked
                  ? t(($) => $.create_dialog.runtime_private_locked_tooltip)
                  : undefined
              }
            >
              {importing
                ? t(($) => $.import_dialog.importing)
                : t(($) => $.import_dialog.import, { count: selected.size })}
            </Button>
          </div>
        </div>
      </DialogContent>
    </Dialog>
  );
}
