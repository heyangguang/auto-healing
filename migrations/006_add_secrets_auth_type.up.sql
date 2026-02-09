-- =============================================================================
-- 密钥服务增强：添加 auth_type 字段
-- =============================================================================

-- 1. 为 secrets_sources 表添加 auth_type 字段
ALTER TABLE secrets_sources ADD COLUMN IF NOT EXISTS auth_type VARCHAR(20) DEFAULT 'ssh_key';

-- 2. 更新现有数据的 auth_type（默认为 ssh_key）
UPDATE secrets_sources SET auth_type = 'ssh_key' WHERE auth_type IS NULL;

-- 3. 设置 NOT NULL 约束
ALTER TABLE secrets_sources ALTER COLUMN auth_type SET NOT NULL;

-- 4. 创建索引
CREATE INDEX IF NOT EXISTS idx_secrets_sources_auth_type ON secrets_sources(auth_type);
