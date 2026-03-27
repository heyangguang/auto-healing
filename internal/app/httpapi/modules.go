package httpapi

import (
	"github.com/company/auto-healing/internal/config"
	"github.com/company/auto-healing/internal/middleware"
	accessmodule "github.com/company/auto-healing/internal/modules/access"
	accesshttp "github.com/company/auto-healing/internal/modules/access/httpapi"
	automationmodule "github.com/company/auto-healing/internal/modules/automation"
	automationhttp "github.com/company/auto-healing/internal/modules/automation/httpapi"
	engagementmodule "github.com/company/auto-healing/internal/modules/engagement"
	engagementhttp "github.com/company/auto-healing/internal/modules/engagement/httpapi"
	integrationsmodule "github.com/company/auto-healing/internal/modules/integrations"
	integrationshttp "github.com/company/auto-healing/internal/modules/integrations/httpapi"
	opsmodule "github.com/company/auto-healing/internal/modules/ops"
	opshttp "github.com/company/auto-healing/internal/modules/ops/httpapi"
	secretsmodule "github.com/company/auto-healing/internal/modules/secrets"
	secretshttp "github.com/company/auto-healing/internal/modules/secrets/httpapi"
)

type moduleSet struct {
	access       *accessmodule.Module
	automation   *automationmodule.Module
	engagement   *engagementmodule.Module
	integrations *integrationsmodule.Module
	ops          *opsmodule.Module
	secrets      *secretsmodule.Module
	routes       moduleRegistrars
}

type moduleRegistrars struct {
	access       accesshttp.Registrar
	automation   automationhttp.Registrar
	engagement   engagementhttp.Registrar
	integrations integrationshttp.Registrar
	ops          opshttp.Registrar
	secrets      secretshttp.Registrar
}

func newModules(cfg *config.Config) moduleSet {
	middlewareDeps := middleware.NewRuntimeDeps()
	access := accessmodule.New(cfg)
	automation := automationmodule.New()
	engagement := engagementmodule.New()
	integrations := integrationsmodule.New()
	ops := opsmodule.New()
	secrets := secretsmodule.New()

	return moduleSet{
		access:       access,
		automation:   automation,
		engagement:   engagement,
		integrations: integrations,
		ops:          ops,
		secrets:      secrets,
			routes: moduleRegistrars{
				access: accesshttp.New(accesshttp.Dependencies{
					Auth:          access.Auth,
					Impersonation: access.Impersonation,
					Middleware:    middlewareDeps,
					Permission:    access.Permission,
					Role:          access.Role,
					Tenant:        access.Tenant,
				TenantUser:    access.TenantUser,
				User:          access.User,
			}),
			automation: automationhttp.New(automationhttp.Dependencies{
				Execution: automation.Execution,
				Healing:   automation.Healing,
				Schedule:  automation.Schedule,
			}),
			engagement: engagementhttp.New(engagementhttp.Dependencies{
				Activity:     engagement.Activity,
				Dashboard:    engagement.Dashboard,
				Notification: engagement.Notification,
				Preference:   engagement.Preference,
				Search:       engagement.Search,
				SiteMessage:  engagement.SiteMessage,
				Workbench:    engagement.Workbench,
			}),
			integrations: integrationshttp.New(integrationshttp.Dependencies{
				CMDB:     integrations.CMDB,
				GitRepo:  integrations.GitRepo,
				Playbook: integrations.Playbook,
				Plugin:   integrations.Plugin,
			}),
			ops: opshttp.New(opshttp.Dependencies{
				Audit:              ops.Audit,
				BlacklistExemption: ops.BlacklistExemption,
				CommandBlacklist:   ops.CommandBlacklist,
				Dictionary:         ops.Dictionary,
				PlatformAudit:      ops.PlatformAudit,
				PlatformSettings:   ops.PlatformSettings,
			}),
			secrets: secretshttp.New(secretshttp.Dependencies{
				Secrets: secrets.Secrets,
			}),
		},
	}
}
