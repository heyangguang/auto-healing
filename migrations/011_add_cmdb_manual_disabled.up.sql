-- 给 CMDB 配置项添加手动禁用字段
ALTER TABLE cmdb_items ADD COLUMN IF NOT EXISTS manual_disabled BOOLEAN DEFAULT FALSE;
ALTER TABLE cmdb_items ADD COLUMN IF NOT EXISTS manual_disabled_reason VARCHAR(500);
ALTER TABLE cmdb_items ADD COLUMN IF NOT EXISTS manual_disabled_at TIMESTAMPTZ;
