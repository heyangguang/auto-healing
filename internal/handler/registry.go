package handler

import "github.com/company/auto-healing/internal/config"

// HandlerOverrides 用于按域注入已构造的处理器，避免大一统构造器重复 new。
type HandlerOverrides struct {
	Auth          *AuthHandler
	User          *UserHandler
	TenantUser    *TenantUserHandler
	Role          *RoleHandler
	Permission    *PermissionHandler
	Tenant        *TenantHandler
	Impersonation *ImpersonationHandler
}

// Handlers 所有处理器集合。
type Handlers struct {
	Auth               *AuthHandler
	User               *UserHandler
	TenantUser         *TenantUserHandler
	Role               *RoleHandler
	Permission         *PermissionHandler
	Plugin             *PluginHandler
	CMDB               *CMDBHandler
	Secrets            *SecretsHandler
	GitRepo            *GitRepoHandler
	Playbook           *PlaybookHandler
	Execution          *ExecutionHandler
	Schedule           *ScheduleHandler
	Notification       *NotificationHandler
	Healing            *HealingHandler
	Dashboard          *DashboardHandler
	Preference         *PreferenceHandler
	Audit              *AuditHandler
	PlatformAudit      *PlatformAuditHandler
	Activity           *UserActivityHandler
	Search             *SearchHandler
	SiteMessage        *SiteMessageHandler
	PlatformSettings   *PlatformSettingsHandler
	Tenant             *TenantHandler
	Workbench          *WorkbenchHandler
	Dictionary         *DictionaryHandler
	Impersonation      *ImpersonationHandler
	CommandBlacklist   *CommandBlacklistHandler
	BlacklistExemption *BlacklistExemptionHandler
}

// NewHandlers 创建所有处理器。
func NewHandlers(cfg *config.Config) *Handlers {
	return NewHandlersWithOverrides(cfg, HandlerOverrides{})
}

// NewHandlersWithOverrides 创建处理器集合，并允许已模块化的域注入自己的构造结果。
func NewHandlersWithOverrides(cfg *config.Config, overrides HandlerOverrides) *Handlers {
	authHandler := overrides.Auth
	if authHandler == nil {
		authHandler = NewAuthHandler(cfg)
	}
	userHandler := overrides.User
	if userHandler == nil {
		userHandler = NewUserHandler(authHandler.authSvc)
	}
	tenantUserHandler := overrides.TenantUser
	if tenantUserHandler == nil {
		tenantUserHandler = NewTenantUserHandler(authHandler.authSvc)
	}
	roleHandler := overrides.Role
	if roleHandler == nil {
		roleHandler = NewRoleHandler()
	}
	permissionHandler := overrides.Permission
	if permissionHandler == nil {
		permissionHandler = NewPermissionHandler()
	}
	tenantHandler := overrides.Tenant
	if tenantHandler == nil {
		tenantHandler = NewTenantHandler(authHandler.authSvc)
	}
	impersonationHandler := overrides.Impersonation
	if impersonationHandler == nil {
		impersonationHandler = NewImpersonationHandler()
	}
	handlers := &Handlers{
		Auth:               authHandler,
		User:               userHandler,
		TenantUser:         tenantUserHandler,
		Role:               roleHandler,
		Permission:         permissionHandler,
		Plugin:             NewPluginHandler(),
		CMDB:               NewCMDBHandler(),
		Secrets:            NewSecretsHandler(),
		GitRepo:            NewGitRepoHandler(),
		Playbook:           NewPlaybookHandler(),
		Execution:          NewExecutionHandler(),
		Schedule:           NewScheduleHandler(),
		Notification:       NewNotificationHandler(),
		Healing:            NewHealingHandler(),
		Dashboard:          NewDashboardHandler(),
		Preference:         NewPreferenceHandler(),
		Audit:              NewAuditHandler(),
		PlatformAudit:      NewPlatformAuditHandler(),
		Activity:           NewUserActivityHandler(),
		Search:             NewSearchHandler(),
		SiteMessage:        NewSiteMessageHandler(),
		PlatformSettings:   NewPlatformSettingsHandler(),
		Tenant:             tenantHandler,
		Workbench:          NewWorkbenchHandler(),
		Dictionary:         NewDictionaryHandler(),
		Impersonation:      impersonationHandler,
		CommandBlacklist:   NewCommandBlacklistHandler(),
		BlacklistExemption: NewBlacklistExemptionHandler(),
	}
	registerHandlerCleanup(handlers.Execution.Shutdown)
	registerHandlerCleanup(handlers.Healing.Shutdown)
	registerHandlerCleanup(handlers.Plugin.Shutdown)
	registerHandlerCleanup(handlers.GitRepo.Shutdown)
	return handlers
}
