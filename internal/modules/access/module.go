package access

import (
	"github.com/company/auto-healing/internal/config"
	"github.com/company/auto-healing/internal/database"
	"github.com/company/auto-healing/internal/handler"
	"github.com/company/auto-healing/internal/repository"
)

// Module 聚合 access 域的处理器构造。
type Module struct {
	Auth          *handler.AuthHandler
	User          *handler.UserHandler
	TenantUser    *handler.TenantUserHandler
	Role          *handler.RoleHandler
	Permission    *handler.PermissionHandler
	Tenant        *handler.TenantHandler
	Impersonation *handler.ImpersonationHandler
}

// New 创建 access 域模块。
func New(cfg *config.Config) *Module {
	authHandler := handler.NewAuthHandler(cfg)
	authSvc := authHandler.GetAuthService()
	userRepo := repository.NewUserRepository()
	roleRepo := repository.NewRoleRepository()
	tenantRepo := repository.NewTenantRepository()
	permissionRepo := repository.NewPermissionRepository()

	return &Module{
		Auth: authHandler,
		User: handler.NewUserHandlerWithDeps(handler.UserHandlerDeps{
			UserRepo:    userRepo,
			RoleRepo:    roleRepo,
			AuthService: authSvc,
		}),
		TenantUser: handler.NewTenantUserHandlerWithDeps(handler.TenantUserHandlerDeps{
			AuthService: authSvc,
			TenantRepo:  tenantRepo,
			UserRepo:    userRepo,
			RoleRepo:    roleRepo,
		}),
		Role: handler.NewRoleHandlerWithDeps(handler.RoleHandlerDeps{
			RoleRepo:       roleRepo,
			PermissionRepo: permissionRepo,
		}),
		Permission: handler.NewPermissionHandlerWithDeps(handler.PermissionHandlerDeps{
			PermissionRepo: permissionRepo,
		}),
		Tenant: handler.NewTenantHandlerWithDeps(handler.TenantHandlerDeps{
			TenantRepo:  tenantRepo,
			RoleRepo:    roleRepo,
			UserRepo:    userRepo,
			AuthService: authSvc,
		}),
		Impersonation: handler.NewImpersonationHandlerWithDeps(handler.ImpersonationHandlerDeps{
			ImpersonationRepo: repository.NewImpersonationRepository(),
			TenantRepo:        tenantRepo,
			AuditRepo:         repository.NewAuditLogRepository(database.DB),
			PlatformAuditRepo: repository.NewPlatformAuditLogRepository(),
			SiteMessageRepo:   repository.NewSiteMessageRepository(),
		}),
	}
}
