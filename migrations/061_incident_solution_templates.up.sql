CREATE TABLE IF NOT EXISTS incident_solution_templates (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID REFERENCES tenants(id) ON DELETE CASCADE,
    name VARCHAR(200) NOT NULL,
    description TEXT DEFAULT '',
    resolution_template TEXT NOT NULL,
    work_notes_template TEXT NOT NULL,
    default_close_code VARCHAR(100) DEFAULT '',
    default_close_status VARCHAR(50) DEFAULT 'resolved',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (tenant_id, name)
);

CREATE INDEX IF NOT EXISTS idx_incident_solution_templates_tenant_id
    ON incident_solution_templates (tenant_id);
