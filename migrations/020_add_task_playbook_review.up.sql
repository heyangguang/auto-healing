-- 添加任务模板的 Playbook 变量变更检测字段
ALTER TABLE execution_tasks
ADD COLUMN IF NOT EXISTS playbook_variables_snapshot JSONB DEFAULT '[]',
ADD COLUMN IF NOT EXISTS needs_review BOOLEAN DEFAULT FALSE,
ADD COLUMN IF NOT EXISTS changed_variables JSONB DEFAULT '[]';

-- 添加注释
COMMENT ON COLUMN execution_tasks.playbook_variables_snapshot IS 'Playbook 变量快照（创建时复制）';
COMMENT ON COLUMN execution_tasks.needs_review IS '是否需要审核（Playbook 变量已变更）';
COMMENT ON COLUMN execution_tasks.changed_variables IS '变更的变量名列表';

-- 创建索引用于快速查询需要审核的任务
CREATE INDEX IF NOT EXISTS idx_execution_tasks_needs_review ON execution_tasks(needs_review) WHERE needs_review = TRUE;
