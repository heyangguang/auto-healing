-- 回滚 scope 字段
ALTER TABLE roles DROP CONSTRAINT IF EXISTS chk_role_scope;
ALTER TABLE roles DROP COLUMN IF EXISTS scope;
