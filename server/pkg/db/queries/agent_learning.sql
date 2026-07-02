-- Agent learning reports

-- name: CreateAgentLearningReport :one
INSERT INTO agent_learning_report (
    workspace_id, issue_id, task_id, agent_id, summary, metadata
) VALUES (
    @workspace_id,
    sqlc.narg(issue_id),
    sqlc.narg(task_id),
    @agent_id,
    @summary,
    COALESCE(sqlc.narg('metadata'), '{}'::jsonb)
)
RETURNING *;

-- name: CreateAgentEvolutionSuggestion :one
INSERT INTO agent_evolution_suggestion (
    report_id, workspace_id, scope, risk, status, target_type, target_id,
    title, rationale, proposed_content, metadata
) VALUES (
    @report_id,
    @workspace_id,
    @scope,
    @risk,
    COALESCE(sqlc.narg('status'), 'pending'),
    @target_type,
    sqlc.narg(target_id),
    @title,
    @rationale,
    @proposed_content,
    COALESCE(sqlc.narg('metadata'), '{}'::jsonb)
)
RETURNING *;

-- name: ListAgentLearningReportsByIssue :many
SELECT * FROM agent_learning_report
WHERE workspace_id = @workspace_id AND issue_id = @issue_id
ORDER BY created_at DESC;

-- name: ListAgentLearningReportsByAgent :many
SELECT * FROM agent_learning_report
WHERE workspace_id = @workspace_id AND agent_id = @agent_id
ORDER BY created_at DESC;

-- name: GetAgentLearningReportInWorkspace :one
SELECT * FROM agent_learning_report
WHERE id = @id AND workspace_id = @workspace_id;

-- name: ListAgentEvolutionSuggestionsByReport :many
SELECT * FROM agent_evolution_suggestion
WHERE workspace_id = @workspace_id AND report_id = @report_id
ORDER BY created_at ASC;

-- name: GetAgentEvolutionSuggestionInWorkspace :one
SELECT * FROM agent_evolution_suggestion
WHERE id = @id AND workspace_id = @workspace_id;

-- name: MarkAgentEvolutionSuggestionApplied :one
UPDATE agent_evolution_suggestion
SET status = 'applied', applied_at = now(), updated_at = now()
WHERE id = @id AND workspace_id = @workspace_id AND status = 'pending'
RETURNING *;

-- name: DismissAgentEvolutionSuggestion :one
UPDATE agent_evolution_suggestion
SET status = 'dismissed', dismissed_at = now(), updated_at = now()
WHERE id = @id AND workspace_id = @workspace_id AND status = 'pending'
RETURNING *;

-- name: CreateAgentEvolutionApplication :one
INSERT INTO agent_evolution_application (
    suggestion_id, workspace_id, applied_by, target_type, target_id,
    before_content, after_content
) VALUES (
    @suggestion_id,
    @workspace_id,
    sqlc.narg(applied_by),
    @target_type,
    @target_id,
    @before_content,
    @after_content
)
RETURNING *;

-- name: ListAgentEvolutionApplicationsBySuggestion :many
SELECT * FROM agent_evolution_application
WHERE workspace_id = @workspace_id AND suggestion_id = @suggestion_id
ORDER BY created_at DESC;
