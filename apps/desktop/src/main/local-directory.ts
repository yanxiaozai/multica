import { ipcMain, dialog, BrowserWindow } from "electron";
import { execFile } from "node:child_process";
import { access, stat } from "fs/promises";
import { constants as fsConstants } from "fs";
import { basename, isAbsolute } from "path";

export interface DetectGitRemoteResult {
  ok: boolean;
  /** The raw `remote.origin.url` value when ok=true. */
  remote_url?: string;
  /** Set when ok=false. */
  reason?: "not_absolute" | "not_git_repo" | "no_origin" | "git_not_found" | "error";
  error?: string;
}

export interface PickDirectoryResult {
  ok: boolean;
  path?: string;
  basename?: string;
  /** Set when ok=false. "cancelled" = user dismissed; otherwise an error blurb. */
  reason?: "cancelled" | "no_window" | "error";
  error?: string;
}

export interface ValidateLocalDirectoryResult {
  ok: boolean;
  /** When ok=false, identifies which check failed so the renderer can render a
   *  specific message without parsing free-form text. */
  reason?:
    | "not_absolute"
    | "not_found"
    | "not_a_directory"
    | "not_readable"
    | "not_writable"
    | "error";
  error?: string;
}

async function validateLocalDirectory(
  path: string,
): Promise<ValidateLocalDirectoryResult> {
  if (!path || !isAbsolute(path)) {
    return { ok: false, reason: "not_absolute" };
  }
  try {
    const st = await stat(path);
    if (!st.isDirectory()) return { ok: false, reason: "not_a_directory" };
  } catch (err) {
    const code = (err as NodeJS.ErrnoException).code;
    if (code === "ENOENT") return { ok: false, reason: "not_found" };
    return { ok: false, reason: "error", error: errorMessage(err) };
  }
  try {
    await access(path, fsConstants.R_OK);
  } catch {
    return { ok: false, reason: "not_readable" };
  }
  try {
    await access(path, fsConstants.W_OK);
  } catch {
    return { ok: false, reason: "not_writable" };
  }
  return { ok: true };
}

function errorMessage(err: unknown): string {
  return err instanceof Error ? err.message : String(err);
}

// Reads `remote.origin.url` for a local git working tree. Powers the
// project GitLab panel's "auto-detect from local repo" affordance. Never
// throws — any failure (not a repo, no origin, git missing) degrades to a
// typed reason so the renderer can fall back to the manual entry form.
function detectGitRemote(path: string): Promise<DetectGitRemoteResult> {
  if (!path || !isAbsolute(path)) {
    return Promise.resolve({ ok: false, reason: "not_absolute" });
  }
  return new Promise((resolve) => {
    // `git -C <path>` handles worktrees and linked .git layouts where a raw
    // .git/config read would miss. 5s cap so a hung git never blocks the IPC.
    execFile(
      "git",
      ["-C", path, "config", "--get", "remote.origin.url"],
      { timeout: 5_000, maxBuffer: 1 << 16 },
      (err, stdout) => {
        if (err) {
          const code = (err as NodeJS.ErrnoException).code;
          // ENOENT on the binary itself → git not installed. Everything else
          // (exit code 1, etc.) means the path isn't a repo or has no origin.
          if (code === "ENOENT") {
            resolve({ ok: false, reason: "git_not_found", error: errorMessage(err) });
            return;
          }
          const stderr = String((err as { stderr?: Buffer | string }).stderr ?? "");
          // `git config --get` exits 1 when the key is absent; a non-repo
          // surfaces as "not a git repository" on stderr.
          if (/not a git repository/i.test(stderr)) {
            resolve({ ok: false, reason: "not_git_repo", error: errorMessage(err) });
          } else {
            resolve({ ok: false, reason: "no_origin", error: errorMessage(err) });
          }
          return;
        }
        const remoteUrl = String(stdout).trim();
        if (!remoteUrl) {
          resolve({ ok: false, reason: "no_origin" });
          return;
        }
        resolve({ ok: true, remote_url: remoteUrl });
      },
    );
  });
}

export function setupLocalDirectory(
  windowGetter: () => BrowserWindow | null,
): void {
  ipcMain.handle(
    "local-directory:pick",
    async (_event, defaultPath?: string): Promise<PickDirectoryResult> => {
      const win = windowGetter();
      if (!win) return { ok: false, reason: "no_window" };
      try {
        const result = await dialog.showOpenDialog(win, {
          // Multiple-selection is intentionally disabled — a project_resource
          // points at a single directory, and the create flow expects one
          // path per click. Multi-add would have to be a separate UX.
          properties: ["openDirectory", "createDirectory"],
          ...(defaultPath ? { defaultPath } : {}),
        });
        if (result.canceled || result.filePaths.length === 0) {
          return { ok: false, reason: "cancelled" };
        }
        const picked = result.filePaths[0];
        if (!picked) return { ok: false, reason: "cancelled" };
        return { ok: true, path: picked, basename: basename(picked) };
      } catch (err) {
        return { ok: false, reason: "error", error: errorMessage(err) };
      }
    },
  );

  ipcMain.handle(
    "local-directory:validate",
    (_event, path: string): Promise<ValidateLocalDirectoryResult> =>
      validateLocalDirectory(path),
  );

  ipcMain.handle(
    "local-directory:detect-git-remote",
    (_event, path: string): Promise<DetectGitRemoteResult> => detectGitRemote(path),
  );
}
