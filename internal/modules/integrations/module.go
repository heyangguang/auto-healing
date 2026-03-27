package integrations

import (
	automationrepo "github.com/company/auto-healing/internal/modules/automation/repository"
	integrationhttp "github.com/company/auto-healing/internal/modules/integrations/httpapi"
	integrationrepo "github.com/company/auto-healing/internal/modules/integrations/repository"
	gitSvc "github.com/company/auto-healing/internal/modules/integrations/service/git"
	"github.com/company/auto-healing/internal/modules/integrations/service/playbook"
	"github.com/company/auto-healing/internal/modules/integrations/service/plugin"
	secretsrepo "github.com/company/auto-healing/internal/modules/secrets/repository"
	secretsSvc "github.com/company/auto-healing/internal/modules/secrets/service/secrets"
	platformlifecycle "github.com/company/auto-healing/internal/platform/lifecycle"
	cmdbrepo "github.com/company/auto-healing/internal/platform/repository/cmdb"
	incidentrepo "github.com/company/auto-healing/internal/platform/repository/incident"
)

// Module 聚合 integrations 域处理器构造。
type Module struct {
	Plugin   *integrationhttp.PluginHandler
	CMDB     *integrationhttp.CMDBHandler
	GitRepo  *integrationhttp.GitRepoHandler
	Playbook *integrationhttp.PlaybookHandler
}

// New 创建 integrations 域模块。
func New() *Module {
	gitRepoRepo := integrationrepo.NewGitRepositoryRepository()
	playbookRepo := integrationrepo.NewPlaybookRepository()
	executionRepo := automationrepo.NewExecutionRepository()
	pluginRepo := integrationrepo.NewPluginRepository()
	pluginSyncLogRepo := integrationrepo.NewPluginSyncLogRepository()
	cmdbRepo := cmdbrepo.NewCMDBItemRepository()
	incidentRepo := incidentrepo.NewIncidentRepository()
	httpClient := plugin.NewHTTPClient()
	pluginService := plugin.NewServiceWithDeps(plugin.ServiceDeps{
		PluginRepo:   pluginRepo,
		SyncLogRepo:  pluginSyncLogRepo,
		CMDBRepo:     cmdbRepo,
		IncidentRepo: incidentRepo,
		HTTPClient:   httpClient,
	})
	gitService := gitSvc.NewServiceWithDeps(gitSvc.ServiceDeps{
		Repo:         gitRepoRepo,
		PlaybookRepo: playbookRepo,
		PlaybookSvc: func() *playbook.Service {
			return playbook.NewServiceWithDeps(playbook.ServiceDeps{
				Repo:          playbookRepo,
				GitRepo:       gitRepoRepo,
				ExecutionRepo: executionRepo,
			})
		},
	})
	playbookService := playbook.NewServiceWithDeps(playbook.ServiceDeps{
		Repo:          playbookRepo,
		GitRepo:       gitRepoRepo,
		ExecutionRepo: executionRepo,
	})
	incidentService := plugin.NewIncidentServiceWithDeps(plugin.IncidentServiceDeps{
		IncidentRepo: incidentRepo,
		PluginRepo:   pluginRepo,
		HTTPClient:   httpClient,
	})
	cmdbService := plugin.NewCMDBServiceWithDeps(plugin.CMDBServiceDeps{
		CMDBRepo: cmdbRepo,
	})
	secretService := secretsSvc.NewServiceWithDeps(secretsSvc.ServiceDeps{
		Repo: secretsrepo.NewSecretsSourceRepository(),
	})
	module := &Module{
		Plugin: integrationhttp.NewPluginHandlerWithDeps(integrationhttp.PluginHandlerDeps{
			PluginService:   pluginService,
			IncidentService: incidentService,
		}),
		CMDB: integrationhttp.NewCMDBHandlerWithDeps(integrationhttp.CMDBHandlerDeps{
			Service:       cmdbService,
			SecretService: secretService,
		}),
		GitRepo: integrationhttp.NewGitRepoHandlerWithDeps(integrationhttp.GitRepoHandlerDeps{
			Service: gitService,
		}),
		Playbook: integrationhttp.NewPlaybookHandlerWithDeps(integrationhttp.PlaybookHandlerDeps{
			Service: playbookService,
		}),
	}
	platformlifecycle.RegisterCleanup(module.Plugin.Shutdown)
	platformlifecycle.RegisterCleanup(module.GitRepo.Shutdown)
	return module
}
