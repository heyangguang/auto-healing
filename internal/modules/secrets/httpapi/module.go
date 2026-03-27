package httpapi

import "github.com/company/auto-healing/internal/handler"

type Dependencies struct {
	Secrets *handler.SecretsHandler
}

type Registrar struct {
	deps Dependencies
}

func New(deps Dependencies) Registrar {
	return Registrar{deps: deps}
}
