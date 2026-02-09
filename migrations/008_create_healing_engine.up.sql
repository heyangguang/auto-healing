-- 008_create_healing_engine.up.sql
-- 自愈引擎相关表（使用 UUID 主键）

-- 1. healing_flows（自愈流程）
CREATE TABLE IF NOT EXISTS healing_flows (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name            VARCHAR(255) NOT NULL,
    description     TEXT,
    nodes           JSONB NOT NULL DEFAULT '[]',
    edges           JSONB NOT NULL DEFAULT '[]',
    is_active       BOOLEAN NOT NULL DEFAULT true,
    created_by      UUID REFERENCES users(id),
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_healing_flows_is_active ON healing_flows(is_active);
CREATE INDEX IF NOT EXISTS idx_healing_flows_created_by ON healing_flows(created_by);

-- 2. healing_rules（自愈规则）
CREATE TABLE IF NOT EXISTS healing_rules (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name            VARCHAR(255) NOT NULL,
    description     TEXT,
    priority        INTEGER NOT NULL DEFAULT 0,
    trigger_mode    VARCHAR(20) NOT NULL DEFAULT 'auto',
    conditions      JSONB NOT NULL DEFAULT '[]',
    match_mode      VARCHAR(10) NOT NULL DEFAULT 'all',
    flow_id         UUID REFERENCES healing_flows(id) ON DELETE SET NULL,
    is_active       BOOLEAN NOT NULL DEFAULT false,
    last_run_at     TIMESTAMPTZ,
    created_by      UUID REFERENCES users(id),
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_healing_rules_is_active ON healing_rules(is_active);
CREATE INDEX IF NOT EXISTS idx_healing_rules_priority ON healing_rules(priority DESC);
CREATE INDEX IF NOT EXISTS idx_healing_rules_flow_id ON healing_rules(flow_id);

-- 3. flow_instances（流程实例）
CREATE TABLE IF NOT EXISTS flow_instances (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    flow_id         UUID NOT NULL REFERENCES healing_flows(id),
    rule_id         UUID REFERENCES healing_rules(id) ON DELETE SET NULL,
    incident_id     UUID REFERENCES incidents(id),
    status          VARCHAR(20) NOT NULL DEFAULT 'pending',
    current_node_id VARCHAR(100),
    context         JSONB NOT NULL DEFAULT '{}',
    node_states     JSONB NOT NULL DEFAULT '{}',
    error_message   TEXT,
    started_at      TIMESTAMPTZ,
    completed_at    TIMESTAMPTZ,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_flow_instances_flow_id ON flow_instances(flow_id);
CREATE INDEX IF NOT EXISTS idx_flow_instances_rule_id ON flow_instances(rule_id);
CREATE INDEX IF NOT EXISTS idx_flow_instances_incident_id ON flow_instances(incident_id);
CREATE INDEX IF NOT EXISTS idx_flow_instances_status ON flow_instances(status);

-- 4. approval_tasks（审批任务）
CREATE TABLE IF NOT EXISTS approval_tasks (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    flow_instance_id    UUID NOT NULL REFERENCES flow_instances(id) ON DELETE CASCADE,
    node_id             VARCHAR(100) NOT NULL,
    initiated_by        UUID REFERENCES users(id),
    approvers           JSONB NOT NULL DEFAULT '[]',
    approver_roles      JSONB NOT NULL DEFAULT '[]',
    status              VARCHAR(20) NOT NULL DEFAULT 'pending',
    timeout_at          TIMESTAMPTZ,
    decided_by          UUID REFERENCES users(id),
    decided_at          TIMESTAMPTZ,
    decision_comment    TEXT,
    notification_sent   BOOLEAN NOT NULL DEFAULT false,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_approval_tasks_flow_instance_id ON approval_tasks(flow_instance_id);
CREATE INDEX IF NOT EXISTS idx_approval_tasks_status ON approval_tasks(status);
CREATE INDEX IF NOT EXISTS idx_approval_tasks_timeout_at ON approval_tasks(timeout_at) WHERE status = 'pending';

-- 5. flow_execution_logs（流程执行日志）
CREATE TABLE IF NOT EXISTS flow_execution_logs (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    flow_instance_id    UUID NOT NULL REFERENCES flow_instances(id) ON DELETE CASCADE,
    node_id             VARCHAR(100) NOT NULL,
    node_type           VARCHAR(50) NOT NULL,
    level               VARCHAR(20) NOT NULL DEFAULT 'info',
    message             TEXT NOT NULL,
    details             JSONB,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_flow_execution_logs_instance_id ON flow_execution_logs(flow_instance_id);
CREATE INDEX IF NOT EXISTS idx_flow_execution_logs_node_id ON flow_execution_logs(node_id);
CREATE INDEX IF NOT EXISTS idx_flow_execution_logs_level ON flow_execution_logs(level);

-- 6. incidents 表扩展
ALTER TABLE incidents ADD COLUMN IF NOT EXISTS scanned BOOLEAN NOT NULL DEFAULT false;
ALTER TABLE incidents ADD COLUMN IF NOT EXISTS matched_rule_id UUID REFERENCES healing_rules(id) ON DELETE SET NULL;
ALTER TABLE incidents ADD COLUMN IF NOT EXISTS healing_flow_instance_id UUID REFERENCES flow_instances(id) ON DELETE SET NULL;

CREATE INDEX IF NOT EXISTS idx_incidents_scanned ON incidents(scanned) WHERE scanned = false;
CREATE INDEX IF NOT EXISTS idx_incidents_matched_rule_id ON incidents(matched_rule_id);
CREATE INDEX IF NOT EXISTS idx_incidents_healing_flow_instance_id ON incidents(healing_flow_instance_id);

-- 7. 添加自愈引擎相关权限
INSERT INTO permissions (code, name, module, resource, action) VALUES
    ('healing:flows:view', '查看自愈流程', 'healing', 'flows', 'view'),
    ('healing:flows:create', '创建自愈流程', 'healing', 'flows', 'create'),
    ('healing:flows:update', '更新自愈流程', 'healing', 'flows', 'update'),
    ('healing:flows:delete', '删除自愈流程', 'healing', 'flows', 'delete'),
    ('healing:rules:view', '查看自愈规则', 'healing', 'rules', 'view'),
    ('healing:rules:create', '创建自愈规则', 'healing', 'rules', 'create'),
    ('healing:rules:update', '更新自愈规则', 'healing', 'rules', 'update'),
    ('healing:rules:delete', '删除自愈规则', 'healing', 'rules', 'delete'),
    ('healing:instances:view', '查看流程实例', 'healing', 'instances', 'view'),
    ('healing:approvals:view', '查看审批任务', 'healing', 'approvals', 'view'),
    ('healing:approvals:approve', '审批操作', 'healing', 'approvals', 'approve')
ON CONFLICT (code) DO NOTHING;

-- 8. 给 admin 角色添加所有自愈权限
INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id
FROM roles r, permissions p
WHERE r.name = 'admin' AND p.module = 'healing'
ON CONFLICT DO NOTHING;
