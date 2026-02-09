-- 回滚维护模式改动

-- 删除维护日志表
DROP TABLE IF EXISTS cmdb_maintenance_logs;

-- 删除维护模式字段
ALTER TABLE cmdb_items DROP COLUMN IF EXISTS maintenance_reason;
ALTER TABLE cmdb_items DROP COLUMN IF EXISTS maintenance_start_at;
ALTER TABLE cmdb_items DROP COLUMN IF EXISTS maintenance_end_at;

-- 恢复旧字段
ALTER TABLE cmdb_items ADD COLUMN IF NOT EXISTS manual_disabled BOOLEAN DEFAULT FALSE;
ALTER TABLE cmdb_items ADD COLUMN IF NOT EXISTS manual_disabled_reason VARCHAR(500);
ALTER TABLE cmdb_items ADD COLUMN IF NOT EXISTS manual_disabled_at TIMESTAMP;
