package integrations

import (
	integrationhttp "github.com/company/auto-healing/internal/modules/integrations/httpapi"
	gitSvc "github.com/company/auto-healing/internal/modules/integrations/service/git"
	"github.com/company/auto-healing/internal/modules/integrations/service/playbook"
	"github.com/company/auto-healing/internal/modules/integrations/service/plugin"
	platformlifecycle "github.com/company/auto-healing/internal/platform/lifecycle"
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
	pluginService := plugin.NewService()
	gitService := gitSvc.NewService()
	module := &Module{
		Plugin: integrationhttp.NewPluginHandlerWithDeps(integrationhttp.PluginHandlerDeps{
			PluginService:   pluginService,
			IncidentService: plugin.NewIncidentService(),
		}),
		CMDB: integrationhttp.NewCMDBHandlerWithDeps(integrationhttp.CMDBHandlerDeps{
			Service: plugin.NewCMDBService(),
		}),
		GitRepo: integrationhttp.NewGitRepoHandlerWithDeps(integrationhttp.GitRepoHandlerDeps{
			Service: gitService,
		}),
		Playbook: integrationhttp.NewPlaybookHandlerWithDeps(integrationhttp.PlaybookHandlerDeps{
			Service: playbook.NewService(),
		}),
	}
	platformlifecycle.RegisterCleanup(module.Plugin.Shutdown)
	platformlifecycle.RegisterCleanup(module.GitRepo.Shutdown)
	return module
}
