// Desktop-only helpers for importing Claude Code sub-agents
// (`~/.claude/agents/*.md`) into a workspace.
//
// These mirror the local-directory.ts pattern: the preload exposes a
// `desktopAPI.listClaudeAgentFiles` bridge, and this module wraps it so view
// components can SSR-render on web (where `window.desktopAPI` is undefined)
// and degrade to empty results instead of crashing. Frontmatter parsing
// reuses @multica/core/skills/frontmatter so the YAML split stays consistent
// with skill import.

import { parseFrontmatter } from "@multica/core/skills/frontmatter";

/** Raw file as returned by the main-process file reader. */
export interface ClaudeAgentFile {
  fileName: string;
  rawContent: string;
}

/** Parsed view of one Claude Code sub-agent file, ready to seed an agent. */
export interface ClaudeAgentDefinition {
  /** Display name — frontmatter `name`, falling back to the file name. */
  name: string;
  description: string;
  /** Frontmatter `model` (sonnet/opus/...). Not mapped to a runtime model —
   *  the import dialog leaves model selection to the user. */
  model: string;
  /** Frontmatter `color`, kept for potential avatar hints. */
  color: string;
  /** Full markdown body (instructions), including any Agent Contract block. */
  body: string;
}

interface DesktopClaudeAgentsAPI {
  listClaudeAgentFiles?: () => Promise<ClaudeAgentFile[]>;
}

function readDesktopAPI(): DesktopClaudeAgentsAPI | undefined {
  if (typeof window === "undefined") return undefined;
  return (window as unknown as { desktopAPI?: DesktopClaudeAgentsAPI })
    .desktopAPI;
}

/** True when the renderer is running inside the Electron desktop shell AND
 *  the preload has exposed the Claude-agents bridge. Web builds return false,
 *  so the "Import from Claude Code" button never renders there. */
export function isClaudeAgentImportSupported(): boolean {
  const api = readDesktopAPI();
  return typeof api?.listClaudeAgentFiles === "function";
}

/** Fetch the raw Claude agent files from disk. Returns [] on web or when the
 *  `~/.claude/agents` directory is missing. */
export async function listClaudeAgentFiles(): Promise<ClaudeAgentFile[]> {
  const api = readDesktopAPI();
  if (!api?.listClaudeAgentFiles) return [];
  try {
    return await api.listClaudeAgentFiles();
  } catch (err) {
    console.warn("[claude-agents] failed to list files:", err);
    return [];
  }
}

/** Split a raw `.md` file into the fields needed to create a Multica agent.
 *  Falls back to `fileName` when the frontmatter omits `name`, so every file
 *  yields a creatable agent regardless of authoring style. */
export function parseClaudeAgentFile(
  file: ClaudeAgentFile,
): ClaudeAgentDefinition {
  const { frontmatter, body } = parseFrontmatter(file.rawContent);
  const fm = frontmatter ?? {};
  return {
    name: (fm.name ?? file.fileName).trim(),
    description: (fm.description ?? "").trim(),
    model: (fm.model ?? "").trim(),
    color: (fm.color ?? "").trim(),
    body: body.trim(),
  };
}
