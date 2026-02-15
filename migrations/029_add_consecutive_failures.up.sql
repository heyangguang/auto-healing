-- 同步/定时任务连续失败自动暂停机制
-- max_failures: 用户配置的最大连续失败次数，0 表示不启用自动暂停
-- consecutive_failures: 运行时连续失败计数器
-- pause_reason: 自动暂停时记录的暂停原因

-- git_repositories
ALTER TABLE git_repositories ADD COLUMN IF NOT EXISTS max_failures INT DEFAULT 5;
ALTER TABLE git_repositories ADD COLUMN IF NOT EXISTS consecutive_failures INT DEFAULT 0;
ALTER TABLE git_repositories ADD COLUMN IF NOT EXISTS pause_reason VARCHAR(500);

-- plugins
ALTER TABLE plugins ADD COLUMN IF NOT EXISTS max_failures INT DEFAULT 5;
ALTER TABLE plugins ADD COLUMN IF NOT EXISTS consecutive_failures INT DEFAULT 0;
ALTER TABLE plugins ADD COLUMN IF NOT EXISTS pause_reason VARCHAR(500);

-- execution_schedules
ALTER TABLE execution_schedules ADD COLUMN IF NOT EXISTS max_failures INT DEFAULT 5;
ALTER TABLE execution_schedules ADD COLUMN IF NOT EXISTS consecutive_failures INT DEFAULT 0;
ALTER TABLE execution_schedules ADD COLUMN IF NOT EXISTS pause_reason VARCHAR(500);
