-- Migration: 002_create_core_tables.up.sql
-- 核心业务表

-- ==================== 插件模块 ====================

-- 插件表
CREATE TABLE IF NOT EXISTS plugins (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(100) NOT NULL UNIQUE,
    type VARCHAR(50) NOT NULL,                    -- itsm, cmdb
    adapter VARCHAR(50) NOT NULL DEFAULT 'servicenow', -- servicenow, jira, zabbix, custom
    description TEXT,
    version VARCHAR(20) NOT NULL DEFAULT '1.0.0',
    config JSONB NOT NULL,
    field_mapping JSONB NOT NULL DEFAULT '{}',
    sync_filter JSONB,                            -- 同步过滤器配置
    sync_enabled BOOLEAN DEFAULT true,
    sync_interval_minutes INTEGER DEFAULT 5,
    last_sync_at TIMESTAMP WITH TIME ZONE,
    next_sync_at TIMESTAMP WITH TIME ZONE,
    status VARCHAR(20) DEFAULT 'inactive',
    error_message TEXT,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- 插件同步日志表
CREATE TABLE IF NOT EXISTS plugin_sync_logs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    plugin_id UUID NOT NULL REFERENCES plugins(id) ON DELETE CASCADE,
    sync_type VARCHAR(20) NOT NULL,
    status VARCHAR(20) NOT NULL,
    records_fetched INTEGER DEFAULT 0,
    records_processed INTEGER DEFAULT 0,
    records_failed INTEGER DEFAULT 0,
    started_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    completed_at TIMESTAMP WITH TIME ZONE,
    error_message TEXT,
    details JSONB DEFAULT '{}'
);

-- ==================== 工作流模块 ====================

