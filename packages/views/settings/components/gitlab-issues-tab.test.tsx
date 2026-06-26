// @vitest-environment jsdom

import type { ReactNode } from "react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { cleanup, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { I18nProvider } from "@multica/core/i18n/react";
import enCommon from "../../locales/en/common.json";
import enSettings from "../../locales/en/settings.json";

type MemberRole = "owner" | "admin" | "member" | "guest";

const mockCreateIntegration = vi.hoisted(() => vi.fn());
const mockUpdateIntegration = vi.hoisted(() => vi.fn());
const mockDeleteIntegration = vi.hoisted(() => vi.fn());
const mockTestIntegration = vi.hoisted(() => vi.fn());
const mockToastSuccess = vi.hoisted(() => vi.fn());
const mockToastError = vi.hoisted(() => vi.fn());

const membersRef = vi.hoisted(() => ({
  current: [{ user_id: "user-1", role: "owner" as MemberRole }],
}));
const integrationsRef = vi.hoisted(() => ({
  current: [] as Array<{
    id: string;
    name: string;
    base_url: string;
    default_issue_skill_id: string | null;
    polling_enabled: boolean;
    default_poll_interval_seconds: number;
    config?: Record<string, unknown>;
  }>,
}));
const syncConfigsRef = vi.hoisted(() => ({
  current: [] as Array<{
    id: string;
    integration_id: string;
    remote_project_ref: string;
    scope_type: string;
    sync_enabled: boolean;
  }>,
}));
const skillsRef = vi.hoisted(() => ({
  current: [
    { id: "skill-issue", name: "issue-creator" },
    { id: "skill-other", name: "other-skill" },
  ],
  isError: false,
}));

vi.mock("@tanstack/react-query", () => ({
  useQuery: (opts: { queryKey: unknown[] }) => {
    const key = JSON.stringify(opts.queryKey);
    if (key.includes("members")) return { data: membersRef.current };
    if (key.includes("integrations")) {
      return { data: { integrations: integrationsRef.current, total: integrationsRef.current.length } };
    }
    if (key.includes("sync-configs")) {
      return { data: { sync_configs: syncConfigsRef.current, total: syncConfigsRef.current.length } };
    }
    if (key.includes("skills")) {
      return { data: skillsRef.current, isError: skillsRef.isError };
    }
    return { data: undefined };
  },
  queryOptions: <T,>(opts: T) => opts,
}));

vi.mock("@multica/core/hooks", () => ({
  useWorkspaceId: () => "workspace-1",
}));

vi.mock("@multica/core/auth", () => {
  const useAuthStore = Object.assign(
    (sel?: (s: { user: { id: string } }) => unknown) =>
      sel ? sel({ user: { id: "user-1" } }) : { user: { id: "user-1" } },
    { getState: () => ({ user: { id: "user-1" } }) },
  );
  return { useAuthStore };
});

vi.mock("@multica/core/workspace/queries", () => ({
  memberListOptions: () => ({ queryKey: ["members"], queryFn: vi.fn() }),
  skillListOptions: () => ({ queryKey: ["skills"], queryFn: vi.fn() }),
}));

vi.mock("@multica/core/issue-bridge", () => ({
  issueIntegrationsOptions: () => ({
    queryKey: ["issue-bridge", "integrations"],
    queryFn: vi.fn(),
  }),
  issueSyncConfigsOptions: () => ({
    queryKey: ["issue-bridge", "sync-configs"],
    queryFn: vi.fn(),
  }),
  useCreateGitLabIssueIntegration: () => ({
    isPending: false,
    mutateAsync: mockCreateIntegration,
  }),
  useUpdateIssueIntegration: () => ({
    isPending: false,
    mutateAsync: mockUpdateIntegration,
  }),
  useDeleteIssueIntegration: () => ({
    isPending: false,
    mutateAsync: mockDeleteIntegration,
  }),
  useTestIssueIntegration: () => ({
    isPending: false,
    mutateAsync: mockTestIntegration,
  }),
}));

vi.mock("sonner", () => ({
  toast: {
    success: mockToastSuccess,
    error: mockToastError,
  },
}));

import { GitLabIssuesTab } from "./gitlab-issues-tab";

const TEST_RESOURCES = {
  en: { common: enCommon, settings: enSettings },
};

function I18nWrapper({ children }: { children: ReactNode }) {
  return (
    <I18nProvider locale="en" resources={TEST_RESOURCES}>
      {children}
    </I18nProvider>
  );
}

function renderTab() {
  return render(<GitLabIssuesTab />, { wrapper: I18nWrapper });
}

function resetFixtures() {
  vi.clearAllMocks();
  membersRef.current = [{ user_id: "user-1", role: "owner" }];
  integrationsRef.current = [];
  syncConfigsRef.current = [];
  skillsRef.current = [
    { id: "skill-issue", name: "issue-creator" },
    { id: "skill-other", name: "other-skill" },
  ];
  skillsRef.isError = false;
  mockCreateIntegration.mockResolvedValue({});
  mockUpdateIntegration.mockResolvedValue({});
  mockDeleteIntegration.mockResolvedValue(undefined);
  mockTestIntegration.mockResolvedValue({
    ok: true,
    provider: "gitlab",
    username: "octo",
    name: "Octo User",
  });
}

