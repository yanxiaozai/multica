import { ipcMain } from "electron";
import { readdir, readFile, stat } from "fs/promises";
import { basename, extname, isAbsolute, join, relative, sep } from "path";
import { parseFrontmatter } from "@multica/core/skills/frontmatter";

type AuditState = {
  mode: string;
  skipped: boolean;
  skip_reason?: string;
  skipped_by?: string;
  skipped_at?: string;
};

type FileDocument = {
  doc_kind: string;
  title: string;
  body: string;
  source_path: string;
};

type FileSnapshot = {
  epics: Array<{
    key: string;
    title: string;
    description: string;
    stability: string;
    documents: FileDocument[];
    modules: Array<{
      key: string;
      title: string;
      description: string;
      stability: string;
      documents: FileDocument[];
    }>;
  }>;
  issues: Array<{
    issue: string;
    title: string;
    primary: string;
    related: string[];
    status: string;
    owner: string;
    current_stage: string;
    current_loop: string;
    last_result: string;
    open_questions: string[];
    blockers: string[];
    next_handoff: string;
    audit: AuditState;
    updated_at: string;
    source_path: string;
  }>;
  decisions: Array<{
    title: string;
    body: string;
    actor: string;
    source_path: string;
  }>;
  skipped_issue_files: string[];
};

export type ReadSpecSnapshotResult =
  | { ok: true; snapshot: FileSnapshot }
  | {
      ok: false;
      reason: "not_absolute" | "not_found" | "not_a_directory" | "no_spec" | "error";
      error?: string;
    };

const MODULE_DOC_FILES: Record<string, string> = {
  requirements: "requirements.md",
  design: "design.md",
  architecture: "architecture.md",
  frontend: "frontend.md",
  backend: "backend.md",
  implementation: "implementation.md",
  testing: "testing.md",
  review: "review.md",
  acceptance: "acceptance.md",
};

const EPIC_DOC_FILES: Record<string, string> = {
  plan: "plan.md",
  risks: "risks.md",
  requirements: "requirements.md",
  design: "design.md",
  architecture: "architecture.md",
  frontend: "frontend.md",
  backend: "backend.md",
  implementation: "implementation.md",
  testing: "testing.md",
  review: "review.md",
  acceptance: "acceptance.md",
};

export function setupSpecMemory(): void {
  ipcMain.handle(
    "spec-memory:read-snapshot",
    async (_event, root: string): Promise<ReadSpecSnapshotResult> => {
      try {
        return await readSpecSnapshot(root);
      } catch (err) {
        return { ok: false, reason: "error", error: errorMessage(err) };
      }
    },
  );
}

async function readSpecSnapshot(root: string): Promise<ReadSpecSnapshotResult> {
  if (!root || !isAbsolute(root)) return { ok: false, reason: "not_absolute" };
  const rootStat = await stat(root).catch((err: NodeJS.ErrnoException) => {
    if (err.code === "ENOENT") return null;
    throw err;
  });
  if (!rootStat) return { ok: false, reason: "not_found" };
  if (!rootStat.isDirectory()) return { ok: false, reason: "not_a_directory" };

  const specDir = join(root, ".spec");
  const specStat = await stat(specDir).catch((err: NodeJS.ErrnoException) => {
    if (err.code === "ENOENT") return null;
    throw err;
  });
  if (!specStat) return { ok: false, reason: "no_spec" };
  if (!specStat.isDirectory()) return { ok: false, reason: "not_a_directory" };

  const snapshot: FileSnapshot = {
    epics: await readEpics(root),
    issues: [],
    decisions: await readDecisions(root),
    skipped_issue_files: [],
  };
  const issueResult = await readIssues(root);
  snapshot.issues = issueResult.issues;
  snapshot.skipped_issue_files = issueResult.skipped;
  return { ok: true, snapshot };
}

