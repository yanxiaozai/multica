package specmem

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestInitUpdateReadAndHandoff(t *testing.T) {
	root := t.TempDir()
	now := time.Date(2026, 6, 29, 10, 0, 0, 0, time.UTC)

	if err := Init(root, InitOptions{Epic: "agent-memory", Module: "spec-cli", Issue: "40", Now: now}); err != nil {
		t.Fatalf("Init() error = %v", err)
	}

	status, err := GetStatus(root, StatusOptions{Epic: "agent-memory", Module: "spec-cli", Issue: "40"})
	if err != nil {
		t.Fatalf("GetStatus() error = %v", err)
	}
	if !status.ProjectIndexExists || !status.ModuleIndexExists || !status.IssueStateExists {
		t.Fatalf("expected project, module, and issue state, got %#v", status)
	}
	if status.IssueState.Primary != "agent-memory/spec-cli" {
		t.Fatalf("unexpected issue primary mapping: %#v", status.IssueState)
	}
	if status.IssueState.Audit.Mode != "required" || status.IssueState.Audit.Skipped {
		t.Fatalf("unexpected audit state: %#v", status.IssueState.Audit)
	}
	if _, err := os.Stat(filepath.Join(root, ".spec", "epics", "agent-memory", "spec-cli", "handoff.md")); !os.IsNotExist(err) {
		t.Fatalf("handoff.md should not be a module document, err=%v", err)
	}

	if err := Update(root, UpdateOptions{
		Epic:            "agent-memory",
		Module:          "spec-cli",
		Issue:           "40",
		Doc:             "requirements",
		Content:         "# Requirements\n\nPersist project memory.\n",
		Status:          "in_progress",
		Stage:           "design",
		Owner:           "mini",
		SkipAuditReason: "tiny documentation-only change",
		Actor:           "tester",
		Now:             now.Add(time.Hour),
	}); err != nil {
		t.Fatalf("Update() error = %v", err)
	}

	read, err := Read(root, ReadOptions{Epic: "agent-memory", Module: "spec-cli", Doc: "requirements"})
	if err != nil {
		t.Fatalf("Read() error = %v", err)
	}
	if !strings.Contains(read, "Persist project memory") {
		t.Fatalf("requirements doc was not updated: %q", read)
	}

	status, err = GetStatus(root, StatusOptions{Epic: "agent-memory", Module: "spec-cli", Issue: "40"})
	if err != nil {
		t.Fatalf("GetStatus() after update error = %v", err)
	}
	if status.IssueState.Status != "in_progress" || status.IssueState.CurrentStage != "design" || status.IssueState.Owner != "mini" {
		t.Fatalf("issue state not updated: %#v", status.IssueState)
	}
	if !status.IssueState.Audit.Skipped || status.IssueState.Audit.SkipReason == "" || status.IssueState.Audit.SkippedBy != "tester" {
		t.Fatalf("audit skip not recorded: %#v", status.IssueState.Audit)
	}

	if err := AppendHandoff(root, HandoffOptions{
		Epic:    "agent-memory",
		Module:  "spec-cli",
		Issue:   "40",
		Summary: "Next agent should wire the UI.",
		Actor:   "tester",
		To:      "lch",
		Now:     now.Add(2 * time.Hour),
	}); err != nil {
		t.Fatalf("AppendHandoff() error = %v", err)
	}
	handoff, err := os.ReadFile(filepath.Join(root, ".spec", "issues", "40.md"))
	if err != nil {
		t.Fatalf("read issue state: %v", err)
	}
	if !strings.Contains(string(handoff), "Next agent should wire the UI.") || !strings.Contains(string(handoff), "next_handoff: lch") {
		t.Fatalf("handoff note missing: %s", handoff)
	}
}

func TestRejectsIssueStateUpdatesWithoutIssue(t *testing.T) {
	root := t.TempDir()
	err := Update(root, UpdateOptions{
		Epic:   "agent-memory",
		Module: "spec-cli",
		Stage:  "design",
	})
	if err == nil {
		t.Fatal("expected issue state update without --issue to fail")
	}
	if !strings.Contains(err.Error(), "--issue is required") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestBindIssueCreatesMapping(t *testing.T) {
	root := t.TempDir()
	if err := BindIssue(root, BindIssueOptions{
		Issue:   "MUL-40",
		Title:   "Appointment management",
		Primary: "lms-core/appointment",
		Related: []string{"lms-core/customer:customer detail link"},
	}); err != nil {
		t.Fatalf("BindIssue() error = %v", err)
	}
	status, err := GetStatus(root, StatusOptions{Issue: "MUL-40"})
	if err != nil {
		t.Fatalf("GetStatus() error = %v", err)
	}
	if status.IssueState.Primary != "lms-core/appointment" || len(status.IssueState.Related) != 1 {
		t.Fatalf("mapping not recorded: %#v", status.IssueState)
	}
}

func TestRejectsUnsafeScopeSegments(t *testing.T) {
	root := t.TempDir()
	err := Init(root, InitOptions{Epic: "../escape", Module: "spec-cli"})
	if err == nil {
		t.Fatal("expected unsafe epic to be rejected")
	}
	if !strings.Contains(err.Error(), "invalid epic") {
		t.Fatalf("unexpected error: %v", err)
	}

	err = Init(root, InitOptions{Epic: "safe", Module: "nested/path"})
	if err == nil {
		t.Fatal("expected unsafe module to be rejected")
	}
	if !strings.Contains(err.Error(), "invalid module") {
		t.Fatalf("unexpected error: %v", err)
	}
}
