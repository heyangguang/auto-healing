package httpapi

import "github.com/company/auto-healing/internal/middleware"

type Dependencies struct {
	Auth          *AuthHandler
	Impersonation *ImpersonationHandler
	Middleware    middleware.RuntimeDeps
	Permission    *PermissionHandler
	Role          *RoleHandler
	Tenant        *TenantHandler
	TenantUser    *TenantUserHandler
	User          *UserHandler
}

type Registrar struct {
	deps Dependencies
}

func New(deps Dependencies) Registrar {
	return Registrar{deps: deps}
}

func (r Registrar) Dependencies() Dependencies {
	return r.deps
}