async function readEpics(root: string): Promise<FileSnapshot["epics"]> {
  const epicsDir = join(root, ".spec", "epics");
  const entries = await readDirSafe(epicsDir);
  const epics: FileSnapshot["epics"] = [];
  for (const entry of entries) {
    if (!entry.isDirectory()) continue;
    const epicKey = entry.name;
    const epicDir = join(epicsDir, epicKey);
    const indexDoc = await readDocument(root, join(epicDir, "00-index.md"), "index");
    const fallbackTitle = titleCase(epicKey.replaceAll("-", " "));
    const documents: FileDocument[] = indexDoc ? [indexDoc] : [];
    for (const docKind of Object.keys(EPIC_DOC_FILES).sort()) {
      const doc = await readDocument(root, join(epicDir, EPIC_DOC_FILES[docKind]!), docKind);
      if (doc) documents.push(doc);
    }
    const modules: FileSnapshot["epics"][number]["modules"] = [];
    for (const moduleEntry of await readDirSafe(epicDir)) {
      if (!moduleEntry.isDirectory()) continue;
      modules.push(await readModule(root, epicKey, moduleEntry.name));
    }
    modules.sort((a, b) => a.key.localeCompare(b.key));
    epics.push({
      key: epicKey,
      title: indexDoc?.title || fallbackTitle,
      description: indexDoc ? firstParagraph(indexDoc.body) : "",
      stability: indexDoc ? extractStability(indexDoc.body) || "draft" : "draft",
      documents,
      modules,
    });
  }
  epics.sort((a, b) => a.key.localeCompare(b.key));
  return epics;
}

async function readModule(root: string, epicKey: string, moduleKey: string) {
  const moduleDir = join(root, ".spec", "epics", epicKey, moduleKey);
  const indexDoc = await readDocument(root, join(moduleDir, "00-index.md"), "index");
  const fallbackTitle = titleCase(moduleKey.replaceAll("-", " "));
  const documents: FileDocument[] = indexDoc ? [indexDoc] : [];
  for (const docKind of Object.keys(MODULE_DOC_FILES).sort()) {
    const doc = await readDocument(root, join(moduleDir, MODULE_DOC_FILES[docKind]!), docKind);
    if (doc) documents.push(doc);
  }
  return {
    key: moduleKey,
    title: indexDoc?.title || fallbackTitle,
    description: indexDoc ? firstParagraph(indexDoc.body) : "",
    stability: indexDoc ? extractStability(indexDoc.body) || "draft" : "draft",
    documents,
  };
}

async function readDecisions(root: string): Promise<FileSnapshot["decisions"]> {
  const decisionsDir = join(root, ".spec", "decisions");
  const entries = await readDirSafe(decisionsDir);
  const decisions: FileSnapshot["decisions"] = [];
  for (const entry of entries) {
    if (!entry.isFile() || extname(entry.name) !== ".md") continue;
    const path = join(decisionsDir, entry.name);
    const body = await readFile(path, "utf8");
    decisions.push({
      title: firstMarkdownHeading(body, basename(entry.name, ".md")),
      body,
      actor: "",
      source_path: relPath(root, path),
    });
  }
  decisions.sort((a, b) => a.source_path.localeCompare(b.source_path));
  return decisions;
}

async function readIssues(root: string): Promise<{
  issues: FileSnapshot["issues"];
  skipped: string[];
}> {
  const issuesDir = join(root, ".spec", "issues");
  const entries = await readDirSafe(issuesDir);
  const issues: FileSnapshot["issues"] = [];
  const skipped: string[] = [];
  for (const entry of entries) {
    if (!entry.isFile() || extname(entry.name) !== ".md") continue;
    const path = join(issuesDir, entry.name);
    const issueRef = basename(entry.name, ".md");
    try {
      issues.push(await readIssue(root, path, issueRef));
    } catch {
      skipped.push(relPath(root, path));
    }
  }
  issues.sort((a, b) => a.source_path.localeCompare(b.source_path));
  skipped.sort();
  return { issues, skipped };
}

