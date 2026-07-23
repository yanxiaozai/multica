"use client";

import { useMemo, useState } from "react";
import {
  CircleHelp,
  Hash,
  MessageSquare,
  Search,
  Sparkles,
  Workflow,
} from "lucide-react";
import { useQueries, useQuery } from "@tanstack/react-query";
import type { Agent, AgentTask, Issue } from "@multica/core/types";
import { agentTasksOptions } from "@multica/core/agents";
import { useWorkspaceId } from "@multica/core/hooks";
import { useWorkspacePaths } from "@multica/core/paths";
import { issueDetailOptions } from "@multica/core/issues/queries";
import { Button } from "@multica/ui/components/ui/button";
import { Input } from "@multica/ui/components/ui/input";
import { Skeleton } from "@multica/ui/components/ui/skeleton";
import { AppLink } from "../../../navigation";
import { TranscriptButton } from "../../../common/task-transcript";
import { taskStatusConfig } from "../../config";
import { useT, useTimeAgo } from "../../../i18n";

const INITIAL_LIMIT = 40;
const PAGE_SIZE = 40;

type TaskRunFilter = "all" | "active" | "finished";

const ACTIVE_STATUSES = new Set<AgentTask["status"]>([
  "queued",
  "dispatched",
  "waiting_local_directory",
  "running",
]);

const FINISHED_STATUSES = new Set<AgentTask["status"]>([
  "completed",
  "failed",
  "cancelled",
]);

interface TasksTabProps {
  agent: Agent;
}

export function TasksTab({ agent }: TasksTabProps) {
  const { t } = useT("agents");
  const wsId = useWorkspaceId();
  const [search, setSearch] = useState("");
  const [filter, setFilter] = useState<TaskRunFilter>("all");
  const [limit, setLimit] = useState(INITIAL_LIMIT);

  const { data: tasks = [], isLoading } = useQuery(
    agentTasksOptions(wsId, agent.id),
  );

  const sortedTasks = useMemo(() => prepareAgentTasksForDisplay(tasks), [tasks]);
  const filteredTasks = useMemo(
    () => filterTaskRuns(sortedTasks, filter, search),
    [sortedTasks, filter, search],
  );
  const visibleTasks = filteredTasks.slice(0, limit);
  const hasMore = filteredTasks.length > visibleTasks.length;

  const issueIds = useMemo(
    () =>
      Array.from(
        new Set(visibleTasks.map((task) => task.issue_id).filter(Boolean)),
      ),
    [visibleTasks],
  );
  const issueQueries = useQueries({
    queries: issueIds.map((id) => issueDetailOptions(wsId, id)),
  });
  const issueMap = useMemo(() => {
    const map = new Map<string, Issue>();
    issueQueries.forEach((query, index) => {
      const id = issueIds[index];
      if (id && query.data) map.set(id, query.data);
    });
    return map;
  }, [issueIds, issueQueries]);

  if (isLoading) {
    return <TasksTabSkeleton />;
  }

  return (
    <div className="flex h-full min-h-0 flex-col">
      <div className="flex shrink-0 flex-wrap items-center justify-between gap-3 border-b px-4 py-3">
        <div className="relative min-w-0 flex-1 sm:max-w-sm">
          <Search className="pointer-events-none absolute left-2.5 top-1/2 h-3.5 w-3.5 -translate-y-1/2 text-muted-foreground" />
          <Input
            value={search}
            onChange={(event) => {
              setSearch(event.target.value);
              setLimit(INITIAL_LIMIT);
            }}
            placeholder={t(($) => $.tab_body.tasks.search_placeholder)}
            className="h-8 pl-8 text-sm"
          />
        </div>
        <div className="flex items-center gap-1">
          {(["all", "active", "finished"] as const).map((value) => (
            <Button
              key={value}
              type="button"
              variant="outline"
              size="sm"
              className={
                filter === value
                  ? "bg-accent text-accent-foreground hover:bg-accent/80"
                  : "text-muted-foreground"
              }
              onClick={() => {
                setFilter(value);
                setLimit(INITIAL_LIMIT);
              }}
            >
              {t(($) => $.tab_body.tasks.filter[value])}
            </Button>
          ))}
        </div>
      </div>

      {sortedTasks.length === 0 ? (
        <EmptyState
          title={t(($) => $.tab_body.tasks.empty_title)}
          description={t(($) => $.tab_body.tasks.empty_description)}
        />
      ) : filteredTasks.length === 0 ? (
        <EmptyState
          title={t(($) => $.tab_body.tasks.no_match_title)}
          description={t(($) => $.tab_body.tasks.no_match_description)}
        />
      ) : (
        <div className="flex-1 min-h-0 overflow-y-auto p-3">
          <div className="space-y-1.5">
            {visibleTasks.map((task) => (
              <TaskRunRow
                key={task.id}
                agent={agent}
                task={task}
                issue={task.issue_id ? issueMap.get(task.issue_id) : undefined}
              />
            ))}
          </div>
          {hasMore && (
            <button
              type="button"
              onClick={() => setLimit((value) => value + PAGE_SIZE)}
              className="mt-3 rounded text-xs text-muted-foreground transition-colors hover:text-foreground"
            >
              {t(($) => $.tab_body.tasks.show_more, {
                shown: visibleTasks.length,
                total: filteredTasks.length,
              })}
            </button>
          )}
        </div>
      )}
    </div>
  );
}

