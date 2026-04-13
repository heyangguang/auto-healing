ALTER TABLE platform_audit_logs
    ADD COLUMN IF NOT EXISTS principal_username VARCHAR(200),
    ADD COLUMN IF NOT EXISTS subject_scope VARCHAR(40),
    ADD COLUMN IF NOT EXISTS subject_tenant_id UUID,
    ADD COLUMN IF NOT EXISTS subject_tenant_name VARCHAR(200),
    ADD COLUMN IF NOT EXISTS failure_reason VARCHAR(100),
    ADD COLUMN IF NOT EXISTS auth_method VARCHAR(100);

CREATE INDEX IF NOT EXISTS idx_plat_audit_subject_scope ON platform_audit_logs(subject_scope);
CREATE INDEX IF NOT EXISTS idx_plat_audit_subject_tenant_id ON platform_audit_logs(subject_tenant_id);
CREATE INDEX IF NOT EXISTS idx_plat_audit_failure_reason ON platform_audit_logs(failure_reason);
CREATE INDEX IF NOT EXISTS idx_plat_audit_auth_method ON platform_audit_logs(auth_method);