describe("GitLabIssuesTab", () => {
  beforeEach(resetFixtures);
  afterEach(cleanup);

  it("renders setup form for admins when no integration exists", async () => {
    const user = userEvent.setup();
    renderTab();

    await user.click(screen.getByRole("button", { name: /^Add connection$/ }));

    expect((screen.getByLabelText(/^Base URL$/) as HTMLInputElement).value).toBe(
      "https://gitlab.com",
    );
    expect(
      (screen.getByLabelText(/^Issue creation skill$/) as HTMLSelectElement).value,
    ).toBe("skill-issue");
  });

  it("renders read-only hint for non-admin members", () => {
    membersRef.current = [{ user_id: "user-1", role: "member" }];
    renderTab();

    expect(screen.getByText(/^Read-only view\./)).toBeTruthy();
    expect(screen.queryByRole("button", { name: /^Add connection$/ })).toBeNull();
  });

  it("does not prefill token when editing an integration", async () => {
    const user = userEvent.setup();
    integrationsRef.current = [
      {
        id: "gitlab-1",
        name: "Work GitLab",
        base_url: "https://gitlab.example.com",
        default_issue_skill_id: "skill-other",
        polling_enabled: true,
        default_poll_interval_seconds: 600,
        config: { group: "platform" },
      },
    ];
    renderTab();

    await user.click(screen.getByRole("button", { name: /^Edit$/ }));

    expect((screen.getByLabelText(/^Access token$/) as HTMLInputElement).value).toBe("");
    expect(screen.getByPlaceholderText(/Leave blank/)).toBeTruthy();
  });

  it("creates a gitlab integration with token and default skill", async () => {
    const user = userEvent.setup();
    renderTab();

    await user.click(screen.getByRole("button", { name: /^Add connection$/ }));
    await user.clear(screen.getByLabelText(/^Name$/));
    await user.type(screen.getByLabelText(/^Name$/), "Internal GitLab");
    await user.type(screen.getByLabelText(/^Access token$/), "glpat-secret");
    await user.click(screen.getByRole("button", { name: /^Save$/ }));

    await waitFor(() => {
      expect(mockCreateIntegration).toHaveBeenCalledWith(
        expect.objectContaining({
          name: "Internal GitLab",
          base_url: "https://gitlab.com",
          token: "glpat-secret",
          default_issue_skill_id: "skill-issue",
          polling_enabled: false,
          default_poll_interval_seconds: 300,
        }),
      );
    });
  });

  it("updates an integration without sending a blank token", async () => {
    const user = userEvent.setup();
    integrationsRef.current = [
      {
        id: "gitlab-1",
        name: "Work GitLab",
        base_url: "https://gitlab.example.com",
        default_issue_skill_id: "skill-other",
        polling_enabled: true,
        default_poll_interval_seconds: 600,
        config: { group: "platform" },
      },
    ];
    renderTab();

    await user.click(screen.getByRole("button", { name: /^Edit$/ }));
    await user.clear(screen.getByLabelText(/^Name$/));
    await user.type(screen.getByLabelText(/^Name$/), "Renamed GitLab");
    await user.click(screen.getByRole("button", { name: /^Save$/ }));

    await waitFor(() => {
      expect(mockUpdateIntegration).toHaveBeenCalledWith({
        id: "gitlab-1",
        data: expect.objectContaining({
          config: { group: "platform" },
        }),
      });
    });
    expect(mockUpdateIntegration.mock.calls[0]![0].data).not.toHaveProperty("token");
    expect(mockUpdateIntegration.mock.calls[0]![0].data.name).toBe("Renamed GitLab");
  });

  it("keeps existing connections visible when the skill list fails", async () => {
    const user = userEvent.setup();
    skillsRef.isError = true;
    integrationsRef.current = [
      {
        id: "gitlab-1",
        name: "Work GitLab",
        base_url: "https://gitlab.example.com",
        default_issue_skill_id: "skill-other",
        polling_enabled: true,
        default_poll_interval_seconds: 600,
        config: { group: "platform" },
      },
    ];
    renderTab();

    expect(screen.getByText("Work GitLab")).toBeTruthy();
    await user.click(screen.getByRole("button", { name: /^Edit$/ }));
    expect(screen.getByText(/Skill list is unavailable/)).toBeTruthy();
  });

  it("tests connection and shows the returned user", async () => {
    const user = userEvent.setup();
    integrationsRef.current = [
      {
        id: "gitlab-1",
        name: "Work GitLab",
        base_url: "https://gitlab.example.com",
        default_issue_skill_id: "skill-other",
        polling_enabled: true,
        default_poll_interval_seconds: 600,
        config: { group: "platform" },
      },
    ];
    renderTab();

    await user.click(screen.getByRole("button", { name: /^Test connection$/ }));

    await waitFor(() => {
      expect(mockTestIntegration).toHaveBeenCalledWith("gitlab-1");
      expect(mockToastSuccess).toHaveBeenCalledWith("Connected as Octo User");
    });
  });
});
