package handler

import "github.com/company/auto-healing/internal/config"

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
	authHandler := NewAuthHandler(cfg)
	handlers := &Handlers{
		Auth:               authHandler,
		User:               NewUserHandler(authHandler.authSvc),
		TenantUser:         NewTenantUserHandler(authHandler.authSvc),
		Role:               NewRoleHandler(),
		Permission:         NewPermissionHandler(),
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
		Tenant:             NewTenantHandler(authHandler.authSvc),
		Workbench:          NewWorkbenchHandler(),
		Dictionary:         NewDictionaryHandler(),
		Impersonation:      NewImpersonationHandler(),
		CommandBlacklist:   NewCommandBlacklistHandler(),
		BlacklistExemption: NewBlacklistExemptionHandler(),
	}
	registerHandlerCleanup(handlers.Execution.Shutdown)
	registerHandlerCleanup(handlers.Healing.Shutdown)
	registerHandlerCleanup(handlers.Plugin.Shutdown)
	registerHandlerCleanup(handlers.GitRepo.Shutdown)
	return handlers
}
