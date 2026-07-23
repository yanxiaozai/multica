DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint WHERE conname = 'agent_spec_profile_object'
    ) THEN
        ALTER TABLE agent
        ADD CONSTRAINT agent_spec_profile_object
        CHECK (jsonb_typeof(spec_profile) = 'object');
    END IF;
END $$;
