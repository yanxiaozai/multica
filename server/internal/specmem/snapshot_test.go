package specmem

import (
	"os"
	"path/filepath"
	"testing"
)

func TestReadFileSnapshotDurableDocs(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, ".spec/epics/scheduling/00-index.md", "# Scheduling\n\n- Stability: active\n\nBooking domain.")
	writeTestFile(t, root, ".spec/epics/scheduling/appointment/00-index.md", "# Appointment\n\n- Stability: stable\n\nAppointment module.")
	writeTestFile(t, root, ".spec/epics/scheduling/appointment/requirements.md", "# Requirements\n\n- Create booking")
	writeTestFile(t, root, ".spec/decisions/2026-06-29-booking.md", "# Booking Decision\n\nUse explicit slots.")
	writeTestFile(t, root, ".spec/issues/ISS-1.md", `---
issue: ISS-1
title: Booking fix
primary: scheduling/appointment
related:
  - scheduling/calendar - slot lookup
status: in_progress
owner: mini
current_stage: implementation
current_loop: implementation
last_result: pending
open_questions:
  - Which timezone?
blockers:
  - Copy missing
next_handoff: lch verifies
audit:
  mode: required
  skipped: false
updated_at: 2026-06-29T00:00:00Z
---
# Issue ISS-1
`)

	snapshot, err := ReadFileSnapshot(root)
	if err != nil {
		t.Fatalf("ReadFileSnapshot() error = %v", err)
	}
	if len(snapshot.Epics) != 1 {
		t.Fatalf("epics len = %d, want 1", len(snapshot.Epics))
	}
	epic := snapshot.Epics[0]
	if epic.Key != "scheduling" || epic.Title != "Scheduling" || epic.Stability != "active" {
		t.Fatalf("epic = %+v", epic)
	}
	if len(epic.Modules) != 1 {
		t.Fatalf("modules len = %d, want 1", len(epic.Modules))
	}
	module := epic.Modules[0]
	if module.Key != "appointment" || module.Title != "Appointment" || module.Stability != "stable" {
		t.Fatalf("module = %+v", module)
	}
	if len(module.Documents) != 2 {
		t.Fatalf("module docs len = %d, want 2", len(module.Documents))
	}
	if module.Documents[0].DocKind != "index" || module.Documents[1].DocKind != "requirements" {
		t.Fatalf("module doc kinds = %q, %q", module.Documents[0].DocKind, module.Documents[1].DocKind)
	}
	if len(snapshot.Decisions) != 1 || snapshot.Decisions[0].Title != "Booking Decision" {
		t.Fatalf("decisions = %+v", snapshot.Decisions)
	}
	if len(snapshot.Issues) != 1 || snapshot.Issues[0].Issue != "ISS-1" {
		t.Fatalf("issues = %+v", snapshot.Issues)
	}
	if snapshot.Issues[0].Primary != "scheduling/appointment" || len(snapshot.Issues[0].Related) != 1 {
		t.Fatalf("issue mapping = %+v", snapshot.Issues[0])
	}
	if len(snapshot.SkippedIssueFiles) != 0 {
		t.Fatalf("skipped issue files = %+v", snapshot.SkippedIssueFiles)
	}
}

func TestWriteFileSnapshotSkipsExistingUnlessForced(t *testing.T) {
	root := t.TempDir()
	snap := FileSnapshot{
		Epics: []FileEpic{
			{
				Key:       "scheduling",
				Title:     "Scheduling",
				Stability: "active",
				Documents: []FileDocument{
					{DocKind: "index", Body: "# Scheduling\n"},
				},
				Modules: []FileModule{
					{
						Key:       "appointment",
						Title:     "Appointment",
						Stability: "stable",
						Documents: []FileDocument{
							{DocKind: "requirements", Body: "# Requirements\n"},
						},
					},
				},
			},
		},
		Decisions: []FileDecision{{Title: "Decision", Body: "# Decision\n"}},
		Issues:    []FileIssue{{Issue: "ISS-1", Title: "Issue", Status: "todo", CurrentStage: "requirements", CurrentLoop: "requirements", LastResult: "pending", Audit: AuditState{Mode: "required"}}},
	}
	writeTestFile(t, root, ".spec/epics/scheduling/appointment/requirements.md", "local edits")

	summary, err := WriteFileSnapshot(root, snap, false)
	if err != nil {
		t.Fatalf("WriteFileSnapshot() error = %v", err)
	}
	if summary.Documents != 1 || len(summary.SkippedExisting) != 1 {
		t.Fatalf("summary = %+v", summary)
	}
	data, err := os.ReadFile(filepath.Join(root, ".spec/epics/scheduling/appointment/requirements.md"))
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "local edits" {
		t.Fatalf("existing file overwritten without force: %q", data)
	}

	summary, err = WriteFileSnapshot(root, snap, true)
	if err != nil {
		t.Fatalf("WriteFileSnapshot(force) error = %v", err)
	}
	if len(summary.SkippedExisting) != 0 {
		t.Fatalf("force skipped files: %+v", summary)
	}
	data, err = os.ReadFile(filepath.Join(root, ".spec/epics/scheduling/appointment/requirements.md"))
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "# Requirements\n" {
		t.Fatalf("force did not overwrite: %q", data)
	}
}

func TestWriteFileSnapshotSanitizesPathSegments(t *testing.T) {
	root := t.TempDir()
	snap := FileSnapshot{
		Epics: []FileEpic{
			{
				Key: "../outside",
				Modules: []FileModule{
					{
						Key: "../module",
						Documents: []FileDocument{
							{DocKind: "../requirements", Body: "# Requirements\n"},
						},
					},
				},
			},
		},
		Issues: []FileIssue{{Issue: "../ISS-1", Title: "Issue", Status: "todo", CurrentStage: "requirements", CurrentLoop: "requirements", LastResult: "pending", Audit: AuditState{Mode: "required"}}},
	}

	if _, err := WriteFileSnapshot(root, snap, false); err != nil {
		t.Fatalf("WriteFileSnapshot() error = %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "outside")); !os.IsNotExist(err) {
		t.Fatalf("path escaped root, stat err = %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, ".spec/epics/epic/module/document.md")); err != nil {
		t.Fatalf("sanitized document not written: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, ".spec/issues/issue.md")); err != nil {
		t.Fatalf("sanitized issue not written: %v", err)
	}
}

func writeTestFile(t *testing.T, root, rel, body string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}
