package access

import (
	"github.com/company/auto-healing/internal/config"
	accesshttp "github.com/company/auto-healing/internal/modules/access/httpapi"
	accessrepo "github.com/company/auto-healing/internal/modules/access/repository"
	authservice "github.com/company/auto-healing/internal/modules/access/service/auth"
	engagementrepo "github.com/company/auto-healing/internal/modules/engagement/repository"
	engagementservice "github.com/company/auto-healing/internal/modules/engagement/service"
	"github.com/company/auto-healing/internal/pkg/jwt"
	auditrepo "github.com/company/auto-healing/internal/platform/repository/audit"
	settingsrepo "github.com/company/auto-healing/internal/platform/repository/settings"
	"gorm.io/gorm"
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

type ModuleDeps struct {
	AuthService       *authservice.Service
	JWTService        *jwt.Service
	UserRepo          *accessrepo.UserRepository
	RoleRepo          *accessrepo.RoleRepository
	TenantRepo        *accessrepo.TenantRepository
	PermissionRepo    *accessrepo.PermissionRepository
	InvitationRepo    *accessrepo.InvitationRepository
	ImpersonationRepo *accessrepo.ImpersonationRepository
	SiteMessageRepo   *engagementrepo.SiteMessageRepository
	SettingsRepo      *settingsrepo.PlatformSettingsRepository
	EmailService      *engagementservice.PlatformEmailService
	AuditRepo         *auditrepo.AuditLogRepository
	PlatformAuditRepo *auditrepo.PlatformAuditLogRepository
}

func NewWithDB(cfg *config.Config, db *gorm.DB) *Module {
	return NewWithDeps(DefaultModuleDepsWithDB(cfg, db))
}

func DefaultModuleDepsWithDB(cfg *config.Config, db *gorm.DB) ModuleDeps {
	settingsRepo := settingsrepo.NewPlatformSettingsRepositoryWithDB(db)
	userRepo := accessrepo.NewUserRepositoryWithDB(db)
	roleRepo := accessrepo.NewRoleRepositoryWithDB(db)
	tenantRepo := accessrepo.NewTenantRepositoryWithDB(db)
	permissionRepo := accessrepo.NewPermissionRepositoryWithDB(db)
	invitationRepo := accessrepo.NewInvitationRepositoryWithDB(db)
	impersonationRepo := accessrepo.NewImpersonationRepositoryWithDB(db)
	siteMessageRepo := engagementrepo.NewSiteMessageRepositoryWithDB(db)
	jwtSvc := jwt.NewService(jwt.Config{
		Secret:          cfg.JWT.Secret,
		AccessTokenTTL:  cfg.JWT.AccessTokenTTL(),
		RefreshTokenTTL: cfg.JWT.RefreshTokenTTL(),
		Issuer:          cfg.JWT.Issuer,
	}, accesshttp.NewAuthTokenBlacklistStore())
	return ModuleDeps{
		AuthService: authservice.NewServiceWithDeps(authservice.ServiceDeps{
			UserRepo:       userRepo,
			RoleRepo:       roleRepo,
			PermissionRepo: permissionRepo,
			TenantRepo:     tenantRepo,
			JWTService:     jwtSvc,
			DB:             db,
		}),
		JWTService:        jwtSvc,
		UserRepo:          userRepo,
		RoleRepo:          roleRepo,
		PermissionRepo:    permissionRepo,
		TenantRepo:        tenantRepo,
		InvitationRepo:    invitationRepo,
		ImpersonationRepo: impersonationRepo,
		SiteMessageRepo:   siteMessageRepo,
		SettingsRepo:      settingsRepo,
		EmailService: engagementservice.NewPlatformEmailServiceWithDeps(engagementservice.PlatformEmailServiceDeps{
			SettingsRepo: settingsRepo,
		}),
		AuditRepo:         auditrepo.NewAuditLogRepository(db),
		PlatformAuditRepo: auditrepo.NewPlatformAuditLogRepositoryWithDB(db),
	}
}

func NewWithDeps(deps ModuleDeps) *Module {
	authHandler := accesshttp.NewAuthHandlerWithDeps(accesshttp.AuthHandlerDeps{
		AuthService:       deps.AuthService,
		JWTService:        deps.JWTService,
		AuditRepo:         deps.AuditRepo,
		PlatformAuditRepo: deps.PlatformAuditRepo,
		UserRepo:          deps.UserRepo,
		TenantRepo:        deps.TenantRepo,
		PermissionRepo:    deps.PermissionRepo,
	})
	return &Module{
		Auth: authHandler,
		User: accesshttp.NewUserHandlerWithDeps(accesshttp.UserHandlerDeps{
			UserRepo:    deps.UserRepo,
			RoleRepo:    deps.RoleRepo,
			AuthService: deps.AuthService,
		}),
		TenantUser: accesshttp.NewTenantUserHandlerWithDeps(accesshttp.TenantUserHandlerDeps{
			AuthService: deps.AuthService,
			TenantRepo:  deps.TenantRepo,
			UserRepo:    deps.UserRepo,
			RoleRepo:    deps.RoleRepo,
		}),
		Role: accesshttp.NewRoleHandlerWithDeps(accesshttp.RoleHandlerDeps{
			RoleRepo:       deps.RoleRepo,
			PermissionRepo: deps.PermissionRepo,
		}),
		Permission: accesshttp.NewPermissionHandlerWithDeps(accesshttp.PermissionHandlerDeps{
			PermissionRepo: deps.PermissionRepo,
		}),
		Tenant: accesshttp.NewTenantHandlerWithDeps(accesshttp.TenantHandlerDeps{
			TenantRepo:     deps.TenantRepo,
			RoleRepo:       deps.RoleRepo,
			UserRepo:       deps.UserRepo,
			AuthService:    deps.AuthService,
			InvitationRepo: deps.InvitationRepo,
			SettingsRepo:   deps.SettingsRepo,
			EmailService:   deps.EmailService,
		}),
		Impersonation: accesshttp.NewImpersonationHandlerWithDeps(accesshttp.ImpersonationHandlerDeps{
			ImpersonationRepo: deps.ImpersonationRepo,
			TenantRepo:        deps.TenantRepo,
			AuditRepo:         deps.AuditRepo,
			PlatformAuditRepo: deps.PlatformAuditRepo,
			SiteMessageRepo:   deps.SiteMessageRepo,
		}),
	}
}
