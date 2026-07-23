import { ipcMain } from "electron";
import { readdir, readFile } from "fs/promises";
import { homedir } from "os";
import { join } from "path";

// A single Claude Code sub-agent file as read off disk. The renderer parses
// the YAML frontmatter (reusing @multica/core/skills/frontmatter) so the main
// process stays a dumb file reader — same division of labour as
// local-directory.ts, and it keeps the YAML dependency out of the main bundle.

/** Raw contents of one `~/.claude/agents/*.md` file. */
export interface ClaudeAgentFile {
  /** File name without extension, e.g. "shuai" for shuai.md. Used as the
   *  display-name fallback when the frontmatter has no `name` field. */
  fileName: string;
  /** Full file contents (frontmatter + markdown body), unmodified. */
  rawContent: string;
}

async function listClaudeAgentFiles(): Promise<ClaudeAgentFile[]> {
  const dir = join(homedir(), ".claude", "agents");
  let names: string[];
  try {
    names = await readdir(dir);
  } catch (err) {
    // Missing directory or permission error: degrade to "nothing to import"
    // rather than surfacing an error to the user. Common on machines that
    // have never run Claude Code.
    const code = (err as NodeJS.ErrnoException).code;
    if (code !== "ENOENT" && code !== "ENOTDIR") {
      console.warn("[claude-agents] failed to read agents dir:", dir, err);
    }
    return [];
  }

  const mdNames = names.filter((n) => n.toLowerCase().endsWith(".md"));
  const files: ClaudeAgentFile[] = [];
  for (const name of mdNames) {
    try {
      const rawContent = await readFile(join(dir, name), "utf-8");
      files.push({ fileName: name.slice(0, -3), rawContent });
    } catch (err) {
      // Skip unreadable files (broken symlink, permission) but keep going —
      // one bad file shouldn't hide the rest.
      console.warn("[claude-agents] failed to read file:", name, err);
    }
  }
  return files;
}

export function setupClaudeAgents(): void {
  ipcMain.handle("claude-agents:list", (): Promise<ClaudeAgentFile[]> =>
    listClaudeAgentFiles(),
  );
}
