CREATE TABLE IF NOT EXISTS agent_learning_report (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id uuid NOT NULL REFERENCES workspace(id) ON DELETE CASCADE,
    issue_id uuid REFERENCES issue(id) ON DELETE SET NULL,
    task_id uuid REFERENCES agent_task_queue(id) ON DELETE SET NULL,
    agent_id uuid NOT NULL REFERENCES agent(id) ON DELETE CASCADE,
    summary text NOT NULL DEFAULT '',
    metadata jsonb NOT NULL DEFAULT '{}'::jsonb,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_agent_learning_report_workspace_issue
    ON agent_learning_report(workspace_id, issue_id, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_agent_learning_report_workspace_agent
    ON agent_learning_report(workspace_id, agent_id, created_at DESC);

CREATE TABLE IF NOT EXISTS agent_evolution_suggestion (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    report_id uuid NOT NULL REFERENCES agent_learning_report(id) ON DELETE CASCADE,
    workspace_id uuid NOT NULL REFERENCES workspace(id) ON DELETE CASCADE,
    scope text NOT NULL,
    risk text NOT NULL,
    status text NOT NULL DEFAULT 'pending',
    target_type text NOT NULL,
    target_id uuid,
    title text NOT NULL DEFAULT '',
    rationale text NOT NULL DEFAULT '',
    proposed_content text NOT NULL DEFAULT '',
    metadata jsonb NOT NULL DEFAULT '{}'::jsonb,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    applied_at timestamptz,
    dismissed_at timestamptz,
    CONSTRAINT agent_evolution_suggestion_scope_check
        CHECK (scope IN ('personal_agent', 'workspace_skill', 'builtin_skill_candidate')),
    CONSTRAINT agent_evolution_suggestion_risk_check
        CHECK (risk IN ('safe', 'review', 'manual')),
    CONSTRAINT agent_evolution_suggestion_status_check
        CHECK (status IN ('pending', 'applied', 'dismissed')),
    CONSTRAINT agent_evolution_suggestion_target_type_check
        CHECK (target_type IN ('agent', 'skill', 'builtin_skill'))
);

CREATE INDEX IF NOT EXISTS idx_agent_evolution_suggestion_report
    ON agent_evolution_suggestion(report_id, created_at ASC);

CREATE INDEX IF NOT EXISTS idx_agent_evolution_suggestion_workspace_status
    ON agent_evolution_suggestion(workspace_id, status, created_at DESC);

CREATE TABLE IF NOT EXISTS agent_evolution_application (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    suggestion_id uuid NOT NULL REFERENCES agent_evolution_suggestion(id) ON DELETE CASCADE,
    workspace_id uuid NOT NULL REFERENCES workspace(id) ON DELETE CASCADE,
    applied_by uuid REFERENCES "user"(id) ON DELETE SET NULL,
    target_type text NOT NULL,
    target_id uuid NOT NULL,
    before_content text NOT NULL DEFAULT '',
    after_content text NOT NULL DEFAULT '',
    created_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT agent_evolution_application_target_type_check
        CHECK (target_type IN ('agent', 'skill'))
);

CREATE INDEX IF NOT EXISTS idx_agent_evolution_application_suggestion
    ON agent_evolution_application(suggestion_id, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_agent_evolution_application_workspace
    ON agent_evolution_application(workspace_id, created_at DESC);
