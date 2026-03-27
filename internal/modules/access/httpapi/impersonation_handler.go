package httpapi

import (
	"github.com/company/auto-healing/internal/database"
	accessrepo "github.com/company/auto-healing/internal/modules/access/repository"
	auditrepo "github.com/company/auto-healing/internal/platform/repository/audit"
	"github.com/company/auto-healing/internal/repository"
)

// ImpersonationHandler Impersonation 处理器
type ImpersonationHandler struct {
	repo              *accessrepo.ImpersonationRepository
	tenantRepo        *accessrepo.TenantRepository
	auditRepo         *auditrepo.AuditLogRepository
	platformAuditRepo *auditrepo.PlatformAuditLogRepository
	siteMessageRepo   *repository.SiteMessageRepository
}

type ImpersonationHandlerDeps struct {
	ImpersonationRepo *accessrepo.ImpersonationRepository
	TenantRepo        *accessrepo.TenantRepository
	AuditRepo         *auditrepo.AuditLogRepository
	PlatformAuditRepo *auditrepo.PlatformAuditLogRepository
	SiteMessageRepo   *repository.SiteMessageRepository
}

// NewImpersonationHandler 创建 Impersonation 处理器
func NewImpersonationHandler() *ImpersonationHandler {
	return NewImpersonationHandlerWithDeps(ImpersonationHandlerDeps{
		ImpersonationRepo: accessrepo.NewImpersonationRepository(),
		TenantRepo:        accessrepo.NewTenantRepository(),
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
