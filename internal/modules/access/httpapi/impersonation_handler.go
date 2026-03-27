package httpapi

import (
	"github.com/company/auto-healing/internal/database"
	auditrepo "github.com/company/auto-healing/internal/platform/repository/audit"
	"github.com/company/auto-healing/internal/repository"
)

// ImpersonationHandler Impersonation 处理器
type ImpersonationHandler struct {
	repo              *repository.ImpersonationRepository
	tenantRepo        *repository.TenantRepository
	auditRepo         *auditrepo.AuditLogRepository
	platformAuditRepo *auditrepo.PlatformAuditLogRepository
	siteMessageRepo   *repository.SiteMessageRepository
}

type ImpersonationHandlerDeps struct {
	ImpersonationRepo *repository.ImpersonationRepository
	TenantRepo        *repository.TenantRepository
	AuditRepo         *auditrepo.AuditLogRepository
	PlatformAuditRepo *auditrepo.PlatformAuditLogRepository
	SiteMessageRepo   *repository.SiteMessageRepository
}

// NewImpersonationHandler 创建 Impersonation 处理器
func NewImpersonationHandler() *ImpersonationHandler {
	return NewImpersonationHandlerWithDeps(ImpersonationHandlerDeps{
		ImpersonationRepo: repository.NewImpersonationRepository(),
		TenantRepo:        repository.NewTenantRepository(),
		AuditRepo:         auditrepo.NewAuditLogRepository(database.DB),
		PlatformAuditRepo: auditrepo.NewPlatformAuditLogRepository(),
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
