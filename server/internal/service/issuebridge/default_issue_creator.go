package issuebridge

const (
	DefaultIssueCreatorName        = "issue-creator"
	DefaultIssueCreatorDescription = "Create clear, actionable GitLab issues from Multica issues."
	DefaultIssueCreatorContent     = `---
name: issue-creator
description: Create clear, actionable GitLab issues from Multica issues.
---

# Issue Creator

Use this skill when turning a Multica issue into a GitLab issue.

## Workflow

1. Preserve the user's intent from the Multica issue.
2. Write a concise GitLab issue title that describes the outcome.
3. Include important Multica context in the GitLab issue body, including acceptance details, project context, and agent-relevant notes when present.
4. Keep the issue actionable for a human or agent assignee.

## Output

Create one GitLab issue with:

- A specific title.
- A body that summarizes the source Multica issue.
- Any relevant labels or project context from Multica.
`
)
