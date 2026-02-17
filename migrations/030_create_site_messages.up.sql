-- 站内信（Site Messages）- 独立于通知管理模块
-- 三张表: 消息主表、已读状态表、全局设置表

-- 站内信消息主表
CREATE TABLE IF NOT EXISTS site_messages (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    category VARCHAR(50) NOT NULL,
    title VARCHAR(500) NOT NULL,
    content TEXT NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    expires_at TIMESTAMP WITH TIME ZONE
);

CREATE INDEX IF NOT EXISTS idx_site_messages_category ON site_messages(category);
CREATE INDEX IF NOT EXISTS idx_site_messages_created_at ON site_messages(created_at DESC);
CREATE INDEX IF NOT EXISTS idx_site_messages_expires_at ON site_messages(expires_at);

-- 站内信已读状态表（懒创建：标记已读时才插入）
CREATE TABLE IF NOT EXISTS site_message_reads (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    message_id UUID NOT NULL REFERENCES site_messages(id) ON DELETE CASCADE,
    user_id UUID NOT NULL,
    read_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_site_message_read_unique ON site_message_reads(message_id, user_id);
CREATE INDEX IF NOT EXISTS idx_site_message_reads_user_id ON site_message_reads(user_id);

-- 站内信全局设置（单行表）
CREATE TABLE IF NOT EXISTS site_message_settings (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    retention_days INT NOT NULL DEFAULT 90,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- 插入默认设置行
INSERT INTO site_message_settings (retention_days) VALUES (90)
ON CONFLICT DO NOTHING;
