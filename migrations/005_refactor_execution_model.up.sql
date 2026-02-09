-- =============================================================================
-- 重构执行模型：分离任务模板和执行记录
-- =============================================================================

-- 1. 创建 execution_runs 表（执行记录）
CREATE TABLE IF NOT EXISTS execution_runs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    task_id UUID NOT NULL,
    status VARCHAR(50) DEFAULT 'pending',  -- pending, running, success, failed, cancelled, timeout
    exit_code INT,
    stats JSONB DEFAULT '{}',
    stdout TEXT,
    stderr TEXT,
    triggered_by VARCHAR(200),
    started_at TIMESTAMPTZ,
    completed_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    
    CONSTRAINT fk_execution_runs_task 
        FOREIGN KEY (task_id) REFERENCES execution_tasks(id) ON DELETE CASCADE
);

-- 创建索引
CREATE INDEX IF NOT EXISTS idx_execution_runs_task_id ON execution_runs(task_id);
CREATE INDEX IF NOT EXISTS idx_execution_runs_status ON execution_runs(status);
CREATE INDEX IF NOT EXISTS idx_execution_runs_created_at ON execution_runs(created_at DESC);

-- 2. 修改 execution_logs: 添加 run_id 字段（关联执行记录）
ALTER TABLE execution_logs ADD COLUMN IF NOT EXISTS run_id UUID;

-- 为新字段添加外键约束（级联删除）
DO $$ 
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'fk_execution_logs_run') THEN
        ALTER TABLE execution_logs ADD CONSTRAINT fk_execution_logs_run 
            FOREIGN KEY (run_id) REFERENCES execution_runs(id) ON DELETE CASCADE;
    END IF;
END $$;

-- 创建 run_id 索引
CREATE INDEX IF NOT EXISTS idx_execution_logs_run_id ON execution_logs(run_id);

-- 3. 从 execution_tasks 移除执行结果字段（保留任务模板字段）
ALTER TABLE execution_tasks DROP COLUMN IF EXISTS status;
ALTER TABLE execution_tasks DROP COLUMN IF EXISTS exit_code;
ALTER TABLE execution_tasks DROP COLUMN IF EXISTS stats;
ALTER TABLE execution_tasks DROP COLUMN IF EXISTS stdout;
ALTER TABLE execution_tasks DROP COLUMN IF EXISTS stderr;
ALTER TABLE execution_tasks DROP COLUMN IF EXISTS started_at;
ALTER TABLE execution_tasks DROP COLUMN IF EXISTS completed_at;
ALTER TABLE execution_tasks DROP COLUMN IF EXISTS timeout_at;
ALTER TABLE execution_tasks DROP COLUMN IF EXISTS last_run_at;

-- 4. 添加任务名称字段
ALTER TABLE execution_tasks ADD COLUMN IF NOT EXISTS name VARCHAR(200);

-- 5. 清理 execution_logs 中的 task_id 字段（改用 run_id）
-- 移除 NOT NULL 约束，因为新架构不再需要 task_id
ALTER TABLE execution_logs ALTER COLUMN task_id DROP NOT NULL;

-- 6. 删除之前添加的 run_id 相关迁移中的列（如果存在重复）
-- 这是为了确保干净的迁移状态
