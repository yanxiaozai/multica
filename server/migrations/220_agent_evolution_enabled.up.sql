ALTER TABLE agent
ADD COLUMN IF NOT EXISTS agent_evolution_enabled boolean NOT NULL DEFAULT false;
