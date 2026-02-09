-- 为 execution_schedules 表添加 status 字段
-- 状态值：running(运行中), pending(待执行), completed(已完成), disabled(已禁用)

ALTER TABLE execution_schedules ADD COLUMN IF NOT EXISTS status VARCHAR(20) DEFAULT 'disabled';

-- 根据现有数据更新状态
-- 1. 禁用的调度 → disabled
UPDATE execution_schedules SET status = 'disabled' WHERE enabled = false;

-- 2. 循环调度 + 启用 → running
UPDATE execution_schedules SET status = 'running' WHERE enabled = true AND is_recurring = true;

-- 3. 单次调度 + 启用 + 已执行 → completed
UPDATE execution_schedules SET status = 'completed' WHERE enabled = true AND is_recurring = false AND last_run_at IS NOT NULL;

-- 4. 单次调度 + 启用 + 未执行 → pending
UPDATE execution_schedules SET status = 'pending' WHERE enabled = true AND is_recurring = false AND last_run_at IS NULL;

-- 创建索引
CREATE INDEX IF NOT EXISTS idx_execution_schedules_status ON execution_schedules(status);
