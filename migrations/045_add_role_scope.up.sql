-- 为角色表添加 scope 字段，区分平台级和租户级角色
ALTER TABLE roles ADD COLUMN scope VARCHAR(20) NOT NULL DEFAULT 'tenant';

-- 设置 platform_admin 为平台级角色
UPDATE roles SET scope = 'platform' WHERE name = 'platform_admin';

-- 添加 CHECK 约束
ALTER TABLE roles ADD CONSTRAINT chk_role_scope CHECK (scope IN ('platform', 'tenant'));
