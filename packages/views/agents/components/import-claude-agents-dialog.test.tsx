// @vitest-environment jsdom

import "@testing-library/jest-dom/vitest";
import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import type { ReactNode } from "react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen, fireEvent, cleanup, waitFor } from "@testing-library/react";
import type { MemberWithUser, RuntimeDevice } from "@multica/core/types";
import { I18nProvider } from "@multica/core/i18n/react";
import { WorkspaceSlugProvider } from "@multica/core/paths";
import { NavigationProvider, type NavigationAdapter } from "../../navigation";
import enCommon from "../../locales/en/common.json";
import enAgents from "../../locales/en/agents.json";

const navigationStub: NavigationAdapter = {
  push: vi.fn(),
  replace: vi.fn(),
  back: vi.fn(),
  pathname: "/",
  searchParams: new URLSearchParams(),
  getShareableUrl: (path: string) => path,
};

const TEST_RESOURCES = { en: { common: enCommon, agents: enAgents } };

vi.mock("@multica/core/hooks", () => ({
  useWorkspaceId: () => "ws-1",
}));

// Feed the dialog a deterministic set of parsed agent files without going
// through the preload bridge. parseClaudeAgentFile is the real
// implementation so the fixture mirrors production parsing.
vi.mock("../../platform", async () => {
  const actual = await vi.importActual<typeof import("../../platform")>(
    "../../platform",
  );
  return {
    ...actual,
    listClaudeAgentFiles: vi.fn(),
  };
});

// Provider logos + avatars pull in SVGs and hit the api; irrelevant here.
vi.mock("../../runtimes/components/provider-logo", () => ({
  ProviderLogo: () => null,
}));
vi.mock("../../common/actor-avatar", () => ({
  ActorAvatar: () => null,
}));

// Base UI's ScrollArea calls Element.getAnimations(), which jsdom doesn't
// implement — the component fires it from a setTimeout that lands after the
// test resolves and surfaces as an uncaught exception. Render a plain
// passthrough since scroll behaviour isn't under test.
vi.mock("@multica/ui/components/ui/scroll-area", () => ({
  ScrollArea: ({ children }: { children: ReactNode }) => (
    <div>{children}</div>
  ),
}));

// createAgent is referenced inside the vi.mock factory, which vitest hoists
// above top-level consts — define it via vi.hoisted so it exists at factory
// eval time.
const { createAgent } = vi.hoisted(() => ({
  createAgent: vi.fn(),
}));

vi.mock("@multica/core/api", () => ({
  api: { createAgent },
  ApiError: class ApiError extends Error {
    status: number;
    constructor(message: string, status: number) {
      super(message);
      this.status = status;
    }
  },
}));

const { toastSuccess, toastWarning } = vi.hoisted(() => ({
  toastSuccess: vi.fn(),
  toastWarning: vi.fn(),
}));
vi.mock("sonner", () => ({
  toast: { success: toastSuccess, warning: toastWarning, error: vi.fn() },
}));

import { ImportClaudeAgentsDialog } from "./import-claude-agents-dialog";
import { listClaudeAgentFiles } from "../../platform";

const ME = "user-me";

const members: MemberWithUser[] = [
  {
    id: "m-me",
    user_id: ME,
    workspace_id: "ws-1",
    role: "member",
    name: "Me",
    email: "me@example.com",
    avatar_url: null,
    created_at: "2026-01-01T00:00:00Z",
  },
];

function makeRuntime(overrides: Partial<RuntimeDevice>): RuntimeDevice {
  return {
    id: "rt",
    workspace_id: "ws-1",
    daemon_id: null,
    name: "Test Runtime",
    runtime_mode: "local",
    provider: "claude",
    launch_header: "",
    status: "online",
    device_info: "host.local",
    metadata: {},
    owner_id: ME,
    visibility: "private",
    last_seen_at: "2026-04-27T11:59:50Z",
    created_at: "2026-04-01T00:00:00Z",
    updated_at: "2026-04-01T00:00:00Z",
    ...overrides,
  };
}

function agentFile(name: string, description: string, body = "instructions") {
  return {
    fileName: name,
    rawContent:
      `---\nname: ${name}\ndescription: "${description}"\nmodel: sonnet\n---\n${body}`,
  };
}

