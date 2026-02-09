-- 回滚调度类型重构

-- 1. 添加回 is_recurring 字段
ALTER TABLE execution_schedules ADD COLUMN IF NOT EXISTS is_recurring BOOLEAN DEFAULT true;

-- 2. 恢复数据
UPDATE execution_schedules SET is_recurring = true WHERE schedule_type = 'cron';
UPDATE execution_schedules SET is_recurring = false WHERE schedule_type = 'once';

-- 3. 恢复 schedule_expr 非空约束
ALTER TABLE execution_schedules ALTER COLUMN schedule_expr SET NOT NULL;

-- 4. 删除新字段
DROP INDEX IF EXISTS idx_execution_schedules_schedule_type;
ALTER TABLE execution_schedules DROP COLUMN IF EXISTS schedule_type;
ALTER TABLE execution_schedules DROP COLUMN IF EXISTS scheduled_at;
