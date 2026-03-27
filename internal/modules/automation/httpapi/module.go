package httpapi

import "github.com/company/auto-healing/internal/handler"

type Dependencies struct {
	Execution *handler.ExecutionHandler
	Healing   *handler.HealingHandler
	Schedule  *handler.ScheduleHandler
}

type Registrar struct {
	deps Dependencies
}

func New(deps Dependencies) Registrar {
	return Registrar{deps: deps}
}
