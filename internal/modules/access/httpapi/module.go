package httpapi

import "github.com/company/auto-healing/internal/handler"

type Dependencies struct {
	Auth          *handler.AuthHandler
	Impersonation *handler.ImpersonationHandler
	Permission    *handler.PermissionHandler
	Role          *handler.RoleHandler
	Tenant        *handler.TenantHandler
	TenantUser    *handler.TenantUserHandler
	User          *handler.UserHandler
}

type Registrar struct {
	deps Dependencies
}

func New(deps Dependencies) Registrar {
	return Registrar{deps: deps}
}
