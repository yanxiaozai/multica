DROP TABLE IF EXISTS issue_sync_config;
DROP TABLE IF EXISTS issue_integration;
ALTER TABLE skill DROP CONSTRAINT IF EXISTS skill_workspace_id_id_unique;
