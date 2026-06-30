ALTER TABLE agent
ADD COLUMN spec_profile JSONB NOT NULL DEFAULT '{}'::jsonb;

