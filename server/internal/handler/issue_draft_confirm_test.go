package handler

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCreateRemoteIssueUsesSupportedGlabDescriptionFlag(t *testing.T) {
	binDir := t.TempDir()
	argsFile := filepath.Join(t.TempDir(), "glab-args.txt")
	writeExecutable(t, filepath.Join(binDir, "git"), `#!/bin/sh
if [ "$1" = "remote" ] && [ "$2" = "get-url" ] && [ "$3" = "origin" ]; then
  echo "https://gitlab.com/acme/repo.git"
  exit 0
fi
exit 1
`)
	writeExecutable(t, filepath.Join(binDir, "glab"), `#!/bin/sh
printf '%s\n' "$@" > "$GLAB_ARGS_FILE"
echo "https://gitlab.com/acme/repo/-/issues/1"
`)
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("GLAB_ARGS_FILE", argsFile)

	url, err := createRemoteIssue(context.Background(), t.TempDir(), "New issue", "## Summary")
	if err != nil {
		t.Fatalf("createRemoteIssue() error = %v", err)
	}
	if url != "https://gitlab.com/acme/repo/-/issues/1" {
		t.Fatalf("createRemoteIssue() url = %q", url)
	}
	rawArgs, err := os.ReadFile(argsFile)
	if err != nil {
		t.Fatalf("read glab args: %v", err)
	}
	args := strings.Split(strings.TrimSpace(string(rawArgs)), "\n")
	for _, arg := range args {
		if arg == "--description-file" {
			t.Fatalf("glab args must not use unsupported --description-file: %v", args)
		}
	}
	if !containsArgPair(args, "--description", "## Summary") {
		t.Fatalf("glab args missing --description body: %v", args)
	}
	if !containsArg(args, "--yes") {
		t.Fatalf("glab args missing --yes: %v", args)
	}
}

func writeExecutable(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o755); err != nil {
		t.Fatalf("write executable %s: %v", path, err)
	}
}

func containsArg(args []string, want string) bool {
	for _, arg := range args {
		if arg == want {
			return true
		}
	}
	return false
}

func containsArgPair(args []string, key, value string) bool {
	for i := 0; i+1 < len(args); i++ {
		if args[i] == key && args[i+1] == value {
			return true
		}
	}
	return false
}