-- 工作流表
CREATE TABLE IF NOT EXISTS workflows (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(200) NOT NULL,
    description TEXT,
    version INTEGER DEFAULT 1,
    status VARCHAR(20) DEFAULT 'draft',
    trigger_type VARCHAR(50) NOT NULL,
    trigger_config JSONB DEFAULT '{}',
    created_by VARCHAR(200),
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- 工作流节点表
CREATE TABLE IF NOT EXISTS workflow_nodes (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workflow_id UUID NOT NULL REFERENCES workflows(id) ON DELETE CASCADE,
    node_type VARCHAR(50) NOT NULL,
    name VARCHAR(200) NOT NULL,
    description TEXT,
    config JSONB NOT NULL DEFAULT '{}',
    position_x INTEGER DEFAULT 0,
    position_y INTEGER DEFAULT 0,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- 工作流边表
CREATE TABLE IF NOT EXISTS workflow_edges (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workflow_id UUID NOT NULL REFERENCES workflows(id) ON DELETE CASCADE,
    source_node_id UUID NOT NULL REFERENCES workflow_nodes(id) ON DELETE CASCADE,
    target_node_id UUID NOT NULL REFERENCES workflow_nodes(id) ON DELETE CASCADE,
    condition_expression TEXT,
    label VARCHAR(100),
    priority INTEGER DEFAULT 0,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- 工单/事件表
CREATE TABLE IF NOT EXISTS incidents (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    plugin_id UUID REFERENCES plugins(id),                -- 可空，插件删除后为 NULL
    source_plugin_name VARCHAR(100),                      -- 插件名称（插件删除后保留）
    external_id VARCHAR(200) NOT NULL,
    title VARCHAR(500) NOT NULL,
    description TEXT,
    severity VARCHAR(20),
    priority VARCHAR(20),
    status VARCHAR(50),
    category VARCHAR(100),
    affected_ci VARCHAR(200),
    affected_service VARCHAR(200),
    assignee VARCHAR(200),
    reporter VARCHAR(200),
    raw_data JSONB NOT NULL,
    healing_status VARCHAR(50) DEFAULT 'pending',
    workflow_instance_id UUID,
    source_created_at TIMESTAMP WITH TIME ZONE,
    source_updated_at TIMESTAMP WITH TIME ZONE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- 唯一约束改为条件约束：只有当 plugin_id 不为空时才检查唯一性
CREATE UNIQUE INDEX IF NOT EXISTS idx_incidents_plugin_external 
    ON incidents(plugin_id, external_id) WHERE plugin_id IS NOT NULL;

CREATE INDEX idx_incidents_status ON incidents(healing_status);
CREATE INDEX idx_incidents_severity ON incidents(severity);
CREATE INDEX idx_incidents_plugin ON incidents(plugin_id);
CREATE INDEX idx_incidents_source_plugin_name ON incidents(source_plugin_name);

-- CMDB 配置项表
CREATE TABLE IF NOT EXISTS cmdb_items (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    plugin_id UUID REFERENCES plugins(id),                -- 可空，插件删除后为 NULL
    source_plugin_name VARCHAR(100),                      -- 插件名称（插件删除后保留）
    external_id VARCHAR(200) NOT NULL,
    name VARCHAR(200) NOT NULL,
    type VARCHAR(50),                                     -- server, application, network, database
    status VARCHAR(50),                                   -- active, inactive, maintenance
    ip_address VARCHAR(50),
    hostname VARCHAR(200),
    os VARCHAR(100),
    os_version VARCHAR(100),
    cpu VARCHAR(100),
    memory VARCHAR(50),
    disk VARCHAR(100),
    location VARCHAR(200),
    owner VARCHAR(200),
    environment VARCHAR(50),                              -- prod, test, dev
    manufacturer VARCHAR(100),
    model VARCHAR(100),
    serial_number VARCHAR(100),
    department VARCHAR(100),
    dependencies JSONB DEFAULT '[]',
    tags JSONB DEFAULT '{}',
    raw_data JSONB NOT NULL,
    source_created_at TIMESTAMP WITH TIME ZONE,
    source_updated_at TIMESTAMP WITH TIME ZONE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- 唯一约束：只有当 plugin_id 不为空时才检查唯一性
CREATE UNIQUE INDEX IF NOT EXISTS idx_cmdb_items_plugin_external 
    ON cmdb_items(plugin_id, external_id) WHERE plugin_id IS NOT NULL;

CREATE INDEX idx_cmdb_items_plugin ON cmdb_items(plugin_id);
CREATE INDEX idx_cmdb_items_source_plugin_name ON cmdb_items(source_plugin_name);
CREATE INDEX idx_cmdb_items_type ON cmdb_items(type);
CREATE INDEX idx_cmdb_items_status ON cmdb_items(status);
CREATE INDEX idx_cmdb_items_hostname ON cmdb_items(hostname);
CREATE INDEX idx_cmdb_items_ip_address ON cmdb_items(ip_address);
CREATE INDEX idx_cmdb_items_environment ON cmdb_items(environment);

-- 工作流实例表
CREATE TABLE IF NOT EXISTS workflow_instances (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workflow_id UUID NOT NULL REFERENCES workflows(id),
    incident_id UUID REFERENCES incidents(id),
    status VARCHAR(50) DEFAULT 'running',
    current_node_id UUID REFERENCES workflow_nodes(id),
    context JSONB DEFAULT '{}',
    started_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    completed_at TIMESTAMP WITH TIME ZONE,
    error_message TEXT
);

-- 更新 incidents 表的外键
ALTER TABLE incidents 
ADD CONSTRAINT fk_incidents_workflow_instance 
FOREIGN KEY (workflow_instance_id) REFERENCES workflow_instances(id);

-- 节点执行记录表
CREATE TABLE IF NOT EXISTS node_executions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workflow_instance_id UUID NOT NULL REFERENCES workflow_instances(id) ON DELETE CASCADE,
    node_id UUID NOT NULL REFERENCES workflow_nodes(id),
    status VARCHAR(50) NOT NULL,
    input_data JSONB DEFAULT '{}',
    output_data JSONB DEFAULT '{}',
    started_at TIMESTAMP WITH TIME ZONE,
    completed_at TIMESTAMP WITH TIME ZONE,
    retry_count INTEGER DEFAULT 0,
    error_message TEXT
);

-- ==================== 执行模块 ====================

-- Git 仓库表
CREATE TABLE IF NOT EXISTS git_repositories (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(100) NOT NULL UNIQUE,
    url VARCHAR(500) NOT NULL,
    default_branch VARCHAR(100) DEFAULT 'main',
    auth_type VARCHAR(20) DEFAULT 'none',     -- none, token, password, ssh_key
    auth_config JSONB,                        -- 认证配置
    local_path VARCHAR(500),
    branches JSONB,                           -- 分支列表  
    last_sync_at TIMESTAMP WITH TIME ZONE,
    last_commit_id VARCHAR(40),               -- 最后同步的 commit ID
    status VARCHAR(20) DEFAULT 'pending',     -- pending, ready, syncing, error
    error_message TEXT,
    -- 定时同步配置
    sync_enabled BOOLEAN DEFAULT FALSE,
    sync_interval VARCHAR(20) DEFAULT '1h',    -- 同步间隔，如 10s, 5m, 1h
    next_sync_at TIMESTAMP WITH TIME ZONE,
    -- Playbook 配置
    main_playbook VARCHAR(200),               -- 主入口文件
    config_mode VARCHAR(20),                  -- auto / enhanced
    variables JSONB DEFAULT '[]',             -- 变量定义
    is_active BOOLEAN DEFAULT FALSE,          -- 是否激活
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_git_repositories_status ON git_repositories(status);
CREATE INDEX IF NOT EXISTS idx_git_repositories_sync ON git_repositories(sync_enabled, next_sync_at);

-- Playbook 表
CREATE TABLE IF NOT EXISTS playbooks (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    repository_id UUID NOT NULL REFERENCES git_repositories(id) ON DELETE CASCADE,
    name VARCHAR(200) NOT NULL,
    description TEXT,
    file_path VARCHAR(500) NOT NULL,
    tags JSONB DEFAULT '[]',
    required_vars JSONB DEFAULT '[]',
    default_inventory VARCHAR(500),
    default_extra_vars JSONB DEFAULT '{}',
    default_timeout_minutes INTEGER DEFAULT 60,
    is_active BOOLEAN DEFAULT true,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    UNIQUE(repository_id, file_path)
);

-- 执行任务表
CREATE TABLE IF NOT EXISTS execution_tasks (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    playbook_id UUID NOT NULL REFERENCES playbooks(id),
    workflow_instance_id UUID REFERENCES workflow_instances(id),
    node_execution_id UUID REFERENCES node_executions(id),
    triggered_by VARCHAR(200),
    target_hosts TEXT NOT NULL,
    extra_vars JSONB DEFAULT '{}',
    status VARCHAR(50) DEFAULT 'pending',
    ansible_job_id VARCHAR(100),
    exit_code INTEGER,
    stdout TEXT,
    stderr TEXT,
    stats JSONB,
    scheduled_at TIMESTAMP WITH TIME ZONE,
    started_at TIMESTAMP WITH TIME ZONE,
    completed_at TIMESTAMP WITH TIME ZONE,
    timeout_at TIMESTAMP WITH TIME ZONE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE INDEX idx_execution_tasks_status ON execution_tasks(status);
CREATE INDEX idx_execution_tasks_playbook ON execution_tasks(playbook_id);

-- ==================== 通知模块 ====================

-- 通知渠道表
CREATE TABLE IF NOT EXISTS notification_channels (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(200) NOT NULL UNIQUE,
    type VARCHAR(50) NOT NULL,
    description TEXT,
    config JSONB NOT NULL,
    is_active BOOLEAN DEFAULT true,
    is_default BOOLEAN DEFAULT false,
    rate_limit_per_minute INTEGER,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- 通知模板表
CREATE TABLE IF NOT EXISTS notification_templates (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(200) NOT NULL,
    description TEXT,
    event_type VARCHAR(50),
    supported_channels JSONB DEFAULT '[]',
    subject_template TEXT,
    body_template TEXT NOT NULL,
    format VARCHAR(20) DEFAULT 'text',
    available_variables JSONB DEFAULT '[]',
    is_active BOOLEAN DEFAULT true,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- 通知日志表
CREATE TABLE IF NOT EXISTS notification_logs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    template_id UUID REFERENCES notification_templates(id),
    channel_id UUID NOT NULL REFERENCES notification_channels(id),
    workflow_instance_id UUID REFERENCES workflow_instances(id),
    incident_id UUID REFERENCES incidents(id),
    recipients JSONB NOT NULL,
    subject TEXT,
    body TEXT NOT NULL,
    status VARCHAR(50) DEFAULT 'pending',
    external_message_id VARCHAR(200),
    response_data JSONB,
    error_message TEXT,
    retry_count INTEGER DEFAULT 0,
    next_retry_at TIMESTAMP WITH TIME ZONE,
    sent_at TIMESTAMP WITH TIME ZONE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- ==================== 日志模块 ====================

-- 审计日志表
CREATE TABLE IF NOT EXISTS audit_logs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID REFERENCES users(id),
    username VARCHAR(200),
    ip_address VARCHAR(45),
    user_agent TEXT,
    action VARCHAR(100) NOT NULL,
    resource_type VARCHAR(100) NOT NULL,
    resource_id UUID,
    resource_name VARCHAR(200),
    request_method VARCHAR(10),
    request_path VARCHAR(500),
    request_body JSONB,
    response_status INTEGER,
    changes JSONB,
    status VARCHAR(20) NOT NULL,
    error_message TEXT,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE INDEX idx_audit_logs_user ON audit_logs(user_id);
CREATE INDEX idx_audit_logs_action ON audit_logs(action);
CREATE INDEX idx_audit_logs_resource ON audit_logs(resource_type, resource_id);
CREATE INDEX idx_audit_logs_created ON audit_logs(created_at);

-- 执行日志表
CREATE TABLE IF NOT EXISTS execution_logs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    task_id UUID NOT NULL REFERENCES execution_tasks(id) ON DELETE CASCADE,
    workflow_instance_id UUID REFERENCES workflow_instances(id),
    node_execution_id UUID REFERENCES node_executions(id),
    log_level VARCHAR(20) NOT NULL,
    stage VARCHAR(50) NOT NULL,
    message TEXT NOT NULL,
    host VARCHAR(200),
    task_name VARCHAR(200),
    play_name VARCHAR(200),
    details JSONB DEFAULT '{}',
    sequence INTEGER NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE INDEX idx_execution_logs_task ON execution_logs(task_id);
CREATE INDEX idx_execution_logs_level ON execution_logs(log_level);
CREATE INDEX idx_execution_logs_sequence ON execution_logs(task_id, sequence);

-- 工作流日志表
CREATE TABLE IF NOT EXISTS workflow_logs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workflow_instance_id UUID NOT NULL REFERENCES workflow_instances(id) ON DELETE CASCADE,
    node_id UUID REFERENCES workflow_nodes(id),
    log_level VARCHAR(20) NOT NULL,
    stage VARCHAR(50) NOT NULL,
    message TEXT NOT NULL,
    details JSONB DEFAULT '{}',
    sequence INTEGER NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE INDEX idx_workflow_logs_instance ON workflow_logs(workflow_instance_id);

-- ==================== 密钥管理模块 ====================

-- 密钥源表
CREATE TABLE IF NOT EXISTS secrets_sources (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(100) NOT NULL UNIQUE,
    type VARCHAR(20) NOT NULL,             -- vault, file, webhook
    config JSONB NOT NULL,                 -- 连接配置（加密存储）
    is_default BOOLEAN DEFAULT false,
    priority INTEGER DEFAULT 0,
    status VARCHAR(20) DEFAULT 'active',
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE INDEX idx_secrets_sources_type ON secrets_sources(type);
CREATE INDEX idx_secrets_sources_status ON secrets_sources(status);
