-- Add tenant_id to user_preferences and site_message_reads

-- 1. user_preferences (user settings are tenant-isolated)
ALTER TABLE user_preferences
    ADD COLUMN tenant_id UUID REFERENCES tenants(id) ON DELETE CASCADE;

CREATE INDEX idx_user_preferences_tenant ON user_preferences(tenant_id);

-- 2. site_message_reads (read status is tenant-isolated)
ALTER TABLE site_message_reads
    ADD COLUMN tenant_id UUID REFERENCES tenants(id) ON DELETE CASCADE;

CREATE INDEX idx_site_message_reads_tenant ON site_message_reads(tenant_id);
