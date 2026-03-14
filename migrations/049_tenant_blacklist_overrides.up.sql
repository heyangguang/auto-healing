-- 049_tenant_blacklist_overrides.up.sql
-- 租户对系统级黑名单规则的独立开关覆盖表
-- 每个租户可以独立启用/禁用系统规则（is_system=true, tenant_id=NULL）
-- 租户自有规则（tenant_id=当前租户）的开关仍直接存在 command_blacklist 表上

CREATE TABLE IF NOT EXISTS tenant_blacklist_overrides (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id  UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    rule_id    UUID NOT NULL REFERENCES command_blacklist(id) ON DELETE CASCADE,
    is_active  BOOLEAN NOT NULL DEFAULT false,
    created_at TIMESTAMPTZ DEFAULT now(),
    updated_at TIMESTAMPTZ DEFAULT now(),
    UNIQUE(tenant_id, rule_id)
);

CREATE INDEX IF NOT EXISTS idx_tbl_overrides_tenant  ON tenant_blacklist_overrides(tenant_id);
CREATE INDEX IF NOT EXISTS idx_tbl_overrides_rule    ON tenant_blacklist_overrides(rule_id);
CREATE INDEX IF NOT EXISTS idx_tbl_overrides_active  ON tenant_blacklist_overrides(tenant_id, is_active);
