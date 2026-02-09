-- Migration: 001_create_users_and_rbac.up.sql
-- 用户和权限相关表

-- 用户表
CREATE TABLE IF NOT EXISTS users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    username VARCHAR(100) NOT NULL UNIQUE,
    email VARCHAR(200) NOT NULL UNIQUE,
    password_hash VARCHAR(200) NOT NULL,
    display_name VARCHAR(200),
    phone VARCHAR(50),
    avatar_url VARCHAR(500),
    status VARCHAR(20) DEFAULT 'active',
    last_login_at TIMESTAMP WITH TIME ZONE,
    last_login_ip VARCHAR(45),
    password_changed_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    failed_login_count INTEGER DEFAULT 0,
    locked_until TIMESTAMP WITH TIME ZONE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- 角色表
CREATE TABLE IF NOT EXISTS roles (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(100) NOT NULL UNIQUE,
    display_name VARCHAR(200) NOT NULL,
    description TEXT,
    is_system BOOLEAN DEFAULT false,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- 权限表
CREATE TABLE IF NOT EXISTS permissions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    code VARCHAR(200) NOT NULL UNIQUE,
    name VARCHAR(200) NOT NULL,
    description TEXT,
    module VARCHAR(50) NOT NULL,
    resource VARCHAR(50) NOT NULL,
    action VARCHAR(50) NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- 角色权限关联表
CREATE TABLE IF NOT EXISTS role_permissions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    role_id UUID NOT NULL REFERENCES roles(id) ON DELETE CASCADE,
    permission_id UUID NOT NULL REFERENCES permissions(id) ON DELETE CASCADE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    UNIQUE(role_id, permission_id)
);

-- 用户角色关联表
CREATE TABLE IF NOT EXISTS user_roles (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    role_id UUID NOT NULL REFERENCES roles(id) ON DELETE CASCADE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    UNIQUE(user_id, role_id)
);

-- Token 黑名单
CREATE TABLE IF NOT EXISTS token_blacklist (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    token_jti VARCHAR(100) NOT NULL UNIQUE,
    user_id UUID REFERENCES users(id),
    expired_at TIMESTAMP WITH TIME ZONE NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE INDEX idx_token_blacklist_jti ON token_blacklist(token_jti);
CREATE INDEX idx_token_blacklist_expired ON token_blacklist(expired_at);

-- 刷新 Token 表
CREATE TABLE IF NOT EXISTS refresh_tokens (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token_hash VARCHAR(200) NOT NULL UNIQUE,
    device_info JSONB,
    expired_at TIMESTAMP WITH TIME ZONE NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- 插入默认角色
INSERT INTO roles (name, display_name, description, is_system) VALUES
('super_admin', '超级管理员', '拥有所有权限', true),
('admin', '管理员', '管理用户和系统配置', true),
('operator', '运维人员', '执行运维操作', true),
('viewer', '只读用户', '只能查看信息', true)
ON CONFLICT (name) DO NOTHING;

-- 插入预定义权限
INSERT INTO permissions (code, name, module, resource, action) VALUES
-- 插件模块
('plugin:list', '查看插件列表', 'plugin', 'plugin', 'read'),
('plugin:detail', '查看插件详情', 'plugin', 'plugin', 'read'),
('plugin:create', '创建插件', 'plugin', 'plugin', 'create'),
('plugin:update', '更新插件', 'plugin', 'plugin', 'update'),
('plugin:delete', '删除插件', 'plugin', 'plugin', 'delete'),
('plugin:sync', '触发同步', 'plugin', 'plugin', 'execute'),
('plugin:test', '测试连接', 'plugin', 'plugin', 'execute'),
-- 工作流模块
('workflow:list', '查看工作流列表', 'workflow', 'workflow', 'read'),
('workflow:detail', '查看工作流详情', 'workflow', 'workflow', 'read'),
('workflow:create', '创建工作流', 'workflow', 'workflow', 'create'),
('workflow:update', '更新工作流', 'workflow', 'workflow', 'update'),
('workflow:delete', '删除工作流', 'workflow', 'workflow', 'delete'),
('workflow:activate', '激活工作流', 'workflow', 'workflow', 'execute'),
('workflow:run', '手动触发执行', 'workflow', 'workflow', 'execute'),
-- 执行模块
('repository:list', '查看仓库列表', 'execution', 'repository', 'read'),
('repository:create', '添加仓库', 'execution', 'repository', 'create'),
('repository:update', '更新仓库', 'execution', 'repository', 'update'),
('repository:delete', '删除仓库', 'execution', 'repository', 'delete'),
('repository:sync', '同步仓库', 'execution', 'repository', 'execute'),
('playbook:list', '查看Playbook列表', 'execution', 'playbook', 'read'),
('playbook:execute', '执行Playbook', 'execution', 'playbook', 'execute'),
('task:list', '查看任务列表', 'execution', 'task', 'read'),
('task:detail', '查看任务详情', 'execution', 'task', 'read'),
('task:cancel', '取消任务', 'execution', 'task', 'execute'),
-- 通知模块
('channel:list', '查看通知渠道', 'notification', 'channel', 'read'),
('channel:create', '创建通知渠道', 'notification', 'channel', 'create'),
('channel:update', '更新通知渠道', 'notification', 'channel', 'update'),
('channel:delete', '删除通知渠道', 'notification', 'channel', 'delete'),
('template:list', '查看通知模板', 'notification', 'template', 'read'),
('template:create', '创建通知模板', 'notification', 'template', 'create'),
('template:update', '更新通知模板', 'notification', 'template', 'update'),
('template:delete', '删除通知模板', 'notification', 'template', 'delete'),
-- 用户管理
('user:list', '查看用户列表', 'user', 'user', 'read'),
('user:create', '创建用户', 'user', 'user', 'create'),
('user:update', '更新用户', 'user', 'user', 'update'),
('user:delete', '删除用户', 'user', 'user', 'delete'),
('user:reset_password', '重置密码', 'user', 'user', 'manage'),
-- 角色管理
('role:list', '查看角色列表', 'role', 'role', 'read'),
('role:create', '创建角色', 'role', 'role', 'create'),
('role:update', '更新角色', 'role', 'role', 'update'),
('role:delete', '删除角色', 'role', 'role', 'delete'),
('role:assign', '分配权限', 'role', 'role', 'manage'),
-- 系统管理
('audit:list', '查看审计日志', 'system', 'audit', 'read'),
('audit:export', '导出审计日志', 'system', 'audit', 'export'),
('system:settings', '系统设置', 'system', 'settings', 'manage')
ON CONFLICT (code) DO NOTHING;

-- 为超级管理员分配所有权限
INSERT INTO role_permissions (role_id, permission_id)
SELECT 
    (SELECT id FROM roles WHERE name = 'super_admin'),
    id
FROM permissions
ON CONFLICT DO NOTHING;
