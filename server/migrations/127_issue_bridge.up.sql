CREATE TABLE issue_integration (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id UUID NOT NULL REFERENCES workspace(id) ON DELETE CASCADE,
    provider TEXT NOT NULL,
    name TEXT NOT NULL,
    base_url TEXT NOT NULL,
    encrypted_token TEXT NOT NULL DEFAULT '',
    default_issue_skill_id UUID REFERENCES skill(id) ON DELETE SET NULL,
    polling_enabled BOOLEAN NOT NULL DEFAULT false,
    default_poll_interval_seconds INTEGER NOT NULL DEFAULT 300,
    config JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT issue_integration_provider_check CHECK (provider IN ('gitlab')),
    CONSTRAINT issue_integration_base_url_check CHECK (base_url ~ '^https?://'),
    CONSTRAINT issue_integration_poll_interval_check CHECK (default_poll_interval_seconds >= 60),
    CONSTRAINT issue_integration_config_object_check CHECK (jsonb_typeof(config) = 'object'),
    UNIQUE (workspace_id, id),
    UNIQUE (workspace_id, provider, name)
);

ALTER TABLE skill
    ADD CONSTRAINT skill_workspace_id_id_unique UNIQUE (workspace_id, id);

ALTER TABLE issue_integration
    ADD CONSTRAINT issue_integration_default_skill_workspace_fk
    FOREIGN KEY (workspace_id, default_issue_skill_id)
    REFERENCES skill(workspace_id, id)
    ON DELETE SET NULL (default_issue_skill_id);

CREATE INDEX issue_integration_workspace_idx
    ON issue_integration (workspace_id, provider);

CREATE TABLE issue_sync_config (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id UUID NOT NULL REFERENCES workspace(id) ON DELETE CASCADE,
    integration_id UUID NOT NULL REFERENCES issue_integration(id) ON DELETE CASCADE,
    scope_type TEXT NOT NULL,
    scope_id UUID NOT NULL,
    remote_project_ref TEXT NOT NULL,
    sync_enabled BOOLEAN NOT NULL DEFAULT false,
    poll_interval_seconds INTEGER,
    state_mapping JSONB NOT NULL DEFAULT '{"opened":"backlog","closed":"done"}'::jsonb,
    auto_assign_enabled BOOLEAN NOT NULL DEFAULT false,
    default_assignee_type TEXT,
    default_assignee_id UUID,
    last_poll_at TIMESTAMPTZ,
    last_successful_poll_at TIMESTAMPTZ,
    last_error TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT issue_sync_config_scope_type_check CHECK (scope_type IN ('project', 'repo_resource')),
    CONSTRAINT issue_sync_config_poll_interval_check CHECK (poll_interval_seconds IS NULL OR poll_interval_seconds >= 60),
    CONSTRAINT issue_sync_config_state_mapping_object_check CHECK (jsonb_typeof(state_mapping) = 'object'),
    CONSTRAINT issue_sync_config_assignee_type_check CHECK (
        default_assignee_type IS NULL OR default_assignee_type IN ('agent', 'squad')
    ),
    CONSTRAINT issue_sync_config_auto_assign_pair_check CHECK (
        (auto_assign_enabled = false)
        OR (default_assignee_type IS NOT NULL AND default_assignee_id IS NOT NULL)
    ),
    FOREIGN KEY (workspace_id, integration_id)
        REFERENCES issue_integration(workspace_id, id)
        ON DELETE CASCADE,
    UNIQUE (workspace_id, scope_type, scope_id)
);

CREATE INDEX issue_sync_config_integration_idx
    ON issue_sync_config (integration_id);

CREATE INDEX issue_sync_config_poll_idx
    ON issue_sync_config (sync_enabled, last_poll_at)
    WHERE sync_enabled = true;
