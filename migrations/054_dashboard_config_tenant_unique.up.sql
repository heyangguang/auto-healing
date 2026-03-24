-- Align dashboard_configs uniqueness with tenant-aware model/repository semantics.

ALTER TABLE dashboard_configs
    ADD COLUMN IF NOT EXISTS tenant_id UUID REFERENCES tenants(id) ON DELETE CASCADE;

UPDATE dashboard_configs
SET tenant_id = '00000000-0000-0000-0000-000000000001'
WHERE tenant_id IS NULL;

ALTER TABLE dashboard_configs
    DROP CONSTRAINT IF EXISTS dashboard_configs_user_id_key;

DROP INDEX IF EXISTS idx_dashboard_tenant_user;

CREATE UNIQUE INDEX IF NOT EXISTS idx_dashboard_tenant_user
    ON dashboard_configs(user_id, tenant_id);

CREATE UNIQUE INDEX IF NOT EXISTS idx_dashboard_configs_null_tenant_unique
    ON dashboard_configs(user_id)
    WHERE tenant_id IS NULL;
