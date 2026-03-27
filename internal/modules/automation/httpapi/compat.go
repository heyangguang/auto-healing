package httpapi

import (
	"github.com/company/auto-healing/internal/database"
	automationrepo "github.com/company/auto-healing/internal/modules/automation/repository"
	"github.com/company/auto-healing/internal/modules/automation/service/execution"
	healing "github.com/company/auto-healing/internal/modules/automation/service/healing"
	"github.com/company/auto-healing/internal/modules/automation/service/schedule"
	engagementrepo "github.com/company/auto-healing/internal/modules/engagement/repository"
	incidentrepo "github.com/company/auto-healing/internal/platform/repository/incident"
	"gorm.io/gorm"
)

// NewExecutionHandler 保留兼容零参构造，生产路径应使用显式 deps。
func NewExecutionHandler() *ExecutionHandler {
	return NewExecutionHandlerWithDeps(ExecutionHandlerDeps{
		Service: execution.NewService(),
	})
}

// NewScheduleHandler 保留兼容零参构造，生产路径应使用显式 deps。
func NewScheduleHandler() *ScheduleHandler {
	return NewScheduleHandlerWithDeps(ScheduleHandlerDeps{
		Service: schedule.NewService(),
	})
}

// NewHealingHandler 保留兼容零参构造，生产路径应使用显式 deps。
func NewHealingHandler() *HealingHandler {
	return NewHealingHandlerWithDB(database.DB)
}

func NewHealingHandlerWithDB(db *gorm.DB) *HealingHandler {
	scheduler := healing.NewSchedulerWithDB(db)
	return NewHealingHandlerWithDeps(HealingHandlerDeps{
		FlowRepo:         automationrepo.NewHealingFlowRepositoryWithDB(db),
		RuleRepo:         automationrepo.NewHealingRuleRepositoryWithDB(db),
		InstanceRepo:     automationrepo.NewFlowInstanceRepositoryWithDB(db),
		ApprovalRepo:     automationrepo.NewApprovalTaskRepositoryWithDB(db),
		IncidentRepo:     incidentrepo.NewIncidentRepositoryWithDB(db),
		NotificationRepo: engagementrepo.NewNotificationRepository(db),
		Executor:         scheduler.Executor(),
		Scheduler:        scheduler,
	})
}
