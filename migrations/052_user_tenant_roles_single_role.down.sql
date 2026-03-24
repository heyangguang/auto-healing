-- Rollback to allow multiple roles per user within a tenant again.

DROP INDEX IF EXISTS idx_user_tenant_roles_unique;

CREATE UNIQUE INDEX IF NOT EXISTS idx_user_tenant_roles_unique
    ON user_tenant_roles(user_id, tenant_id, role_id);