function renderDialog(runtimes: RuntimeDevice[]) {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  const onClose = vi.fn();
  render(
    <I18nProvider locale="en" resources={TEST_RESOURCES}>
      <QueryClientProvider client={queryClient}>
        <WorkspaceSlugProvider slug="test-ws">
          <NavigationProvider value={navigationStub}>
            <ImportClaudeAgentsDialog
              runtimes={runtimes}
              members={members}
              currentUserId={ME}
              onClose={onClose}
            />
          </NavigationProvider>
        </WorkspaceSlugProvider>
      </QueryClientProvider>
    </I18nProvider>,
  );
  return { onClose };
}

describe("ImportClaudeAgentsDialog", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });
  afterEach(() => {
    cleanup();
    document.body.innerHTML = "";
  });

  it("renders the empty state when no local agents are found", async () => {
    vi.mocked(listClaudeAgentFiles).mockResolvedValue([]);
    renderDialog([makeRuntime({})]);
    expect(await screen.findByText(/No Claude Code agents found/i)).toBeInTheDocument();
  });

  it("lists every parsed local agent", async () => {
    vi.mocked(listClaudeAgentFiles).mockResolvedValue([
      agentFile("shuai", "Java backend"),
      agentFile("han", "Architect"),
    ]);
    renderDialog([makeRuntime({})]);
    expect(await screen.findByText("shuai")).toBeInTheDocument();
    expect(screen.getByText("han")).toBeInTheDocument();
  });

  it("creates every selected agent on import and toasts success", async () => {
    vi.mocked(listClaudeAgentFiles).mockResolvedValue([
      agentFile("shuai", "Java backend"),
      agentFile("han", "Architect"),
    ]);
    // RuntimePicker auto-seeds the first usable runtime (owned by ME).
    renderDialog([makeRuntime({ id: "rt-mine" })]);

    await screen.findByText("shuai");
    // Select both via the "Select all" affordance.
    fireEvent.click(screen.getByText("Select all"));

    const importBtn = screen
      .getAllByRole("button")
      .find((b) => /^Import /.test(b.textContent ?? "")) as HTMLButtonElement;
    expect(importBtn).toBeTruthy();
    fireEvent.click(importBtn);

    await waitFor(() => expect(createAgent).toHaveBeenCalledTimes(2));
    expect(createAgent.mock.calls[0]?.[0]).toMatchObject({
      name: "shuai",
      runtime_id: "rt-mine",
      visibility: "workspace",
    });
    expect(createAgent.mock.calls[1]?.[0]).toMatchObject({
      name: "han",
      instructions: "instructions",
    });
    // model is intentionally NOT forwarded.
    expect(createAgent.mock.calls[0]?.[0].model).toBeUndefined();
    await waitFor(() => expect(toastSuccess).toHaveBeenCalled());
    expect(toastWarning).not.toHaveBeenCalled();
  });

  it("keeps importing the rest when one agent hits a 409 name conflict", async () => {
    vi.mocked(listClaudeAgentFiles).mockResolvedValue([
      agentFile("shuai", "Java backend"),
      agentFile("han", "Architect"),
      agentFile("mini", "Mini"),
    ]);
    // The second create conflicts; the first and third must still land.
    createAgent
      .mockResolvedValueOnce({ id: "a1" })
      .mockRejectedValueOnce(new Error("an agent named \"han\" already exists"))
      .mockResolvedValueOnce({ id: "a3" });

    renderDialog([makeRuntime({ id: "rt-mine" })]);
    await screen.findByText("shuai");
    fireEvent.click(screen.getByText("Select all"));

    const importBtn = screen
      .getAllByRole("button")
      .find((b) => /^Import /.test(b.textContent ?? "")) as HTMLButtonElement;
    fireEvent.click(importBtn);

    await waitFor(() => expect(createAgent).toHaveBeenCalledTimes(3));
    await waitFor(() => expect(toastSuccess).toHaveBeenCalled());
    // Partial failure is surfaced as a warning that names the failed agent.
    expect(toastWarning).toHaveBeenCalled();
  });
});
