package httpapi

import "github.com/company/auto-healing/internal/handler"

type Dependencies struct {
	CMDB     *handler.CMDBHandler
	GitRepo  *handler.GitRepoHandler
	Playbook *handler.PlaybookHandler
	Plugin   *handler.PluginHandler
}

type Registrar struct {
	deps Dependencies
}

func New(deps Dependencies) Registrar {
	return Registrar{deps: deps}
}
