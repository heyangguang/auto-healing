package handler

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
