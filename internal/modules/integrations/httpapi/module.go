package httpapi

type Dependencies struct {
	CMDB     *CMDBHandler
	GitRepo  *GitRepoHandler
	Playbook *PlaybookHandler
	Plugin   *PluginHandler
}

type Registrar struct {
	deps Dependencies
}

func New(deps Dependencies) Registrar {
	return Registrar{deps: deps}
}
