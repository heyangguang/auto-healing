package healing

import (
	"github.com/company/auto-healing/internal/database"
	automationrepo "github.com/company/auto-healing/internal/modules/automation/repository"
	incidentrepo "github.com/company/auto-healing/internal/platform/repository/incident"
	"gorm.io/gorm"
	"time"
)

func DefaultSchedulerDeps(executor *FlowExecutor) SchedulerDeps {
	return DefaultSchedulerDepsWithDB(database.DB, executor)
}

func DefaultSchedulerRuntimeDeps() SchedulerDeps {
	return DefaultSchedulerRuntimeDepsWithDB(database.DB)
}

func DefaultSchedulerDepsWithDB(db *gorm.DB, executor *FlowExecutor) SchedulerDeps {
	return SchedulerDeps{
		RuleRepo:     automationrepo.NewHealingRuleRepositoryWithDB(db),
		FlowRepo:     automationrepo.NewHealingFlowRepositoryWithDB(db),
		InstanceRepo: automationrepo.NewFlowInstanceRepositoryWithDB(db),
		IncidentRepo: incidentrepo.NewIncidentRepositoryWithDB(db),
		ApprovalRepo: automationrepo.NewApprovalTaskRepositoryWithDB(db),
		Matcher:      NewRuleMatcher(),
		Executor:     executor,
		Interval:     10 * time.Second,
		Lifecycle:    newAsyncLifecycle(),
		Sem:          make(chan struct{}, 10),
	}
}

func DefaultSchedulerRuntimeDepsWithDB(db *gorm.DB) SchedulerDeps {
	return DefaultSchedulerDepsWithDB(db, NewFlowExecutorWithDB(db))
}

// NewScheduler 保留兼容零参构造，生产路径应使用显式 deps。
func NewScheduler() *Scheduler {
	return NewSchedulerWithDB(database.DB)
}

func NewSchedulerWithDB(db *gorm.DB) *Scheduler {
	return NewSchedulerWithDeps(DefaultSchedulerRuntimeDepsWithDB(db))
}
