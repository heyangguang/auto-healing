-- 创建定时任务调度表
-- 将定时配置从 execution_tasks 分离，支持一个任务模板关联多个调度计划

CREATE TABLE IF NOT EXISTS execution_schedules (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(200) NOT NULL,
    task_id UUID NOT NULL REFERENCES execution_tasks(id) ON DELETE CASCADE,
    schedule_expr VARCHAR(50) NOT NULL,
    is_recurring BOOLEAN DEFAULT true,
    next_run_at TIMESTAMPTZ,
    last_run_at TIMESTAMPTZ,
    enabled BOOLEAN DEFAULT true,
    description TEXT DEFAULT '',
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

-- 创建索引
CREATE INDEX IF NOT EXISTS idx_execution_schedules_task_id ON execution_schedules(task_id);
CREATE INDEX IF NOT EXISTS idx_execution_schedules_enabled ON execution_schedules(enabled);
CREATE INDEX IF NOT EXISTS idx_execution_schedules_next_run_at ON execution_schedules(next_run_at);

-- 迁移现有定时任务数据（如果 execution_tasks 中有定时配置）
INSERT INTO execution_schedules (name, task_id, schedule_expr, is_recurring, next_run_at, enabled)
SELECT 
    name || ' - 定时调度', 
    id, 
    schedule_expr, 
    is_recurring, 
    next_run_at, 
    true
FROM execution_tasks
WHERE schedule_expr IS NOT NULL AND schedule_expr != ''
ON CONFLICT DO NOTHING;
