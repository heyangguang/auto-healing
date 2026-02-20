-- Rollback tenant_id from user_preferences and site_message_reads

DROP INDEX IF EXISTS idx_site_message_reads_tenant;
ALTER TABLE site_message_reads DROP COLUMN IF EXISTS tenant_id;

DROP INDEX IF EXISTS idx_user_preferences_tenant;
ALTER TABLE user_preferences DROP COLUMN IF EXISTS tenant_id;
