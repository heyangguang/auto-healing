-- 添加 config_mode 字段
ALTER TABLE playbooks ADD COLUMN IF NOT EXISTS config_mode VARCHAR(20);

-- 将 draft 状态改为 pending
UPDATE playbooks SET status = 'pending' WHERE status = 'draft';

-- 将 outdated 状态改为 pending  
UPDATE playbooks SET status = 'pending' WHERE status = 'outdated';

-- 为已扫描过的记录设置 config_mode = 'auto'
UPDATE playbooks SET config_mode = 'auto' WHERE last_scanned_at IS NOT NULL AND config_mode IS NULL;
