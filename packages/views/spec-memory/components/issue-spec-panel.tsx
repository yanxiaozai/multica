"use client";

import { useEffect, useMemo, useState } from "react";
import { Link2, Save, ShieldCheck } from "lucide-react";
import { useQuery } from "@tanstack/react-query";
import {
  issueSpecOptions,
  specEpicListOptions,
  specModuleListOptions,
  useUpdateIssueSpecMapping,
  useUpdateIssueSpecState,
} from "@multica/core/spec-memory";
import { useWorkspaceId } from "@multica/core/hooks";
import type {
  SpecEpic,
  SpecIssueState,
  SpecModule,
  UpdateIssueSpecStateRequest,
} from "@multica/core/types";
import { Button } from "@multica/ui/components/ui/button";
import { Checkbox } from "@multica/ui/components/ui/checkbox";
import { Input } from "@multica/ui/components/ui/input";
import { Label } from "@multica/ui/components/ui/label";
import {
  NativeSelect,
  NativeSelectOption,
} from "@multica/ui/components/ui/native-select";
import { Skeleton } from "@multica/ui/components/ui/skeleton";
import { Textarea } from "@multica/ui/components/ui/textarea";
import { cn } from "@multica/ui/lib/utils";
import { toast } from "sonner";

const emptyState: UpdateIssueSpecStateRequest = {
  status: "",
  owner: "",
  current_stage: "",
  current_loop: "",
  last_result: "",
  open_questions: [],
  blockers: [],
  next_handoff: "",
  audit_mode: "default",
  audit_skipped: false,
  audit_skip_reason: "",
  audit_skipped_by: "",
};

function linesToArray(value: string): string[] {
  return value
    .split("\n")
    .map((line) => line.trim())
    .filter(Boolean);
}

function arrayToLines(value: string[] | undefined): string {
  return (value ?? []).join("\n");
}

function toDraft(state: SpecIssueState | null): UpdateIssueSpecStateRequest {
  if (!state) return emptyState;
  return {
    status: state.status,
    owner: state.owner,
    current_stage: state.current_stage,
    current_loop: state.current_loop,
    last_result: state.last_result,
    open_questions: state.open_questions,
    blockers: state.blockers,
    next_handoff: state.next_handoff,
    audit_mode: state.audit_mode,
    audit_skipped: state.audit_skipped,
    audit_skip_reason: state.audit_skip_reason,
    audit_skipped_by: state.audit_skipped_by,
  };
}

