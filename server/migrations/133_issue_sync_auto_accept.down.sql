ALTER TABLE issue_sync_config
DROP CONSTRAINT IF EXISTS issue_sync_config_sync_mode_check;

ALTER TABLE issue_sync_config
DROP CONSTRAINT IF EXISTS issue_sync_config_auto_accept_label_check;

ALTER TABLE issue_sync_config
DROP COLUMN IF EXISTS auto_accept_label;

ALTER TABLE issue_sync_config
DROP COLUMN IF EXISTS sync_mode;
