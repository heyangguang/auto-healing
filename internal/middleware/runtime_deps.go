package middleware

import (
	"context"

	"github.com/company/auto-healing/internal/database"
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
	return NewRuntimeDepsWithDB(database.DB)
}

func NewRuntimeDepsWithDB(db *gorm.DB) RuntimeDeps {
	if db == nil {
		db = database.DB
	}
	return RuntimeDeps{
		UserRepo:          accessrepo.NewUserRepositoryWithDB(db),
		TenantRepo:        accessrepo.NewTenantRepositoryWithDB(db),
		PermissionRepo:    accessrepo.NewPermissionRepositoryWithDB(db),
		RoleRepo:          accessrepo.NewRoleRepositoryWithDB(db),
		ImpersonationRepo: accessrepo.NewImpersonationRepositoryWithDB(db),
		AuditRepo:         auditrepo.NewAuditLogRepository(db),
		PlatformAuditRepo: auditrepo.NewPlatformAuditLogRepositoryWithDB(db),
		DB:                db,
	}
}

func (deps RuntimeDeps) withDefaults() RuntimeDeps {
	if deps.DB == nil {
		deps.DB = database.DB
	}
	if deps.UserRepo == nil {
		deps.UserRepo = accessrepo.NewUserRepositoryWithDB(deps.DB)
	}
	if deps.TenantRepo == nil {
		deps.TenantRepo = accessrepo.NewTenantRepositoryWithDB(deps.DB)
	}
	if deps.PermissionRepo == nil {
		deps.PermissionRepo = accessrepo.NewPermissionRepositoryWithDB(deps.DB)
	}
	if deps.RoleRepo == nil {
		deps.RoleRepo = accessrepo.NewRoleRepositoryWithDB(deps.DB)
	}
	if deps.ImpersonationRepo == nil {
		deps.ImpersonationRepo = accessrepo.NewImpersonationRepositoryWithDB(deps.DB)
	}
	if deps.AuditRepo == nil {
		deps.AuditRepo = auditrepo.NewAuditLogRepository(deps.DB)
	}
	if deps.PlatformAuditRepo == nil {
		deps.PlatformAuditRepo = auditrepo.NewPlatformAuditLogRepositoryWithDB(deps.DB)
	}
	return deps
}
