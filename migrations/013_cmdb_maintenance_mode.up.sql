-- 简化 CMDB 状态管理：删除 manual_disabled 相关字段，添加维护模式字段
-- status 改为三种值：active, offline, maintenance

-- 删除旧字段
ALTER TABLE cmdb_items DROP COLUMN IF EXISTS manual_disabled;
ALTER TABLE cmdb_items DROP COLUMN IF EXISTS manual_disabled_reason;
ALTER TABLE cmdb_items DROP COLUMN IF EXISTS manual_disabled_at;

-- 添加维护模式字段
ALTER TABLE cmdb_items ADD COLUMN IF NOT EXISTS maintenance_reason VARCHAR(500);
ALTER TABLE cmdb_items ADD COLUMN IF NOT EXISTS maintenance_start_at TIMESTAMPTZ;
ALTER TABLE cmdb_items ADD COLUMN IF NOT EXISTS maintenance_end_at TIMESTAMPTZ;

-- 创建维护日志表
CREATE TABLE IF NOT EXISTS cmdb_maintenance_logs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    cmdb_item_id UUID NOT NULL REFERENCES cmdb_items(id) ON DELETE CASCADE,
    cmdb_item_name VARCHAR(200),
    action VARCHAR(20) NOT NULL, -- enter, exit
    reason VARCHAR(500),
    scheduled_end_at TIMESTAMPTZ,
    actual_end_at TIMESTAMPTZ,
    exit_type VARCHAR(20), -- manual, auto
    operator VARCHAR(100),
    created_at TIMESTAMPTZ DEFAULT NOW()
);

-- 索引
CREATE INDEX IF NOT EXISTS idx_cmdb_maintenance_logs_item_id ON cmdb_maintenance_logs(cmdb_item_id);
CREATE INDEX IF NOT EXISTS idx_cmdb_items_status ON cmdb_items(status);
CREATE INDEX IF NOT EXISTS idx_cmdb_items_maintenance_end_at ON cmdb_items(maintenance_end_at) WHERE status = 'maintenance';
