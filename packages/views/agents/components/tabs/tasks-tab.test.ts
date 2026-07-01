import { describe, expect, it } from "vitest";
import type { AgentTask } from "@multica/core/types";
import { prepareAgentTasksForDisplay } from "./tasks-tab";

const BASE_TIME = new Date("2026-06-01T12:00:00Z").getTime();

function task(overrides: Partial<AgentTask>): AgentTask {
  return {
    id: "task",
    agent_id: "agent-1",
    runtime_id: "runtime-1",
    issue_id: "",
    status: "completed",
    priority: 0,
    dispatched_at: null,
    started_at: null,
    completed_at: null,
    result: null,
    error: null,
    created_at: new Date(BASE_TIME).toISOString(),
    ...overrides,
  };
}

describe("prepareAgentTasksForDisplay", () => {
  it("keeps task runs that are not linked to issues", () => {
    const tasks = [
      task({ id: "chat", chat_session_id: "chat-1" }),
      task({ id: "autopilot", autopilot_run_id: "auto-1" }),
      task({ id: "quick", kind: "quick_create" }),
      task({ id: "issue", issue_id: "issue-1" }),
    ];

    expect(prepareAgentTasksForDisplay(tasks).map((t) => t.id)).toEqual([
      "chat",
      "autopilot",
      "quick",
      "issue",
    ]);
  });

  it("sorts newest activity first using completed, started, then created time", () => {
    const tasks = [
      task({
        id: "created",
        created_at: new Date(BASE_TIME + 30_000).toISOString(),
      }),
      task({
        id: "started",
        started_at: new Date(BASE_TIME + 60_000).toISOString(),
      }),
      task({
        id: "completed",
        completed_at: new Date(BASE_TIME + 90_000).toISOString(),
      }),
    ];

    expect(prepareAgentTasksForDisplay(tasks).map((t) => t.id)).toEqual([
      "completed",
      "started",
      "created",
    ]);
  });
});
