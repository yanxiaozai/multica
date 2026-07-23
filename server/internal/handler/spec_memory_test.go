package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

func TestIssueSpecMappingAndStateRoundTrip(t *testing.T) {
	issueID := createMetadataTestIssue(t, "Spec memory issue")
	_, moduleID := createSpecMemoryTestModule(t)

	body := map[string]any{
		"primary": map[string]any{
			"module_id": moduleID,
			"reason":    "main implementation surface",
		},
	}
	w := httptest.NewRecorder()
	req := newRequest("PUT", "/api/issues/"+issueID+"/spec/mapping", body)
	req = withURLParam(req, "id", issueID)
	testHandler.UpdateIssueSpecMapping(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("UpdateIssueSpecMapping: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	stateBody := map[string]any{
		"status":            "in_progress",
		"owner":             "mini",
		"current_stage":     "implementation",
		"current_loop":      "implementation",
		"last_result":       "pending",
		"open_questions":    []string{"Which audit owner?"},
		"blockers":          []string{"Waiting for product copy"},
		"next_handoff":      "lch verifies acceptance evidence",
		"audit_mode":        "skipped",
		"audit_skipped":     true,
		"audit_skip_reason": "docs-only check",
		"audit_skipped_by":  "boss",
	}
	w = httptest.NewRecorder()
	req = newRequest("PUT", "/api/issues/"+issueID+"/spec/state", stateBody)
	req = withURLParam(req, "id", issueID)
	testHandler.UpdateIssueSpecState(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("UpdateIssueSpecState: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	w = httptest.NewRecorder()
	req = newRequest("GET", "/api/issues/"+issueID+"/spec", nil)
	req = withURLParam(req, "id", issueID)
	testHandler.GetIssueSpec(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("GetIssueSpec: expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var got SpecIssueContextResponse
	if err := json.NewDecoder(w.Body).Decode(&got); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if got.State == nil || got.State.Owner != "mini" || got.State.CurrentStage != "implementation" {
		t.Fatalf("state not returned: %+v", got.State)
	}
	if len(got.Mappings) != 1 || got.Mappings[0].ModuleID == nil || *got.Mappings[0].ModuleID != moduleID {
		t.Fatalf("mapping not returned: %+v", got.Mappings)
	}
	if got.Metadata["spec_primary"] != moduleID || got.Metadata["audit_mode"] != "skipped" {
		t.Fatalf("metadata not synced: %+v", got.Metadata)
	}
}

func TestIssueSpecStateRequiresSkipReason(t *testing.T) {
	issueID := createMetadataTestIssue(t, "Spec memory validation")
	w := httptest.NewRecorder()
	req := newRequest("PUT", "/api/issues/"+issueID+"/spec/state", map[string]any{
		"audit_mode":    "skipped",
		"audit_skipped": true,
	})
	req = withURLParam(req, "id", issueID)
	testHandler.UpdateIssueSpecState(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestSyncSpecFromFilesImportsDurableDocs(t *testing.T) {
	issueID := createMetadataTestIssue(t, "Spec sync issue")
	body := map[string]any{
		"epics": []map[string]any{
			{
				"key":         "sync-test",
				"title":       "Sync Test",
				"description": "Imported epic",
				"stability":   "active",
				"documents": []map[string]any{
					{
						"doc_kind":    "index",
						"title":       "Sync Test",
						"body":        "# Sync Test\n",
						"source_path": ".spec/epics/sync-test/00-index.md",
					},
				},
				"modules": []map[string]any{
					{
						"key":         "appointment",
						"title":       "Appointment",
						"description": "Imported module",
						"stability":   "stable",
						"documents": []map[string]any{
							{
								"doc_kind":    "requirements",
								"title":       "Requirements",
								"body":        "# Requirements\n",
								"source_path": ".spec/epics/sync-test/appointment/requirements.md",
							},
						},
					},
				},
			},
		},
		"issues": []map[string]any{
			{
				"issue":          issueID,
				"primary":        "sync-test/appointment",
				"status":         "in_progress",
				"owner":          "mini",
				"current_stage":  "implementation",
				"current_loop":   "implementation",
				"last_result":    "pending",
				"open_questions": []string{"Question from file"},
				"blockers":       []string{"Blocker from file"},
				"next_handoff":   "lch verifies",
				"audit": map[string]any{
					"mode":    "required",
					"skipped": false,
				},
				"source_path": ".spec/issues/" + issueID + ".md",
			},
			{
				"issue":       "missing-issue",
				"source_path": ".spec/issues/missing-issue.md",
			},
		},
		"decisions": []map[string]any{
			{
				"title":       "Sync decision",
				"body":        "# Sync decision\n",
				"source_path": ".spec/decisions/sync-decision.md",
			},
		},
		"skipped_issue_files": []string{".spec/issues/40.md"},
	}
	w := httptest.NewRecorder()
	req := newRequest("POST", "/api/spec/sync/from-files", body)
	testHandler.SyncSpecFromFiles(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("SyncSpecFromFiles: expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var got SyncSpecFromFilesResponse
	if err := json.NewDecoder(w.Body).Decode(&got); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if got.Epics != 1 || got.Modules != 1 || got.Documents != 2 || got.Decisions != 1 {
		t.Fatalf("unexpected counts: %+v", got)
	}
	if got.Issues != 1 || got.IssueMappings != 1 {
		t.Fatalf("unexpected issue counts: %+v", got)
	}
	if len(got.SkippedIssueFiles) != 2 {
		t.Fatalf("skipped issue files = %+v", got.SkippedIssueFiles)
	}

	w = httptest.NewRecorder()
	req = newRequest("GET", "/api/spec/sync/to-files", nil)
	testHandler.SyncSpecToFiles(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("SyncSpecToFiles: expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var exported SyncSpecToFilesResponse
	if err := json.NewDecoder(w.Body).Decode(&exported); err != nil {
		t.Fatalf("decode export response: %v", err)
	}
	if len(exported.Epics) == 0 {
		t.Fatalf("exported epics = %+v", exported.Epics)
	}
	var syncEpic *SyncSpecEpicInput
	for i := range exported.Epics {
		if exported.Epics[i].Key == "sync-test" {
			syncEpic = &exported.Epics[i]
			break
		}
	}
	if syncEpic == nil || len(syncEpic.Modules) != 1 || len(syncEpic.Modules[0].Documents) != 1 {
		t.Fatalf("exported sync epic = %+v", exported.Epics)
	}
	if len(exported.Decisions) == 0 || exported.Decisions[0].Title != "Sync decision" {
		t.Fatalf("exported decisions = %+v", exported.Decisions)
	}
	createdIssue, err := testHandler.Queries.GetIssue(context.Background(), parseUUID(issueID))
	if err != nil {
		t.Fatalf("GetIssue(%s): %v", issueID, err)
	}
	ws, err := testHandler.Queries.GetWorkspace(context.Background(), parseUUID(testWorkspaceID))
	if err != nil {
		t.Fatalf("GetWorkspace: %v", err)
	}
	wantIssueRef := ws.IssuePrefix + "-" + strconv.Itoa(int(createdIssue.Number))
	var exportedIssue *SyncSpecIssueInput
	for i := range exported.Issues {
		if exported.Issues[i].Issue == wantIssueRef {
			exportedIssue = &exported.Issues[i]
			break
		}
	}
	if exportedIssue == nil || exportedIssue.Issue != wantIssueRef || exportedIssue.Primary != "sync-test/appointment" || exportedIssue.Owner != "mini" {
		t.Fatalf("exported issue = %+v", exported.Issues)
	}
}

func createSpecMemoryTestModule(t *testing.T) (string, string) {
	t.Helper()
	wsID := parseUUID(testWorkspaceID)
	epic, err := testHandler.Queries.UpsertSpecEpic(context.Background(), db.UpsertSpecEpicParams{
		WorkspaceID: wsID,
		Key:         "spec-memory-test",
		Title:       "Spec Memory Test",
		Description: "test epic",
		Stability:   "active",
	})
	if err != nil {
		t.Fatalf("seed spec epic: %v", err)
	}
	module, err := testHandler.Queries.UpsertSpecModule(context.Background(), db.UpsertSpecModuleParams{
		WorkspaceID: wsID,
		EpicID:      epic.ID,
		Key:         "issue-state",
		Title:       "Issue State",
		Description: "test module",
		Stability:   "active",
	})
	if err != nil {
		t.Fatalf("seed spec module: %v", err)
	}
	return uuidToString(epic.ID), uuidToString(module.ID)
}
