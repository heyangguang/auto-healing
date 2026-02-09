-- 009_create_flow_execution_logs.up.sql
-- 自愈流程执行日志表
-- 用于记录 Workflow 流程中每个节点的执行日志

CREATE TABLE IF NOT EXISTS flow_execution_logs (
    id                  BIGSERIAL PRIMARY KEY,
    flow_instance_id    BIGINT NOT NULL REFERENCES flow_instances(id) ON DELETE CASCADE,
    node_id             VARCHAR(100) NOT NULL,
    node_type           VARCHAR(50) NOT NULL,
    level               VARCHAR(20) NOT NULL DEFAULT 'info',
    message             TEXT NOT NULL,
    details             JSONB,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- 索引
CREATE INDEX idx_flow_logs_instance ON flow_execution_logs(flow_instance_id);
CREATE INDEX idx_flow_logs_node ON flow_execution_logs(flow_instance_id, node_id);
CREATE INDEX idx_flow_logs_level ON flow_execution_logs(level);
CREATE INDEX idx_flow_logs_created ON flow_execution_logs(created_at);

-- 注释
COMMENT ON TABLE flow_execution_logs IS '自愈流程执行日志表';
COMMENT ON COLUMN flow_execution_logs.id IS '日志ID';
COMMENT ON COLUMN flow_execution_logs.flow_instance_id IS '关联的流程实例ID';
COMMENT ON COLUMN flow_execution_logs.node_id IS '节点ID';
COMMENT ON COLUMN flow_execution_logs.node_type IS '节点类型 (start/end/host_extractor/cmdb_validator/execution/notification/approval/condition)';
COMMENT ON COLUMN flow_execution_logs.level IS '日志级别 (debug/info/warn/error)';
COMMENT ON COLUMN flow_execution_logs.message IS '日志消息';
COMMENT ON COLUMN flow_execution_logs.details IS '详细信息 (JSON格式，如 Ansible stdout/stderr)';
COMMENT ON COLUMN flow_execution_logs.created_at IS '创建时间';
