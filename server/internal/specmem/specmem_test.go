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
	issueFile, err := os.ReadFile(filepath.Join(root, ".spec", "issues", "40.md"))
	if err != nil {
		t.Fatalf("read issue state template: %v", err)
	}
	for _, want := range []string{
		"## Current Workflow",
		"Active thread:",
		"## Comment Workflows",
		"### <trigger_comment_id>",
		"Intent: new_request | resume | constraint | question | no_action",
		"Status: active | interrupted | completed | superseded | blocked",
		"Next action:",
	} {
		if !strings.Contains(string(issueFile), want) {
			t.Fatalf("issue state template missing %q:\n%s", want, issueFile)
		}
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

func TestIssueReferenceUsesRepositoryIssueNumberFile(t *testing.T) {
	root := t.TempDir()
	now := time.Date(2026, 7, 1, 10, 0, 0, 0, time.UTC)

	if err := Init(root, InitOptions{Issue: "#50", Title: "Slow comment workflow", Now: now}); err != nil {
		t.Fatalf("Init() error = %v", err)
	}

	if _, err := os.Stat(filepath.Join(root, ".spec", "issues", "50.md")); err != nil {
		t.Fatalf("canonical issue file not written: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, ".spec", "issues", "#50.md")); !os.IsNotExist(err) {
		t.Fatalf("raw issue ref file should not be written, err=%v", err)
	}

	status, err := GetStatus(root, StatusOptions{Issue: "#50"})
	if err != nil {
		t.Fatalf("GetStatus() error = %v", err)
	}
	if !status.IssueStateExists {
		t.Fatal("canonical issue state was not found")
	}
	if status.Issue != "50" || status.IssueState.Issue != "50" {
		t.Fatalf("issue ref was not canonicalized: status=%#v state=%#v", status.Issue, status.IssueState.Issue)
	}

	if _, err := StartWorkflow(root, WorkflowOptions{
		Issue:     "#50",
		CommentID: "comment-a",
		Intent:    "new_request",
		Status:    "active",
		Now:       now.Add(time.Minute),
	}); err != nil {
		t.Fatalf("StartWorkflow() error = %v", err)
	}
	data, err := os.ReadFile(filepath.Join(root, ".spec", "issues", "50.md"))
	if err != nil {
		t.Fatalf("read canonical issue file: %v", err)
	}
	if !strings.Contains(string(data), "comment_id: comment-a") {
		t.Fatalf("workflow was not written to canonical issue file:\n%s", data)
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

func TestIssueCommentWorkflowLifecycle(t *testing.T) {
	root := t.TempDir()
	now := time.Date(2026, 7, 1, 12, 0, 0, 0, time.UTC)
	if err := Init(root, InitOptions{Issue: "ISS-1", Now: now}); err != nil {
		t.Fatalf("Init() error = %v", err)
	}

	started, err := StartWorkflow(root, WorkflowOptions{
		Issue:          "ISS-1",
		CommentID:      "comment-a",
		Intent:         "new_request",
		Status:         "active",
		Owner:          "mini",
		Stage:          "implementing",
		LastResult:     "scoped appointment copy",
		NextAction:     "patch booking page",
		TriggerComment: "comment-a",
		Now:            now.Add(time.Minute),
	})
	if err != nil {
		t.Fatalf("StartWorkflow() error = %v", err)
	}
	if started.CommentID != "comment-a" || started.Intent != "new_request" || started.Status != "active" {
		t.Fatalf("unexpected started workflow: %#v", started)
	}

	updated, err := UpdateWorkflow(root, WorkflowOptions{
		Issue:      "ISS-1",
		CommentID:  "comment-a",
		Status:     "interrupted",
		Stage:      "testing",
		NextAction: "run mini-program regression",
		Now:        now.Add(2 * time.Minute),
	})
	if err != nil {
		t.Fatalf("UpdateWorkflow() error = %v", err)
	}
	if updated.Intent != "new_request" || updated.Owner != "mini" {
		t.Fatalf("UpdateWorkflow should preserve omitted fields: %#v", updated)
	}
	if updated.Status != "interrupted" || updated.Stage != "testing" || updated.NextAction != "run mini-program regression" {
		t.Fatalf("UpdateWorkflow did not apply changes: %#v", updated)
	}

	resumed, err := ResumeWorkflow(root, ResumeWorkflowOptions{
		Issue:       "ISS-1",
		FromComment: "comment-a",
		Trigger:     "comment-b",
		Owner:       "boss",
		Now:         now.Add(3 * time.Minute),
	})
	if err != nil {
		t.Fatalf("ResumeWorkflow() error = %v", err)
	}
	if resumed.CommentID != "comment-b" || resumed.Intent != "resume" || resumed.ParentWorkflow != "comment-a" {
		t.Fatalf("unexpected resumed workflow: %#v", resumed)
	}
	if resumed.NextAction != "run mini-program regression" {
		t.Fatalf("resume should inherit next action, got %#v", resumed)
	}

	workflows, current, err := ListWorkflows(root, "ISS-1")
	if err != nil {
		t.Fatalf("ListWorkflows() error = %v", err)
	}
	if len(workflows) != 2 {
		t.Fatalf("workflow count = %d, want 2: %#v", len(workflows), workflows)
	}
	if current.ActiveThread != "comment-b" || current.ResumeTarget != "comment-a" || current.Owner != "boss" {
		t.Fatalf("unexpected current workflow: %#v", current)
	}

	got, err := GetWorkflow(root, "ISS-1", "comment-a")
	if err != nil {
		t.Fatalf("GetWorkflow() error = %v", err)
	}
	if got.Status != "interrupted" {
		t.Fatalf("GetWorkflow() = %#v", got)
	}

	read, err := os.ReadFile(filepath.Join(root, ".spec", "issues", "ISS-1.md"))
	if err != nil {
		t.Fatalf("read issue state: %v", err)
	}
	text := string(read)
	for _, want := range []string{
		"current_workflow:",
		"comment_workflows:",
		"## Current Workflow",
		"Active thread: comment-b",
		"Resume target: comment-a",
		"### comment-a",
		"Intent: new_request",
		"Status: interrupted",
		"### comment-b",
		"Intent: resume",
		"Parent workflow: comment-a",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("workflow file missing %q:\n%s", want, text)
		}
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
