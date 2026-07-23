package execenv

import (
	"strings"
	"testing"
)

func TestRuntimeBriefIncludesSpecMemory(t *testing.T) {
	brief := buildMetaSkillContent("codex", TaskContextForEnv{})
	assertSpecMemoryBrief(t, brief)
}

func TestRuntimeBriefIncludesAgentSpecProfile(t *testing.T) {
	brief := buildMetaSkillContent("codex", TaskContextForEnv{
		AgentName:        "seven",
		AgentSpecProfile: `{"primary_outputs":["requirements.md"],"audit_role":"requirements-gate"}`,
	})
	assertAgentSpecProfile(t, brief)
}

func assertSpecMemoryBrief(t *testing.T, brief string) {
	t.Helper()
	for _, want := range []string{
		"## Spec Memory",
		"multica spec status --output json",
		"`spec-memory` skill",
		"multica spec read --issue <id> --epic <name> --module <name>",
		".spec/issues/<issue-id>.md",
		"audit gate is required by default",
		"--skip-audit-reason",
		"multica spec handoff --issue <id>",
		"multica spec workflow list <issue-id>",
		"multica spec workflow resume <issue-id> --from <old_comment_id> --trigger <trigger_comment_id>",
		"Do not write current owner",
		"## Current Workflow",
		"## Comment Workflows",
		"trigger_comment_id",
		"Intent: new_request | resume | constraint | question | no_action",
		"Status: active | interrupted | completed | superseded | blocked",
	} {
		if !strings.Contains(brief, want) {
			t.Fatalf("runtime brief missing %q\n%s", want, brief)
		}
	}
}

func assertAgentSpecProfile(t *testing.T, brief string) {
	t.Helper()
	for _, want := range []string{
		"## Spec Memory Role Profile",
		"structured contribution profile",
		`"primary_outputs":["requirements.md"]`,
		`"audit_role":"requirements-gate"`,
	} {
		if !strings.Contains(brief, want) {
			t.Fatalf("runtime brief missing %q\n%s", want, brief)
		}
	}
}
