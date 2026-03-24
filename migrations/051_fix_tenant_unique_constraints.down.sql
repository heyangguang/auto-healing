-- Rollback tenant-aware uniqueness adjustments

DROP INDEX IF EXISTS idx_pref_tenant_user;
CREATE UNIQUE INDEX IF NOT EXISTS idx_user_preferences_user_id
    ON user_preferences(user_id);

DROP INDEX IF EXISTS idx_site_message_read_tenant_unique;
CREATE UNIQUE INDEX IF NOT EXISTS idx_site_message_read_unique
    ON site_message_reads(message_id, user_id);

ALTER TABLE site_message_reads
    ALTER COLUMN tenant_id DROP NOT NULL;
