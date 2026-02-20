-- 1. 创建平台审计日志表
CREATE TABLE IF NOT EXISTS platform_audit_logs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID,
    username VARCHAR(200),
    ip_address VARCHAR(45),
    user_agent TEXT,
    category VARCHAR(20) NOT NULL DEFAULT 'operation',
    action VARCHAR(100) NOT NULL,
    resource_type VARCHAR(100) NOT NULL,
    resource_id UUID,
    resource_name VARCHAR(200),
    request_method VARCHAR(10),
    request_path VARCHAR(500),
    request_body JSONB,
    response_status INTEGER,
    changes JSONB,
    status VARCHAR(20) NOT NULL,
    error_message TEXT,
    created_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX idx_plat_audit_created ON platform_audit_logs(created_at);
CREATE INDEX idx_plat_audit_user ON platform_audit_logs(user_id);
CREATE INDEX idx_plat_audit_category ON platform_audit_logs(category);
CREATE INDEX idx_plat_audit_action ON platform_audit_logs(action);

-- 2. 给现有 audit_logs 增加 category 字段
ALTER TABLE audit_logs ADD COLUMN IF NOT EXISTS category VARCHAR(20) NOT NULL DEFAULT 'operation';
CREATE INDEX IF NOT EXISTS idx_audit_logs_category ON audit_logs(category);

-- 3. 清空现有审计日志（用户要求从头开始）
TRUNCATE TABLE audit_logs;
