package integrations

import "github.com/company/auto-healing/internal/handler"

// Module 聚合 integrations 域处理器构造。
type Module struct {
	Plugin   *handler.PluginHandler
	CMDB     *handler.CMDBHandler
	GitRepo  *handler.GitRepoHandler
	Playbook *handler.PlaybookHandler
}

// New 创建 integrations 域模块。
func New() *Module {
	return &Module{
		Plugin:   handler.NewPluginHandler(),
		CMDB:     handler.NewCMDBHandler(),
		GitRepo:  handler.NewGitRepoHandler(),
		Playbook: handler.NewPlaybookHandler(),
	}
}
