import { describe, it, expect } from "vitest";
import {
  parseClaudeAgentFile,
  isClaudeAgentImportSupported,
} from "./claude-agents";

describe("parseClaudeAgentFile", () => {
  it("maps frontmatter name/description and full body", () => {
    const raw =
      "---\n" +
      'name: shuai\n' +
      'description: "Java/Spring 后端工程师"\n' +
      "model: sonnet\n" +
      "color: orange\n" +
      "---\n" +
      "你是一位后端工程师。\n\n## 核心职责\n\n实现服务。";
    const def = parseClaudeAgentFile({ fileName: "shuai", rawContent: raw });
    expect(def.name).toBe("shuai");
    expect(def.description).toBe("Java/Spring 后端工程师");
    expect(def.model).toBe("sonnet");
    expect(def.color).toBe("orange");
    expect(def.body).toBe("你是一位后端工程师。\n\n## 核心职责\n\n实现服务。");
  });

  it("keeps the Agent Contract JSON block inside the body", () => {
    const raw =
      "---\nname: han\ndescription: 架构师\n---\n" +
      "<!-- AGENT_CONTRACT_START -->\n" +
      "## Agent Contract\n```json\n" +
      '{\n  "role": "架构师",\n  "use_when": ["架构方案"]\n}\n' +
      "```\n" +
      "<!-- AGENT_CONTRACT_END -->\n\n正文。";
    const def = parseClaudeAgentFile({ fileName: "han", rawContent: raw });
    expect(def.body).toContain("AGENT_CONTRACT_START");
    expect(def.body).toContain('"role": "架构师"');
    expect(def.body).toContain("正文。");
  });

  it("falls back to the file name when frontmatter omits name", () => {
    const raw = "---\ndescription: no name here\n---\nbody";
    const def = parseClaudeAgentFile({ fileName: "nameless", rawContent: raw });
    expect(def.name).toBe("nameless");
    expect(def.description).toBe("no name here");
  });

  it("falls back to the file name when there is no frontmatter at all", () => {
    const def = parseClaudeAgentFile({
      fileName: "plain",
      rawContent: "just a markdown body",
    });
    expect(def.name).toBe("plain");
    expect(def.description).toBe("");
    expect(def.body).toBe("just a markdown body");
  });

  it("trims trailing whitespace carried through the pipeline", () => {
    // Real Claude agent files author frontmatter at column 0; the trim here
    // only guards against stray trailing spaces on the resolved values.
    const raw =
      "---\nname: \"spaced\" \ndescription: \"desc\" \n---\nbody";
    const def = parseClaudeAgentFile({ fileName: "x", rawContent: raw });
    expect(def.name).toBe("spaced");
    expect(def.description).toBe("desc");
  });
});

describe("isClaudeAgentImportSupported", () => {
  it("returns false when window.desktopAPI is absent (web)", () => {
    expect(isClaudeAgentImportSupported()).toBe(false);
  });

  it("returns true when the preload exposes listClaudeAgentFiles", () => {
    const prev = (globalThis as { window?: unknown }).window;
    (globalThis as { window: unknown }).window = {
      desktopAPI: {
        listClaudeAgentFiles: async () => [],
      },
    };
    try {
      expect(isClaudeAgentImportSupported()).toBe(true);
    } finally {
      if (prev === undefined) {
        delete (globalThis as { window?: unknown }).window;
      } else {
        (globalThis as { window: unknown }).window = prev;
      }
    }
  });
});
