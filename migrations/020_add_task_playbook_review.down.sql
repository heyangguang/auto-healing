-- 移除任务模板的 Playbook 变量变更检测字段
DROP INDEX IF EXISTS idx_execution_tasks_needs_review;

ALTER TABLE execution_tasks
DROP COLUMN IF EXISTS playbook_variables_snapshot,
DROP COLUMN IF EXISTS needs_review,
DROP COLUMN IF EXISTS changed_variables;
