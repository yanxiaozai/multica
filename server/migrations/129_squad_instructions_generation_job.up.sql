CREATE TABLE squad_instructions_generation_job (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id UUID NOT NULL REFERENCES workspace(id) ON DELETE CASCADE,
    squad_id UUID NOT NULL REFERENCES squad(id) ON DELETE CASCADE,
    task_id UUID REFERENCES agent_task_queue(id) ON DELETE SET NULL,
    created_by UUID NOT NULL REFERENCES "user"(id) ON DELETE CASCADE,
    status TEXT NOT NULL DEFAULT 'queued'
        CHECK (status IN ('queued', 'running', 'completed', 'failed', 'cancelled')),
    mode TEXT NOT NULL DEFAULT 'agent_docs',
    draft TEXT NOT NULL DEFAULT '',
    instructions TEXT NOT NULL DEFAULT '',
    error TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    completed_at TIMESTAMPTZ
);

CREATE INDEX idx_squad_instructions_generation_job_squad_created
    ON squad_instructions_generation_job(squad_id, created_at DESC);

CREATE INDEX idx_squad_instructions_generation_job_task
    ON squad_instructions_generation_job(task_id)
    WHERE task_id IS NOT NULL;
