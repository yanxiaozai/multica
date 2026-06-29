// @vitest-environment jsdom

import "@testing-library/jest-dom/vitest";
import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { cleanup, fireEvent, screen } from "@testing-library/react";
import { renderWithI18n } from "../../test/i18n";
import { ProjectGitLabSyncSection } from "./project-gitlab-sync";

// useWorkspaceId + the two react-query reads + the two mutations are the
// component's entire surface area. Hoisted mocks so the factory can reference
// them after vitest hoists vi.mock.
const mocks = vi.hoisted(() => ({
  wsId: "ws-1",
  integrations: [] as Array<{
    id: string;
    name: string;
    base_url: string;
  }>,
  syncConfigs: [] as Array<{
    id: string;
    scope_type: string;
    scope_id: string;
    remote_project_ref: string;
    sync_enabled: boolean;
    poll_interval_seconds: number | null;
    auto_assign_enabled: boolean;
    default_assignee_id: string | null;
    last_poll_at: string | null;
    last_error: string;
  }>,
  agents: [] as Array<{ id: string; name: string; archived_at: string | null }>,
  resources: [] as Array<{
    id: string;
    resource_type: string;
    resource_ref: { local_path?: string };
  }>,
  createConfig: vi.fn(),
  updateConfig: vi.fn(),
  importIssues: vi.fn(),
}));

vi.mock("@multica/core/hooks", () => ({
  useWorkspaceId: () => mocks.wsId,
}));

vi.mock("@multica/core/issue-bridge", () => ({
  issueIntegrationsOptions: () => ({ queryKey: ["integrations"] }),
  issueSyncConfigsOptions: () => ({ queryKey: ["sync-configs"] }),
  useCreateIssueSyncConfig: () => ({
    mutateAsync: mocks.createConfig,
    isPending: false,
  }),
  useUpdateIssueSyncConfig: () => ({
    mutateAsync: mocks.updateConfig,
    isPending: false,
  }),
  useImportProjectGitLabIssues: () => ({
    mutateAsync: mocks.importIssues,
    isPending: false,
  }),
}));

vi.mock("@multica/core/workspace/queries", () => ({
  agentListOptions: () => ({ queryKey: ["agents"] }),
}));

vi.mock("@multica/core/projects", () => ({
  projectResourcesOptions: () => ({ queryKey: ["project-resources"] }),
}));

// Auto-detect lives in the platform module. parseGitRemote is the REAL
// implementation (so the SSH→HTTPS host match is exercised end-to-end);
// detectGitRemote is stubbed per-test to feed it a remote URL.
vi.mock("../../platform", async () => {
  const actual = await vi.importActual<typeof import("../../platform")>(
    "../../platform",
  );
  return {
    ...actual,
    detectGitRemote: vi.fn(),
    isDesktopShell: () => true,
  };
});

vi.mock("@tanstack/react-query", () => ({
  useQuery: (options: { queryKey: readonly unknown[] }) => {
    const key = String(options.queryKey[0]);
    if (key === "integrations") return { data: { integrations: mocks.integrations } };
    if (key === "sync-configs") return { data: { sync_configs: mocks.syncConfigs } };
    if (key === "agents") return { data: mocks.agents };
    if (key === "project-resources") return { data: mocks.resources };
    return { data: undefined };
  },
}));

vi.mock("sonner", () => ({
  toast: { success: vi.fn(), error: vi.fn(), message: vi.fn() },
}));

import { toast } from "sonner";
import { detectGitRemote } from "../../platform";

function renderSection(projectId = "proj-1") {
  return renderWithI18n(<ProjectGitLabSyncSection projectId={projectId} />);
}

// A project-scoped config with the polling/auto-assign fields defaulted off.
function projConfig(over: Partial<typeof mocks.syncConfigs[number]> = {}) {
  return {
    id: "cfg-1",
    scope_type: "project",
    scope_id: "proj-1",
    remote_project_ref: "group/proj",
    sync_enabled: false,
    poll_interval_seconds: 300,
    auto_assign_enabled: false,
    default_assignee_id: null,
    last_poll_at: null,
    last_error: "",
    ...over,
  };
}

