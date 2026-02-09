-- 回滚：移除 last_run_at 字段
ALTER TABLE execution_schedules DROP COLUMN IF EXISTS last_run_at;
