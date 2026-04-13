DROP INDEX IF EXISTS idx_plat_audit_auth_method;
DROP INDEX IF EXISTS idx_plat_audit_failure_reason;
DROP INDEX IF EXISTS idx_plat_audit_subject_tenant_id;
DROP INDEX IF EXISTS idx_plat_audit_subject_scope;

ALTER TABLE platform_audit_logs
    DROP COLUMN IF EXISTS auth_method,
    DROP COLUMN IF EXISTS failure_reason,
    DROP COLUMN IF EXISTS subject_tenant_name,
    DROP COLUMN IF EXISTS subject_tenant_id,
    DROP COLUMN IF EXISTS subject_scope,
    DROP COLUMN IF EXISTS principal_username;
