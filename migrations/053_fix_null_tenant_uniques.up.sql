-- Ensure platform-scoped rows (tenant_id IS NULL) remain unique as well.

-- 1. user_preferences: only one NULL-tenant row per user
DELETE FROM user_preferences a
USING user_preferences b
WHERE a.id < b.id
  AND a.user_id = b.user_id
  AND a.tenant_id IS NULL
  AND b.tenant_id IS NULL;

CREATE UNIQUE INDEX IF NOT EXISTS idx_user_preferences_null_tenant_unique
    ON user_preferences(user_id)
    WHERE tenant_id IS NULL;

-- 2. user_favorites: only one NULL-tenant favorite per menu key/user
DELETE FROM user_favorites a
USING user_favorites b
WHERE a.id < b.id
  AND a.user_id = b.user_id
  AND a.menu_key = b.menu_key
  AND a.tenant_id IS NULL
  AND b.tenant_id IS NULL;

CREATE UNIQUE INDEX IF NOT EXISTS idx_user_favorite_null_tenant_unique
    ON user_favorites(user_id, menu_key)
    WHERE tenant_id IS NULL;

-- 3. user_recents: only one NULL-tenant recent item per menu key/user
DELETE FROM user_recents a
USING user_recents b
WHERE a.id < b.id
  AND a.user_id = b.user_id
  AND a.menu_key = b.menu_key
  AND a.tenant_id IS NULL
  AND b.tenant_id IS NULL;

CREATE UNIQUE INDEX IF NOT EXISTS idx_user_recent_null_tenant_unique
    ON user_recents(user_id, menu_key)
    WHERE tenant_id IS NULL;
