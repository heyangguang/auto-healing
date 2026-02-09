-- 回滚：恢复 execution_tasks 表的定时任务字段

ALTER TABLE execution_tasks ADD COLUMN IF NOT EXISTS schedule_expr VARCHAR(50);
ALTER TABLE execution_tasks ADD COLUMN IF NOT EXISTS is_recurring BOOLEAN DEFAULT false;
ALTER TABLE execution_tasks ADD COLUMN IF NOT EXISTS next_run_at TIMESTAMP;
