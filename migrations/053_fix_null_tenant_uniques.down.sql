-- Rollback NULL-tenant partial unique indexes

DROP INDEX IF EXISTS idx_user_preferences_null_tenant_unique;
DROP INDEX IF EXISTS idx_user_favorite_null_tenant_unique;
DROP INDEX IF EXISTS idx_user_recent_null_tenant_unique;
