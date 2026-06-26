-- name: ListIssueIntegrationsByWorkspace :many
SELECT * FROM issue_integration
WHERE workspace_id = $1
ORDER BY provider ASC, name ASC, created_at ASC;

-- name: GetIssueIntegrationInWorkspace :one
SELECT * FROM issue_integration
WHERE id = $1 AND workspace_id = $2;

-- name: GetIssueIntegrationByProviderName :one
SELECT * FROM issue_integration
WHERE workspace_id = $1 AND provider = $2 AND name = $3;

-- name: CreateIssueIntegration :one
INSERT INTO issue_integration (
    workspace_id, provider, name, base_url, encrypted_token,
    default_issue_skill_id, polling_enabled, default_poll_interval_seconds, config
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9
)
RETURNING *;

-- name: UpdateIssueIntegration :one
UPDATE issue_integration
SET name = $3,
    base_url = $4,
    encrypted_token = COALESCE(sqlc.narg('encrypted_token'), encrypted_token),
    default_issue_skill_id = $5,
    polling_enabled = $6,
    default_poll_interval_seconds = $7,
    config = $8,
    updated_at = now()
WHERE id = $1 AND workspace_id = $2
RETURNING *;

-- name: DeleteIssueIntegration :one
DELETE FROM issue_integration
WHERE id = $1 AND workspace_id = $2
RETURNING id;

-- name: ListIssueSyncConfigsByWorkspace :many
SELECT * FROM issue_sync_config
WHERE workspace_id = $1
ORDER BY created_at ASC;

-- name: ListIssueSyncConfigsByIntegration :many
SELECT * FROM issue_sync_config
WHERE workspace_id = $1 AND integration_id = $2
ORDER BY created_at ASC;

-- name: GetIssueSyncConfigInWorkspace :one
SELECT * FROM issue_sync_config
WHERE id = $1 AND workspace_id = $2;

-- name: CreateIssueSyncConfig :one
INSERT INTO issue_sync_config (
    workspace_id, integration_id, scope_type, scope_id, remote_project_ref,
    sync_enabled, poll_interval_seconds, state_mapping,
    auto_assign_enabled, default_assignee_type, default_assignee_id
) VALUES (
    $1, $2, $3, $4, $5,
    $6, $7, $8,
    $9, $10, $11
)
RETURNING *;

-- name: UpdateIssueSyncConfig :one
UPDATE issue_sync_config
SET remote_project_ref = $3,
    sync_enabled = $4,
    poll_interval_seconds = $5,
    state_mapping = $6,
    auto_assign_enabled = $7,
    default_assignee_type = $8,
    default_assignee_id = $9,
    updated_at = now()
WHERE id = $1 AND workspace_id = $2
RETURNING *;

-- name: DeleteIssueSyncConfig :one
DELETE FROM issue_sync_config
WHERE id = $1 AND workspace_id = $2
RETURNING id;
