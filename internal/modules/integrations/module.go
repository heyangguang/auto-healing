package integrations

import (
	"github.com/company/auto-healing/internal/handler"
	gitSvc "github.com/company/auto-healing/internal/modules/integrations/service/git"
	"github.com/company/auto-healing/internal/modules/integrations/service/playbook"
	"github.com/company/auto-healing/internal/modules/integrations/service/plugin"
	platformlifecycle "github.com/company/auto-healing/internal/platform/lifecycle"
)

// Module 聚合 integrations 域处理器构造。
type Module struct {
	Plugin   *handler.PluginHandler
	CMDB     *handler.CMDBHandler
	GitRepo  *handler.GitRepoHandler
	Playbook *handler.PlaybookHandler
}

// New 创建 integrations 域模块。
func New() *Module {
	pluginService := plugin.NewService()
	gitService := gitSvc.NewService()
	module := &Module{
		Plugin: handler.NewPluginHandlerWithDeps(handler.PluginHandlerDeps{
			PluginService:   pluginService,
			IncidentService: plugin.NewIncidentService(),
		}),
		CMDB: handler.NewCMDBHandlerWithDeps(handler.CMDBHandlerDeps{
			Service: plugin.NewCMDBService(),
		}),
		GitRepo: handler.NewGitRepoHandlerWithDeps(handler.GitRepoHandlerDeps{
			Service: gitService,
		}),
		Playbook: handler.NewPlaybookHandlerWithDeps(handler.PlaybookHandlerDeps{
			Service: playbook.NewService(),
		}),
	}
	platformlifecycle.RegisterCleanup(module.Plugin.Shutdown)
	platformlifecycle.RegisterCleanup(module.GitRepo.Shutdown)
	return module
}
