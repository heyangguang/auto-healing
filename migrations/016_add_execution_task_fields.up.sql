-- 为 execution_tasks 添加 description 和 secrets_source_ids 字段
-- 支持任务模板编辑功能

ALTER TABLE execution_tasks ADD COLUMN IF NOT EXISTS description TEXT DEFAULT '';
ALTER TABLE execution_tasks ADD COLUMN IF NOT EXISTS secrets_source_ids JSONB DEFAULT '[]';
