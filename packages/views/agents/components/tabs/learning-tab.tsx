"use client";

import { useMemo, useState } from "react";
import {
  BookOpenCheck,
  CheckCircle2,
  CircleSlash,
  Lightbulb,
  Loader2,
  Sparkles,
} from "lucide-react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { toast } from "sonner";
import type {
  Agent,
  AgentEvolutionSuggestion,
  AgentLearningReport,
} from "@multica/core/types";
import {
  agentLearningKeys,
  agentLearningReportsOptions,
} from "@multica/core/agents";
import { api } from "@multica/core/api";
import { useWorkspaceId } from "@multica/core/hooks";
import { workspaceKeys } from "@multica/core/workspace/queries";
import { Button } from "@multica/ui/components/ui/button";
import { Switch } from "@multica/ui/components/ui/switch";
import { Skeleton } from "@multica/ui/components/ui/skeleton";
import { useT } from "../../../i18n";

interface LearningTabProps {
  agent: Agent;
  canEdit: boolean;
  onUpdate: (id: string, data: Record<string, unknown>) => Promise<void>;
}

export function LearningTab({ agent, canEdit, onUpdate }: LearningTabProps) {
  const { t } = useT("agents");
  const wsId = useWorkspaceId();
  const qc = useQueryClient();
  const [applyingId, setApplyingId] = useState<string | null>(null);
  const [toggling, setToggling] = useState(false);
  const reportsQuery = useQuery(agentLearningReportsOptions(wsId, agent.id));
  const reports = reportsQuery.data ?? [];
  const pendingCount = useMemo(
    () =>
      reports.reduce(
        (sum, report) =>
          sum + report.suggestions.filter((s) => s.status === "pending").length,
        0,
      ),
    [reports],
  );

  const refreshLearning = () => {
    qc.invalidateQueries({ queryKey: agentLearningKeys.reports(wsId, agent.id) });
    qc.invalidateQueries({ queryKey: workspaceKeys.agents(wsId) });
    qc.invalidateQueries({ queryKey: workspaceKeys.skills(wsId) });
  };

  const toggleAutoEvolution = async (enabled: boolean) => {
    setToggling(true);
    try {
      await onUpdate(agent.id, { agent_evolution_enabled: enabled });
    } finally {
      setToggling(false);
    }
  };

  const applySuggestion = async (suggestion: AgentEvolutionSuggestion) => {
    setApplyingId(suggestion.id);
    try {
      await api.applyAgentEvolutionSuggestion(suggestion.id);
      refreshLearning();
      toast.success(t(($) => $.tab_body.learning.apply_success_toast));
    } catch (e) {
      toast.error(
        e instanceof Error
          ? e.message
          : t(($) => $.tab_body.learning.apply_failed_toast),
      );
    } finally {
      setApplyingId(null);
    }
  };

  return (
    <div className="space-y-4">
      <div className="flex flex-col gap-3 rounded-lg border p-4 sm:flex-row sm:items-center sm:justify-between">
        <div className="flex min-w-0 items-start gap-3">
          <div className="flex h-9 w-9 shrink-0 items-center justify-center rounded-md bg-primary/10 text-primary">
            <Sparkles className="h-4 w-4" />
          </div>
          <div className="min-w-0">
            <div className="text-sm font-medium">
              {t(($) => $.tab_body.learning.auto_title)}
            </div>
            <p className="mt-1 text-xs leading-5 text-muted-foreground">
              {t(($) => $.tab_body.learning.auto_hint)}
            </p>
          </div>
        </div>
        <Switch
          checked={agent.agent_evolution_enabled === true}
          onCheckedChange={toggleAutoEvolution}
          disabled={!canEdit || toggling}
          aria-label={t(($) => $.tab_body.learning.auto_title)}
        />
      </div>

      <div className="flex items-center justify-between gap-3">
        <div>
          <h2 className="text-sm font-medium">
            {t(($) => $.tab_body.learning.reports_title)}
          </h2>
          <p className="mt-1 text-xs text-muted-foreground">
            {t(($) => $.tab_body.learning.reports_hint, { count: pendingCount })}
          </p>
        </div>
        <Button
          type="button"
          variant="outline"
          size="sm"
          onClick={() => reportsQuery.refetch()}
          disabled={reportsQuery.isFetching}
        >
          {reportsQuery.isFetching && <Loader2 className="h-3.5 w-3.5 animate-spin" />}
          {t(($) => $.tab_body.learning.refresh)}
        </Button>
      </div>

      {reportsQuery.isLoading ? (
        <LearningSkeleton />
      ) : reports.length === 0 ? (
        <div className="flex flex-col items-center justify-center rounded-lg border border-dashed py-12 text-center">
          <BookOpenCheck className="h-8 w-8 text-muted-foreground/40" />
          <p className="mt-3 text-sm text-muted-foreground">
            {t(($) => $.tab_body.learning.empty_title)}
          </p>
          <p className="mt-1 max-w-sm text-xs leading-5 text-muted-foreground">
            {t(($) => $.tab_body.learning.empty_hint)}
          </p>
        </div>
      ) : (
        <div className="space-y-3">
          {reports.map((report) => (
            <LearningReportItem
              key={report.id}
              report={report}
              canEdit={canEdit}
              applyingId={applyingId}
              onApply={applySuggestion}
            />
          ))}
        </div>
      )}
    </div>
  );
}

