-- 回滚定时调度执行参数覆盖字段
ALTER TABLE execution_schedules DROP COLUMN IF EXISTS target_hosts_override;
ALTER TABLE execution_schedules DROP COLUMN IF EXISTS extra_vars_override;
ALTER TABLE execution_schedules DROP COLUMN IF EXISTS secrets_source_ids;
ALTER TABLE execution_schedules DROP COLUMN IF EXISTS skip_notification;
ALTER TABLE execution_schedules DROP COLUMN IF EXISTS additional_recipients;
