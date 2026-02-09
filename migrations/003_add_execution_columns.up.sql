-- Migration: 003_add_execution_columns.up.sql
-- 执行任务扩展字段

-- 添加新字段到 execution_tasks 表
ALTER TABLE execution_tasks ADD COLUMN IF NOT EXISTS repository_id UUID REFERENCES git_repositories(id);
ALTER TABLE execution_tasks ADD COLUMN IF NOT EXISTS executor_type VARCHAR(20) DEFAULT 'local';
ALTER TABLE execution_tasks ADD COLUMN IF NOT EXISTS commit_id VARCHAR(40);
ALTER TABLE execution_tasks ADD COLUMN IF NOT EXISTS schedule_expr VARCHAR(50);
ALTER TABLE execution_tasks ADD COLUMN IF NOT EXISTS is_recurring BOOLEAN DEFAULT FALSE;
ALTER TABLE execution_tasks ADD COLUMN IF NOT EXISTS next_run_at TIMESTAMP WITH TIME ZONE;
ALTER TABLE execution_tasks ADD COLUMN IF NOT EXISTS last_run_at TIMESTAMP WITH TIME ZONE;
ALTER TABLE execution_tasks ADD COLUMN IF NOT EXISTS updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW();

-- 修改 playbook_id 为可空（因为现在直接从 repository 执行）
ALTER TABLE execution_tasks ALTER COLUMN playbook_id DROP NOT NULL;

-- 添加索引
CREATE INDEX IF NOT EXISTS idx_execution_tasks_repository ON execution_tasks(repository_id);
CREATE INDEX IF NOT EXISTS idx_execution_tasks_executor ON execution_tasks(executor_type);
CREATE INDEX IF NOT EXISTS idx_execution_tasks_commit ON execution_tasks(commit_id);
CREATE INDEX IF NOT EXISTS idx_execution_tasks_schedule ON execution_tasks(is_recurring, next_run_at) WHERE is_recurring = true;
