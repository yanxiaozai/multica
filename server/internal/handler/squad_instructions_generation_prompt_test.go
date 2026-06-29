package handler

import (
	"context"
	"strings"
	"testing"

	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

func setAgentDocForGenerationTest(t *testing.T, agentID, description, instructions string) {
	t.Helper()
	if _, err := testPool.Exec(context.Background(), `
		UPDATE agent SET description = $2, instructions = $3 WHERE id = $1
	`, agentID, description, instructions); err != nil {
		t.Fatalf("set agent doc: %v", err)
	}
}

func TestBuildSquadInstructionsGenerationPromptUsesAgentDocs(t *testing.T) {
	ctx := context.Background()
	leaderID, _ := seededLeaderAgent(t)
	setAgentDocForGenerationTest(t, leaderID, "Routes work across the team.", "leader routing instructions")
	squad := seedSquadForBriefing(t, leaderID, "AI Doc Squad", "existing instructions")

	frontend := createHandlerTestAgent(t, "Frontend Summarized Agent", []byte("[]"))
	setAgentDocForGenerationTest(t, frontend, "React and shared views.", "frontend specialist instructions")
	addAgentMember(t, squad.ID, frontend, "frontend implementer")
	assignSkillToAgent(t, frontend, "react")

	backend := createHandlerTestAgent(t, "Backend Summarized Agent", []byte("[]"))
	setAgentDocForGenerationTest(t, backend, "Go and sqlc APIs.", "backend specialist instructions")
	addAgentMember(t, squad.ID, backend, "backend implementer")
	assignSkillToAgent(t, backend, "go")

	archived := createHandlerTestAgent(t, "Archived Summarized Agent", []byte("[]"))
	setAgentDocForGenerationTest(t, archived, "Archived.", "archived specialist instructions")
	addAgentMember(t, squad.ID, archived, "do not use")
	if _, err := testPool.Exec(ctx, `UPDATE agent SET archived_at = now(), archived_by = $2 WHERE id = $1`, archived, testUserID); err != nil {
		t.Fatalf("archive agent: %v", err)
	}

	_, userID, userName := seededHumanMember(t)
	addHumanMember(t, squad.ID, userID, "approval")

	prompt, err := testHandler.buildSquadInstructionsGenerationPrompt(ctx, squad, "current editor draft")
	if err != nil {
		t.Fatalf("build prompt: %v", err)
	}

	for _, want := range []string{
		"## Required Output",
		"Return only markdown suitable for squad.instructions",
		"## Leader Agent",
		"leader routing instructions",
		"### Frontend Summarized Agent",
		"frontend specialist instructions",
		"Skills: react",
		"### Backend Summarized Agent",
		"backend specialist instructions",
		"Skills: go",
		"Existing draft: current editor draft",
		"## Human Escalation Contacts",
		userName + ": approval",
	} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("prompt missing %q\n--- prompt ---\n%s", want, prompt)
		}
	}
	if strings.Contains(prompt, "archived specialist instructions") {
		t.Fatalf("prompt included archived agent\n--- prompt ---\n%s", prompt)
	}
}

func TestBoundedAgentDocMarksTruncatedInput(t *testing.T) {
	longDoc := strings.Repeat("A", squadInstructionsAgentDocMaxChars+20)
	got := boundedAgentDoc(longDoc)
	if len(got) <= squadInstructionsAgentDocMaxChars {
		t.Fatalf("expected truncation marker to increase rendered length")
	}
	if !strings.Contains(got, "[truncated: agent instructions exceeded prompt budget]") {
		t.Fatalf("missing truncation marker: %q", got[len(got)-80:])
	}
}

func TestBuildSquadInstructionsGenerationPromptEmptyAgentDoc(t *testing.T) {
	ctx := context.Background()
	leaderID, _ := seededLeaderAgent(t)
	squad := seedSquadForBriefing(t, leaderID, "AI Empty Doc Squad", "")

	agent := createHandlerTestAgent(t, "Empty Doc Agent", []byte("[]"))
	addAgentMember(t, squad.ID, agent, "")

	loaded, err := testHandler.Queries.GetSquadInWorkspace(ctx, db.GetSquadInWorkspaceParams{
		ID:          squad.ID,
		WorkspaceID: util.MustParseUUID(testWorkspaceID),
	})
	if err != nil {
		t.Fatalf("reload squad: %v", err)
	}

	prompt, err := testHandler.buildSquadInstructionsGenerationPrompt(ctx, loaded, "")
	if err != nil {
		t.Fatalf("build prompt: %v", err)
	}
	if !strings.Contains(prompt, "### Empty Doc Agent") || !strings.Contains(prompt, "Instructions:\n[empty]") {
		t.Fatalf("expected empty instructions marker\n--- prompt ---\n%s", prompt)
	}
}
