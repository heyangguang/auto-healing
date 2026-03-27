package middleware

import (
	"context"

	accessmodel "github.com/company/auto-healing/internal/modules/access/model"
	accessrepo "github.com/company/auto-healing/internal/modules/access/repository"
	auditrepo "github.com/company/auto-healing/internal/platform/repository/audit"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type userLookupRepository interface {
	GetByID(context.Context, uuid.UUID) (*accessmodel.User, error)
}

type tenantLookupRepository interface {
	GetByID(context.Context, uuid.UUID) (*accessmodel.Tenant, error)
}

type userTenantLister interface {
	GetUserTenants(context.Context, uuid.UUID, string) ([]accessmodel.Tenant, error)
}

type permissionCodeRepository interface {
	GetPlatformPermissionCodes(context.Context, uuid.UUID) ([]string, error)
	GetTenantPermissionCodes(context.Context, uuid.UUID, uuid.UUID) ([]string, error)
}

type RuntimeDeps struct {
	UserRepo          *accessrepo.UserRepository
	TenantRepo        *accessrepo.TenantRepository
	PermissionRepo    *accessrepo.PermissionRepository
	RoleRepo          *accessrepo.RoleRepository
	ImpersonationRepo *accessrepo.ImpersonationRepository
	AuditRepo         *auditrepo.AuditLogRepository
	PlatformAuditRepo *auditrepo.PlatformAuditLogRepository
	DB                *gorm.DB
}

func NewRuntimeDeps() RuntimeDeps {
	return RuntimeDeps{
		UserRepo:          accessrepo.NewUserRepository(),
		TenantRepo:        accessrepo.NewTenantRepository(),
		PermissionRepo:    accessrepo.NewPermissionRepository(),
		RoleRepo:          accessrepo.NewRoleRepository(),
		ImpersonationRepo: accessrepo.NewImpersonationRepository(),
		AuditRepo:         auditrepo.NewAuditLogRepository(defaultDatabase()),
		PlatformAuditRepo: auditrepo.NewPlatformAuditLogRepository(),
		DB:                defaultDatabase(),
	}
}

func (deps RuntimeDeps) withDefaults() RuntimeDeps {
	if deps.DB == nil {
		deps.DB = defaultDatabase()
	}
	if deps.UserRepo == nil {
		deps.UserRepo = accessrepo.NewUserRepository()
	}
	if deps.TenantRepo == nil {
		deps.TenantRepo = accessrepo.NewTenantRepository()
	}
	if deps.PermissionRepo == nil {
		deps.PermissionRepo = accessrepo.NewPermissionRepository()
	}
	if deps.RoleRepo == nil {
		deps.RoleRepo = accessrepo.NewRoleRepository()
	}
	if deps.ImpersonationRepo == nil {
		deps.ImpersonationRepo = accessrepo.NewImpersonationRepository()
	}
	if deps.AuditRepo == nil {
		deps.AuditRepo = auditrepo.NewAuditLogRepository(deps.DB)
	}
	if deps.PlatformAuditRepo == nil {
		deps.PlatformAuditRepo = auditrepo.NewPlatformAuditLogRepository()
	}
	return deps
}
