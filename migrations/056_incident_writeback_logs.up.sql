CREATE TABLE IF NOT EXISTS incident_writeback_logs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID,
    incident_id UUID NOT NULL,
    plugin_id UUID,
    external_id VARCHAR(200) NOT NULL,
    action VARCHAR(20) NOT NULL,
    trigger_source VARCHAR(50) NOT NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'pending',
    request_method VARCHAR(10),
    request_url TEXT,
    request_payload JSONB NOT NULL DEFAULT '{}'::jsonb,
    response_status_code INTEGER,
    response_body TEXT,
    error_message TEXT,
    operator_user_id UUID,
    operator_name VARCHAR(200),
    flow_instance_id UUID,
    execution_run_id UUID,
    started_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    finished_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_incident_writeback_logs_tenant_id ON incident_writeback_logs (tenant_id);
CREATE INDEX IF NOT EXISTS idx_incident_writeback_logs_incident_id ON incident_writeback_logs (incident_id);
CREATE INDEX IF NOT EXISTS idx_incident_writeback_logs_plugin_id ON incident_writeback_logs (plugin_id);
CREATE INDEX IF NOT EXISTS idx_incident_writeback_logs_flow_instance_id ON incident_writeback_logs (flow_instance_id);
CREATE INDEX IF NOT EXISTS idx_incident_writeback_logs_execution_run_id ON incident_writeback_logs (execution_run_id);
CREATE INDEX IF NOT EXISTS idx_incident_writeback_logs_status ON incident_writeback_logs (status);
CREATE INDEX IF NOT EXISTS idx_incident_writeback_logs_created_at ON incident_writeback_logs (created_at DESC);
