package access

import (
	"github.com/company/auto-healing/internal/config"
	"github.com/company/auto-healing/internal/database"
	accesshttp "github.com/company/auto-healing/internal/modules/access/httpapi"
	accessrepo "github.com/company/auto-healing/internal/modules/access/repository"
	authservice "github.com/company/auto-healing/internal/modules/access/service/auth"
	engagementrepo "github.com/company/auto-healing/internal/modules/engagement/repository"
	engagementservice "github.com/company/auto-healing/internal/modules/engagement/service"
	"github.com/company/auto-healing/internal/pkg/jwt"
	auditrepo "github.com/company/auto-healing/internal/platform/repository/audit"
	settingsrepo "github.com/company/auto-healing/internal/platform/repository/settings"
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
	userRepo := accessrepo.NewUserRepository()
	roleRepo := accessrepo.NewRoleRepository()
	tenantRepo := accessrepo.NewTenantRepository()
	permissionRepo := accessrepo.NewPermissionRepository()
	invitationRepo := accessrepo.NewInvitationRepository()
	impersonationRepo := accessrepo.NewImpersonationRepository()
	siteMessageRepo := engagementrepo.NewSiteMessageRepository()
	settingsRepo := settingsrepo.NewPlatformSettingsRepository()
	jwtSvc := jwt.NewService(jwt.Config{
		Secret:          cfg.JWT.Secret,
		AccessTokenTTL:  cfg.JWT.AccessTokenTTL(),
		RefreshTokenTTL: cfg.JWT.RefreshTokenTTL(),
		Issuer:          cfg.JWT.Issuer,
	}, accesshttp.NewAuthTokenBlacklistStore())
	authSvc := authservice.NewServiceWithDeps(authservice.ServiceDeps{
		UserRepo:       userRepo,
		RoleRepo:       roleRepo,
		PermissionRepo: permissionRepo,
		TenantRepo:     tenantRepo,
		JWTService:     jwtSvc,
		DB:             database.DB,
	})
	authHandler := accesshttp.NewAuthHandlerWithDeps(accesshttp.AuthHandlerDeps{
		AuthService:       authSvc,
		JWTService:        jwtSvc,
		AuditRepo:         auditrepo.NewAuditLogRepository(database.DB),
		PlatformAuditRepo: auditrepo.NewPlatformAuditLogRepository(),
		UserRepo:          userRepo,
		TenantRepo:        tenantRepo,
		PermissionRepo:    permissionRepo,
	})

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
			TenantRepo:     tenantRepo,
			RoleRepo:       roleRepo,
			UserRepo:       userRepo,
			AuthService:    authSvc,
			InvitationRepo: invitationRepo,
			SettingsRepo:   settingsRepo,
			EmailService:   engagementservice.NewPlatformEmailService(),
		}),
		Impersonation: accesshttp.NewImpersonationHandlerWithDeps(accesshttp.ImpersonationHandlerDeps{
			ImpersonationRepo: impersonationRepo,
			TenantRepo:        tenantRepo,
			AuditRepo:         auditrepo.NewAuditLogRepository(database.DB),
			PlatformAuditRepo: auditrepo.NewPlatformAuditLogRepository(),
			SiteMessageRepo:   siteMessageRepo,
		}),
	}
}
