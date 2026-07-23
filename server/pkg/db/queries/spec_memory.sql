-- name: ListSpecEpics :many
SELECT * FROM spec_epic
WHERE workspace_id = $1
ORDER BY key ASC;

-- name: GetSpecEpicInWorkspace :one
SELECT * FROM spec_epic
WHERE id = $1 AND workspace_id = $2;

-- name: UpsertSpecEpic :one
INSERT INTO spec_epic (
    workspace_id, key, title, description, stability
) VALUES (
    $1, $2, $3, $4, $5
)
ON CONFLICT (workspace_id, key) DO UPDATE SET
    title = EXCLUDED.title,
    description = EXCLUDED.description,
    stability = EXCLUDED.stability,
    updated_at = now()
RETURNING *;

-- name: ListSpecModules :many
SELECT * FROM spec_module
WHERE workspace_id = $1
  AND (sqlc.narg('epic_id')::uuid IS NULL OR epic_id = sqlc.narg('epic_id'))
ORDER BY key ASC;

-- name: GetSpecModuleInWorkspace :one
SELECT * FROM spec_module
WHERE id = $1 AND workspace_id = $2;

-- name: UpsertSpecModule :one
INSERT INTO spec_module (
    workspace_id, epic_id, key, title, description, stability
) VALUES (
    $1, $2, $3, $4, $5, $6
)
ON CONFLICT (epic_id, key) DO UPDATE SET
    title = EXCLUDED.title,
    description = EXCLUDED.description,
    stability = EXCLUDED.stability,
    updated_at = now()
RETURNING *;

-- name: ListSpecDocumentsByModule :many
SELECT * FROM spec_document
WHERE workspace_id = $1 AND module_id = $2
ORDER BY doc_kind ASC;

-- name: ListSpecDocumentsByEpic :many
SELECT * FROM spec_document
WHERE workspace_id = $1 AND epic_id = $2
ORDER BY doc_kind ASC;

-- name: UpsertSpecModuleDocument :one
INSERT INTO spec_document (
    workspace_id, module_id, doc_kind, title, body, source_path
) VALUES (
    $1, $2, $3, $4, $5, $6
)
ON CONFLICT (module_id, doc_kind) WHERE module_id IS NOT NULL DO UPDATE SET
    title = EXCLUDED.title,
    body = EXCLUDED.body,
    source_path = EXCLUDED.source_path,
    updated_at = now()
RETURNING *;

-- name: UpsertSpecEpicDocument :one
INSERT INTO spec_document (
    workspace_id, epic_id, doc_kind, title, body, source_path
) VALUES (
    $1, $2, $3, $4, $5, $6
)
ON CONFLICT (epic_id, doc_kind) WHERE epic_id IS NOT NULL DO UPDATE SET
    title = EXCLUDED.title,
    body = EXCLUDED.body,
    source_path = EXCLUDED.source_path,
    updated_at = now()
RETURNING *;

-- name: GetSpecIssueState :one
SELECT * FROM spec_issue_state
WHERE workspace_id = $1 AND issue_id = $2;

-- name: UpsertSpecIssueState :one
INSERT INTO spec_issue_state (
    workspace_id, issue_id, status, owner, current_stage, current_loop,
    last_result, open_questions, blockers, next_handoff, audit_mode,
    audit_skipped, audit_skip_reason, audit_skipped_by, audit_skipped_at
) VALUES (
    $1, $2, $3, $4, $5, $6,
    $7, $8, $9, $10, $11,
    $12, $13, $14, $15
)
ON CONFLICT (issue_id) DO UPDATE SET
    status = EXCLUDED.status,
    owner = EXCLUDED.owner,
    current_stage = EXCLUDED.current_stage,
    current_loop = EXCLUDED.current_loop,
    last_result = EXCLUDED.last_result,
    open_questions = EXCLUDED.open_questions,
    blockers = EXCLUDED.blockers,
    next_handoff = EXCLUDED.next_handoff,
    audit_mode = EXCLUDED.audit_mode,
    audit_skipped = EXCLUDED.audit_skipped,
    audit_skip_reason = EXCLUDED.audit_skip_reason,
    audit_skipped_by = EXCLUDED.audit_skipped_by,
    audit_skipped_at = EXCLUDED.audit_skipped_at,
    updated_at = now()
RETURNING *;

-- name: ListSpecIssueMappings :many
SELECT sim.*
FROM spec_issue_mapping sim
WHERE sim.workspace_id = $1 AND sim.issue_id = $2
ORDER BY
    CASE sim.mapping_kind WHEN 'primary' THEN 0 ELSE 1 END,
    sim.created_at ASC;

-- name: ListSpecIssueStates :many
SELECT sis.*, i.title AS issue_title, i.number AS issue_number, w.issue_prefix
FROM spec_issue_state sis
JOIN issue i ON i.id = sis.issue_id
JOIN workspace w ON w.id = sis.workspace_id
WHERE sis.workspace_id = $1
ORDER BY i.updated_at DESC;

-- name: UpsertSpecIssuePrimaryMapping :one
INSERT INTO spec_issue_mapping (
    workspace_id, issue_id, epic_id, module_id, mapping_kind, reason
) VALUES (
    $1, $2, $3, $4, 'primary', $5
)
ON CONFLICT (issue_id) WHERE mapping_kind = 'primary' DO UPDATE SET
    epic_id = EXCLUDED.epic_id,
    module_id = EXCLUDED.module_id,
    reason = EXCLUDED.reason,
    updated_at = now()
RETURNING *;

-- name: UpsertSpecIssueRelatedMapping :one
INSERT INTO spec_issue_mapping (
    workspace_id, issue_id, epic_id, module_id, mapping_kind, reason
) VALUES (
    $1, $2, $3, $4, 'related', $5
)
ON CONFLICT (issue_id, epic_id, module_id) WHERE mapping_kind = 'related' DO UPDATE SET
    reason = EXCLUDED.reason,
    updated_at = now()
RETURNING *;

-- name: DeleteSpecIssueMappings :exec
DELETE FROM spec_issue_mapping
WHERE workspace_id = $1 AND issue_id = $2;

-- name: CreateSpecDecision :one
INSERT INTO spec_decision (
    workspace_id, epic_id, module_id, title, body, actor, source_path
) VALUES (
    $1, $2, $3, $4, $5, $6, $7
)
RETURNING *;

-- name: ListSpecDecisions :many
SELECT * FROM spec_decision
WHERE workspace_id = $1
  AND (sqlc.narg('epic_id')::uuid IS NULL OR epic_id = sqlc.narg('epic_id'))
  AND (sqlc.narg('module_id')::uuid IS NULL OR module_id = sqlc.narg('module_id'))
ORDER BY created_at DESC;
