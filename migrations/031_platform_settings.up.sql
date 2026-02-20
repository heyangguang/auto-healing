-- 平台级设置（Platform Settings）
-- 通用 KV 设置表，替代 site_message_settings
-- 与租户无关，所有租户共享同一份配置

-- 1. 创建 platform_settings 表
CREATE TABLE IF NOT EXISTS platform_settings (
    key           VARCHAR(100) PRIMARY KEY,
    value         TEXT NOT NULL,
    type          VARCHAR(20) NOT NULL DEFAULT 'string',
    module        VARCHAR(50) NOT NULL,
    label         VARCHAR(200) NOT NULL,
    description   TEXT,
    default_value TEXT,
    updated_at    TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_by    UUID
);

CREATE INDEX IF NOT EXISTS idx_platform_settings_module ON platform_settings(module);

-- 2. 从 site_message_settings 迁移数据到 platform_settings
INSERT INTO platform_settings (key, value, type, module, label, description, default_value, updated_at)
SELECT
    'site_message.retention_days',
    retention_days::text,
    'int',
    'site_message',
    '站内信保留天数',
    '站内信消息的自动过期天数，超过此天数的消息将被自动清理。设置后仅对新消息生效，已有消息的过期时间不变。',
    '90',
    updated_at
FROM site_message_settings
LIMIT 1
ON CONFLICT (key) DO NOTHING;

-- 如果 site_message_settings 为空（没有数据），插入默认值
INSERT INTO platform_settings (key, value, type, module, label, description, default_value)
VALUES (
    'site_message.retention_days',
    '90',
    'int',
    'site_message',
    '站内信保留天数',
    '站内信消息的自动过期天数，超过此天数的消息将被自动清理。设置后仅对新消息生效，已有消息的过期时间不变。',
    '90'
)
ON CONFLICT (key) DO NOTHING;

-- 3. 删除旧的 site_message_settings 表
DROP TABLE IF EXISTS site_message_settings;
