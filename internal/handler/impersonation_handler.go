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

type ImpersonationHandlerDeps struct {
	ImpersonationRepo *repository.ImpersonationRepository
	TenantRepo        *repository.TenantRepository
	AuditRepo         *repository.AuditLogRepository
	PlatformAuditRepo *repository.PlatformAuditLogRepository
	SiteMessageRepo   *repository.SiteMessageRepository
}

// NewImpersonationHandler 创建 Impersonation 处理器
func NewImpersonationHandler() *ImpersonationHandler {
	return NewImpersonationHandlerWithDeps(ImpersonationHandlerDeps{
		ImpersonationRepo: repository.NewImpersonationRepository(),
		TenantRepo:        repository.NewTenantRepository(),
		AuditRepo:         repository.NewAuditLogRepository(database.DB),
		PlatformAuditRepo: repository.NewPlatformAuditLogRepository(),
		SiteMessageRepo:   repository.NewSiteMessageRepository(),
	})
}

func NewImpersonationHandlerWithDeps(deps ImpersonationHandlerDeps) *ImpersonationHandler {
	return &ImpersonationHandler{
		repo:              deps.ImpersonationRepo,
		tenantRepo:        deps.TenantRepo,
		auditRepo:         deps.AuditRepo,
		platformAuditRepo: deps.PlatformAuditRepo,
		siteMessageRepo:   deps.SiteMessageRepo,
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
