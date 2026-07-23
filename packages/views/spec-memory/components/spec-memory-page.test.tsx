import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen, waitFor } from "@testing-library/react";
import type { ReactNode } from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";

const mockListSpecEpics = vi.hoisted(() => vi.fn());
const mockListSpecModules = vi.hoisted(() => vi.fn());
const mockListSpecEpicDocuments = vi.hoisted(() => vi.fn());
const mockListSpecDocuments = vi.hoisted(() => vi.fn());
const mockListProjectResources = vi.hoisted(() => vi.fn());
const mockSyncSpecFromFiles = vi.hoisted(() => vi.fn());
const mockIsDesktopShell = vi.hoisted(() => vi.fn());
const mockReadSpecSnapshot = vi.hoisted(() => vi.fn());
const mockUseLocalDaemonStatus = vi.hoisted(() => vi.fn());

vi.mock("@multica/core/hooks", () => ({
  useWorkspaceId: () => "ws-1",
}));

vi.mock("@multica/core/api", () => ({
  api: {
    listSpecEpics: mockListSpecEpics,
    listSpecModules: mockListSpecModules,
    listSpecEpicDocuments: mockListSpecEpicDocuments,
    listSpecDocuments: mockListSpecDocuments,
    listProjectResources: mockListProjectResources,
    syncSpecFromFiles: mockSyncSpecFromFiles,
  },
}));

vi.mock("../../platform", () => ({
  isDesktopShell: mockIsDesktopShell,
  readSpecSnapshot: mockReadSpecSnapshot,
  useLocalDaemonStatus: mockUseLocalDaemonStatus,
}));

vi.mock("../../layout/page-header", () => ({
  PageHeader: ({ children }: { children: ReactNode }) => (
    <header>{children}</header>
  ),
}));

import { SpecMemoryPage } from "./spec-memory-page";

function renderWithQueryClient(ui: ReactNode) {
  const queryClient = new QueryClient({
    defaultOptions: {
      queries: {
        retry: false,
      },
    },
  });
  return render(
    <QueryClientProvider client={queryClient}>{ui}</QueryClientProvider>,
  );
}

describe("SpecMemoryPage", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockListSpecEpics.mockResolvedValue({ epics: [] });
    mockListSpecModules.mockResolvedValue({ modules: [] });
    mockListSpecEpicDocuments.mockResolvedValue({ documents: [] });
    mockListSpecDocuments.mockResolvedValue({ documents: [] });
    mockListProjectResources.mockResolvedValue({ resources: [], total: 0 });
    mockSyncSpecFromFiles.mockResolvedValue({
      epics: 0,
      modules: 0,
      documents: 0,
      issues: 0,
      issue_mappings: 0,
      decisions: 0,
      skipped_issue_files: [],
    });
    mockIsDesktopShell.mockReturnValue(false);
    mockReadSpecSnapshot.mockResolvedValue({ ok: false, reason: "unsupported" });
    mockUseLocalDaemonStatus.mockReturnValue({ running: false, daemonId: null });
  });

  it("explains how to import local .spec files when no memory has been synced", async () => {
    renderWithQueryClient(<SpecMemoryPage />);

    expect(
      await screen.findByText("No synced spec memory yet"),
    ).toBeInTheDocument();
    expect(
      screen.getByText(/Local \.spec files are not read directly/i),
    ).toBeInTheDocument();
    expect(
      screen.getByText("multica spec sync --from-files"),
    ).toBeInTheDocument();
  });

  it("syncs local .spec files from the current daemon local_directory", async () => {
    const snapshot = {
      epics: [
        {
          key: "lms-core-prototype",
          title: "LMS Core Prototype",
          description: "",
          stability: "active",
          documents: [],
          modules: [],
        },
      ],
      issues: [],
      decisions: [],
      skipped_issue_files: [],
    };
    mockIsDesktopShell.mockReturnValue(true);
    mockUseLocalDaemonStatus.mockReturnValue({
      running: true,
      daemonId: "daemon-1",
    });
    mockListProjectResources.mockResolvedValue({
      resources: [
        {
          id: "res-1",
          project_id: "project-1",
          workspace_id: "ws-1",
          resource_type: "local_directory",
          resource_ref: {
            local_path: "/Users/me/lms-mini",
            daemon_id: "daemon-1",
          },
          label: null,
          position: 0,
          created_at: "",
          created_by: null,
        },
      ],
      total: 1,
    });
    mockReadSpecSnapshot.mockResolvedValue({ ok: true, snapshot });

    renderWithQueryClient(<SpecMemoryPage projectId="project-1" />);

    await waitFor(() => {
      expect(mockReadSpecSnapshot).toHaveBeenCalledWith("/Users/me/lms-mini");
    });
    await waitFor(() => {
      expect(mockSyncSpecFromFiles).toHaveBeenCalledWith(snapshot);
    });
  });

  it("syncs the only local_directory even when the daemon id is not available", async () => {
    const snapshot = {
      epics: [
        {
          key: "lms-core-prototype",
          title: "LMS Core Prototype",
          description: "",
          stability: "active",
          documents: [],
          modules: [],
        },
      ],
      issues: [],
      decisions: [],
      skipped_issue_files: [],
    };
    mockIsDesktopShell.mockReturnValue(true);
    mockUseLocalDaemonStatus.mockReturnValue({
      running: false,
      daemonId: null,
    });
    mockListProjectResources.mockResolvedValue({
      resources: [
        {
          id: "res-1",
          project_id: "project-1",
          workspace_id: "ws-1",
          resource_type: "local_directory",
          resource_ref: {
            local_path: "/Users/me/lms-mini",
            daemon_id: "daemon-1",
          },
          label: null,
          position: 0,
          created_at: "",
          created_by: null,
        },
      ],
      total: 1,
    });
    mockReadSpecSnapshot.mockResolvedValue({ ok: true, snapshot });

    renderWithQueryClient(<SpecMemoryPage projectId="project-1" />);

    await waitFor(() => {
      expect(mockReadSpecSnapshot).toHaveBeenCalledWith("/Users/me/lms-mini");
    });
    await waitFor(() => {
      expect(mockSyncSpecFromFiles).toHaveBeenCalledWith(snapshot);
    });
  });

  it("shows epic-level documents when an epic has no modules", async () => {
    mockListSpecEpics.mockResolvedValue({
      epics: [
        {
          id: "epic-1",
          workspace_id: "ws-1",
          key: "lms-business-modules-prototype",
          title: "Business Modules Prototype",
          description: "",
          stability: "draft",
          created_at: "",
          updated_at: "",
        },
      ],
    });
    mockListSpecModules.mockResolvedValue({ modules: [] });
    mockListSpecEpicDocuments.mockResolvedValue({
      documents: [
        {
          id: "doc-1",
          workspace_id: "ws-1",
          epic_id: "epic-1",
          module_id: null,
          doc_kind: "requirements",
          title: "Requirements",
          body: "# Requirements\n\nBusiness module requirements.",
          source_path:
            ".spec/epics/lms-business-modules-prototype/requirements.md",
          created_at: "",
          updated_at: "",
        },
      ],
    });

    renderWithQueryClient(<SpecMemoryPage projectId="project-1" />);

    expect(await screen.findByText("This epic has no modules.")).toBeInTheDocument();
    expect(await screen.findAllByText("Requirements")).toHaveLength(2);
    expect(
      screen.getByText(/Business module requirements/i),
    ).toBeInTheDocument();
  });
});
