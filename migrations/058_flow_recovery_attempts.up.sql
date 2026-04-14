CREATE TABLE IF NOT EXISTS flow_recovery_attempts (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID,
    flow_instance_id UUID NOT NULL,
    trigger_source VARCHAR(20) NOT NULL,
    current_node_id VARCHAR(100),
    current_node_type VARCHAR(50),
    detect_reason TEXT,
    recovery_action VARCHAR(50),
    status VARCHAR(20) NOT NULL DEFAULT 'started',
    details JSONB NOT NULL DEFAULT '{}'::jsonb,
    error_message TEXT,
    started_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    finished_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_flow_recovery_attempts_tenant_id ON flow_recovery_attempts(tenant_id);
CREATE INDEX IF NOT EXISTS idx_flow_recovery_attempts_flow_instance_id ON flow_recovery_attempts(flow_instance_id);
CREATE INDEX IF NOT EXISTS idx_flow_recovery_attempts_status ON flow_recovery_attempts(status);
CREATE INDEX IF NOT EXISTS idx_flow_recovery_attempts_created_at ON flow_recovery_attempts(created_at DESC);
