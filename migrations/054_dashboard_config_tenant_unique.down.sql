DROP INDEX IF EXISTS idx_dashboard_configs_null_tenant_unique;
DROP INDEX IF EXISTS idx_dashboard_tenant_user;

CREATE UNIQUE INDEX IF NOT EXISTS dashboard_configs_user_id_key
    ON dashboard_configs(user_id);
