package repository

import (
	accessrepo "github.com/company/auto-healing/internal/modules/access/repository"
	"gorm.io/gorm"
)

type TenantRepository = accessrepo.TenantRepository
type UserRepository = accessrepo.UserRepository
type UserListParams = accessrepo.UserListParams
type SimpleUser = accessrepo.SimpleUser
type RoleRepository = accessrepo.RoleRepository
type RoleFilter = accessrepo.RoleFilter
type RoleStats = accessrepo.RoleStats
type RoleUserInfo = accessrepo.RoleUserInfo
type PermissionRepository = accessrepo.PermissionRepository
type PermissionFilter = accessrepo.PermissionFilter
type ImpersonationRepository = accessrepo.ImpersonationRepository

var (
	ErrTenantNotFound                   = accessrepo.ErrTenantNotFound
	ErrUserNotFound                     = accessrepo.ErrUserNotFound
	ErrUserExists                       = accessrepo.ErrUserExists
	ErrRoleNotFound                     = accessrepo.ErrRoleNotFound
	ErrPermissionNotFound               = accessrepo.ErrPermissionNotFound
	ErrTenantPermissionScope            = accessrepo.ErrTenantPermissionScope
	ErrTenantMemberAssociationCorrupted = accessrepo.ErrTenantMemberAssociationCorrupted
	ErrTenantStatsTableNotAllowed       = accessrepo.ErrTenantStatsTableNotAllowed
	ErrImpersonationRequestNotPending   = accessrepo.ErrImpersonationRequestNotPending
)

func NewTenantRepository() *TenantRepository {
	return accessrepo.NewTenantRepository()
}

func NewTenantRepositoryWithDB(db *gorm.DB) *TenantRepository {
	return accessrepo.NewTenantRepositoryWithDB(db)
}

func NewUserRepository() *UserRepository {
	return accessrepo.NewUserRepository()
}

func NewUserRepositoryWithDB(db *gorm.DB) *UserRepository {
	return accessrepo.NewUserRepositoryWithDB(db)
}

func NewRoleRepository() *RoleRepository {
	return accessrepo.NewRoleRepository()
}

func NewRoleRepositoryWithDB(db *gorm.DB) *RoleRepository {
	return accessrepo.NewRoleRepositoryWithDB(db)
}

func NewPermissionRepository() *PermissionRepository {
	return accessrepo.NewPermissionRepository()
}

func NewPermissionRepositoryWithDB(db *gorm.DB) *PermissionRepository {
	return accessrepo.NewPermissionRepositoryWithDB(db)
}

func NewImpersonationRepository() *ImpersonationRepository {
	return accessrepo.NewImpersonationRepository()
}
