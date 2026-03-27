package automation

import (
	"github.com/company/auto-healing/internal/database"
	automationhttp "github.com/company/auto-healing/internal/modules/automation/httpapi"
	automationrepo "github.com/company/auto-healing/internal/modules/automation/repository"
	executionSvc "github.com/company/auto-healing/internal/modules/automation/service/execution"
	healingSvc "github.com/company/auto-healing/internal/modules/automation/service/healing"
	scheduleSvc "github.com/company/auto-healing/internal/modules/automation/service/schedule"
	engagementrepo "github.com/company/auto-healing/internal/modules/engagement/repository"
	platformlifecycle "github.com/company/auto-healing/internal/platform/lifecycle"
	incidentrepo "github.com/company/auto-healing/internal/platform/repository/incident"
)

// Module 聚合 automation 域处理器构造。
type Module struct {
	Execution *automationhttp.ExecutionHandler
	Healing   *automationhttp.HealingHandler
	Schedule  *automationhttp.ScheduleHandler
}

// New 创建 automation 域模块。
func New() *Module {
	executionService := executionSvc.NewService()
	scheduleService := scheduleSvc.NewService()
	scheduler := healingSvc.DefaultScheduler()
	module := &Module{
		Execution: automationhttp.NewExecutionHandlerWithDeps(automationhttp.ExecutionHandlerDeps{
			Service: executionService,
		}),
		Healing: automationhttp.NewHealingHandlerWithDeps(automationhttp.HealingHandlerDeps{
			FlowRepo:         automationrepo.NewHealingFlowRepository(),
			RuleRepo:         automationrepo.NewHealingRuleRepository(),
			InstanceRepo:     automationrepo.NewFlowInstanceRepository(),
			ApprovalRepo:     automationrepo.NewApprovalTaskRepository(),
			IncidentRepo:     incidentrepo.NewIncidentRepository(),
			NotificationRepo: engagementrepo.NewNotificationRepository(database.DB),
			Executor:         scheduler.Executor(),
			Scheduler:        scheduler,
		}),
		Schedule: automationhttp.NewScheduleHandlerWithDeps(automationhttp.ScheduleHandlerDeps{
			Service: scheduleService,
		}),
	}
	platformlifecycle.RegisterCleanup(module.Execution.Shutdown)
	platformlifecycle.RegisterCleanup(module.Healing.Shutdown)
	return module
}
