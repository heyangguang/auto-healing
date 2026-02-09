-- 回滚 status 字段
DROP INDEX IF EXISTS idx_execution_schedules_status;
ALTER TABLE execution_schedules DROP COLUMN IF EXISTS status;
