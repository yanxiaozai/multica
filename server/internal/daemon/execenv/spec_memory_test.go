package execenv

import (
	"strings"
	"testing"
)

func TestRuntimeBriefIncludesSpecMemory(t *testing.T) {
	withLegacyBrief(t)
	brief := buildMetaSkillContent("codex", TaskContextForEnv{})
	assertSpecMemoryBrief(t, brief)
}

func TestSlimRuntimeBriefIncludesSpecMemory(t *testing.T) {
	withSlimBrief(t)
	brief := buildMetaSkillContent("codex", TaskContextForEnv{})
	assertSpecMemoryBrief(t, brief)
}

func TestRuntimeBriefIncludesAgentSpecProfile(t *testing.T) {
	withLegacyBrief(t)
	brief := buildMetaSkillContent("codex", TaskContextForEnv{
		AgentName:        "seven",
		AgentSpecProfile: `{"primary_outputs":["requirements.md"],"audit_role":"requirements-gate"}`,
	})
	assertAgentSpecProfile(t, brief)
}

func TestSlimRuntimeBriefIncludesAgentSpecProfile(t *testing.T) {
	withSlimBrief(t)
	brief := buildMetaSkillContent("codex", TaskContextForEnv{
		AgentName:        "seven",
		AgentSpecProfile: `{"primary_outputs":["requirements.md"],"audit_role":"requirements-gate"}`,
	})
	assertAgentSpecProfile(t, brief)
}

func withLegacyBrief(t *testing.T) {
	t.Helper()
	saved := runtimeFlags.Load()
	runtimeFlags.Store(nil)
	t.Cleanup(func() { runtimeFlags.Store(saved) })
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
		"Do not write current owner",
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
