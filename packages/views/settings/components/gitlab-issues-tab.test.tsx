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
const mockCreateRule = vi.hoisted(() => vi.fn());
const mockUpdateRule = vi.hoisted(() => vi.fn());
const mockDeleteRule = vi.hoisted(() => vi.fn());
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
    scope_id: string;
    remote_project_ref: string;
    scope_type: string;
    sync_enabled: boolean;
    poll_interval_seconds?: number | null;
    state_mapping?: Record<string, unknown>;
    auto_assign_enabled?: boolean;
    default_assignee_type?: string | null;
    default_assignee_id?: string | null;
  }>,
}));
const skillsRef = vi.hoisted(() => ({
  current: [
    { id: "skill-issue", name: "issue-creator" },
    { id: "skill-other", name: "other-skill" },
  ],
  isError: false,
}));
const projectsRef = vi.hoisted(() => ({
  current: [
    { id: "project-1", title: "Alpha Project" },
    { id: "project-2", title: "Beta Project" },
  ],
  isError: false,
}));
const resourcesRef = vi.hoisted(() => ({
  current: {
    "project-1": [
      {
        id: "resource-1",
        project_id: "project-1",
        workspace_id: "workspace-1",
        resource_type: "github_repo",
        resource_ref: { url: "https://github.com/acme/alpha" },
        label: "alpha-repo",
        position: 1,
        created_at: "",
        created_by: null,
      },
    ],
    "project-2": [
      {
        id: "resource-2",
        project_id: "project-2",
        workspace_id: "workspace-1",
        resource_type: "local_directory",
        resource_ref: { local_path: "/tmp/beta", daemon_id: "daemon-1" },
        label: "beta-dir",
        position: 1,
        created_at: "",
        created_by: null,
      },
    ],
  } as Record<string, Array<Record<string, unknown>>>,
  isError: false,
}));
const agentsRef = vi.hoisted(() => ({
  current: [
    { id: "agent-1", name: "Planner Bot", archived_at: null },
    { id: "agent-2", name: "Closer Bot", archived_at: null },
  ],
  isError: false,
}));
const squadsRef = vi.hoisted(() => ({
  current: [{ id: "squad-1", name: "Platform Squad", archived_at: null }],
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
    if (key.includes("\"projects\"") && key.includes("\"list\"")) {
      return { data: projectsRef.current, isError: projectsRef.isError };
    }
    if (key.includes("\"agents\"")) {
      return { data: agentsRef.current, isError: agentsRef.isError };
    }
    if (key.includes("\"squads\"")) {
      return { data: squadsRef.current, isError: squadsRef.isError };
    }
    return { data: undefined };
  },
  useQueries: ({ queries }: { queries: Array<{ queryKey: unknown[] }> }) =>
    queries.map((query) => {
      const projectId = String(query.queryKey[3] ?? "");
      return {
        data: resourcesRef.current[projectId] ?? [],
        isError: resourcesRef.isError,
      };
    }),
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
  agentListOptions: () => ({ queryKey: ["agents"], queryFn: vi.fn() }),
  squadListOptions: () => ({ queryKey: ["squads"], queryFn: vi.fn() }),
}));

vi.mock("@multica/core/projects/queries", () => ({
  projectListOptions: () => ({
    queryKey: ["projects", "workspace-1", "list"],
    queryFn: vi.fn(),
  }),
}));

