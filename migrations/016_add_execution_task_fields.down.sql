-- 回滚 execution_tasks 新增字段

ALTER TABLE execution_tasks DROP COLUMN IF EXISTS description;
ALTER TABLE execution_tasks DROP COLUMN IF EXISTS secrets_source_ids;
