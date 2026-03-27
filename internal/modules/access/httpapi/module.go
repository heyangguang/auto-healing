package httpapi

type Dependencies struct {
	Auth          *AuthHandler
	Impersonation *ImpersonationHandler
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
