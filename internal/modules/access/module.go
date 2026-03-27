package access

import (
	"github.com/company/auto-healing/internal/config"
	"github.com/company/auto-healing/internal/handler"
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

	return &Module{
		Auth:          authHandler,
		User:          handler.NewUserHandler(authSvc),
		TenantUser:    handler.NewTenantUserHandler(authSvc),
		Role:          handler.NewRoleHandler(),
		Permission:    handler.NewPermissionHandler(),
		Tenant:        handler.NewTenantHandler(authSvc),
		Impersonation: handler.NewImpersonationHandler(),
	}
}

// Overrides 将 access 域处理器注入全局 handler 构造。
func (m *Module) Overrides() handler.HandlerOverrides {
	return handler.HandlerOverrides{
		Auth:          m.Auth,
		User:          m.User,
		TenantUser:    m.TenantUser,
		Role:          m.Role,
		Permission:    m.Permission,
		Tenant:        m.Tenant,
		Impersonation: m.Impersonation,
	}
}