describe("ProjectGitLabSyncSection", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mocks.integrations = [];
    mocks.syncConfigs = [];
    mocks.agents = [];
    mocks.resources = [];
    vi.mocked(detectGitRemote).mockResolvedValue({ ok: false, reason: "unsupported" });
  });

  afterEach(() => {
    cleanup();
  });

  it("hints to set up a connection when the workspace has none", () => {
    renderSection();
    // Section is collapsed by default — expand it.
    fireEvent.click(screen.getByText("GitLab Sync"));
    expect(
      screen.getByText(/No GitLab connection in this workspace/i),
    ).toBeInTheDocument();
  });

  it("creates a sync config when connecting", async () => {
    mocks.integrations = [{ id: "itg-1", name: "GitLab", base_url: "https://gitlab.example.com" }];
    renderSection();
    fireEvent.click(screen.getByText("GitLab Sync"));

    // Open the integration select and pick the only option.
    fireEvent.click(screen.getByRole("combobox"));
    fireEvent.click(screen.getByText("GitLab"));

    fireEvent.change(screen.getByPlaceholderText("group/project"), {
      target: { value: "group/proj" },
    });
    fireEvent.click(screen.getByText("Connect"));

    await Promise.resolve();
    expect(mocks.createConfig).toHaveBeenCalledTimes(1);
    expect(mocks.createConfig).toHaveBeenCalledWith(
      expect.objectContaining({
        scope_type: "project",
        scope_id: "proj-1",
        remote_project_ref: "group/proj",
      }),
    );
  });

  it("shows the import button when a config exists and imports on click", async () => {
    mocks.integrations = [{ id: "itg-1", name: "GitLab", base_url: "https://gitlab.example.com" }];
    mocks.syncConfigs = [projConfig()];
    mocks.importIssues.mockResolvedValue({ imported: 3, skipped: 0, failed: 0 });
    renderSection();
    fireEvent.click(screen.getByText("GitLab Sync"));

    expect(screen.getByText(/Linked to group\/proj/i)).toBeInTheDocument();
    fireEvent.click(screen.getByText("Import my issues"));
    await Promise.resolve();

    expect(mocks.importIssues).toHaveBeenCalledWith("proj-1");
    await Promise.resolve();
    expect(toast.success).toHaveBeenCalled();
  });

  it("surfaces a partial-failure toast when some imports fail", async () => {
    mocks.integrations = [{ id: "itg-1", name: "GitLab", base_url: "https://gitlab.example.com" }];
    mocks.syncConfigs = [projConfig()];
    mocks.importIssues.mockResolvedValue({ imported: 2, skipped: 1, failed: 1 });
    renderSection();
    fireEvent.click(screen.getByText("GitLab Sync"));

    fireEvent.click(screen.getByText("Import my issues"));
    await Promise.resolve();
    await Promise.resolve();

    expect(toast.message).toHaveBeenCalled();
    expect(toast.success).not.toHaveBeenCalled();
  });

  it("renders the last sync status when the scheduler has polled", () => {
    mocks.integrations = [{ id: "itg-1", name: "GitLab", base_url: "https://gitlab.example.com" }];
    mocks.syncConfigs = [projConfig({ last_poll_at: "2026-06-27T00:00:00Z" })];
    renderSection();
    fireEvent.click(screen.getByText("GitLab Sync"));
    expect(screen.getByText(/Last sync/i)).toBeInTheDocument();
  });

  it("auto-detects a GitLab remote from a local_directory and one-click connects", async () => {
    // Integration base_url is HTTPS; the local remote is SSH on the same host.
    // Host matching must bridge scheme + SCP form.
    mocks.integrations = [
      { id: "itg-1", name: "GitLab", base_url: "https://gitlab.example.com" },
    ];
    mocks.resources = [
      {
        id: "res-1",
        resource_type: "local_directory",
        resource_ref: { local_path: "/Users/me/repo" },
      },
    ];
    vi.mocked(detectGitRemote).mockResolvedValue({
      ok: true,
      remote_url: "git@gitlab.example.com:group/proj.git",
    });
    renderSection();
    fireEvent.click(screen.getByText("GitLab Sync"));

    // The auto-detect banner appears with the parsed ref + matched integration.
    expect(await screen.findByText(/Auto-detected from local repo/i)).toBeInTheDocument();
    expect(screen.getByText(/group\/proj/)).toBeInTheDocument();
    fireEvent.click(screen.getByText("Connect"));

    await Promise.resolve();
    expect(mocks.createConfig).toHaveBeenCalledTimes(1);
    expect(mocks.createConfig).toHaveBeenCalledWith(
      expect.objectContaining({
        integration_id: "itg-1",
        scope_type: "project",
        scope_id: "proj-1",
        remote_project_ref: "group/proj",
      }),
    );
  });

  it("falls back to manual form when no integration host matches the remote", async () => {
    // Local repo is GitHub, but the only integration is a GitLab host.
    mocks.integrations = [
      { id: "itg-1", name: "GitLab", base_url: "https://gitlab.example.com" },
    ];
    mocks.resources = [
      {
        id: "res-1",
        resource_type: "local_directory",
        resource_ref: { local_path: "/Users/me/repo" },
      },
    ];
    vi.mocked(detectGitRemote).mockResolvedValue({
      ok: true,
      remote_url: "git@github.com:someone/repo.git",
    });
    renderSection();
    fireEvent.click(screen.getByText("GitLab Sync"));

    // No auto-detect banner; the manual Connect form is present instead.
    await Promise.resolve();
    expect(screen.queryByText(/Auto-detected/i)).not.toBeInTheDocument();
    expect(screen.getByPlaceholderText("group/project")).toBeInTheDocument();
  });

  it("saves polling + auto-assign settings via update mutation", async () => {
    mocks.integrations = [{ id: "itg-1", name: "GitLab", base_url: "https://gitlab.example.com" }];
    mocks.agents = [{ id: "agent-1", name: "Coder", archived_at: null }];
    mocks.syncConfigs = [projConfig()];
    renderSection();
    fireEvent.click(screen.getByText("GitLab Sync"));

    // Toggle auto-poll and auto-assign on, then Save.
    const checkboxes = screen.getAllByRole("checkbox");
    // First checkbox = auto-poll, second = auto-assign.
    fireEvent.click(checkboxes[0]!);
    fireEvent.click(checkboxes[1]!);
    fireEvent.click(screen.getByText("Save"));

    await Promise.resolve();
    expect(mocks.updateConfig).toHaveBeenCalledTimes(1);
    expect(mocks.updateConfig).toHaveBeenCalledWith(
      expect.objectContaining({
        id: "cfg-1",
        data: expect.objectContaining({
          sync_enabled: true,
          auto_assign_enabled: true,
          default_assignee_type: "agent",
        }),
      }),
    );
  });
});
