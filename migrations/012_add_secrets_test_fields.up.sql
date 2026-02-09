-- 密钥源配置简化：添加测试相关字段
ALTER TABLE secrets_sources ADD COLUMN IF NOT EXISTS last_test_at TIMESTAMPTZ;
ALTER TABLE secrets_sources ADD COLUMN IF NOT EXISTS last_test_result BOOLEAN;

-- 修改默认状态为 inactive（新创建的密钥源需要测试通过才能启用）
ALTER TABLE secrets_sources ALTER COLUMN status SET DEFAULT 'inactive';
