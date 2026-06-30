/**
 * @vitest-environment jsdom
 */
import { act, renderHook, waitFor } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import type { ReactNode } from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { setApiInstance } from "../api";
import { ApiError, type ApiClient } from "../api/client";
import type { InboxItem } from "../types";
import { useArchiveInbox, useMarkInboxRead } from "./mutations";
import { inboxKeys } from "./queries";

vi.mock("../hooks", () => ({
  useWorkspaceId: () => "ws-1",
}));

const WS_ID = "ws-1";

function makeInboxItem(id: string, overrides: Partial<InboxItem> = {}): InboxItem {
  return {
    id,
    workspace_id: WS_ID,
    recipient_type: "member",
    recipient_id: "member-1",
    type: "new_comment",
    severity: "info",
    issue_id: "issue-1",
    issue_status: "todo",
    title: "Issue",
    body: "Body",
    read: false,
    archived: false,
    created_at: "2026-01-01T00:00:00Z",
    actor_type: null,
    actor_id: null,
    details: {},
    ...overrides,
  };
}

function createWrapper(qc: QueryClient) {
  return function Wrapper({ children }: { children: ReactNode }) {
    return <QueryClientProvider client={qc}>{children}</QueryClientProvider>;
  };
}

describe("inbox mutations", () => {
  let qc: QueryClient;

  beforeEach(() => {
    qc = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  });

  it("removes a stale item when mark-read returns 404", async () => {
    const markInboxRead = vi.fn().mockRejectedValue(
      new ApiError("inbox item not found", 404, "Not Found"),
    );
    setApiInstance({ markInboxRead } as unknown as ApiClient);
    qc.setQueryData<InboxItem[]>(inboxKeys.list(WS_ID), [
      makeInboxItem("stale"),
      makeInboxItem("fresh", { issue_id: "issue-2" }),
    ]);

    const { result } = renderHook(() => useMarkInboxRead(), {
      wrapper: createWrapper(qc),
    });

    await act(async () => {
      result.current.mutate("stale");
    });

    await waitFor(() => expect(result.current.isError).toBe(true));
    expect(qc.getQueryData<InboxItem[]>(inboxKeys.list(WS_ID))?.map((i) => i.id)).toEqual([
      "fresh",
    ]);
  });

  it("removes a stale item when archive returns 404", async () => {
    const archiveInbox = vi.fn().mockRejectedValue(
      new ApiError("inbox item not found", 404, "Not Found"),
    );
    setApiInstance({ archiveInbox } as unknown as ApiClient);
    qc.setQueryData<InboxItem[]>(inboxKeys.list(WS_ID), [
      makeInboxItem("stale"),
      makeInboxItem("fresh", { issue_id: "issue-2" }),
    ]);

    const { result } = renderHook(() => useArchiveInbox(), {
      wrapper: createWrapper(qc),
    });

    await act(async () => {
      result.current.mutate("stale");
    });

    await waitFor(() => expect(result.current.isError).toBe(true));
    expect(qc.getQueryData<InboxItem[]>(inboxKeys.list(WS_ID))?.map((i) => i.id)).toEqual([
      "fresh",
    ]);
  });
});
