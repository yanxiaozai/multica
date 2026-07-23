package handler

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

const squadInstructionsAgentDocMaxChars = 12000

type squadInstructionsGenerationAgentDoc struct {
	Name         string
	Role         string
	Description  string
	Skills       []string
	Instructions string
}

type squadInstructionsGenerationHumanContact struct {
	Name string
	Role string
}

func boundedAgentDoc(value string) string {
	value = strings.TrimSpace(value)
	if len(value) <= squadInstructionsAgentDocMaxChars {
		return value
	}
	return strings.TrimSpace(value[:squadInstructionsAgentDocMaxChars]) + "\n\n[truncated: agent instructions exceeded prompt budget]"
}

func (h *Handler) buildSquadInstructionsGenerationPrompt(ctx context.Context, squad db.Squad, draft string) (string, error) {
	members, err := h.Queries.ListSquadMembers(ctx, squad.ID)
	if err != nil {
		return "", err
	}

	leader, err := h.Queries.GetAgent(ctx, squad.LeaderID)
	if err != nil {
		return "", err
	}

	skillNamesByAgentID, _ := loadSquadMemberSkillNames(ctx, h.Queries, members, util.UUIDToString(squad.LeaderID))
	leaderDoc := squadInstructionsGenerationAgentDoc{
		Name:         leader.Name,
		Description:  strings.TrimSpace(leader.Description),
		Skills:       append([]string(nil), skillNamesByAgentID[util.UUIDToString(leader.ID)]...),
		Instructions: boundedAgentDoc(leader.Instructions),
	}

	agentDocs := make([]squadInstructionsGenerationAgentDoc, 0, len(members))
	humanContacts := make([]squadInstructionsGenerationHumanContact, 0)
	for _, member := range members {
		if member.MemberType == "agent" && util.UUIDToString(member.MemberID) == util.UUIDToString(squad.LeaderID) {
			continue
		}
		switch member.MemberType {
		case "agent":
			agent, err := h.Queries.GetAgent(ctx, member.MemberID)
			if err != nil || agent.ArchivedAt.Valid {
				continue
			}
			agentID := util.UUIDToString(agent.ID)
			agentDocs = append(agentDocs, squadInstructionsGenerationAgentDoc{
				Name:         agent.Name,
				Role:         strings.TrimSpace(member.Role),
				Description:  strings.TrimSpace(agent.Description),
				Skills:       append([]string(nil), skillNamesByAgentID[agentID]...),
				Instructions: boundedAgentDoc(agent.Instructions),
			})
		case "member":
			user, err := h.Queries.GetUser(ctx, member.MemberID)
			if err != nil {
				continue
			}
			humanContacts = append(humanContacts, squadInstructionsGenerationHumanContact{
				Name: user.Name,
				Role: strings.TrimSpace(member.Role),
			})
		}
	}

	sort.SliceStable(agentDocs, func(i, j int) bool {
		return strings.ToLower(agentDocs[i].Name) < strings.ToLower(agentDocs[j].Name)
	})
	sort.SliceStable(humanContacts, func(i, j int) bool {
		return strings.ToLower(humanContacts[i].Name) < strings.ToLower(humanContacts[j].Name)
	})

	return renderSquadInstructionsGenerationPrompt(squad, draft, leaderDoc, agentDocs, humanContacts), nil
}

func renderSquadInstructionsGenerationPrompt(
	squad db.Squad,
	draft string,
	leader squadInstructionsGenerationAgentDoc,
	agents []squadInstructionsGenerationAgentDoc,
	humans []squadInstructionsGenerationHumanContact,
) string {
	var b strings.Builder
	b.WriteString("# Squad Instructions Generation\n\n")
	b.WriteString("## Goal\n\n")
	b.WriteString("Generate markdown for `squad.instructions` by summarizing the saved agent instructions documents below. The output guides the squad leader when delegating future issues.\n\n")
	b.WriteString("## Required Output\n\n")
	b.WriteString("Return only markdown suitable for squad.instructions. Do not create issues, comments, files, commits, or chat messages.\n\n")
	b.WriteString("Use exactly these top-level sections:\n\n")
	b.WriteString("- ## Delegation Strategy\n")
	b.WriteString("- ## Member Routing\n")
	b.WriteString("- ## Coordination Rules\n")
	b.WriteString("- ## Escalation\n\n")
	b.WriteString("Rules:\n\n")
	b.WriteString("- Summarize what each agent is best suited for based on its document.\n")
	b.WriteString("- Mention tasks an agent should not receive when the document makes that clear.\n")
	b.WriteString("- Do not invent skills that are not supported by the agent documents, descriptions, roles, or assigned skills.\n")
	b.WriteString("- Do not rewrite or mention the fixed Squad Operating Protocol; it is injected separately.\n\n")

	b.WriteString("## Squad\n\n")
	fmt.Fprintf(&b, "Name: %s\n", squad.Name)
	fmt.Fprintf(&b, "Description: %s\n", emptyMarker(squad.Description))
	fmt.Fprintf(&b, "Existing draft: %s\n\n", emptyMarker(draft))

	b.WriteString("## Leader Agent\n\n")
	writeAgentDoc(&b, leader)

	b.WriteString("\n## Runnable Agent Members\n\n")
	if len(agents) == 0 {
		b.WriteString("No runnable non-leader agent members are currently in this squad.\n")
	} else {
		for _, agent := range agents {
			fmt.Fprintf(&b, "### %s\n\n", agent.Name)
			writeAgentDoc(&b, agent)
			b.WriteString("\n")
		}
	}

	b.WriteString("## Human Escalation Contacts\n\n")
	if len(humans) == 0 {
		b.WriteString("No human escalation contacts are currently in this squad.\n")
	} else {
		for _, human := range humans {
			fmt.Fprintf(&b, "- %s: %s\n", human.Name, emptyMarker(human.Role))
		}
	}

	return strings.TrimSpace(b.String()) + "\n"
}

func writeAgentDoc(b *strings.Builder, agent squadInstructionsGenerationAgentDoc) {
	fmt.Fprintf(b, "Name: %s\n", agent.Name)
	fmt.Fprintf(b, "Role: %s\n", emptyMarker(agent.Role))
	fmt.Fprintf(b, "Description: %s\n", emptyMarker(agent.Description))
	if len(agent.Skills) == 0 {
		b.WriteString("Skills: none assigned\n")
	} else {
		fmt.Fprintf(b, "Skills: %s\n", strings.Join(agent.Skills, ", "))
	}
	b.WriteString("Instructions:\n")
	if strings.TrimSpace(agent.Instructions) == "" {
		b.WriteString("[empty]\n")
	} else {
		b.WriteString(agent.Instructions)
		b.WriteString("\n")
	}
}

func emptyMarker(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "[empty]"
	}
	return value
}
