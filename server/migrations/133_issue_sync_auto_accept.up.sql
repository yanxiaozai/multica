ALTER TABLE issue_sync_config
ADD COLUMN sync_mode TEXT NOT NULL DEFAULT 'assigned_to_me',
ADD COLUMN auto_accept_label TEXT NOT NULL DEFAULT 'ai-auto';

ALTER TABLE issue_sync_config
ADD CONSTRAINT issue_sync_config_sync_mode_check
CHECK (sync_mode IN ('assigned_to_me', 'auto_accept'));

ALTER TABLE issue_sync_config
ADD CONSTRAINT issue_sync_config_auto_accept_label_check
CHECK (btrim(auto_accept_label) <> '');
