-- 033_add_tenant_id_to_tables.up.sql
-- 为所有租户级表添加 tenant_id 列，并将现有数据归入默认租户

-- ============================================================
-- 1. 核心业务表
-- ============================================================

-- 插件
ALTER TABLE plugins ADD COLUMN IF NOT EXISTS tenant_id UUID DEFAULT '00000000-0000-0000-0000-000000000001' REFERENCES tenants(id);
ALTER TABLE plugin_sync_logs ADD COLUMN IF NOT EXISTS tenant_id UUID DEFAULT '00000000-0000-0000-0000-000000000001' REFERENCES tenants(id);

-- Playbook
ALTER TABLE playbooks ADD COLUMN IF NOT EXISTS tenant_id UUID DEFAULT '00000000-0000-0000-0000-000000000001' REFERENCES tenants(id);
ALTER TABLE playbook_scan_logs ADD COLUMN IF NOT EXISTS tenant_id UUID DEFAULT '00000000-0000-0000-0000-000000000001' REFERENCES tenants(id);

-- Git 仓库
ALTER TABLE git_repositories ADD COLUMN IF NOT EXISTS tenant_id UUID DEFAULT '00000000-0000-0000-0000-000000000001' REFERENCES tenants(id);
ALTER TABLE git_sync_logs ADD COLUMN IF NOT EXISTS tenant_id UUID DEFAULT '00000000-0000-0000-0000-000000000001' REFERENCES tenants(id);

-- 自愈规则
ALTER TABLE healing_rules ADD COLUMN IF NOT EXISTS tenant_id UUID DEFAULT '00000000-0000-0000-0000-000000000001' REFERENCES tenants(id);

-- 自愈流程
ALTER TABLE healing_flows ADD COLUMN IF NOT EXISTS tenant_id UUID DEFAULT '00000000-0000-0000-0000-000000000001' REFERENCES tenants(id);
ALTER TABLE flow_instances ADD COLUMN IF NOT EXISTS tenant_id UUID DEFAULT '00000000-0000-0000-0000-000000000001' REFERENCES tenants(id);
ALTER TABLE flow_execution_logs ADD COLUMN IF NOT EXISTS tenant_id UUID DEFAULT '00000000-0000-0000-0000-000000000001' REFERENCES tenants(id);
ALTER TABLE node_executions ADD COLUMN IF NOT EXISTS tenant_id UUID DEFAULT '00000000-0000-0000-0000-000000000001' REFERENCES tenants(id);

-- 事件
ALTER TABLE incidents ADD COLUMN IF NOT EXISTS tenant_id UUID DEFAULT '00000000-0000-0000-0000-000000000001' REFERENCES tenants(id);

-- ============================================================
-- 2. 执行管理相关
-- ============================================================

ALTER TABLE execution_tasks ADD COLUMN IF NOT EXISTS tenant_id UUID DEFAULT '00000000-0000-0000-0000-000000000001' REFERENCES tenants(id);
ALTER TABLE execution_runs ADD COLUMN IF NOT EXISTS tenant_id UUID DEFAULT '00000000-0000-0000-0000-000000000001' REFERENCES tenants(id);
ALTER TABLE execution_logs ADD COLUMN IF NOT EXISTS tenant_id UUID DEFAULT '00000000-0000-0000-0000-000000000001' REFERENCES tenants(id);
ALTER TABLE execution_schedules ADD COLUMN IF NOT EXISTS tenant_id UUID DEFAULT '00000000-0000-0000-0000-000000000001' REFERENCES tenants(id);

-- ============================================================
-- 3. 通知管理
-- ============================================================

ALTER TABLE notification_channels ADD COLUMN IF NOT EXISTS tenant_id UUID DEFAULT '00000000-0000-0000-0000-000000000001' REFERENCES tenants(id);
ALTER TABLE notification_templates ADD COLUMN IF NOT EXISTS tenant_id UUID DEFAULT '00000000-0000-0000-0000-000000000001' REFERENCES tenants(id);
ALTER TABLE notification_logs ADD COLUMN IF NOT EXISTS tenant_id UUID DEFAULT '00000000-0000-0000-0000-000000000001' REFERENCES tenants(id);

