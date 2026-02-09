-- 015_refactor_playbook_separation.down.sql
-- 回滚 Git 仓库与 Playbook 分离重构

-- 恢复 execution_tasks 的 repository_id 字段
ALTER TABLE execution_tasks ADD COLUMN IF NOT EXISTS repository_id UUID;

-- 恢复 git_repositories 的字段
ALTER TABLE git_repositories ADD COLUMN IF NOT EXISTS main_playbook VARCHAR(200);
ALTER TABLE git_repositories ADD COLUMN IF NOT EXISTS variables JSONB DEFAULT '[]';
ALTER TABLE git_repositories ADD COLUMN IF NOT EXISTS config_mode VARCHAR(20) DEFAULT 'auto';
ALTER TABLE git_repositories ADD COLUMN IF NOT EXISTS is_active BOOLEAN DEFAULT false;

-- 删除扫描日志表
DROP TABLE IF EXISTS playbook_scan_logs;

-- 删除 playbooks 新增字段
ALTER TABLE playbooks DROP COLUMN IF EXISTS variables;
ALTER TABLE playbooks DROP COLUMN IF EXISTS status;
ALTER TABLE playbooks DROP COLUMN IF EXISTS last_scanned_at;
ALTER TABLE playbooks DROP COLUMN IF EXISTS scanned_variables;
ALTER TABLE playbooks DROP COLUMN IF EXISTS created_by;
