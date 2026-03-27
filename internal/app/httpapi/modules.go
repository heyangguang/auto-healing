package httpapi

import (
	"github.com/company/auto-healing/internal/handler"
	accesshttp "github.com/company/auto-healing/internal/modules/access/httpapi"
	automationhttp "github.com/company/auto-healing/internal/modules/automation/httpapi"
	engagementhttp "github.com/company/auto-healing/internal/modules/engagement/httpapi"
	integrationshttp "github.com/company/auto-healing/internal/modules/integrations/httpapi"
	opshttp "github.com/company/auto-healing/internal/modules/ops/httpapi"
	secretshttp "github.com/company/auto-healing/internal/modules/secrets/httpapi"
)

type moduleRegistrars struct {
	access       accesshttp.Registrar
	automation   automationhttp.Registrar
	engagement   engagementhttp.Registrar
	integrations integrationshttp.Registrar
	ops          opshttp.Registrar
	secrets      secretshttp.Registrar
}

func newModules(handlers *handler.Handlers) moduleRegistrars {
	return moduleRegistrars{
		access: accesshttp.New(accesshttp.Dependencies{
			Auth:          handlers.Auth,
			Impersonation: handlers.Impersonation,
			Permission:    handlers.Permission,
			Role:          handlers.Role,
			Tenant:        handlers.Tenant,
			TenantUser:    handlers.TenantUser,
			User:          handlers.User,
		}),
		automation: automationhttp.New(automationhttp.Dependencies{
			Execution: handlers.Execution,
			Healing:   handlers.Healing,
			Schedule:  handlers.Schedule,
		}),
		engagement: engagementhttp.New(engagementhttp.Dependencies{
			Activity:     handlers.Activity,
			Dashboard:    handlers.Dashboard,
			Notification: handlers.Notification,
			Preference:   handlers.Preference,
			Search:       handlers.Search,
			SiteMessage:  handlers.SiteMessage,
			Workbench:    handlers.Workbench,
		}),
		integrations: integrationshttp.New(integrationshttp.Dependencies{
			CMDB:     handlers.CMDB,
			GitRepo:  handlers.GitRepo,
			Playbook: handlers.Playbook,
			Plugin:   handlers.Plugin,
		}),
		ops: opshttp.New(opshttp.Dependencies{
			Audit:              handlers.Audit,
			BlacklistExemption: handlers.BlacklistExemption,
			CommandBlacklist:   handlers.CommandBlacklist,
			Dictionary:         handlers.Dictionary,
			PlatformAudit:      handlers.PlatformAudit,
			PlatformSettings:   handlers.PlatformSettings,
		}),
		secrets: secretshttp.New(secretshttp.Dependencies{
			Secrets: handlers.Secrets,
		}),
	}
}
