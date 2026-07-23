package main

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/multica-ai/multica/server/internal/cli"
	"github.com/spf13/cobra"
)

func newSpecSyncTestCommand(root string) *cobra.Command {
	cmd := &cobra.Command{Use: "sync"}
	cmd.Flags().String("root", root, "")
	cmd.Flags().Bool("from-files", false, "")
	cmd.Flags().Bool("to-files", false, "")
	cmd.Flags().Bool("force", false, "")
	cmd.Flags().String("output", "table", "")
	cmd.Flags().String("server-url", "", "")
	cmd.Flags().String("workspace-id", "", "")
	cmd.Flags().String("profile", "", "")
	return cmd
}

func TestSpecSyncRequiresExplicitDirection(t *testing.T) {
	cmd := newSpecSyncTestCommand(t.TempDir())
	err := runSpecSync(cmd, nil)
	if err == nil {
		t.Fatal("runSpecSync() expected direction error")
	}
	if !strings.Contains(err.Error(), "exactly one of --from-files or --to-files") {
		t.Fatalf("runSpecSync() error = %q", err.Error())
	}
}

func TestSpecSyncFromFilesPostsSnapshot(t *testing.T) {
	root := t.TempDir()
	writeCmdSpecTestFile(t, root, ".spec/epics/scheduling/appointment/requirements.md", "# Requirements\n")

	var sawRequest bool
	original := newSpecAPIClient
	t.Cleanup(func() { newSpecAPIClient = original })
	newSpecAPIClient = func(cmd *cobra.Command) (*cli.APIClient, error) {
		client := cli.NewAPIClient("http://multica.test", "ws-1", "test-token")
		client.HTTPClient = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			if r.URL.Path != "/api/spec/sync/from-files" {
				t.Fatalf("path = %q", r.URL.Path)
			}
			if got := r.Header.Get("X-Workspace-ID"); got != "ws-1" {
				t.Fatalf("workspace header = %q", got)
			}
			var body map[string]any
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatalf("decode body: %v", err)
			}
			epics, _ := body["epics"].([]any)
			if len(epics) != 1 {
				t.Fatalf("epics len = %d, want 1", len(epics))
			}
			sawRequest = true
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     http.Header{"Content-Type": []string{"application/json"}},
				Body:       io.NopCloser(bytes.NewBufferString(`{"epics":1,"modules":1,"documents":1,"decisions":0,"skipped_issue_files":[]}`)),
				Request:    r,
			}, nil
		})}
		return client, nil
	}

	cmd := newSpecSyncTestCommand(root)
	if err := cmd.Flags().Set("from-files", "true"); err != nil {
		t.Fatal(err)
	}
	if err := runSpecSync(cmd, nil); err != nil {
		t.Fatalf("runSpecSync() error = %v", err)
	}
	if !sawRequest {
		t.Fatal("server did not receive sync request")
	}
}

func TestSpecSyncToFilesWritesSnapshot(t *testing.T) {
	root := t.TempDir()

	var sawRequest bool
	original := newSpecAPIClient
	t.Cleanup(func() { newSpecAPIClient = original })
	newSpecAPIClient = func(cmd *cobra.Command) (*cli.APIClient, error) {
		client := cli.NewAPIClient("http://multica.test", "ws-1", "test-token")
		client.HTTPClient = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			if r.URL.Path != "/api/spec/sync/to-files" {
				t.Fatalf("path = %q", r.URL.Path)
			}
			sawRequest = true
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     http.Header{"Content-Type": []string{"application/json"}},
				Body: io.NopCloser(bytes.NewBufferString(`{
					"epics": [{
						"key": "scheduling",
						"title": "Scheduling",
						"stability": "active",
						"documents": [{"doc_kind": "index", "body": "# Scheduling\n"}],
						"modules": [{
							"key": "appointment",
							"title": "Appointment",
							"stability": "stable",
							"documents": [{"doc_kind": "requirements", "body": "# Requirements\n"}]
						}]
					}],
					"issues": [{
						"issue": "#50",
						"title": "Issue",
						"primary": "scheduling/appointment",
						"status": "todo",
						"current_stage": "requirements",
						"current_loop": "requirements",
						"last_result": "pending",
						"audit": {"mode": "required", "skipped": false}
					}],
					"decisions": [{"title": "Decision", "body": "# Decision\n"}]
				}`)),
				Request: r,
			}, nil
		})}
		return client, nil
	}

	cmd := newSpecSyncTestCommand(root)
	if err := cmd.Flags().Set("to-files", "true"); err != nil {
		t.Fatal(err)
	}
	if err := runSpecSync(cmd, nil); err != nil {
		t.Fatalf("runSpecSync() error = %v", err)
	}
	if !sawRequest {
		t.Fatal("server did not receive sync request")
	}
	assertCmdSpecFile(t, root, ".spec/epics/scheduling/00-index.md", "# Scheduling\n")
	assertCmdSpecFile(t, root, ".spec/epics/scheduling/appointment/requirements.md", "# Requirements\n")
	if _, err := os.Stat(filepath.Join(root, ".spec/issues/50.md")); err != nil {
		t.Fatalf("issue file not written: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, ".spec/issues/#50.md")); !os.IsNotExist(err) {
		t.Fatalf("raw issue ref file should not be written, err=%v", err)
	}
}

