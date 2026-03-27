package repository

import "github.com/company/auto-healing/internal/database"

func NewUserRepository() *UserRepository {
	return NewUserRepositoryWithDB(database.DB)
}

func NewTenantRepository() *TenantRepository {
	return NewTenantRepositoryWithDB(database.DB)
}

func NewRoleRepository() *RoleRepository {
	return NewRoleRepositoryWithDB(database.DB)
}

func NewPermissionRepository() *PermissionRepository {
	return NewPermissionRepositoryWithDB(database.DB)
}

func NewInvitationRepository() *InvitationRepository {
	return NewInvitationRepositoryWithDB(database.DB)
}

func NewImpersonationRepository() *ImpersonationRepository {
	return NewImpersonationRepositoryWithDB(database.DB)
}
