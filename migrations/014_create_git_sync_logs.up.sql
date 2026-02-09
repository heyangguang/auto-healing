-- Git 仓库同步日志表
CREATE TABLE IF NOT EXISTS git_sync_logs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    repository_id UUID NOT NULL REFERENCES git_repositories(id) ON DELETE CASCADE,
    trigger_type VARCHAR(20) NOT NULL DEFAULT 'manual', -- manual / scheduled
    action VARCHAR(20) NOT NULL, -- clone / pull
    status VARCHAR(20) NOT NULL, -- success / failed
    commit_id VARCHAR(40), -- 同步后的 commit ID
    branch VARCHAR(100), -- 同步的分支
    duration_ms INTEGER, -- 耗时（毫秒）
    error_message TEXT, -- 错误信息
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- 索引
CREATE INDEX IF NOT EXISTS idx_git_sync_logs_repository_id ON git_sync_logs(repository_id);
CREATE INDEX IF NOT EXISTS idx_git_sync_logs_created_at ON git_sync_logs(created_at DESC);
