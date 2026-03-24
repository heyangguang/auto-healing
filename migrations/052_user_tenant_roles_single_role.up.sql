-- Normalize user_tenant_roles to one role per user per tenant.
-- Keep the newest row when historical duplicates exist.

WITH ranked AS (
    SELECT
        id,
        ROW_NUMBER() OVER (
            PARTITION BY user_id, tenant_id
            ORDER BY created_at DESC, id DESC
        ) AS rn
    FROM user_tenant_roles
)
DELETE FROM user_tenant_roles utr
USING ranked r
WHERE utr.id = r.id
  AND r.rn > 1;

DROP INDEX IF EXISTS idx_user_tenant_roles_unique;

CREATE UNIQUE INDEX IF NOT EXISTS idx_user_tenant_roles_unique
    ON user_tenant_roles(user_id, tenant_id);
