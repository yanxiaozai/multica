CREATE TABLE IF NOT EXISTS spec_epic (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id UUID NOT NULL REFERENCES workspace(id) ON DELETE CASCADE,
    key TEXT NOT NULL,
    title TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    stability TEXT NOT NULL DEFAULT 'draft'
        CHECK (stability IN ('draft', 'active', 'stable', 'deprecated')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT spec_epic_key_unique UNIQUE (workspace_id, key)
);

CREATE TABLE IF NOT EXISTS spec_module (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id UUID NOT NULL REFERENCES workspace(id) ON DELETE CASCADE,
    epic_id UUID NOT NULL REFERENCES spec_epic(id) ON DELETE CASCADE,
    key TEXT NOT NULL,
    title TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    stability TEXT NOT NULL DEFAULT 'draft'
        CHECK (stability IN ('draft', 'active', 'stable', 'deprecated')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT spec_module_key_unique UNIQUE (epic_id, key)
);

CREATE TABLE IF NOT EXISTS spec_document (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id UUID NOT NULL REFERENCES workspace(id) ON DELETE CASCADE,
    epic_id UUID REFERENCES spec_epic(id) ON DELETE CASCADE,
    module_id UUID REFERENCES spec_module(id) ON DELETE CASCADE,
    doc_kind TEXT NOT NULL
        CHECK (doc_kind IN (
            'index', 'plan', 'risks', 'requirements', 'design', 'architecture',
            'backend', 'frontend', 'implementation', 'testing', 'review',
            'acceptance', 'glossary'
        )),
    title TEXT NOT NULL DEFAULT '',
    body TEXT NOT NULL DEFAULT '',
    source_path TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT spec_document_scope_check CHECK (
        (epic_id IS NOT NULL AND module_id IS NULL)
        OR (epic_id IS NULL AND module_id IS NOT NULL)
    )
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_spec_document_epic_kind
    ON spec_document(epic_id, doc_kind)
    WHERE epic_id IS NOT NULL;

CREATE UNIQUE INDEX IF NOT EXISTS idx_spec_document_module_kind
    ON spec_document(module_id, doc_kind)
    WHERE module_id IS NOT NULL;

CREATE TABLE IF NOT EXISTS spec_issue_state (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id UUID NOT NULL REFERENCES workspace(id) ON DELETE CASCADE,
    issue_id UUID NOT NULL REFERENCES issue(id) ON DELETE CASCADE,
    status TEXT NOT NULL DEFAULT 'todo'
        CHECK (status IN ('todo', 'in_progress', 'review', 'blocked', 'done')),
    owner TEXT NOT NULL DEFAULT '',
    current_stage TEXT NOT NULL DEFAULT 'requirements'
        CHECK (current_stage IN ('requirements', 'design', 'implementation', 'testing', 'review', 'acceptance')),
    current_loop TEXT NOT NULL DEFAULT 'requirements'
        CHECK (current_loop IN ('requirements', 'design', 'implementation', 'design-audit', 'fix', 're-audit', 'testing', 'code-review', 'acceptance')),
    last_result TEXT NOT NULL DEFAULT 'pending'
        CHECK (last_result IN ('pending', 'passed', 'failed', 'conditional')),
    open_questions JSONB NOT NULL DEFAULT '[]'::jsonb,
    blockers JSONB NOT NULL DEFAULT '[]'::jsonb,
    next_handoff TEXT NOT NULL DEFAULT '',
    audit_mode TEXT NOT NULL DEFAULT 'required'
        CHECK (audit_mode IN ('required', 'skipped')),
    audit_skipped BOOLEAN NOT NULL DEFAULT false,
    audit_skip_reason TEXT NOT NULL DEFAULT '',
    audit_skipped_by TEXT NOT NULL DEFAULT '',
    audit_skipped_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT spec_issue_state_unique_issue UNIQUE (issue_id),
    CONSTRAINT spec_issue_open_questions_array CHECK (jsonb_typeof(open_questions) = 'array'),
    CONSTRAINT spec_issue_blockers_array CHECK (jsonb_typeof(blockers) = 'array')
);

CREATE TABLE IF NOT EXISTS spec_issue_mapping (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id UUID NOT NULL REFERENCES workspace(id) ON DELETE CASCADE,
    issue_id UUID NOT NULL REFERENCES issue(id) ON DELETE CASCADE,
    epic_id UUID REFERENCES spec_epic(id) ON DELETE SET NULL,
    module_id UUID REFERENCES spec_module(id) ON DELETE SET NULL,
    mapping_kind TEXT NOT NULL
        CHECK (mapping_kind IN ('primary', 'related')),
    reason TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT spec_issue_mapping_scope_check CHECK (epic_id IS NOT NULL OR module_id IS NOT NULL)
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_spec_issue_mapping_primary
    ON spec_issue_mapping(issue_id)
    WHERE mapping_kind = 'primary';

CREATE UNIQUE INDEX IF NOT EXISTS idx_spec_issue_mapping_related
    ON spec_issue_mapping(issue_id, epic_id, module_id)
    WHERE mapping_kind = 'related';

CREATE TABLE IF NOT EXISTS spec_decision (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id UUID NOT NULL REFERENCES workspace(id) ON DELETE CASCADE,
    epic_id UUID REFERENCES spec_epic(id) ON DELETE SET NULL,
    module_id UUID REFERENCES spec_module(id) ON DELETE SET NULL,
    title TEXT NOT NULL,
    body TEXT NOT NULL DEFAULT '',
    actor TEXT NOT NULL DEFAULT '',
    source_path TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_spec_epic_workspace ON spec_epic(workspace_id, key);
CREATE INDEX IF NOT EXISTS idx_spec_module_workspace ON spec_module(workspace_id, epic_id, key);
CREATE INDEX IF NOT EXISTS idx_spec_document_workspace ON spec_document(workspace_id, module_id, doc_kind);
CREATE INDEX IF NOT EXISTS idx_spec_issue_state_workspace ON spec_issue_state(workspace_id, issue_id);
CREATE INDEX IF NOT EXISTS idx_spec_issue_mapping_workspace ON spec_issue_mapping(workspace_id, issue_id);
CREATE INDEX IF NOT EXISTS idx_spec_decision_workspace ON spec_decision(workspace_id, created_at DESC);
