package execenv

import "strings"

func writeSpecMemory(b *strings.Builder) {
	b.WriteString("## Spec Memory\n\n")
	b.WriteString("Projects may contain a `.spec/` directory managed by `multica spec`. Treat it as durable project memory with separate issue execution state and long-lived epic/module documents.\n\n")
	b.WriteString("- Use the `spec-memory` skill whenever a task reads, writes, creates, audits, or reasons about `.spec`, issue mapping, handoff, audit, LOOP Chain, SDD state, or long-term memory.\n")
	b.WriteString("- On entry, run `multica spec status --output json`; if an issue is named, include `--issue <id>`, and if an epic/module is named, include `--epic <name> --module <name>`.\n")
	b.WriteString("- Read durable context with `multica spec read --issue <id> --epic <name> --module <name>` or a focused `--doc issue|requirements|design|architecture|frontend|backend|testing|review|acceptance`.\n")
	b.WriteString("- Write issue execution state to `.spec/issues/<issue-id>.md` via `multica spec update --issue <id>` or `multica spec handoff --issue <id>`.\n")
	b.WriteString("- Write durable requirements, designs, architecture, implementation notes, tests, reviews, acceptance evidence, decisions, and standards to epic/module docs.\n")
	b.WriteString("- Do not write current owner, current stage, issue blockers, open questions, LOOP Chain, or Next Handoff into module `00-index.md`.\n")
	b.WriteString("- The audit gate is required by default. Skip it only when explicitly allowed, and record that with `multica spec update --issue <id> --skip-audit-reason \"...\"`.\n\n")
}
