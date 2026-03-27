package httpapi

type Dependencies struct {
	Secrets *SecretsHandler
}

type Registrar struct {
	deps Dependencies
}

func New(deps Dependencies) Registrar {
	return Registrar{deps: deps}
}
