ALTER TABLE issue_sync_config
ADD COLUMN IF NOT EXISTS sync_mode TEXT NOT NULL DEFAULT 'assigned_to_me',
ADD COLUMN IF NOT EXISTS auto_accept_label TEXT NOT NULL DEFAULT 'ai-auto';

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint WHERE conname = 'issue_sync_config_sync_mode_check'
    ) THEN
        ALTER TABLE issue_sync_config
        ADD CONSTRAINT issue_sync_config_sync_mode_check
        CHECK (sync_mode IN ('assigned_to_me', 'auto_accept'));
    END IF;
END $$;

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint WHERE conname = 'issue_sync_config_auto_accept_label_check'
    ) THEN
        ALTER TABLE issue_sync_config
        ADD CONSTRAINT issue_sync_config_auto_accept_label_check
        CHECK (btrim(auto_accept_label) <> '');
    END IF;
END $$;
