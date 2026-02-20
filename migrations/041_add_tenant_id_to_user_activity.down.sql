-- Rollback tenant_id from user activity tables

-- 1. audit_logs
DROP INDEX IF EXISTS idx_audit_logs_tenant;
ALTER TABLE audit_logs DROP COLUMN IF EXISTS tenant_id;

-- 2. user_recents
DROP INDEX IF EXISTS idx_user_recent;
DROP INDEX IF EXISTS idx_user_recents_tenant;
ALTER TABLE user_recents DROP COLUMN IF EXISTS tenant_id;
CREATE UNIQUE INDEX idx_user_recent ON user_recents(user_id, menu_key);

-- 3. user_favorites
DROP INDEX IF EXISTS idx_user_favorite;
DROP INDEX IF EXISTS idx_user_favorites_tenant;
ALTER TABLE user_favorites DROP COLUMN IF EXISTS tenant_id;
CREATE UNIQUE INDEX idx_user_favorite ON user_favorites(user_id, menu_key);
