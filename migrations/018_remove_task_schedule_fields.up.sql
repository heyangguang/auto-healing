-- 从 execution_tasks 表移除定时任务相关字段
-- 这些字段已迁移至 execution_schedules 表

ALTER TABLE execution_tasks DROP COLUMN IF EXISTS schedule_expr;
ALTER TABLE execution_tasks DROP COLUMN IF EXISTS is_recurring;
ALTER TABLE execution_tasks DROP COLUMN IF EXISTS next_run_at;
