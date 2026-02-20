-- 034_site_message_target_tenant.up.sql
-- 站内信添加 target_tenant_id 字段，支持定向发送

ALTER TABLE site_messages ADD COLUMN IF NOT EXISTS target_tenant_id UUID REFERENCES tenants(id);
CREATE INDEX IF NOT EXISTS idx_site_messages_target_tenant_id ON site_messages(target_tenant_id);
