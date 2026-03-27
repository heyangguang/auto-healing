package healing

import (
	"github.com/company/auto-healing/internal/database"
	"github.com/company/auto-healing/internal/modules/automation/engine/provider/ansible"
	automationrepo "github.com/company/auto-healing/internal/modules/automation/repository"
	"github.com/company/auto-healing/internal/modules/automation/service/execution"
	notificationSvc "github.com/company/auto-healing/internal/modules/engagement/service/notification"
	integrationrepo "github.com/company/auto-healing/internal/modules/integrations/repository"
	cmdbrepo "github.com/company/auto-healing/internal/platform/repository/cmdb"
	incidentrepo "github.com/company/auto-healing/internal/platform/repository/incident"
	"gorm.io/gorm"
)

func DefaultFlowExecutorDeps(executionSvc *execution.Service, notificationService *notificationSvc.Service) FlowExecutorDeps {
	return DefaultFlowExecutorDepsWithDB(database.DB, executionSvc, notificationService)
}

func DefaultFlowExecutorRuntimeDeps() FlowExecutorDeps {
	return DefaultFlowExecutorRuntimeDepsWithDB(database.DB)
}

func DefaultFlowExecutorDepsWithDB(db *gorm.DB, executionSvc *execution.Service, notificationService *notificationSvc.Service) FlowExecutorDeps {
	return FlowExecutorDeps{
		InstanceRepo:    automationrepo.NewFlowInstanceRepositoryWithDB(db),
		ApprovalRepo:    automationrepo.NewApprovalTaskRepositoryWithDB(db),
		FlowRepo:        automationrepo.NewHealingFlowRepositoryWithDB(db),
		FlowLogRepo:     automationrepo.NewFlowLogRepositoryWithDB(db),
		CMDBRepo:        cmdbrepo.NewCMDBItemRepositoryWithDB(db),
		GitRepoRepo:     integrationrepo.NewGitRepositoryRepositoryWithDB(db),
		ExecutionRepo:   automationrepo.NewExecutionRepositoryWithDB(db),
		IncidentRepo:    incidentrepo.NewIncidentRepositoryWithDB(db),
		ExecutionSvc:    executionSvc,
		NotificationSvc: notificationService,
		AnsibleExecutor: ansible.NewLocalExecutor(),
		EventBus:        GetEventBus(),
		Lifecycle:       newAsyncLifecycle(),
	}
}

func DefaultFlowExecutorRuntimeDepsWithDB(db *gorm.DB) FlowExecutorDeps {
	return DefaultFlowExecutorDepsWithDB(db, execution.NewServiceWithDB(db), notificationSvc.NewConfiguredService(db))
}

// NewFlowExecutor 保留兼容零参构造，生产路径应使用显式 deps。
func NewFlowExecutor() *FlowExecutor {
	return NewFlowExecutorWithDB(database.DB)
}

func NewFlowExecutorWithDB(db *gorm.DB) *FlowExecutor {
	return NewFlowExecutorWithDeps(DefaultFlowExecutorRuntimeDepsWithDB(db))
}

func NewFlowExecutorWithDependencies(executionSvc *execution.Service, notificationService *notificationSvc.Service) *FlowExecutor {
	return NewFlowExecutorWithDeps(DefaultFlowExecutorDeps(executionSvc, notificationService))
}