func TestSpecSyncToFilesDoesNotOverwriteWithoutForce(t *testing.T) {
	root := t.TempDir()
	writeCmdSpecTestFile(t, root, ".spec/epics/scheduling/appointment/requirements.md", "local edits")

	original := newSpecAPIClient
	t.Cleanup(func() { newSpecAPIClient = original })
	newSpecAPIClient = func(cmd *cobra.Command) (*cli.APIClient, error) {
		client := cli.NewAPIClient("http://multica.test", "ws-1", "test-token")
		client.HTTPClient = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     http.Header{"Content-Type": []string{"application/json"}},
				Body:       io.NopCloser(bytes.NewBufferString(`{"epics":[{"key":"scheduling","documents":[],"modules":[{"key":"appointment","documents":[{"doc_kind":"requirements","body":"backend copy"}]}]}]}`)),
				Request:    r,
			}, nil
		})}
		return client, nil
	}

	cmd := newSpecSyncTestCommand(root)
	if err := cmd.Flags().Set("to-files", "true"); err != nil {
		t.Fatal(err)
	}
	if err := runSpecSync(cmd, nil); err != nil {
		t.Fatalf("runSpecSync() error = %v", err)
	}
	assertCmdSpecFile(t, root, ".spec/epics/scheduling/appointment/requirements.md", "local edits")
}

func TestSpecWorkflowCommandsWriteIssueState(t *testing.T) {
	root := t.TempDir()

	start := newSpecWorkflowTestCommand(root, "start")
	mustSetFlag(t, start, "comment", "comment-a")
	mustSetFlag(t, start, "intent", "new_request")
	mustSetFlag(t, start, "status", "active")
	mustSetFlag(t, start, "owner", "mini")
	mustSetFlag(t, start, "stage", "implementing")
	mustSetFlag(t, start, "next-action", "patch booking page")
	if err := runSpecWorkflowStart(start, []string{"ISS-1"}); err != nil {
		t.Fatalf("runSpecWorkflowStart() error = %v", err)
	}

	update := newSpecWorkflowTestCommand(root, "update")
	mustSetFlag(t, update, "comment", "comment-a")
	mustSetFlag(t, update, "status", "interrupted")
	mustSetFlag(t, update, "stage", "testing")
	if err := runSpecWorkflowUpdate(update, []string{"ISS-1"}); err != nil {
		t.Fatalf("runSpecWorkflowUpdate() error = %v", err)
	}

	resume := newSpecWorkflowTestCommand(root, "resume")
	mustSetFlag(t, resume, "from", "comment-a")
	mustSetFlag(t, resume, "trigger", "comment-b")
	mustSetFlag(t, resume, "owner", "boss")
	if err := runSpecWorkflowResume(resume, []string{"ISS-1"}); err != nil {
		t.Fatalf("runSpecWorkflowResume() error = %v", err)
	}

	data, err := os.ReadFile(filepath.Join(root, ".spec", "issues", "ISS-1.md"))
	if err != nil {
		t.Fatalf("read issue workflow file: %v", err)
	}
	text := string(data)
	for _, want := range []string{
		"comment_workflows:",
		"comment_id: comment-a",
		"status: interrupted",
		"comment_id: comment-b",
		"intent: resume",
		"parent_workflow: comment-a",
		"Active thread: comment-b",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("workflow command output missing %q:\n%s", want, text)
		}
	}
}

func newSpecWorkflowTestCommand(root, name string) *cobra.Command {
	cmd := &cobra.Command{Use: name}
	cmd.Flags().String("root", root, "")
	cmd.Flags().String("output", "json", "")
	cmd.Flags().String("comment", "", "")
	cmd.Flags().String("intent", "", "")
	cmd.Flags().String("status", "", "")
	cmd.Flags().String("owner", "", "")
	cmd.Flags().String("stage", "", "")
	cmd.Flags().String("last-result", "", "")
	cmd.Flags().String("next-action", "", "")
	cmd.Flags().String("parent-workflow", "", "")
	cmd.Flags().String("trigger-comment", "", "")
	cmd.Flags().String("from", "", "")
	cmd.Flags().String("trigger", "", "")
	return cmd
}

func mustSetFlag(t *testing.T, cmd *cobra.Command, name, value string) {
	t.Helper()
	if err := cmd.Flags().Set(name, value); err != nil {
		t.Fatalf("set %s: %v", name, err)
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (fn roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return fn(r)
}

func writeCmdSpecTestFile(t *testing.T, root, rel, body string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func assertCmdSpecFile(t *testing.T, root, rel, want string) {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
	if err != nil {
		t.Fatalf("read %s: %v", rel, err)
	}
	if string(data) != want {
		t.Fatalf("%s = %q, want %q", rel, data, want)
	}
}
