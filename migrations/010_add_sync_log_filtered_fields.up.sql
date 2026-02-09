-- 添加插件同步日志的筛选记录跟踪字段
ALTER TABLE plugin_sync_logs ADD COLUMN IF NOT EXISTS records_filtered INTEGER DEFAULT 0;
ALTER TABLE plugin_sync_logs ADD COLUMN IF NOT EXISTS records_new INTEGER DEFAULT 0;
ALTER TABLE plugin_sync_logs ADD COLUMN IF NOT EXISTS records_updated INTEGER DEFAULT 0;

COMMENT ON COLUMN plugin_sync_logs.records_filtered IS '被过滤器筛选掉的记录数';
COMMENT ON COLUMN plugin_sync_logs.records_new IS '新增记录数';
COMMENT ON COLUMN plugin_sync_logs.records_updated IS '更新记录数';
