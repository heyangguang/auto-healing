-- 重构调度类型：从 is_recurring 改为 schedule_type (cron/once)
-- schedule_type: 'cron'=循环调度, 'once'=单次调度

-- 1. 添加新字段
ALTER TABLE execution_schedules ADD COLUMN IF NOT EXISTS schedule_type VARCHAR(10) DEFAULT 'cron';
ALTER TABLE execution_schedules ADD COLUMN IF NOT EXISTS scheduled_at TIMESTAMPTZ;

-- 2. 迁移现有数据（所有基于 is_recurring 的调度转为对应类型）
UPDATE execution_schedules SET schedule_type = 'cron' WHERE is_recurring = true;
UPDATE execution_schedules SET schedule_type = 'once' WHERE is_recurring = false;

-- 3. 修改 schedule_expr 为可空（once 模式不需要）
ALTER TABLE execution_schedules ALTER COLUMN schedule_expr DROP NOT NULL;

-- 4. 删除 is_recurring 字段
ALTER TABLE execution_schedules DROP COLUMN IF EXISTS is_recurring;

-- 5. 创建索引
CREATE INDEX IF NOT EXISTS idx_execution_schedules_schedule_type ON execution_schedules(schedule_type);
