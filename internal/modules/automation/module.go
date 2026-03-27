package automation

import (
	"github.com/company/auto-healing/internal/database"
	"github.com/company/auto-healing/internal/handler"
	"github.com/company/auto-healing/internal/repository"
	executionSvc "github.com/company/auto-healing/internal/service/execution"
	healingSvc "github.com/company/auto-healing/internal/service/healing"
	scheduleSvc "github.com/company/auto-healing/internal/service/schedule"
)

// Module 聚合 automation 域处理器构造。
type Module struct {
	Execution *handler.ExecutionHandler
	Healing   *handler.HealingHandler
	Schedule  *handler.ScheduleHandler
}

// New 创建 automation 域模块。
func New() *Module {
	executionService := executionSvc.NewService()
	scheduleService := scheduleSvc.NewService()
	scheduler := healingSvc.DefaultScheduler()
	module := &Module{
		Execution: handler.NewExecutionHandlerWithDeps(handler.ExecutionHandlerDeps{
			Service: executionService,
		}),
		Healing: handler.NewHealingHandlerWithDeps(handler.HealingHandlerDeps{
			FlowRepo:         repository.NewHealingFlowRepository(),
			RuleRepo:         repository.NewHealingRuleRepository(),
			InstanceRepo:     repository.NewFlowInstanceRepository(),
			ApprovalRepo:     repository.NewApprovalTaskRepository(),
			IncidentRepo:     repository.NewIncidentRepository(),
			NotificationRepo: repository.NewNotificationRepository(database.DB),
			Executor:         scheduler.Executor(),
			Scheduler:        scheduler,
		}),
		Schedule: handler.NewScheduleHandlerWithDeps(handler.ScheduleHandlerDeps{
			Service: scheduleService,
		}),
	}
	handler.RegisterCleanup(module.Execution.Shutdown)
	handler.RegisterCleanup(module.Healing.Shutdown)
	return module
}
