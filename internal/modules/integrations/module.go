package integrations

import (
	"github.com/company/auto-healing/internal/database"
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
	"gorm.io/gorm"
)

// Module 聚合 integrations 域处理器构造。
type Module struct {
	Plugin   *integrationhttp.PluginHandler
	CMDB     *integrationhttp.CMDBHandler
	GitRepo  *integrationhttp.GitRepoHandler
	Playbook *integrationhttp.PlaybookHandler
}

type ModuleDeps struct {
	PluginService   *plugin.Service
	IncidentService *plugin.IncidentService
	CMDBService     *plugin.CMDBService
	GitService      *gitSvc.Service
	PlaybookService *playbook.Service
	SecretService   *secretsSvc.Service
}

func DefaultModuleDeps() ModuleDeps {
	return DefaultModuleDepsWithDB(database.DB)
}

func DefaultModuleDepsWithDB(db *gorm.DB) ModuleDeps {
	gitRepoRepo := integrationrepo.NewGitRepositoryRepositoryWithDB(db)
	playbookRepo := integrationrepo.NewPlaybookRepositoryWithDB(db)
	executionRepo := automationrepo.NewExecutionRepositoryWithDB(db)
	pluginRepo := integrationrepo.NewPluginRepositoryWithDB(db)
	pluginSyncLogRepo := integrationrepo.NewPluginSyncLogRepositoryWithDB(db)
	cmdbRepo := cmdbrepo.NewCMDBItemRepositoryWithDB(db)
	incidentRepo := incidentrepo.NewIncidentRepositoryWithDB(db)
	httpClient := plugin.NewHTTPClient()
	pluginService := plugin.NewServiceWithDeps(plugin.ServiceDeps{
		PluginRepo:   pluginRepo,
		SyncLogRepo:  pluginSyncLogRepo,
		CMDBRepo:     cmdbRepo,
		IncidentRepo: incidentRepo,
		HTTPClient:   httpClient,
	})
	return ModuleDeps{
		PluginService: pluginService,
		IncidentService: plugin.NewIncidentServiceWithDeps(plugin.IncidentServiceDeps{
			IncidentRepo: incidentRepo,
			PluginRepo:   pluginRepo,
			HTTPClient:   httpClient,
		}),
		CMDBService: plugin.NewCMDBServiceWithDeps(plugin.CMDBServiceDeps{
			CMDBRepo: cmdbRepo,
		}),
		GitService: gitSvc.NewServiceWithDeps(gitSvc.ServiceDeps{
			Repo:         gitRepoRepo,
			PlaybookRepo: playbookRepo,
			PlaybookSvc: func() *playbook.Service {
				return playbook.NewServiceWithDB(db)
			},
		}),
		PlaybookService: playbook.NewServiceWithDeps(playbook.ServiceDeps{
			Repo:          playbookRepo,
			GitRepo:       gitRepoRepo,
			ExecutionRepo: executionRepo,
		}),
		SecretService: secretsSvc.NewServiceWithDeps(secretsSvc.ServiceDeps{
			Repo: secretsrepo.NewSecretsSourceRepositoryWithDB(db),
		}),
	}
}

// New 创建 integrations 域模块。
func New() *Module {
	return NewWithDB(database.DB)
}

func NewWithDB(db *gorm.DB) *Module {
	return NewWithDeps(DefaultModuleDepsWithDB(db))
}

func NewWithDeps(deps ModuleDeps) *Module {
	module := &Module{
		Plugin: integrationhttp.NewPluginHandlerWithDeps(integrationhttp.PluginHandlerDeps{
			PluginService:   deps.PluginService,
			IncidentService: deps.IncidentService,
		}),
		CMDB: integrationhttp.NewCMDBHandlerWithDeps(integrationhttp.CMDBHandlerDeps{
			Service:       deps.CMDBService,
			SecretService: deps.SecretService,
		}),
		GitRepo: integrationhttp.NewGitRepoHandlerWithDeps(integrationhttp.GitRepoHandlerDeps{
			Service: deps.GitService,
		}),
		Playbook: integrationhttp.NewPlaybookHandlerWithDeps(integrationhttp.PlaybookHandlerDeps{
			Service: deps.PlaybookService,
		}),
	}
	platformlifecycle.RegisterCleanup(module.Plugin.Shutdown)
	platformlifecycle.RegisterCleanup(module.GitRepo.Shutdown)
	return module
}
