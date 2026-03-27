package access

import (
	"github.com/company/auto-healing/internal/config"
	"github.com/company/auto-healing/internal/database"
	accesshttp "github.com/company/auto-healing/internal/modules/access/httpapi"
	"github.com/company/auto-healing/internal/repository"
)

// Module 聚合 access 域的处理器构造。
type Module struct {
	Auth          *accesshttp.AuthHandler
	User          *accesshttp.UserHandler
	TenantUser    *accesshttp.TenantUserHandler
	Role          *accesshttp.RoleHandler
	Permission    *accesshttp.PermissionHandler
	Tenant        *accesshttp.TenantHandler
	Impersonation *accesshttp.ImpersonationHandler
}

// New 创建 access 域模块。
func New(cfg *config.Config) *Module {
	authHandler := accesshttp.NewAuthHandler(cfg)
	authSvc := authHandler.GetAuthService()
	userRepo := repository.NewUserRepository()
	roleRepo := repository.NewRoleRepository()
	tenantRepo := repository.NewTenantRepository()
	permissionRepo := repository.NewPermissionRepository()

	return &Module{
		Auth: authHandler,
		User: accesshttp.NewUserHandlerWithDeps(accesshttp.UserHandlerDeps{
			UserRepo:    userRepo,
			RoleRepo:    roleRepo,
			AuthService: authSvc,
		}),
		TenantUser: accesshttp.NewTenantUserHandlerWithDeps(accesshttp.TenantUserHandlerDeps{
			AuthService: authSvc,
			TenantRepo:  tenantRepo,
			UserRepo:    userRepo,
			RoleRepo:    roleRepo,
		}),
		Role: accesshttp.NewRoleHandlerWithDeps(accesshttp.RoleHandlerDeps{
			RoleRepo:       roleRepo,
			PermissionRepo: permissionRepo,
		}),
		Permission: accesshttp.NewPermissionHandlerWithDeps(accesshttp.PermissionHandlerDeps{
			PermissionRepo: permissionRepo,
		}),
		Tenant: accesshttp.NewTenantHandlerWithDeps(accesshttp.TenantHandlerDeps{
			TenantRepo:  tenantRepo,
			RoleRepo:    roleRepo,
			UserRepo:    userRepo,
			AuthService: authSvc,
		}),
		Impersonation: accesshttp.NewImpersonationHandlerWithDeps(accesshttp.ImpersonationHandlerDeps{
			ImpersonationRepo: repository.NewImpersonationRepository(),
			TenantRepo:        tenantRepo,
			AuditRepo:         repository.NewAuditLogRepository(database.DB),
			PlatformAuditRepo: repository.NewPlatformAuditLogRepository(),
			SiteMessageRepo:   repository.NewSiteMessageRepository(),
		}),
	}
}
