-- Align tenant-aware uniqueness with repository/model assumptions

-- 1. user_preferences: keep platform-user preferences as NULL tenant, and move
-- tenant-user preferences to their earliest known tenant (fall back to default).
UPDATE user_preferences up
SET tenant_id = COALESCE(
        (
            SELECT utr.tenant_id
            FROM user_tenant_roles utr
            WHERE utr.user_id = up.user_id
            ORDER BY utr.created_at ASC, utr.id ASC
            LIMIT 1
        ),
        '00000000-0000-0000-0000-000000000001'
    )
FROM users u
WHERE up.user_id = u.id
  AND up.tenant_id IS NULL
  AND COALESCE(u.is_platform_admin, false) = false;

ALTER TABLE user_preferences
    DROP CONSTRAINT IF EXISTS user_preferences_user_id_key;

DROP INDEX IF EXISTS idx_user_preferences_user_id;
DROP INDEX IF EXISTS idx_pref_tenant_user;

CREATE UNIQUE INDEX IF NOT EXISTS idx_pref_tenant_user
    ON user_preferences(user_id, tenant_id);

-- 2. site_message_reads: make read state tenant-scoped
UPDATE site_message_reads
SET tenant_id = '00000000-0000-0000-0000-000000000001'
WHERE tenant_id IS NULL;

ALTER TABLE site_message_reads
    ALTER COLUMN tenant_id SET NOT NULL;

DROP INDEX IF EXISTS idx_site_message_read_unique;
DROP INDEX IF EXISTS idx_site_message_read_tenant_unique;

CREATE UNIQUE INDEX IF NOT EXISTS idx_site_message_read_tenant_unique
    ON site_message_reads(tenant_id, message_id, user_id);
