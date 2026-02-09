-- 添加执行记录运行时参数快照字段
-- 用于记录每次执行实际使用的参数，方便排错和重试

ALTER TABLE execution_runs
ADD COLUMN IF NOT EXISTS runtime_target_hosts TEXT,
ADD COLUMN IF NOT EXISTS runtime_secrets_source_ids JSONB DEFAULT '[]',
ADD COLUMN IF NOT EXISTS runtime_extra_vars JSONB DEFAULT '{}',
ADD COLUMN IF NOT EXISTS runtime_skip_notification BOOLEAN DEFAULT FALSE;

-- 添加注释
COMMENT ON COLUMN execution_runs.runtime_target_hosts IS '运行时目标主机快照';
COMMENT ON COLUMN execution_runs.runtime_secrets_source_ids IS '运行时密钥源ID列表快照';
COMMENT ON COLUMN execution_runs.runtime_extra_vars IS '运行时变量快照';
COMMENT ON COLUMN execution_runs.runtime_skip_notification IS '运行时跳过通知标记';
