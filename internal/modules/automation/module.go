package automation

import "github.com/company/auto-healing/internal/handler"

// Module 聚合 automation 域处理器构造。
type Module struct {
	Execution *handler.ExecutionHandler
	Healing   *handler.HealingHandler
	Schedule  *handler.ScheduleHandler
}

// New 创建 automation 域模块。
func New() *Module {
	return &Module{
		Execution: handler.NewExecutionHandler(),
		Healing:   handler.NewHealingHandler(),
		Schedule:  handler.NewScheduleHandler(),
	}
}
