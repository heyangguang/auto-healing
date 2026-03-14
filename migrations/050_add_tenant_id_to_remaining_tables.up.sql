-- 050_add_tenant_id_to_remaining_tables.up.sql
-- 为 033 迁移遗漏的 5 张表补充 tenant_id 列

-- ============================================================
-- 1. 工作流相关子表
-- ============================================================

ALTER TABLE workflow_nodes ADD COLUMN IF NOT EXISTS tenant_id UUID DEFAULT '00000000-0000-0000-0000-000000000001' REFERENCES tenants(id);
ALTER TABLE workflow_edges ADD COLUMN IF NOT EXISTS tenant_id UUID DEFAULT '00000000-0000-0000-0000-000000000001' REFERENCES tenants(id);
ALTER TABLE workflow_instances ADD COLUMN IF NOT EXISTS tenant_id UUID DEFAULT '00000000-0000-0000-0000-000000000001' REFERENCES tenants(id);
ALTER TABLE workflow_logs ADD COLUMN IF NOT EXISTS tenant_id UUID DEFAULT '00000000-0000-0000-0000-000000000001' REFERENCES tenants(id);

-- ============================================================
-- 2. 角色-工作区关联表
-- ============================================================

ALTER TABLE role_workspaces ADD COLUMN IF NOT EXISTS tenant_id UUID DEFAULT '00000000-0000-0000-0000-000000000001' REFERENCES tenants(id);

-- ============================================================
-- 3. 创建索引
-- ============================================================

CREATE INDEX IF NOT EXISTS idx_workflow_nodes_tenant_id ON workflow_nodes(tenant_id);
CREATE INDEX IF NOT EXISTS idx_workflow_edges_tenant_id ON workflow_edges(tenant_id);
CREATE INDEX IF NOT EXISTS idx_workflow_instances_tenant_id ON workflow_instances(tenant_id);
CREATE INDEX IF NOT EXISTS idx_workflow_logs_tenant_id ON workflow_logs(tenant_id);
CREATE INDEX IF NOT EXISTS idx_role_workspaces_tenant_id ON role_workspaces(tenant_id);
