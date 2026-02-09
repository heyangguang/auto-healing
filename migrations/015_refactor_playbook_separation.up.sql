-- 015_refactor_playbook_separation.up.sql
-- Git 仓库与 Playbook 分离重构

-- ============================================================
-- 步骤 1: 增强 playbooks 表
-- ============================================================

-- 添加变量配置字段（含来源文件信息）
ALTER TABLE playbooks ADD COLUMN IF NOT EXISTS variables JSONB DEFAULT '[]';

-- 添加状态字段: draft / ready / outdated / invalid
ALTER TABLE playbooks ADD COLUMN IF NOT EXISTS status VARCHAR(20) DEFAULT 'draft';

-- 最后扫描时间
ALTER TABLE playbooks ADD COLUMN IF NOT EXISTS last_scanned_at TIMESTAMPTZ;

-- 扫描到的原始变量（用于合并对比）
ALTER TABLE playbooks ADD COLUMN IF NOT EXISTS scanned_variables JSONB DEFAULT '[]';

-- 创建者
ALTER TABLE playbooks ADD COLUMN IF NOT EXISTS created_by UUID;

-- ============================================================
-- 步骤 2: 创建扫描日志表
-- ============================================================

CREATE TABLE IF NOT EXISTS playbook_scan_logs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    playbook_id UUID NOT NULL REFERENCES playbooks(id) ON DELETE CASCADE,
    trigger_type VARCHAR(20) NOT NULL,  -- manual / repo_sync
    
    -- 扫描统计
    files_scanned INTEGER DEFAULT 0,
    variables_found INTEGER DEFAULT 0,
    new_count INTEGER DEFAULT 0,
    removed_count INTEGER DEFAULT 0,
    
    -- 详细变更
    details JSONB DEFAULT '{}',
    
    created_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_playbook_scan_logs_playbook_id ON playbook_scan_logs(playbook_id);

-- ============================================================
-- 步骤 3: 简化 git_repositories 表
-- ============================================================

-- 删除 Playbook 相关字段（这些字段将由 playbooks 表管理）
ALTER TABLE git_repositories DROP COLUMN IF EXISTS main_playbook;
ALTER TABLE git_repositories DROP COLUMN IF EXISTS variables;
ALTER TABLE git_repositories DROP COLUMN IF EXISTS config_mode;
ALTER TABLE git_repositories DROP COLUMN IF EXISTS is_active;

-- 统一状态值: pending/ready -> synced
UPDATE git_repositories SET status = 'synced' WHERE status = 'ready';
UPDATE git_repositories SET status = 'synced' WHERE status = 'pending';

-- ============================================================
-- 步骤 4: 调整 execution_tasks 表
-- ============================================================

-- playbook_id 改为必填（通过 playbook 关联仓库）
-- 注意：repository_id 将被移除，但先设为可空以便迁移

-- 确保 playbook_id 字段存在
-- ALTER TABLE execution_tasks ALTER COLUMN playbook_id SET NOT NULL;

-- 移除 repository_id 字段（不再需要，通过 playbook 获取）
ALTER TABLE execution_tasks DROP COLUMN IF EXISTS repository_id;
