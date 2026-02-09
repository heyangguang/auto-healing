-- 027: 系统工作区 & 角色-工作区关联
-- 系统工作区（管理员创建的模板，可分配给角色）

CREATE TABLE IF NOT EXISTS system_workspaces (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name        VARCHAR(200) NOT NULL,
    description TEXT DEFAULT '',
    config      JSONB NOT NULL DEFAULT '{}',
    is_default  BOOLEAN NOT NULL DEFAULT false,
    created_by  UUID REFERENCES users(id) ON DELETE SET NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- 角色-工作区关联表（多对多）
CREATE TABLE IF NOT EXISTS role_workspaces (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    role_id      UUID NOT NULL REFERENCES roles(id) ON DELETE CASCADE,
    workspace_id UUID NOT NULL REFERENCES system_workspaces(id) ON DELETE CASCADE,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE(role_id, workspace_id)
);

CREATE INDEX IF NOT EXISTS idx_role_workspaces_role_id ON role_workspaces(role_id);
CREATE INDEX IF NOT EXISTS idx_role_workspaces_workspace_id ON role_workspaces(workspace_id);

COMMENT ON TABLE system_workspaces IS '系统工作区模板，管理员创建并分配给角色';
COMMENT ON TABLE role_workspaces IS '角色与系统工作区的多对多关联';

-- 插入默认运维总览工作区（所有用户可见）
INSERT INTO system_workspaces (name, description, config, is_default) VALUES (
    '运维总览',
    '默认运维监控工作区，所有用户自动可见',
    '{"widgets":[
        {"instanceId":"w-1","widgetId":"stat-incident-total"},
        {"instanceId":"w-2","widgetId":"stat-healing-rate"},
        {"instanceId":"w-3","widgetId":"stat-pending-items"},
        {"instanceId":"w-4","widgetId":"stat-exec-success"},
        {"instanceId":"w-5","widgetId":"chart-incident-status"},
        {"instanceId":"w-6","widgetId":"chart-instance-status"},
        {"instanceId":"w-7","widgetId":"list-recent-instances"},
        {"instanceId":"w-8","widgetId":"list-pending-approvals"}
    ],"layouts":[
        {"i":"w-1","x":0,"y":0,"w":3,"h":2,"minW":2,"minH":2},
        {"i":"w-2","x":3,"y":0,"w":3,"h":2,"minW":2,"minH":2},
        {"i":"w-3","x":6,"y":0,"w":3,"h":2,"minW":2,"minH":2},
        {"i":"w-4","x":9,"y":0,"w":3,"h":2,"minW":2,"minH":2},
        {"i":"w-5","x":0,"y":2,"w":6,"h":3,"minW":4,"minH":3},
        {"i":"w-6","x":6,"y":2,"w":6,"h":3,"minW":4,"minH":3},
        {"i":"w-7","x":0,"y":5,"w":6,"h":5,"minW":4,"minH":3},
        {"i":"w-8","x":6,"y":5,"w":6,"h":5,"minW":4,"minH":3}
    ]}',
    true
) ON CONFLICT DO NOTHING;
