-- Add tenant_id to user activity tables (user_favorites, user_recents, audit_logs)

-- 1. user_favorites
ALTER TABLE user_favorites
    ADD COLUMN tenant_id UUID REFERENCES tenants(id) ON DELETE CASCADE;

CREATE INDEX idx_user_favorites_tenant ON user_favorites(tenant_id);

-- Update unique index to include tenant_id
DROP INDEX IF EXISTS idx_user_favorite;
CREATE UNIQUE INDEX idx_user_favorite ON user_favorites(user_id, menu_key, tenant_id);

-- 2. user_recents  
ALTER TABLE user_recents
    ADD COLUMN tenant_id UUID REFERENCES tenants(id) ON DELETE CASCADE;

CREATE INDEX idx_user_recents_tenant ON user_recents(tenant_id);

-- Update unique index to include tenant_id
DROP INDEX IF EXISTS idx_user_recent;
CREATE UNIQUE INDEX idx_user_recent ON user_recents(user_id, menu_key, tenant_id);

-- 3. audit_logs
ALTER TABLE audit_logs
    ADD COLUMN tenant_id UUID REFERENCES tenants(id) ON DELETE SET NULL;

CREATE INDEX idx_audit_logs_tenant ON audit_logs(tenant_id);
