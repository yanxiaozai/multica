package handler

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

type generatedSquadInstructions struct {
	Instructions string   `json:"instructions"`
	Mode         string   `json:"mode"`
	Warnings     []string `json:"warnings"`
}

type squadInstructionsMember struct {
	Name        string
	Kind        string
	Role        string
	Description string
	Skills      []string
}

func (h *Handler) generateSquadInstructions(ctx context.Context, squad db.Squad) (generatedSquadInstructions, error) {
	members, err := h.Queries.ListSquadMembers(ctx, squad.ID)
	if err != nil {
		return generatedSquadInstructions{}, err
	}

	skillNamesByAgentID, skillsLoaded := loadSquadMemberSkillNames(ctx, h.Queries, members, util.UUIDToString(squad.LeaderID))
	warnings := []string{}
	if !skillsLoaded {
		warnings = append(warnings, "Agent skills could not be loaded; generated routing uses names, descriptions, and roles only.")
	}

	agentMembers := make([]squadInstructionsMember, 0, len(members))
	humanMembers := make([]squadInstructionsMember, 0, len(members))
	for _, member := range members {
		if member.MemberType == "agent" && util.UUIDToString(member.MemberID) == util.UUIDToString(squad.LeaderID) {
			continue
		}
		rendered, ok := h.squadInstructionMember(ctx, member, skillNamesByAgentID)
		if !ok {
			continue
		}
		if rendered.Kind == "agent" {
			agentMembers = append(agentMembers, rendered)
		} else {
			humanMembers = append(humanMembers, rendered)
		}
	}

	sort.SliceStable(agentMembers, func(i, j int) bool {
		return strings.ToLower(agentMembers[i].Name) < strings.ToLower(agentMembers[j].Name)
	})
	sort.SliceStable(humanMembers, func(i, j int) bool {
		return strings.ToLower(humanMembers[i].Name) < strings.ToLower(humanMembers[j].Name)
	})

	return generatedSquadInstructions{
		Instructions: renderSquadInstructionsDraft(squad, agentMembers, humanMembers),
		Mode:         "template",
		Warnings:     warnings,
	}, nil
}

func (h *Handler) squadInstructionMember(ctx context.Context, member db.SquadMember, skillNamesByAgentID map[string][]string) (squadInstructionsMember, bool) {
	role := strings.TrimSpace(member.Role)
	switch member.MemberType {
	case "agent":
		agent, err := h.Queries.GetAgent(ctx, member.MemberID)
		if err != nil || agent.ArchivedAt.Valid {
			return squadInstructionsMember{}, false
		}
		id := util.UUIDToString(member.MemberID)
		return squadInstructionsMember{
			Name:        agent.Name,
			Kind:        "agent",
			Role:        role,
			Description: strings.TrimSpace(agent.Description),
			Skills:      append([]string(nil), skillNamesByAgentID[id]...),
		}, true
	case "member":
		user, err := h.Queries.GetUser(ctx, member.MemberID)
		if err != nil {
			return squadInstructionsMember{}, false
		}
		return squadInstructionsMember{
			Name: user.Name,
			Kind: "member",
			Role: role,
		}, true
	default:
		return squadInstructionsMember{}, false
	}
}

func renderSquadInstructionsDraft(squad db.Squad, agentMembers, humanMembers []squadInstructionsMember) string {
	var b strings.Builder
	b.WriteString("## Delegation Strategy\n\n")
	if strings.TrimSpace(squad.Description) != "" {
		fmt.Fprintf(&b, "- Use this squad for: %s\n", sentenceFragment(squad.Description))
	}
	if len(agentMembers) == 0 {
		b.WriteString("- This squad has no runnable non-leader agent members yet. Explain the gap on the issue instead of doing unrelated work yourself.\n")
	} else {
		b.WriteString("- Match each issue to the member whose role, skills, and description best cover the requested work.\n")
		b.WriteString("- Prefer creating one child issue per assignee when independent work can proceed in parallel.\n")
		b.WriteString("- If a task spans multiple domains, delegate the first clear slice and ask that assignee to split follow-up child issues for the remaining work.\n")
		b.WriteString("- Do not delegate to archived agents or to people outside this squad.\n")
	}
	b.WriteString("- Keep delegation comments short: name the assignee, why they are the fit, and any extra constraints not already present in the issue.\n")

	b.WriteString("\n## Member Routing\n\n")
	if len(agentMembers) == 0 {
		b.WriteString("- No runnable agent members are currently available for delegation.\n")
	} else {
		for _, member := range agentMembers {
			fmt.Fprintf(&b, "- %s: %s\n", member.Name, routingSummary(member))
		}
	}

	if len(humanMembers) > 0 {
		b.WriteString("\n## Human Escalation\n\n")
		for _, member := range humanMembers {
			fmt.Fprintf(&b, "- %s: Escalate when human judgment or approval is needed%s.\n", member.Name, roleSuffix(member.Role))
		}
	}

	return strings.TrimSpace(b.String()) + "\n"
}

func routingSummary(member squadInstructionsMember) string {
	parts := make([]string, 0, 3)
	if member.Role != "" {
		parts = append(parts, "role "+member.Role)
	}
	if len(member.Skills) > 0 {
		parts = append(parts, "skills "+strings.Join(member.Skills, ", "))
	}
	if member.Description != "" {
		parts = append(parts, "description "+sentenceFragment(member.Description))
	}
	if len(parts) == 0 {
		return "Use when the issue clearly matches this agent's existing instructions or prior ownership."
	}
	return "Use for work matching " + strings.Join(parts, "; ") + "."
}

func roleSuffix(role string) string {
	if strings.TrimSpace(role) == "" {
		return ""
	}
	return " for " + strings.TrimSpace(role)
}

func sentenceFragment(value string) string {
	value = strings.Join(strings.Fields(value), " ")
	value = strings.TrimSuffix(value, ".")
	return value
}
