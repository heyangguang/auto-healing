-- 为 execution_schedules 表添加 last_run_at 字段
ALTER TABLE execution_schedules ADD COLUMN IF NOT EXISTS last_run_at TIMESTAMPTZ;
