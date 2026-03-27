package httpapi

import (
	"time"

	accessmodel "github.com/company/auto-healing/internal/modules/access/model"
	"github.com/google/uuid"
)

type invitationValidationResponse struct {
	ID         uuid.UUID `json:"id"`
	Email      string    `json:"email"`
	TenantName string    `json:"tenant_name"`
	TenantCode string    `json:"tenant_code"`
	RoleName   string    `json:"role_name"`
	ExpiresAt  time.Time `json:"expires_at"`
}

type invitationRegisterResponse struct {
	User    *accessmodel.User `json:"user"`
	Message string            `json:"message"`
}

type tenantStatsOverviewResponse struct {
	Tenants []TenantStatsItem  `json:"tenants"`
	Summary TenantStatsSummary `json:"summary"`
}

type tenantTrendResponse struct {
	Dates          []string `json:"dates"`
	Operations     []int64  `json:"operations"`
	AuditLogs      []int64  `json:"audit_logs"`
	TaskExecutions []int64  `json:"task_executions"`
}
