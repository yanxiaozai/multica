import type { SyncSpecFromFilesRequest } from "@multica/core/types";

// Desktop-only helpers for the project_resource local_directory flow.
//
// These wrap the preload `desktopAPI` surface so view components can
// SSR-render on web (where `window.desktopAPI` is undefined) and degrade
// gracefully to no-op promises instead of crashing.

export type PickDirectoryResult = {
  ok: boolean;
  path?: string;
  basename?: string;
  reason?: "cancelled" | "no_window" | "error" | "unsupported";
  error?: string;
};

export type ValidateLocalDirectoryResult = {
  ok: boolean;
  reason?:
    | "not_absolute"
    | "not_found"
    | "not_a_directory"
    | "not_readable"
    | "not_writable"
    | "error"
    | "unsupported";
  error?: string;
};

interface DesktopLocalDirectoryAPI {
  pickDirectory?: (defaultPath?: string) => Promise<PickDirectoryResult>;
  validateLocalDirectory?: (
    path: string,
  ) => Promise<ValidateLocalDirectoryResult>;
  detectGitRemote?: (path: string) => Promise<DetectGitRemoteResult>;
  readSpecSnapshot?: (root: string) => Promise<ReadSpecSnapshotResult>;
}

/** Result of probing a local folder for its git origin remote. `unsupported`
 *  on web (no preload bridge); typed reasons on desktop let the renderer fall
 *  back to the manual entry form cleanly. */
export type DetectGitRemoteResult = {
  ok: boolean;
  remote_url?: string;
  reason?:
    | "not_absolute"
    | "not_git_repo"
    | "no_origin"
    | "git_not_found"
    | "error"
    | "unsupported";
  error?: string;
};

export type ReadSpecSnapshotResult =
  | { ok: true; snapshot: SyncSpecFromFilesRequest }
  | {
      ok: false;
      reason:
        | "not_absolute"
        | "not_found"
        | "not_a_directory"
        | "no_spec"
        | "error"
        | "unsupported";
      error?: string;
    };

/** Parsed view of a git remote URL — the bare host (for matching against a
 *  GitLab integration's base_url) and the project ref (group/sub/proj). */
export interface ParsedGitRemote {
  host: string;
  ref: string;
}

function readDesktopAPI(): DesktopLocalDirectoryAPI | undefined {
  if (typeof window === "undefined") return undefined;
  const api = (window as unknown as { desktopAPI?: DesktopLocalDirectoryAPI })
    .desktopAPI;
  return api;
}

/** True when the renderer is running inside the Electron desktop shell, as
 *  evidenced by the preload-exposed pickDirectory bridge. Avoids hard-coding
 *  navigator/process checks — those vary across electron-vite + jsdom tests. */
export function isDesktopShell(): boolean {
  const api = readDesktopAPI();
  return typeof api?.pickDirectory === "function";
}

export async function pickDirectory(
  defaultPath?: string,
): Promise<PickDirectoryResult> {
  const api = readDesktopAPI();
  if (!api?.pickDirectory) return { ok: false, reason: "unsupported" };
  return api.pickDirectory(defaultPath);
}

export async function validateLocalDirectory(
  path: string,
): Promise<ValidateLocalDirectoryResult> {
  const api = readDesktopAPI();
  if (!api?.validateLocalDirectory) return { ok: false, reason: "unsupported" };
  return api.validateLocalDirectory(path);
}

/** Read `remote.origin.url` for a local git working tree via the desktop
 *  preload. Returns `{ ok: false, reason: "unsupported" }` on web. */
export async function detectGitRemote(
  path: string,
): Promise<DetectGitRemoteResult> {
  const api = readDesktopAPI();
  if (!api?.detectGitRemote) return { ok: false, reason: "unsupported" };
  try {
    return await api.detectGitRemote(path);
  } catch (err) {
    return { ok: false, reason: "error", error: err instanceof Error ? err.message : String(err) };
  }
}

/** Read a local project's .spec folder via the desktop preload. Web cannot
 *  access local files, so it returns `unsupported`. */
export async function readSpecSnapshot(
  root: string,
): Promise<ReadSpecSnapshotResult> {
  const api = readDesktopAPI();
  if (!api?.readSpecSnapshot) return { ok: false, reason: "unsupported" };
  try {
    return await api.readSpecSnapshot(root);
  } catch (err) {
    return { ok: false, reason: "error", error: err instanceof Error ? err.message : String(err) };
  }
}

// SCP-style "git@host:path" — `new URL` can't parse it, so match it first.
// host = group 2, path = group 3.
const SCP_REMOTE_RE = /^[^@\s]+@([^:\s]+):(.+)$/;

/** Parse a git remote URL (SSH SCP form, ssh://, https://, http://, git://)
 *  into a bare host + project ref. Returns null when the URL can't be parsed
 *  or yields an empty ref. Host is lowercased and stripped of port/user;
 *  ref strips a trailing `.git` and leading/trailing slashes. */
export function parseGitRemote(raw: string): ParsedGitRemote | null {
  const url = raw.trim();
  if (!url) return null;

  // ssh://git@host:port/group/proj.git | https://host/group/proj.git | git://…
  if (/^[a-z][a-z0-9+.-]*:\/\//i.test(url)) {
    try {
      const parsed = new URL(url);
      const host = parsed.hostname.toLowerCase();
      const ref = cleanRef(parsed.pathname);
      if (!host || !ref) return null;
      return { host, ref };
    } catch {
      return null;
    }
  }

  // SCP form: git@host:group/proj.git
  const scp = SCP_REMOTE_RE.exec(url);
  if (scp) {
    const host = scp[1]!.toLowerCase();
    const ref = cleanRef(scp[2]!);
    if (!host || !ref) return null;
    return { host, ref };
  }

  return null;
}

function cleanRef(pathname: string): string {
  let ref = pathname.replace(/^\/+|\/+$/g, "");
  // A leading colon can survive on odd SCP variants; strip it.
  ref = ref.replace(/^:/, "");
  if (ref.endsWith(".git")) ref = ref.slice(0, -4);
  return ref.replace(/^\/+|\/+$/g, "");
}