async function readIssue(root: string, path: string, issueRef: string) {
  const raw = await readFile(path, "utf8");
  const parsed = parseFrontmatter(raw).frontmatter ?? {};
  const audit = parseObject(parsed.audit);
  return {
    issue: stringValue(parsed.issue) || issueRef,
    title: stringValue(parsed.title),
    primary: stringValue(parsed.primary),
    related: parseStringArray(parsed.related),
    status: stringValue(parsed.status) || "todo",
    owner: stringValue(parsed.owner),
    current_stage: stringValue(parsed.current_stage) || "requirements",
    current_loop: stringValue(parsed.current_loop) || "requirements",
    last_result: stringValue(parsed.last_result) || "pending",
    open_questions: parseStringArray(parsed.open_questions),
    blockers: parseStringArray(parsed.blockers),
    next_handoff: stringValue(parsed.next_handoff),
    audit: {
      mode: stringValue(audit.mode) || "required",
      skipped: booleanValue(audit.skipped),
      skip_reason: stringValue(audit.skip_reason),
      skipped_by: stringValue(audit.skipped_by),
      skipped_at: stringValue(audit.skipped_at),
    },
    updated_at: stringValue(parsed.updated_at),
    source_path: relPath(root, path),
  };
}

async function readDocument(root: string, path: string, docKind: string): Promise<FileDocument | null> {
  const body = await readFile(path, "utf8").catch((err: NodeJS.ErrnoException) => {
    if (err.code === "ENOENT") return null;
    throw err;
  });
  if (body === null) return null;
  return {
    doc_kind: docKind,
    title: firstMarkdownHeading(body, titleCase(docKind)),
    body,
    source_path: relPath(root, path),
  };
}

async function readDirSafe(path: string) {
  return readdir(path, { withFileTypes: true }).catch((err: NodeJS.ErrnoException) => {
    if (err.code === "ENOENT") return [];
    throw err;
  });
}

function parseObject(value: unknown): Record<string, unknown> {
  if (!value) return {};
  if (typeof value === "object" && !Array.isArray(value)) {
    return value as Record<string, unknown>;
  }
  if (typeof value !== "string") return {};
  try {
    const parsed = JSON.parse(value);
    return parsed && typeof parsed === "object" && !Array.isArray(parsed)
      ? parsed as Record<string, unknown>
      : {};
  } catch {
    return {};
  }
}

function parseStringArray(value: unknown): string[] {
  if (Array.isArray(value)) return value.map(stringValue).filter(Boolean);
  if (typeof value !== "string" || value.trim() === "") return [];
  try {
    const parsed = JSON.parse(value);
    if (Array.isArray(parsed)) return parsed.map(stringValue).filter(Boolean);
  } catch {
    // fall through
  }
  return value.split("\n").map((item) => item.replace(/^-\s*/, "").trim()).filter(Boolean);
}

function stringValue(value: unknown): string {
  if (typeof value === "string") return value;
  if (typeof value === "number" || typeof value === "boolean") return String(value);
  return "";
}

function booleanValue(value: unknown): boolean {
  if (typeof value === "boolean") return value;
  if (typeof value === "string") return value === "true";
  return false;
}

function firstMarkdownHeading(body: string, fallback: string): string {
  for (const line of body.split("\n")) {
    const trimmed = line.trim();
    if (trimmed.startsWith("# ")) {
      const title = trimmed.slice(2).trim();
      if (title) return title;
    }
  }
  return fallback;
}

function firstParagraph(body: string): string {
  for (const block of body.split(/\n\s*\n/)) {
    const trimmed = block.trim();
    if (trimmed && !trimmed.startsWith("#") && !trimmed.startsWith("-")) {
      return trimmed;
    }
  }
  return "";
}

function extractStability(body: string): string {
  for (const line of body.split("\n")) {
    const trimmed = line.trim();
    const lower = trimmed.toLowerCase();
    if (!lower.startsWith("- stability:") && !lower.startsWith("stability:")) continue;
    const value = trimmed.split(":").slice(1).join(":").trim().toLowerCase();
    if (["draft", "active", "stable", "deprecated"].includes(value)) return value;
  }
  return "";
}

function titleCase(value: string): string {
  return value
    .split(/\s+/)
    .filter(Boolean)
    .map((part) => part.slice(0, 1).toUpperCase() + part.slice(1))
    .join(" ");
}

function relPath(root: string, path: string): string {
  return relative(root, path).split(sep).join("/");
}

function errorMessage(err: unknown): string {
  return err instanceof Error ? err.message : String(err);
}
