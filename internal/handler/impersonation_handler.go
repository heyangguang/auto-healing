package handler

import (
	"github.com/company/auto-healing/internal/database"
	"github.com/company/auto-healing/internal/repository"
)

// ImpersonationHandler Impersonation 处理器
type ImpersonationHandler struct {
	repo              *repository.ImpersonationRepository
	tenantRepo        *repository.TenantRepository
	auditRepo         *repository.AuditLogRepository
	platformAuditRepo *repository.PlatformAuditLogRepository
	siteMessageRepo   *repository.SiteMessageRepository
}

// NewImpersonationHandler 创建 Impersonation 处理器
func NewImpersonationHandler() *ImpersonationHandler {
	return &ImpersonationHandler{
		repo:              repository.NewImpersonationRepository(),
		tenantRepo:        repository.NewTenantRepository(),
		auditRepo:         repository.NewAuditLogRepository(database.DB),
		platformAuditRepo: repository.NewPlatformAuditLogRepository(),
		siteMessageRepo:   repository.NewSiteMessageRepository(),
	}
}

type createImpersonationRequest struct {
	TenantID        string `json:"tenant_id" binding:"required"`
	Reason          string `json:"reason"`
	DurationMinutes int    `json:"duration_minutes" binding:"required,min=1,max=1440"`
}

type setApproversRequest struct {
	UserIDs []string `json:"user_ids" binding:"required"`
}
