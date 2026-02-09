-- 为 execution_logs 添加 run_id 字段，用于区分定时任务多次执行的日志批次
ALTER TABLE execution_logs ADD COLUMN IF NOT EXISTS run_id UUID;

-- 为已有数据设置默认 run_id（使用随机 UUID）
UPDATE execution_logs SET run_id = gen_random_uuid() WHERE run_id IS NULL;

-- 添加非空约束
ALTER TABLE execution_logs ALTER COLUMN run_id SET DEFAULT gen_random_uuid();

-- 创建索引便于按批次查询
CREATE INDEX IF NOT EXISTS idx_execution_logs_run_id ON execution_logs(run_id);
CREATE INDEX IF NOT EXISTS idx_execution_logs_task_run ON execution_logs(task_id, run_id);
