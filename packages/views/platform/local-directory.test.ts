import { describe, it, expect } from "vitest";
import { parseGitRemote } from "./local-directory";

describe("parseGitRemote", () => {
  it("parses the SSH SCP form", () => {
    expect(parseGitRemote("git@gitlab.example.com:group/proj.git")).toEqual({
      host: "gitlab.example.com",
      ref: "group/proj",
    });
  });

  it("parses a nested subgroup path in SCP form", () => {
    expect(parseGitRemote("git@gitlab.com:org/team/proj.git")).toEqual({
      host: "gitlab.com",
      ref: "org/team/proj",
    });
  });

  it("parses an HTTPS remote", () => {
    expect(parseGitRemote("https://gitlab.com/group/proj.git")).toEqual({
      host: "gitlab.com",
      ref: "group/proj",
    });
  });

  it("parses ssh:// with a port, stripping the port from the host", () => {
    expect(parseGitRemote("ssh://git@gitlab.example.com:2222/group/proj.git")).toEqual({
      host: "gitlab.example.com",
      ref: "group/proj",
    });
  });

  it("lowercases the host so SSH matches an HTTPS integration", () => {
    expect(parseGitRemote("git@GitLab.Example.COM:group/proj.git")?.host).toBe(
      "gitlab.example.com",
    );
  });

  it("strips a trailing .git but keeps refs that lack it", () => {
    expect(parseGitRemote("git@gitlab.com:group/proj.git")?.ref).toBe("group/proj");
    expect(parseGitRemote("git@gitlab.com:group/proj")?.ref).toBe("group/proj");
  });

  it("returns null for unparseable input", () => {
    expect(parseGitRemote("")).toBeNull();
    expect(parseGitRemote("not a url at all")).toBeNull();
    expect(parseGitRemote("git@host:")).toBeNull(); // empty path
    expect(parseGitRemote("git@:/nohost.git")).toBeNull();
  });
});
