-- name: CreateSquadInstructionsGenerationJob :one
INSERT INTO squad_instructions_generation_job (
    workspace_id, squad_id, created_by, status, mode, draft
)
VALUES ($1, $2, $3, 'queued', $4, $5)
RETURNING *;

-- name: AttachSquadInstructionsGenerationTask :one
UPDATE squad_instructions_generation_job
SET task_id = $2, updated_at = now()
WHERE id = $1
RETURNING *;

-- name: GetSquadInstructionsGenerationJobInWorkspace :one
SELECT *
FROM squad_instructions_generation_job
WHERE id = $1 AND squad_id = $2 AND workspace_id = $3;

-- name: GetSquadInstructionsGenerationJobByTask :one
SELECT *
FROM squad_instructions_generation_job
WHERE task_id = $1;

-- name: MarkSquadInstructionsGenerationRunning :one
UPDATE squad_instructions_generation_job
SET status = 'running', updated_at = now()
WHERE id = $1 AND status = 'queued'
RETURNING *;

-- name: CompleteSquadInstructionsGenerationJob :one
UPDATE squad_instructions_generation_job
SET status = 'completed',
    instructions = $2,
    error = NULL,
    completed_at = now(),
    updated_at = now()
WHERE id = $1
RETURNING *;

-- name: FailSquadInstructionsGenerationJob :one
UPDATE squad_instructions_generation_job
SET status = 'failed',
    error = $2,
    completed_at = now(),
    updated_at = now()
WHERE id = $1 AND status IN ('queued', 'running')
RETURNING *;
