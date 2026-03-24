-- Backfill legacy common-route data into a concrete tenant for non-platform users.
-- Platform users keep NULL tenant_id so their global preferences/activity remain global.

-- 1. user_favorites
UPDATE user_favorites uf
SET tenant_id = COALESCE(
        (
            SELECT utr.tenant_id
            FROM user_tenant_roles utr
            WHERE utr.user_id = uf.user_id
            ORDER BY utr.created_at ASC, utr.id ASC
            LIMIT 1
        ),
        '00000000-0000-0000-0000-000000000001'
    )
FROM users u
WHERE uf.user_id = u.id
  AND uf.tenant_id IS NULL
  AND COALESCE(u.is_platform_admin, false) = false;

-- 2. user_recents
UPDATE user_recents ur
SET tenant_id = COALESCE(
        (
            SELECT utr.tenant_id
            FROM user_tenant_roles utr
            WHERE utr.user_id = ur.user_id
            ORDER BY utr.created_at ASC, utr.id ASC
            LIMIT 1
        ),
        '00000000-0000-0000-0000-000000000001'
    )
FROM users u
WHERE ur.user_id = u.id
  AND ur.tenant_id IS NULL
  AND COALESCE(u.is_platform_admin, false) = false;
