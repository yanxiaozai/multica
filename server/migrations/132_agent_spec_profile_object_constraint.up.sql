ALTER TABLE agent
ADD CONSTRAINT agent_spec_profile_object
CHECK (jsonb_typeof(spec_profile) = 'object');

