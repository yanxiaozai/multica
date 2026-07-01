import { describe, it, expect } from "vitest";
import {
  isActiveTaskStatus,
  pickStageKeys,
  resolveEffectiveTaskStatus,
} from "./task-status-pill";

const patchApplyMessage = [
  {
    id: "msg-1",
    task_id: "task-1",
    issue_id: "issue-1",
    seq: 1,
    type: "tool_use" as const,
    tool: "patch_apply",
    created_at: "2026-07-01T10:01:40Z",
  },
];

describe("pickStageKeys", () => {
  it("returns queued when status is queued and agent is online", () => {
    expect(pickStageKeys("queued", [], "online")).toEqual({ stageKey: "queued" });
  });

  it("returns offline when status is queued and agent is offline", () => {
    expect(pickStageKeys("queued", [], "offline")).toEqual({
      stageKey: "offline",
      static: true,
    });
  });

  it("returns waiting_local_directory on the daemon-emitted hold status", () => {
    // Daemon publishes this when it dequeues a task but another task owns the
    // local_directory's lock. The pill becomes static (no shimmer) because
    // nothing is actively happening from the user's point of view.
    expect(pickStageKeys("waiting_local_directory", [], "online")).toEqual({
      stageKey: "waiting_local_directory",
      static: true,
    });
  });

  it("waiting_local_directory wins over availability hints", () => {
    // Even if availability says reconnecting/offline, the directory-release
    // status is the more specific signal — surface it.
    expect(
      pickStageKeys("waiting_local_directory", [], "unstable"),
    ).toEqual({ stageKey: "waiting_local_directory", static: true });
    expect(
      pickStageKeys("waiting_local_directory", [], "offline"),
    ).toEqual({ stageKey: "waiting_local_directory", static: true });
  });

  it("returns thinking for running with no messages", () => {
    expect(pickStageKeys("running", [], "online")).toEqual({ stageKey: "thinking" });
  });

  it("classifies only non-terminal task statuses as active", () => {
    expect(isActiveTaskStatus("queued")).toBe(true);
    expect(isActiveTaskStatus("dispatched")).toBe(true);
    expect(isActiveTaskStatus("waiting_local_directory")).toBe(true);
    expect(isActiveTaskStatus("running")).toBe(true);
    expect(isActiveTaskStatus("failed")).toBe(false);
    expect(isActiveTaskStatus("completed")).toBe(false);
    expect(isActiveTaskStatus("cancelled")).toBe(false);
    expect(isActiveTaskStatus(undefined)).toBe(false);
  });

  it("upgrades pre-running active statuses when streamed messages arrive", () => {
    expect(resolveEffectiveTaskStatus("queued", patchApplyMessage)).toBe(
      "running",
    );
    expect(resolveEffectiveTaskStatus("dispatched", patchApplyMessage)).toBe(
      "running",
    );
  });

  it("does not let stale streamed tool messages override terminal task status", () => {
    expect(resolveEffectiveTaskStatus("failed", patchApplyMessage)).toBe(
      "failed",
    );
    expect(resolveEffectiveTaskStatus("completed", patchApplyMessage)).toBe(
      "completed",
    );
    expect(resolveEffectiveTaskStatus("cancelled", patchApplyMessage)).toBe(
      "cancelled",
    );
  });
});
