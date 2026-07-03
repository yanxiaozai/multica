-- name: CreateIssueDraftSession :one
INSERT INTO issue_draft_session (
    workspace_id,
    project_id,
    squad_id,
    leader_agent_id,
    primary_project_resource_id,
    primary_local_path_snapshot,
    created_by
) VALUES (
    $1, $2, $3, $4, $5, $6, $7
) RETURNING *;

-- name: GetIssueDraftSessionInWorkspace :one
SELECT * FROM issue_draft_session
WHERE id = $1 AND workspace_id = $2;

-- name: ListIssueDraftSessions :many
SELECT * FROM issue_draft_session
WHERE workspace_id = $1
ORDER BY created_at DESC
LIMIT $2 OFFSET $3;

-- name: UpdateIssueDraftSessionStatus :one
UPDATE issue_draft_session
SET status = $3,
    last_error = $4,
    updated_at = now()
WHERE id = $1 AND workspace_id = $2
RETURNING *;

-- name: MarkIssueDraftCreated :one
UPDATE issue_draft_session
SET status = 'created',
    created_issue_id = $3,
    remote_issue_url = $4,
    spec_file_path = $5,
    git_commit_sha = $6,
    last_error = '',
    updated_at = now()
WHERE id = $1 AND workspace_id = $2
RETURNING *;

-- name: AppendIssueDraftMessage :one
INSERT INTO issue_draft_message (
    session_id,
    workspace_id,
    author_type,
    author_id,
    message_type,
    content,
    metadata
) VALUES (
    $1, $2, $3, $4, $5, $6, $7
) RETURNING *;

-- name: ListIssueDraftMessages :many
SELECT * FROM issue_draft_message
WHERE session_id = $1 AND workspace_id = $2
ORDER BY created_at ASC;

-- name: CreateIssueDraftMemberTask :one
INSERT INTO issue_draft_member_task (
    session_id,
    workspace_id,
    agent_id,
    task_id,
    status,
    skill_basis,
    read_scope
) VALUES (
    $1, $2, $3, $4, $5, $6, $7
) RETURNING *;

-- name: ListIssueDraftMemberTasks :many
SELECT * FROM issue_draft_member_task
WHERE session_id = $1 AND workspace_id = $2
ORDER BY created_at ASC;

-- name: UpdateIssueDraftMemberTaskStatus :one
UPDATE issue_draft_member_task
SET status = $3,
    findings = $4,
    error = $5,
    updated_at = now()
WHERE id = $1 AND workspace_id = $2
RETURNING *;

-- name: LinkIssueDraftMemberTaskQueueItem :one
UPDATE issue_draft_member_task
SET task_id = $3,
    updated_at = now()
WHERE id = $1 AND workspace_id = $2
RETURNING *;

-- name: CreateIssueDraftArtifact :one
INSERT INTO issue_draft_artifact (
    session_id,
    workspace_id,
    artifact_type,
    revision,
    content,
    generated_by_agent_id
) VALUES (
    $1, $2, $3, $4, $5, $6
) RETURNING *;

-- name: ListIssueDraftArtifacts :many
SELECT * FROM issue_draft_artifact
WHERE session_id = $1 AND workspace_id = $2
ORDER BY artifact_type ASC, revision DESC;

-- name: GetLatestIssueDraftArtifact :one
SELECT * FROM issue_draft_artifact
WHERE session_id = $1 AND workspace_id = $2 AND artifact_type = $3
ORDER BY revision DESC
LIMIT 1;

-- name: GetNextIssueDraftArtifactRevision :one
SELECT COALESCE(MAX(revision), 0)::int + 1 AS next_revision
FROM issue_draft_artifact
WHERE session_id = $1 AND workspace_id = $2 AND artifact_type = $3;

-- name: UpsertIssueDraftConfirmStep :one
INSERT INTO issue_draft_confirm_step (
    session_id,
    workspace_id,
    step,
    status,
    external_id,
    result_metadata,
    error
) VALUES (
    $1, $2, $3, $4, $5, $6, $7
)
ON CONFLICT (session_id, step) DO UPDATE
SET status = EXCLUDED.status,
    external_id = EXCLUDED.external_id,
    result_metadata = EXCLUDED.result_metadata,
    error = EXCLUDED.error,
    updated_at = now()
RETURNING *;

-- name: ListIssueDraftConfirmSteps :many
SELECT * FROM issue_draft_confirm_step
WHERE session_id = $1 AND workspace_id = $2
ORDER BY created_at ASC;
