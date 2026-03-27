package httpapi

type Dependencies struct {
	Execution *ExecutionHandler
	Healing   *HealingHandler
	Schedule  *ScheduleHandler
}

type Registrar struct {
	deps Dependencies
}

func New(deps Dependencies) Registrar {
	return Registrar{deps: deps}
}
