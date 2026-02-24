-- ==================== Impersonation 审批机制 ====================
-- 平台管理员需要通过审批流程才能以租户身份访问租户数据

-- Impersonation 申请表（平台级，不带 tenant_id 列）
CREATE TABLE IF NOT EXISTS impersonation_requests (
    id                 UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    requester_id       UUID NOT NULL REFERENCES users(id),
    requester_name     VARCHAR(200) NOT NULL,
    tenant_id          UUID NOT NULL REFERENCES tenants(id),
    tenant_name        VARCHAR(200) NOT NULL,
    reason             TEXT,
    duration_minutes   INT NOT NULL DEFAULT 60 CHECK (duration_minutes >= 1 AND duration_minutes <= 1440),
    status             VARCHAR(20) NOT NULL DEFAULT 'pending',
    approved_by        UUID REFERENCES users(id),
    approved_at        TIMESTAMPTZ,
    session_started_at TIMESTAMPTZ,
    session_expires_at TIMESTAMPTZ,
    completed_at       TIMESTAMPTZ,
    created_at         TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at         TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_impersonation_requests_tenant ON impersonation_requests(tenant_id);
CREATE INDEX idx_impersonation_requests_requester ON impersonation_requests(requester_id);
CREATE INDEX idx_impersonation_requests_status ON impersonation_requests(status);

-- 审批人配置表（租户级）
-- 记录每个租户中有权审批 Impersonation 请求的用户
CREATE TABLE IF NOT EXISTS impersonation_approvers (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id  UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    user_id    UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE(tenant_id, user_id)
);

CREATE INDEX idx_impersonation_approvers_tenant ON impersonation_approvers(tenant_id);