export function prepareAgentTasksForDisplay(tasks: readonly AgentTask[]): AgentTask[] {
  return [...tasks].sort((a, b) => taskActivityTime(b) - taskActivityTime(a));
}

function filterTaskRuns(
  tasks: readonly AgentTask[],
  filter: TaskRunFilter,
  search: string,
): AgentTask[] {
  const query = search.trim().toLowerCase();
  return tasks.filter((task) => {
    if (filter === "active" && !ACTIVE_STATUSES.has(task.status)) return false;
    if (filter === "finished" && !FINISHED_STATUSES.has(task.status)) return false;
    if (!query) return true;
    return [
      task.id,
      task.issue_id,
      task.status,
      task.kind ?? "",
      task.trigger_summary ?? "",
      task.chat_session_id ? "chat" : "",
      task.autopilot_run_id ? "autopilot" : "",
    ]
      .join(" ")
      .toLowerCase()
      .includes(query);
  });
}

function taskActivityTime(task: AgentTask): number {
  const raw = task.completed_at ?? task.started_at ?? task.dispatched_at ?? task.created_at;
  const parsed = Date.parse(raw);
  return Number.isFinite(parsed) ? parsed : 0;
}

function TaskRunRow({
  agent,
  task,
  issue,
}: {
  agent: Agent;
  task: AgentTask;
  issue: Issue | undefined;
}) {
  const { t } = useT("agents");
  const paths = useWorkspacePaths();
  const timeAgo = useTimeAgo();
  const cfg = taskStatusConfig[task.status] ?? taskStatusConfig.queued!;
  const StatusIcon = cfg.icon;
  const source = taskSource(task);
  const SourceIcon = source.icon;
  const hasIssue = task.issue_id !== "";
  const isRunning = task.status === "running";
  const showTranscript =
    Boolean(task.started_at) ||
    task.status === "completed" ||
    task.status === "failed" ||
    task.status === "cancelled";
  const title =
    issue?.title ??
    source.fallback(t) ??
    (hasIssue
      ? t(($) => $.tab_body.activity.issue_short_fallback, {
          prefix: task.issue_id.slice(0, 8),
        })
      : t(($) => $.tab_body.activity.source_untracked));

  return (
    <div className="group flex items-center gap-3 rounded-md border px-3 py-2.5">
      <StatusIcon
        className={`h-4 w-4 shrink-0 ${cfg.color} ${
          isRunning ? "animate-spin" : ""
        }`}
      />
      <div className="min-w-0 flex-1">
        <div className="flex min-w-0 items-center gap-1.5">
          <SourceIcon
            className="h-3.5 w-3.5 shrink-0 text-muted-foreground/70"
            aria-label={source.label(t)}
          />
          {issue && (
            <span className="shrink-0 font-mono text-xs text-muted-foreground">
              {issue.identifier}
            </span>
          )}
          <span className="truncate text-sm">{title}</span>
        </div>
        <div className="mt-0.5 flex flex-wrap items-center gap-x-2 gap-y-0.5 text-xs text-muted-foreground">
          <span>{t(($) => $.tab_body.tasks.status[task.status])}</span>
          <Sep />
          <span>{taskTimeText(task, t, timeAgo)}</span>
          {task.trigger_summary && (
            <>
              <Sep />
              <span className="truncate">{task.trigger_summary}</span>
            </>
          )}
        </div>
      </div>
      <div className="ml-2 flex shrink-0 items-center gap-0.5 opacity-0 transition-opacity duration-100 group-hover:opacity-100 group-focus-within:opacity-100">
        {hasIssue && (
          <AppLink
            href={paths.issueDetail(task.issue_id)}
            aria-label={t(($) => $.tab_body.activity.open_issue_aria)}
            className="rounded px-2 py-1 text-xs text-muted-foreground transition-colors hover:bg-accent/50 hover:text-foreground"
          >
            {t(($) => $.tab_body.tasks.open_issue)}
          </AppLink>
        )}
        {showTranscript && (
          <TranscriptButton
            task={task}
            agentName={agent.name}
            isLive={isRunning}
            title={t(($) => $.tab_body.activity.transcript_tooltip)}
          />
        )}
      </div>
    </div>
  );
}

