-- Migration: 007_add_notification_config.up.sql
-- 通知模块配置扩展

-- 1. 添加执行任务的通知配置
ALTER TABLE execution_tasks 
ADD COLUMN IF NOT EXISTS notification_config JSONB;

COMMENT ON COLUMN execution_tasks.notification_config IS '通知配置: {enabled, on_success, on_failure, on_timeout, template_id, channel_ids, extra_recipients}';

-- 2. 添加渠道的重试配置和默认接收人
ALTER TABLE notification_channels 
ADD COLUMN IF NOT EXISTS retry_config JSONB DEFAULT '{"max_retries": 3, "retry_intervals": [1, 5, 15]}';

ALTER TABLE notification_channels
ADD COLUMN IF NOT EXISTS default_recipients JSONB DEFAULT '[]';

COMMENT ON COLUMN notification_channels.retry_config IS '重试配置: {max_retries, retry_intervals}';
COMMENT ON COLUMN notification_channels.default_recipients IS '默认接收人列表';

-- 3. 通知日志添加重试相关索引
CREATE INDEX IF NOT EXISTS idx_notification_logs_retry 
ON notification_logs(status, next_retry_at) 
WHERE status = 'failed' AND next_retry_at IS NOT NULL;

-- 4. 通知日志添加执行关联
ALTER TABLE notification_logs
ADD COLUMN IF NOT EXISTS execution_run_id UUID REFERENCES execution_runs(id);

CREATE INDEX IF NOT EXISTS idx_notification_logs_execution 
ON notification_logs(execution_run_id);