-- ============================================================
-- 4. CMDB
-- ============================================================

ALTER TABLE cmdb_items ADD COLUMN IF NOT EXISTS tenant_id UUID DEFAULT '00000000-0000-0000-0000-000000000001' REFERENCES tenants(id);
ALTER TABLE cmdb_maintenance_logs ADD COLUMN IF NOT EXISTS tenant_id UUID DEFAULT '00000000-0000-0000-0000-000000000001' REFERENCES tenants(id);

-- ============================================================
-- 5. 密钥管理
-- ============================================================

ALTER TABLE secrets_sources ADD COLUMN IF NOT EXISTS tenant_id UUID DEFAULT '00000000-0000-0000-0000-000000000001' REFERENCES tenants(id);

-- ============================================================
-- 6. 站内信
-- ============================================================

ALTER TABLE site_messages ADD COLUMN IF NOT EXISTS tenant_id UUID DEFAULT '00000000-0000-0000-0000-000000000001' REFERENCES tenants(id);

-- ============================================================
-- 7. 审批
-- ============================================================

ALTER TABLE approval_tasks ADD COLUMN IF NOT EXISTS tenant_id UUID DEFAULT '00000000-0000-0000-0000-000000000001' REFERENCES tenants(id);

-- ============================================================
-- 8. 工作流（旧版）
-- ============================================================

ALTER TABLE workflows ADD COLUMN IF NOT EXISTS tenant_id UUID DEFAULT '00000000-0000-0000-0000-000000000001' REFERENCES tenants(id);

-- ============================================================
-- 9. 仪表板
-- ============================================================

ALTER TABLE system_workspaces ADD COLUMN IF NOT EXISTS tenant_id UUID DEFAULT '00000000-0000-0000-0000-000000000001' REFERENCES tenants(id);

-- ============================================================
-- 10. 创建索引（提升租户过滤性能）
-- ============================================================

CREATE INDEX IF NOT EXISTS idx_plugins_tenant_id ON plugins(tenant_id);
CREATE INDEX IF NOT EXISTS idx_playbooks_tenant_id ON playbooks(tenant_id);
CREATE INDEX IF NOT EXISTS idx_git_repositories_tenant_id ON git_repositories(tenant_id);
CREATE INDEX IF NOT EXISTS idx_healing_rules_tenant_id ON healing_rules(tenant_id);
CREATE INDEX IF NOT EXISTS idx_healing_flows_tenant_id ON healing_flows(tenant_id);
CREATE INDEX IF NOT EXISTS idx_flow_instances_tenant_id ON flow_instances(tenant_id);
CREATE INDEX IF NOT EXISTS idx_incidents_tenant_id ON incidents(tenant_id);
CREATE INDEX IF NOT EXISTS idx_execution_tasks_tenant_id ON execution_tasks(tenant_id);
CREATE INDEX IF NOT EXISTS idx_execution_runs_tenant_id ON execution_runs(tenant_id);
CREATE INDEX IF NOT EXISTS idx_execution_schedules_tenant_id ON execution_schedules(tenant_id);
CREATE INDEX IF NOT EXISTS idx_notification_channels_tenant_id ON notification_channels(tenant_id);
CREATE INDEX IF NOT EXISTS idx_notification_templates_tenant_id ON notification_templates(tenant_id);
CREATE INDEX IF NOT EXISTS idx_cmdb_items_tenant_id ON cmdb_items(tenant_id);
CREATE INDEX IF NOT EXISTS idx_secrets_sources_tenant_id ON secrets_sources(tenant_id);
CREATE INDEX IF NOT EXISTS idx_site_messages_tenant_id ON site_messages(tenant_id);
CREATE INDEX IF NOT EXISTS idx_approval_tasks_tenant_id ON approval_tasks(tenant_id);
CREATE INDEX IF NOT EXISTS idx_system_workspaces_tenant_id ON system_workspaces(tenant_id);