export function IssueSpecPanel({
  issueId,
  className,
}: {
  issueId: string;
  className?: string;
}) {
  const wsId = useWorkspaceId();
  const issueSpec = useQuery(issueSpecOptions(wsId, issueId));
  const epics = useQuery(specEpicListOptions(wsId));
  const [selectedEpicId, setSelectedEpicId] = useState("");
  const modules = useQuery(specModuleListOptions(wsId, selectedEpicId));
  const updateMapping = useUpdateIssueSpecMapping(issueId);
  const updateState = useUpdateIssueSpecState(issueId);

  const primary = issueSpec.data?.mappings.find(
    (mapping) => mapping.mapping_kind === "primary",
  );
  const initialModuleId = primary?.module_id ?? "";
  const initialEpicId = primary?.epic_id ?? "";

  const [moduleId, setModuleId] = useState("");
  const [mappingReason, setMappingReason] = useState("");
  const [draft, setDraft] =
    useState<UpdateIssueSpecStateRequest>(emptyState);
  const [openQuestions, setOpenQuestions] = useState("");
  const [blockers, setBlockers] = useState("");

  useEffect(() => {
    if (initialEpicId) setSelectedEpicId(initialEpicId);
  }, [initialEpicId]);

  useEffect(() => {
    setModuleId(initialModuleId);
    setMappingReason(primary?.reason ?? "");
  }, [initialModuleId, primary?.reason]);

  useEffect(() => {
    const next = toDraft(issueSpec.data?.state ?? null);
    setDraft(next);
    setOpenQuestions(arrayToLines(next.open_questions));
    setBlockers(arrayToLines(next.blockers));
  }, [issueSpec.data?.state]);

  const selectedEpic = useMemo(
    () => epics.data?.find((epic) => epic.id === selectedEpicId) ?? null,
    [epics.data, selectedEpicId],
  );
  const selectedModule = useMemo(
    () => modules.data?.find((module) => module.id === moduleId) ?? null,
    [modules.data, moduleId],
  );

  const saveMapping = () => {
    if (!selectedEpicId && !moduleId) return;
    updateMapping.mutate(
      {
        primary: {
          epic_id: selectedEpicId || selectedModule?.epic_id || null,
          module_id: moduleId || null,
          reason: mappingReason,
        },
      },
      {
        onSuccess: () => toast.success("Spec mapping saved"),
        onError: (error) =>
          toast.error(error instanceof Error ? error.message : "Save failed"),
      },
    );
  };

  const saveState = () => {
    updateState.mutate(
      {
        ...draft,
        open_questions: linesToArray(openQuestions),
        blockers: linesToArray(blockers),
      },
      {
        onSuccess: () => toast.success("Spec state saved"),
        onError: (error) =>
          toast.error(error instanceof Error ? error.message : "Save failed"),
      },
    );
  };

  if (issueSpec.isLoading) {
    return (
      <div className={cn("space-y-3", className)}>
        <Skeleton className="h-8 w-40" />
        <Skeleton className="h-28 w-full" />
        <Skeleton className="h-40 w-full" />
      </div>
    );
  }

  return (
    <section className={cn("space-y-5", className)}>
      <div className="flex items-center justify-between gap-3">
        <div className="min-w-0">
          <h2 className="truncate text-sm font-semibold">Spec memory</h2>
          <p className="text-xs text-muted-foreground">
            Issue-scoped mapping and execution state.
          </p>
        </div>
      </div>

      <div className="rounded-lg border bg-background p-3">
        <div className="mb-3 flex items-center gap-2 text-xs font-medium">
          <Link2 className="size-3.5" />
          Mapping
        </div>
        <div className="grid gap-3">
          <LabeledSelect
            label="Epic"
            value={selectedEpicId}
            onChange={(value) => {
              setSelectedEpicId(value);
              setModuleId("");
            }}
            items={epics.data ?? []}
            placeholder="No epic"
          />
          <LabeledSelect
            label="Module"
            value={moduleId}
            onChange={setModuleId}
            items={modules.data ?? []}
            placeholder={selectedEpic ? "No module" : "Pick an epic first"}
            disabled={!selectedEpicId}
          />
          <div className="grid gap-1.5">
            <Label className="text-xs">Reason</Label>
            <Input
              value={mappingReason}
              onChange={(event) => setMappingReason(event.target.value)}
              placeholder="Why this issue touches this area"
            />
          </div>
          <Button
            type="button"
            size="sm"
            onClick={saveMapping}
            disabled={updateMapping.isPending || (!selectedEpicId && !moduleId)}
          >
            <Save className="size-3.5" />
            Save mapping
          </Button>
        </div>
      </div>

      <div className="rounded-lg border bg-background p-3">
        <div className="mb-3 flex items-center gap-2 text-xs font-medium">
          <ShieldCheck className="size-3.5" />
          Execution state
        </div>
        <div className="grid gap-3">
          <div className="grid grid-cols-2 gap-3">
            <TextField
              label="Status"
              value={draft.status ?? ""}
              onChange={(status) => setDraft((prev) => ({ ...prev, status }))}
            />
            <TextField
              label="Owner"
              value={draft.owner ?? ""}
              onChange={(owner) => setDraft((prev) => ({ ...prev, owner }))}
            />
            <TextField
              label="Stage"
              value={draft.current_stage ?? ""}
              onChange={(current_stage) =>
                setDraft((prev) => ({ ...prev, current_stage }))
              }
            />
            <TextField
              label="Loop"
              value={draft.current_loop ?? ""}
              onChange={(current_loop) =>
                setDraft((prev) => ({ ...prev, current_loop }))
              }
            />
          </div>
          <TextareaField
            label="Last result"
            value={draft.last_result ?? ""}
            onChange={(last_result) =>
              setDraft((prev) => ({ ...prev, last_result }))
            }
          />
          <TextareaField
            label="Open questions"
            value={openQuestions}
            onChange={setOpenQuestions}
          />
          <TextareaField label="Blockers" value={blockers} onChange={setBlockers} />
          <TextareaField
            label="Next handoff"
            value={draft.next_handoff ?? ""}
            onChange={(next_handoff) =>
              setDraft((prev) => ({ ...prev, next_handoff }))
            }
          />
          <div className="flex items-center gap-2">
            <Checkbox
              checked={!!draft.audit_skipped}
              onCheckedChange={(checked) =>
                setDraft((prev) => ({
                  ...prev,
                  audit_skipped: checked === true,
                }))
              }
            />
            <span className="text-xs text-muted-foreground">Skip audit</span>
          </div>
          {draft.audit_skipped ? (
            <TextField
              label="Skip reason"
              value={draft.audit_skip_reason ?? ""}
              onChange={(audit_skip_reason) =>
                setDraft((prev) => ({ ...prev, audit_skip_reason }))
              }
            />
          ) : null}
          <Button
            type="button"
            size="sm"
            onClick={saveState}
            disabled={updateState.isPending}
          >
            <Save className="size-3.5" />
            Save state
          </Button>
        </div>
      </div>
    </section>
  );
}

function LabeledSelect<T extends SpecEpic | SpecModule>({
  label,
  value,
  onChange,
  items,
  placeholder,
  disabled,
}: {
  label: string;
  value: string;
  onChange: (value: string) => void;
  items: T[];
  placeholder: string;
  disabled?: boolean;
}) {
  return (
    <div className="grid gap-1.5">
      <Label className="text-xs">{label}</Label>
      <NativeSelect
        value={value}
        onChange={(event) => onChange(event.target.value)}
        disabled={disabled}
        className="w-full"
      >
        <NativeSelectOption value="">{placeholder}</NativeSelectOption>
        {items.map((item) => (
          <NativeSelectOption key={item.id} value={item.id}>
            {item.key} · {item.title}
          </NativeSelectOption>
        ))}
      </NativeSelect>
    </div>
  );
}

function TextField({
  label,
  value,
  onChange,
}: {
  label: string;
  value: string;
  onChange: (value: string) => void;
}) {
  return (
    <div className="grid gap-1.5">
      <Label className="text-xs">{label}</Label>
      <Input value={value} onChange={(event) => onChange(event.target.value)} />
    </div>
  );
}

function TextareaField({
  label,
  value,
  onChange,
}: {
  label: string;
  value: string;
  onChange: (value: string) => void;
}) {
  return (
    <div className="grid gap-1.5">
      <Label className="text-xs">{label}</Label>
      <Textarea
        value={value}
        onChange={(event) => onChange(event.target.value)}
        className="min-h-20 resize-y text-sm"
      />
    </div>
  );
}