type AgentsT = ReturnType<typeof useT<"agents">>["t"];
type TimeAgoFn = (dateStr: string) => string;

function taskTimeText(task: AgentTask, t: AgentsT, timeAgo: TimeAgoFn): string {
  if (task.completed_at) {
    return t(($) => $.tab_body.tasks.completed_prefix, {
      when: timeAgo(task.completed_at!),
    });
  }
  if (task.started_at) {
    return t(($) => $.tab_body.activity.started_prefix, {
      when: timeAgo(task.started_at!),
    });
  }
  if (task.dispatched_at) {
    return t(($) => $.tab_body.activity.dispatched_prefix, {
      when: timeAgo(task.dispatched_at!),
    });
  }
  return t(($) => $.tab_body.tasks.created_prefix, {
    when: timeAgo(task.created_at),
  });
}

function taskSource(task: AgentTask): {
  icon: typeof Hash;
  label: (t: AgentsT) => string;
  fallback: (t: AgentsT) => string | null;
} {
  if (task.issue_id) {
    return {
      icon: Hash,
      label: (t) => t(($) => $.tab_body.activity.source_issue),
      fallback: () => null,
    };
  }
  if (task.kind === "squad_instructions_generation") {
    return {
      icon: Sparkles,
      label: (t) =>
        t(($) => $.tab_body.activity.source_squad_instructions_generation_short),
      fallback: (t) =>
        t(($) => $.tab_body.activity.source_squad_instructions_generation),
    };
  }
  if (task.kind === "quick_create") {
    return {
      icon: Sparkles,
      label: (t) => t(($) => $.tab_body.activity.source_quick_create),
      fallback: (t) => t(($) => $.tab_body.activity.source_quick_create),
    };
  }
  if (task.chat_session_id) {
    return {
      icon: MessageSquare,
      label: (t) => t(($) => $.tab_body.activity.source_chat),
      fallback: (t) => t(($) => $.tab_body.activity.source_chat_session),
    };
  }
  if (task.autopilot_run_id) {
    return {
      icon: Workflow,
      label: (t) => t(($) => $.tab_body.activity.source_autopilot),
      fallback: (t) => t(($) => $.tab_body.activity.source_autopilot_run),
    };
  }
  return {
    icon: CircleHelp,
    label: (t) => t(($) => $.tab_body.activity.source_untracked),
    fallback: (t) => t(($) => $.tab_body.activity.source_untracked),
  };
}

function EmptyState({
  title,
  description,
}: {
  title: string;
  description: string;
}) {
  return (
    <div className="flex flex-1 min-h-0 flex-col items-center justify-center gap-2 px-6 py-16 text-center text-muted-foreground">
      <CircleHelp className="h-10 w-10 text-muted-foreground/40" />
      <p className="text-sm">{title}</p>
      <p className="max-w-sm text-xs">{description}</p>
    </div>
  );
}

function TasksTabSkeleton() {
  return (
    <div className="flex h-full min-h-0 flex-col">
      <div className="flex h-14 shrink-0 items-center justify-between border-b px-4">
        <Skeleton className="h-8 w-64 rounded-md" />
        <div className="flex gap-1">
          <Skeleton className="h-8 w-16 rounded-md" />
          <Skeleton className="h-8 w-16 rounded-md" />
          <Skeleton className="h-8 w-20 rounded-md" />
        </div>
      </div>
      <div className="flex flex-1 flex-col gap-2 p-3">
        {Array.from({ length: 8 }).map((_, index) => (
          <Skeleton key={index} className="h-14 rounded-md" />
        ))}
      </div>
    </div>
  );
}

function Sep() {
  return <span className="text-muted-foreground/40">·</span>;
}