function LearningReportItem({
  report,
  canEdit,
  applyingId,
  onApply,
}: {
  report: AgentLearningReport;
  canEdit: boolean;
  applyingId: string | null;
  onApply: (suggestion: AgentEvolutionSuggestion) => void;
}) {
  const { t } = useT("agents");

  return (
    <section className="rounded-lg border">
      <div className="border-b px-4 py-3">
        <div className="flex items-center justify-between gap-3">
          <div className="min-w-0">
            <div className="text-sm font-medium">
              {report.summary || t(($) => $.tab_body.learning.report_fallback)}
            </div>
            <div className="mt-1 text-xs text-muted-foreground">
              {formatDateTime(report.created_at)}
            </div>
          </div>
          <span className="shrink-0 rounded-md border px-2 py-1 text-xs text-muted-foreground">
            {t(($) => $.tab_body.learning.suggestion_count, {
              count: report.suggestions.length,
            })}
          </span>
        </div>
      </div>
      <div className="divide-y">
        {report.suggestions.length === 0 ? (
          <div className="px-4 py-5 text-xs text-muted-foreground">
            {t(($) => $.tab_body.learning.no_suggestions)}
          </div>
        ) : (
          report.suggestions.map((suggestion) => (
            <SuggestionRow
              key={suggestion.id}
              suggestion={suggestion}
              canEdit={canEdit}
              applying={applyingId === suggestion.id}
              onApply={() => onApply(suggestion)}
            />
          ))
        )}
      </div>
    </section>
  );
}

function SuggestionRow({
  suggestion,
  canEdit,
  applying,
  onApply,
}: {
  suggestion: AgentEvolutionSuggestion;
  canEdit: boolean;
  applying: boolean;
  onApply: () => void;
}) {
  const { t } = useT("agents");
  const actionable =
    canEdit &&
    suggestion.status === "pending" &&
    suggestion.risk !== "manual" &&
    suggestion.scope !== "builtin_skill_candidate";

  return (
    <div className="flex flex-col gap-3 px-4 py-3 lg:flex-row lg:items-start">
      <div className="flex min-w-0 flex-1 gap-3">
        <SuggestionIcon suggestion={suggestion} />
        <div className="min-w-0 flex-1">
          <div className="flex flex-wrap items-center gap-2">
            <span className="text-sm font-medium">
              {suggestion.title || t(($) => $.tab_body.learning.suggestion_fallback)}
            </span>
            <Pill>{t(($) => $.tab_body.learning.scope[suggestion.scope])}</Pill>
            <Pill>{t(($) => $.tab_body.learning.risk[suggestion.risk])}</Pill>
            <Pill>{t(($) => $.tab_body.learning.status[suggestion.status])}</Pill>
          </div>
          {suggestion.rationale && (
            <p className="mt-1 text-xs leading-5 text-muted-foreground">
              {suggestion.rationale}
            </p>
          )}
          <pre className="mt-2 max-h-28 overflow-auto whitespace-pre-wrap rounded-md bg-muted px-3 py-2 text-xs leading-5 text-muted-foreground">
            {suggestion.proposed_content}
          </pre>
        </div>
      </div>
      <Button
        type="button"
        size="sm"
        variant={suggestion.status === "applied" ? "outline" : "default"}
        disabled={!actionable || applying}
        onClick={onApply}
        className="self-start"
      >
        {applying && <Loader2 className="h-3.5 w-3.5 animate-spin" />}
        {buttonLabel(t, suggestion)}
      </Button>
    </div>
  );
}

function SuggestionIcon({ suggestion }: { suggestion: AgentEvolutionSuggestion }) {
  if (suggestion.status === "applied") {
    return <CheckCircle2 className="mt-0.5 h-4 w-4 shrink-0 text-emerald-600" />;
  }
  if (suggestion.risk === "manual" || suggestion.scope === "builtin_skill_candidate") {
    return <CircleSlash className="mt-0.5 h-4 w-4 shrink-0 text-muted-foreground" />;
  }
  return <Lightbulb className="mt-0.5 h-4 w-4 shrink-0 text-amber-600" />;
}

function Pill({ children }: { children: React.ReactNode }) {
  return (
    <span className="rounded-md border bg-background px-1.5 py-0.5 text-[11px] text-muted-foreground">
      {children}
    </span>
  );
}

function buttonLabel(
  t: ReturnType<typeof useT<"agents">>["t"],
  suggestion: AgentEvolutionSuggestion,
) {
  if (suggestion.status === "applied") {
    return t(($) => $.tab_body.learning.applied_action);
  }
  if (suggestion.status === "dismissed") {
    return t(($) => $.tab_body.learning.dismissed_action);
  }
  if (suggestion.risk === "manual") {
    return t(($) => $.tab_body.learning.manual_action);
  }
  if (suggestion.scope === "builtin_skill_candidate") {
    return t(($) => $.tab_body.learning.builtin_action);
  }
  if (suggestion.scope === "workspace_skill") {
    return t(($) => $.tab_body.learning.apply_skill_action);
  }
  return t(($) => $.tab_body.learning.apply_agent_action);
}

function formatDateTime(value: string) {
  if (!value) return "";
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return value;
  return date.toLocaleString();
}

function LearningSkeleton() {
  return (
    <div className="space-y-3">
      {[0, 1].map((i) => (
        <div key={i} className="rounded-lg border p-4">
          <Skeleton className="h-4 w-48" />
          <Skeleton className="mt-3 h-3 w-2/3" />
          <Skeleton className="mt-4 h-20 w-full" />
        </div>
      ))}
    </div>
  );
}
