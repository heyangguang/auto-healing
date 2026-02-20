-- 多租户架构 — 租户表 + 用户租户角色表
-- Step 2 & 3: 创建 tenants 表、user_tenant_roles 表，迁移 user_roles 数据

-- 1. 创建 tenants 表
CREATE TABLE IF NOT EXISTS tenants (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name        VARCHAR(200) NOT NULL,
    code        VARCHAR(50) NOT NULL,
    description TEXT,
    status      VARCHAR(20) NOT NULL DEFAULT 'active',
    created_at  TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at  TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_tenants_code ON tenants(code);

-- 2. 插入默认租户（所有现有数据归入此租户）
INSERT INTO tenants (id, name, code, description, status)
VALUES (
    '00000000-0000-0000-0000-000000000001',
    '默认租户',
    'default',
    '系统默认租户，所有历史数据自动归入此租户',
    'active'
)
ON CONFLICT (code) DO NOTHING;

-- 3. 创建 user_tenant_roles 表（替代 user_roles）
CREATE TABLE IF NOT EXISTS user_tenant_roles (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id    UUID NOT NULL,
    tenant_id  UUID NOT NULL,
    role_id    UUID NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_user_tenant_roles_user ON user_tenant_roles(user_id);
CREATE INDEX IF NOT EXISTS idx_user_tenant_roles_tenant ON user_tenant_roles(tenant_id);
CREATE UNIQUE INDEX IF NOT EXISTS idx_user_tenant_roles_unique ON user_tenant_roles(user_id, tenant_id, role_id);

-- 4. 从 user_roles 迁移数据到 user_tenant_roles（绑定到 default 租户）
INSERT INTO user_tenant_roles (user_id, tenant_id, role_id)
SELECT user_id, '00000000-0000-0000-0000-000000000001', role_id
FROM user_roles
ON CONFLICT (user_id, tenant_id, role_id) DO NOTHING;

-- 5. User 表增加 is_platform_admin 字段
ALTER TABLE users ADD COLUMN IF NOT EXISTS is_platform_admin BOOLEAN DEFAULT FALSE;

-- 6. 将 admin 用户标记为平台管理员
UPDATE users SET is_platform_admin = true WHERE username = 'admin';

-- 7. Role 表增加 tenant_id 字段（NULL = 系统模板角色）
ALTER TABLE roles ADD COLUMN IF NOT EXISTS tenant_id UUID;
CREATE INDEX IF NOT EXISTS idx_roles_tenant ON roles(tenant_id);
