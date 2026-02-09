-- 为定时调度添加执行参数覆盖字段
ALTER TABLE execution_schedules ADD COLUMN IF NOT EXISTS target_hosts_override TEXT;
ALTER TABLE execution_schedules ADD COLUMN IF NOT EXISTS extra_vars_override JSONB;
ALTER TABLE execution_schedules ADD COLUMN IF NOT EXISTS secrets_source_ids JSONB DEFAULT '[]';
ALTER TABLE execution_schedules ADD COLUMN IF NOT EXISTS skip_notification BOOLEAN DEFAULT FALSE;
ALTER TABLE execution_schedules ADD COLUMN IF NOT EXISTS additional_recipients JSONB;

COMMENT ON COLUMN execution_schedules.target_hosts_override IS '覆盖目标主机';
COMMENT ON COLUMN execution_schedules.extra_vars_override IS '覆盖变量';
COMMENT ON COLUMN execution_schedules.secrets_source_ids IS '覆盖密钥源';
COMMENT ON COLUMN execution_schedules.skip_notification IS '跳过通知';
COMMENT ON COLUMN execution_schedules.additional_recipients IS '额外接收者';
