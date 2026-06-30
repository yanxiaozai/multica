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
    sync_enabled, poll_interval_seconds, state_mapping, sync_mode, auto_accept_label,
    auto_assign_enabled, default_assignee_type, default_assignee_id
) VALUES (
    $1, $2, $3, $4, $5,
    $6, $7, $8, $9, $10,
    $11, $12, $13
)
RETURNING *;

-- name: UpdateIssueSyncConfig :one
UPDATE issue_sync_config
SET remote_project_ref = $3,
    sync_enabled = $4,
    poll_interval_seconds = $5,
    state_mapping = $6,
    sync_mode = $7,
    auto_accept_label = $8,
    auto_assign_enabled = $9,
    default_assignee_type = $10,
    default_assignee_id = $11,
    updated_at = now()
WHERE id = $1 AND workspace_id = $2
RETURNING *;

-- name: DeleteIssueSyncConfig :one
DELETE FROM issue_sync_config
WHERE id = $1 AND workspace_id = $2
RETURNING id;

-- name: GetIssueSyncConfigByScope :one
-- Loads the sync config that owns a given scope (e.g. the project-scoped
-- config for scope_type='project', scope_id=<project id>). Used by the
-- import path to resolve integration_id / remote_project_ref / state_mapping
-- / auto-assign for a project.
SELECT * FROM issue_sync_config
WHERE workspace_id = $1 AND scope_type = $2 AND scope_id = $3;

-- name: GetIssueBridgeItemByRemote :one
-- Idempotency lookup: has this GitLab issue (integration + iid) already been
-- imported? The UNIQUE(integration_id, remote_iid) constraint backs this.
SELECT * FROM issue_bridge_item
WHERE integration_id = $1 AND remote_iid = $2;

-- name: CreateIssueBridgeItem :one
INSERT INTO issue_bridge_item (
    workspace_id, issue_id, integration_id,
    remote_project_ref, remote_iid, remote_url, remote_updated_at
) VALUES (
    $1, $2, $3, $4, $5, $6, $7
)
RETURNING *;

-- name: ListDueIssueSyncConfigs :many
-- Picks sync configs whose poll is due now: enabled, and either never polled
-- or past their effective interval. The effective interval falls back to the
-- integration's default when the per-config poll_interval_seconds is NULL.
-- Backs the IssueSyncPoll scheduler job.
SELECT c.* FROM issue_sync_config c
JOIN issue_integration i ON i.id = c.integration_id
WHERE c.sync_enabled = true
  AND (
    c.last_poll_at IS NULL
    OR c.last_poll_at + (COALESCE(c.poll_interval_seconds, i.default_poll_interval_seconds) * interval '1 second') < now()
  )
ORDER BY c.last_poll_at ASC NULLS FIRST, c.created_at ASC;

-- name: MarkIssueSyncPollSuccess :one
-- Records a successful poll: both watermarks advance to now() and any prior
-- error clears. Split from the failure variant so sqlc infers clean param
-- types (a single CASE-WHEN-$2 query made sqlc type $2 as timestamptz).
UPDATE issue_sync_config
SET last_poll_at = now(),
    last_successful_poll_at = now(),
    last_error = '',
    updated_at = now()
WHERE id = $1
RETURNING *;

-- name: MarkIssueSyncPollFailure :one
-- Records a failed poll: last_poll_at advances (so the interval still
-- applies), last_successful_poll_at is preserved, and last_error captures
-- the failure for the UI / next-cycle diagnosis.
UPDATE issue_sync_config
SET last_poll_at = now(),
    last_error = $2,
    updated_at = now()
WHERE id = $1
RETURNING *;
