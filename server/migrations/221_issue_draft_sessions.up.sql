CREATE TABLE IF NOT EXISTS issue_draft_session (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id uuid NOT NULL REFERENCES workspace(id) ON DELETE CASCADE,
    project_id uuid NOT NULL REFERENCES project(id) ON DELETE CASCADE,
    squad_id uuid NOT NULL REFERENCES squad(id) ON DELETE RESTRICT,
    leader_agent_id uuid NOT NULL REFERENCES agent(id) ON DELETE RESTRICT,
    primary_project_resource_id uuid NOT NULL REFERENCES project_resource(id) ON DELETE RESTRICT,
    primary_local_path_snapshot text NOT NULL,
    status text NOT NULL DEFAULT 'clarifying',
    created_by uuid NOT NULL REFERENCES member(id) ON DELETE RESTRICT,
    created_issue_id uuid REFERENCES issue(id) ON DELETE SET NULL,
    remote_issue_url text NOT NULL DEFAULT '',
    spec_file_path text NOT NULL DEFAULT '',
    git_commit_sha text NOT NULL DEFAULT '',
    last_error text NOT NULL DEFAULT '',
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT issue_draft_session_status_chk CHECK (
        status IN ('clarifying', 'delegating', 'drafting', 'ready_for_review', 'creating', 'created', 'failed', 'cancelled')
    )
);

CREATE TABLE IF NOT EXISTS issue_draft_message (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    session_id uuid NOT NULL REFERENCES issue_draft_session(id) ON DELETE CASCADE,
    workspace_id uuid NOT NULL REFERENCES workspace(id) ON DELETE CASCADE,
    author_type text NOT NULL,
    author_id uuid,
    message_type text NOT NULL,
    content text NOT NULL,
    metadata jsonb NOT NULL DEFAULT '{}'::jsonb,
    created_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT issue_draft_message_author_type_chk CHECK (author_type IN ('member', 'agent', 'system')),
    CONSTRAINT issue_draft_message_type_chk CHECK (
        message_type IN ('user_message', 'question', 'answer', 'finding', 'draft_preview', 'status', 'error')
    )
);

CREATE TABLE IF NOT EXISTS issue_draft_member_task (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    session_id uuid NOT NULL REFERENCES issue_draft_session(id) ON DELETE CASCADE,
    workspace_id uuid NOT NULL REFERENCES workspace(id) ON DELETE CASCADE,
    agent_id uuid NOT NULL REFERENCES agent(id) ON DELETE RESTRICT,
    task_id uuid REFERENCES agent_task_queue(id) ON DELETE SET NULL,
    status text NOT NULL DEFAULT 'queued',
    skill_basis text NOT NULL DEFAULT '',
    read_scope jsonb NOT NULL DEFAULT '{}'::jsonb,
    findings text NOT NULL DEFAULT '',
    error text NOT NULL DEFAULT '',
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT issue_draft_member_task_status_chk CHECK (
        status IN ('queued', 'running', 'completed', 'failed', 'cancelled')
    )
);

CREATE TABLE IF NOT EXISTS issue_draft_artifact (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    session_id uuid NOT NULL REFERENCES issue_draft_session(id) ON DELETE CASCADE,
    workspace_id uuid NOT NULL REFERENCES workspace(id) ON DELETE CASCADE,
    artifact_type text NOT NULL,
    revision integer NOT NULL,
    content text NOT NULL,
    generated_by_agent_id uuid REFERENCES agent(id) ON DELETE SET NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT issue_draft_artifact_type_chk CHECK (
        artifact_type IN ('detailed_spec', 'multica_issue', 'remote_issue')
    ),
    UNIQUE (session_id, artifact_type, revision)
);

CREATE TABLE IF NOT EXISTS issue_draft_confirm_step (
    session_id uuid NOT NULL REFERENCES issue_draft_session(id) ON DELETE CASCADE,
    workspace_id uuid NOT NULL REFERENCES workspace(id) ON DELETE CASCADE,
    step text NOT NULL,
    status text NOT NULL DEFAULT 'pending',
    external_id text NOT NULL DEFAULT '',
    result_metadata jsonb NOT NULL DEFAULT '{}'::jsonb,
    error text NOT NULL DEFAULT '',
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (session_id, step),
    CONSTRAINT issue_draft_confirm_step_name_chk CHECK (
        step IN ('create_multica_issue', 'write_spec', 'commit_and_push', 'create_remote_issue', 'link_outputs')
    ),
    CONSTRAINT issue_draft_confirm_step_status_chk CHECK (
        status IN ('pending', 'running', 'succeeded', 'failed')
    )
);

CREATE INDEX IF NOT EXISTS idx_issue_draft_session_workspace_created
    ON issue_draft_session (workspace_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_issue_draft_message_session_created
    ON issue_draft_message (session_id, created_at ASC);
CREATE INDEX IF NOT EXISTS idx_issue_draft_member_task_session
    ON issue_draft_member_task (session_id, created_at ASC);
CREATE INDEX IF NOT EXISTS idx_issue_draft_artifact_session_type_revision
    ON issue_draft_artifact (session_id, artifact_type, revision DESC);
