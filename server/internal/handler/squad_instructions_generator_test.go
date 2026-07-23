package handler

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/multica-ai/multica/server/internal/middleware"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

func TestRenderSquadInstructionsDraftIncludesAgentRouting(t *testing.T) {
	squad := db.Squad{Description: "product UI and backend integration work."}
	out := renderSquadInstructionsDraft(squad, []squadInstructionsMember{
		{
			Name:        "Frontend Agent",
			Kind:        "agent",
			Role:        "frontend implementer",
			Description: "React views, shared UI, interaction polish.",
			Skills:      []string{"react", "shadcn"},
		},
		{
			Name:        "Backend Agent",
			Kind:        "agent",
			Role:        "backend implementer",
			Description: "Go APIs, sqlc, PostgreSQL.",
			Skills:      []string{"go", "postgres"},
		},
	}, nil)

	for _, want := range []string{
		"## Delegation Strategy",
		"Use this squad for: product UI and backend integration work",
		"## Member Routing",
		"Frontend Agent: Use for work matching role frontend implementer; skills react, shadcn; description React views, shared UI, interaction polish.",
		"Backend Agent: Use for work matching role backend implementer; skills go, postgres; description Go APIs, sqlc, PostgreSQL.",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("generated instructions missing %q\n---\n%s", want, out)
		}
	}
}

func TestRenderSquadInstructionsDraftHandlesEmptyRunnableSquad(t *testing.T) {
	out := renderSquadInstructionsDraft(db.Squad{}, nil, []squadInstructionsMember{
		{Name: "Product Owner", Kind: "member", Role: "approval"},
	})

	for _, want := range []string{
		"no runnable non-leader agent members",
		"No runnable agent members are currently available",
		"## Human Escalation",
		"Product Owner: Escalate when human judgment or approval is needed for approval.",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("generated instructions missing %q\n---\n%s", want, out)
		}
	}
}

func TestGenerateSquadInstructionsHandlerUsesWorkspaceContext(t *testing.T) {
	leaderID, _ := seededLeaderAgent(t)
	squad := seedSquadForBriefing(t, leaderID, "Generator Handler Squad", "")

	body, err := json.Marshal(map[string]string{"mode": "template"})
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}
	req := httptest.NewRequest(http.MethodPost, "/api/squads/"+util.UUIDToString(squad.ID)+"/instructions/generate", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-User-ID", testUserID)
	req = withURLParam(req, "id", util.UUIDToString(squad.ID))
	req = req.WithContext(middleware.SetMemberContext(req.Context(), testWorkspaceID, db.Member{
		WorkspaceID: util.MustParseUUID(testWorkspaceID),
		UserID:      util.MustParseUUID(testUserID),
		Role:        "owner",
	}))

	w := httptest.NewRecorder()
	testHandler.GenerateSquadInstructions(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("GenerateSquadInstructions status = %d, body = %s", w.Code, w.Body.String())
	}

	var resp generatedSquadInstructions
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.Mode != "template" {
		t.Fatalf("mode = %q, want template", resp.Mode)
	}
	if !strings.Contains(resp.Instructions, "## Delegation Strategy") {
		t.Fatalf("instructions missing delegation strategy\n---\n%s", resp.Instructions)
	}
}
