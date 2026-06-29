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
						"issue": "ISS-1",
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
	if _, err := os.Stat(filepath.Join(root, ".spec/issues/ISS-1.md")); err != nil {
		t.Fatalf("issue file not written: %v", err)
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