vi.mock("@multica/core/projects", () => ({
  projectResourcesOptions: (_wsId: string, projectId: string) => ({
    queryKey: ["projects", "workspace-1", "detail", projectId, "resources"],
    queryFn: vi.fn(),
  }),
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
  useCreateIssueSyncConfig: () => ({
    isPending: false,
    mutateAsync: mockCreateRule,
  }),
  useUpdateIssueSyncConfig: () => ({
    isPending: false,
    mutateAsync: mockUpdateRule,
  }),
  useDeleteIssueSyncConfig: () => ({
    isPending: false,
    mutateAsync: mockDeleteRule,
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
  projectsRef.current = [
    { id: "project-1", title: "Alpha Project" },
    { id: "project-2", title: "Beta Project" },
  ];
  projectsRef.isError = false;
  resourcesRef.current = {
    "project-1": [
      {
        id: "resource-1",
        project_id: "project-1",
        workspace_id: "workspace-1",
        resource_type: "github_repo",
        resource_ref: { url: "https://github.com/acme/alpha" },
        label: "alpha-repo",
        position: 1,
        created_at: "",
        created_by: null,
      },
    ],
    "project-2": [
      {
        id: "resource-2",
        project_id: "project-2",
        workspace_id: "workspace-1",
        resource_type: "local_directory",
        resource_ref: { local_path: "/tmp/beta", daemon_id: "daemon-1" },
        label: "beta-dir",
        position: 1,
        created_at: "",
        created_by: null,
      },
    ],
  };
  resourcesRef.isError = false;
  agentsRef.current = [
    { id: "agent-1", name: "Planner Bot", archived_at: null },
    { id: "agent-2", name: "Closer Bot", archived_at: null },
  ];
  agentsRef.isError = false;
  squadsRef.current = [
    { id: "squad-1", name: "Platform Squad", archived_at: null },
  ];
  squadsRef.isError = false;
  mockCreateIntegration.mockResolvedValue({});
  mockUpdateIntegration.mockResolvedValue({});
  mockDeleteIntegration.mockResolvedValue(undefined);
  mockTestIntegration.mockResolvedValue({
    ok: true,
    provider: "gitlab",
    username: "octo",
    name: "Octo User",
  });
  mockCreateRule.mockResolvedValue({});
  mockUpdateRule.mockResolvedValue({});
  mockDeleteRule.mockResolvedValue(undefined);
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

  it("disables rule creation until a connection exists", () => {
    renderTab();

    expect(
      screen.getByRole("button", { name: /^Add rule$/ }).hasAttribute("disabled"),
    ).toBe(true);
  });

  it("creates a project sync rule with poll interval", async () => {
    const user = userEvent.setup();
    integrationsRef.current = [
      {
        id: "gitlab-1",
        name: "Work GitLab",
        base_url: "https://gitlab.example.com",
        default_issue_skill_id: "skill-other",
        polling_enabled: true,
        default_poll_interval_seconds: 600,
        config: {},
      },
    ];
    renderTab();

    await user.click(screen.getByRole("button", { name: /^Add rule$/ }));
    await user.selectOptions(screen.getByLabelText(/^Scope$/), "project");
    await user.selectOptions(screen.getByLabelText(/^Project or resource$/), "project-2");
    await user.type(screen.getByLabelText(/^GitLab project ref$/), "acme/backend");
    await user.type(screen.getByLabelText(/^Polling interval \(seconds\)$/), "120");
    await user.click(screen.getByRole("switch", { name: /^Enable sync$/ }));
    await user.click(screen.getByRole("button", { name: /^Save$/ }));

    await waitFor(() => {
      expect(mockCreateRule).toHaveBeenCalledWith({
        integration_id: "gitlab-1",
        scope_type: "project",
        scope_id: "project-2",
        remote_project_ref: "acme/backend",
        sync_mode: "assigned_to_me",
        auto_accept_label: "ai-auto",
        sync_enabled: true,
        poll_interval_seconds: 120,
        state_mapping: { opened: "backlog", closed: "done" },
        auto_assign_enabled: false,
        default_assignee_type: null,
        default_assignee_id: null,
      });
    });
  });

  it("creates a repo-resource sync rule", async () => {
    const user = userEvent.setup();
    integrationsRef.current = [
      {
        id: "gitlab-1",
        name: "Work GitLab",
        base_url: "https://gitlab.example.com",
        default_issue_skill_id: "skill-other",
        polling_enabled: true,
        default_poll_interval_seconds: 600,
        config: {},
      },
    ];
    renderTab();

    await user.click(screen.getByRole("button", { name: /^Add rule$/ }));
    await user.selectOptions(screen.getByLabelText(/^Scope$/), "repo_resource");
    await user.selectOptions(screen.getByLabelText(/^Project or resource$/), "resource-2");
    await user.type(screen.getByLabelText(/^GitLab project ref$/), "acme/desktop");
    await user.click(screen.getByRole("button", { name: /^Save$/ }));

    await waitFor(() => {
      expect(mockCreateRule).toHaveBeenCalledWith(
        expect.objectContaining({
          scope_type: "repo_resource",
          scope_id: "resource-2",
          remote_project_ref: "acme/desktop",
        }),
      );
    });
  });

  it("requires assignee when auto-assignment is enabled", async () => {
    const user = userEvent.setup();
    integrationsRef.current = [
      {
        id: "gitlab-1",
        name: "Work GitLab",
        base_url: "https://gitlab.example.com",
        default_issue_skill_id: "skill-other",
        polling_enabled: true,
        default_poll_interval_seconds: 600,
        config: {},
      },
    ];
    renderTab();

    await user.click(screen.getByRole("button", { name: /^Add rule$/ }));
    await user.selectOptions(screen.getByLabelText(/^Project or resource$/), "project-1");
    await user.type(screen.getByLabelText(/^GitLab project ref$/), "acme/service");
    await user.click(
      screen.getByRole("switch", { name: /^Auto-assign imported issues$/ }),
    );
    await user.click(screen.getByRole("button", { name: /^Save$/ }));

    await waitFor(() => {
      expect(mockToastError).toHaveBeenCalledWith(
        "Choose an agent or squad for auto-assignment.",
      );
    });
    expect(mockCreateRule).not.toHaveBeenCalled();
  });

  it("creates an auto-accept sync rule with a default assignee", async () => {
    const user = userEvent.setup();
    integrationsRef.current = [
      {
        id: "gitlab-1",
        name: "Work GitLab",
        base_url: "https://gitlab.example.com",
        default_issue_skill_id: "skill-other",
        polling_enabled: true,
        default_poll_interval_seconds: 600,
        config: {},
      },
    ];
    renderTab();

    await user.click(screen.getByRole("button", { name: /^Add rule$/ }));
    await user.selectOptions(screen.getByLabelText(/^Project or resource$/), "project-1");
    await user.type(screen.getByLabelText(/^GitLab project ref$/), "acme/service");
    await user.selectOptions(screen.getByLabelText(/^Sync mode$/), "auto_accept");
    await user.selectOptions(screen.getByLabelText(/^Assignee type$/), "agent");
    await user.selectOptions(screen.getByLabelText(/^Default assignee$/), "agent-1");
    await user.click(screen.getByRole("button", { name: /^Save$/ }));

    await waitFor(() => {
      expect(mockCreateRule).toHaveBeenCalledWith(
        expect.objectContaining({
          remote_project_ref: "acme/service",
          sync_mode: "auto_accept",
          auto_accept_label: "ai-auto",
          auto_assign_enabled: true,
          default_assignee_type: "agent",
          default_assignee_id: "agent-1",
        }),
      );
    });
  });

  it("saves auto-assignment to an agent", async () => {
    const user = userEvent.setup();
    integrationsRef.current = [
      {
        id: "gitlab-1",
        name: "Work GitLab",
        base_url: "https://gitlab.example.com",
        default_issue_skill_id: "skill-other",
        polling_enabled: true,
        default_poll_interval_seconds: 600,
        config: {},
      },
    ];
    syncConfigsRef.current = [
      {
        id: "sync-1",
        integration_id: "gitlab-1",
        scope_id: "project-1",
        remote_project_ref: "acme/service",
        scope_type: "project",
        sync_enabled: true,
        poll_interval_seconds: 300,
        state_mapping: { opened: "todo", closed: "finished" },
        auto_assign_enabled: false,
        default_assignee_type: null,
        default_assignee_id: null,
      },
    ];
    renderTab();

    await user.click(screen.getAllByRole("button", { name: /^Edit$/ })[1]!);
    expect(screen.getByLabelText(/^GitLab connection$/)).toHaveProperty(
      "disabled",
      true,
    );
    expect(screen.getByLabelText(/^Scope$/)).toHaveProperty("disabled", true);
    expect(screen.getByLabelText(/^Project or resource$/)).toHaveProperty(
      "disabled",
      true,
    );
    await user.click(
      screen.getByRole("switch", { name: /^Auto-assign imported issues$/ }),
    );
    await user.selectOptions(screen.getByLabelText(/^Assignee type$/), "agent");
    await user.selectOptions(screen.getByLabelText(/^Default assignee$/), "agent-2");
    await user.click(screen.getByRole("button", { name: /^Save$/ }));

    await waitFor(() => {
      expect(mockUpdateRule).toHaveBeenCalledWith({
        id: "sync-1",
        data: expect.objectContaining({
          auto_assign_enabled: true,
          default_assignee_type: "agent",
          default_assignee_id: "agent-2",
          state_mapping: { opened: "todo", closed: "finished" },
        }),
      });
    });
  });

  it.each([
    ["projects", () => { projectsRef.isError = true; }],
    ["resources", () => { resourcesRef.isError = true; }],
    ["agents", () => { agentsRef.isError = true; }],
    ["squads", () => { squadsRef.isError = true; }],
  ])(
    "shows a scoped error and disables rule editing when %s fail",
    (_source, failQuery) => {
    integrationsRef.current = [
      {
        id: "gitlab-1",
        name: "Work GitLab",
        base_url: "https://gitlab.example.com",
        default_issue_skill_id: "skill-other",
        polling_enabled: true,
        default_poll_interval_seconds: 600,
        config: {},
      },
    ];
    syncConfigsRef.current = [
      {
        id: "sync-1",
        integration_id: "gitlab-1",
        scope_id: "project-1",
        remote_project_ref: "acme/service",
        scope_type: "project",
        sync_enabled: true,
        poll_interval_seconds: 300,
        state_mapping: { opened: "todo", closed: "finished" },
        auto_assign_enabled: false,
        default_assignee_type: null,
        default_assignee_id: null,
      },
    ];
    failQuery();

    renderTab();

    expect(screen.getByText(/Rule options could not be loaded/)).toBeTruthy();
    expect(screen.getByText("acme/service")).toBeTruthy();
    expect(
      screen.getByRole("button", { name: /^Add rule$/ }).hasAttribute("disabled"),
    ).toBe(true);
    expect(screen.getAllByRole("button", { name: /^Edit$/ })[1]).toHaveProperty(
      "disabled",
      true,
    );
    },
  );

  it("deletes a sync rule after confirmation", async () => {
    const user = userEvent.setup();
    integrationsRef.current = [
      {
        id: "gitlab-1",
        name: "Work GitLab",
        base_url: "https://gitlab.example.com",
        default_issue_skill_id: "skill-other",
        polling_enabled: true,
        default_poll_interval_seconds: 600,
        config: {},
      },
    ];
    syncConfigsRef.current = [
      {
        id: "sync-1",
        integration_id: "gitlab-1",
        scope_id: "project-1",
        remote_project_ref: "acme/service",
        scope_type: "project",
        sync_enabled: true,
        poll_interval_seconds: 300,
        state_mapping: { opened: "backlog", closed: "done" },
        auto_assign_enabled: false,
        default_assignee_type: null,
        default_assignee_id: null,
      },
    ];
    renderTab();

    await user.click(screen.getAllByRole("button", { name: /^Delete$/ })[1]!);
    await user.click(
      await screen.findByRole("button", { name: /^Delete rule$/ }),
    );

    await waitFor(() => {
      expect(mockDeleteRule).toHaveBeenCalledWith("sync-1");
    });
  });
});
