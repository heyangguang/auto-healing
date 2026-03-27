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

func NewRuntimeDepsWithDB(db *gorm.DB) RuntimeDeps {
	db = requireRuntimeDB(db)
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

func requireRuntimeDB(db *gorm.DB) *gorm.DB {
	if db == nil {
		panic("middleware runtime deps require explicit db")
	}
	return db
}

func (deps RuntimeDeps) requireDB() *gorm.DB {
	return requireRuntimeDB(deps.DB)
}

func (deps RuntimeDeps) requireUserRepo() *accessrepo.UserRepository {
	if deps.UserRepo == nil {
		panic("middleware runtime deps require UserRepo")
	}
	return deps.UserRepo
}

func (deps RuntimeDeps) requireTenantRepo() *accessrepo.TenantRepository {
	if deps.TenantRepo == nil {
		panic("middleware runtime deps require TenantRepo")
	}
	return deps.TenantRepo
}

func (deps RuntimeDeps) requirePermissionRepo() *accessrepo.PermissionRepository {
	if deps.PermissionRepo == nil {
		panic("middleware runtime deps require PermissionRepo")
	}
	return deps.PermissionRepo
}

func (deps RuntimeDeps) requireRoleRepo() *accessrepo.RoleRepository {
	if deps.RoleRepo == nil {
		panic("middleware runtime deps require RoleRepo")
	}
	return deps.RoleRepo
}

func (deps RuntimeDeps) requireImpersonationRepo() *accessrepo.ImpersonationRepository {
	if deps.ImpersonationRepo == nil {
		panic("middleware runtime deps require ImpersonationRepo")
	}
	return deps.ImpersonationRepo
}

func (deps RuntimeDeps) requireAuditRepo() *auditrepo.AuditLogRepository {
	if deps.AuditRepo == nil {
		panic("middleware runtime deps require AuditRepo")
	}
	return deps.AuditRepo
}

func (deps RuntimeDeps) requirePlatformAuditRepo() *auditrepo.PlatformAuditLogRepository {
	if deps.PlatformAuditRepo == nil {
		panic("middleware runtime deps require PlatformAuditRepo")
	}
	return deps.PlatformAuditRepo
}
