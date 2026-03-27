package automation

import (
	"github.com/company/auto-healing/internal/modules/automation/engine/provider/ansible"
	automationhttp "github.com/company/auto-healing/internal/modules/automation/httpapi"
	automationrepo "github.com/company/auto-healing/internal/modules/automation/repository"
	executionSvc "github.com/company/auto-healing/internal/modules/automation/service/execution"
	healingSvc "github.com/company/auto-healing/internal/modules/automation/service/healing"
	scheduleSvc "github.com/company/auto-healing/internal/modules/automation/service/schedule"
	engagementrepo "github.com/company/auto-healing/internal/modules/engagement/repository"
	notification "github.com/company/auto-healing/internal/modules/engagement/service/notification"
	integrationrepo "github.com/company/auto-healing/internal/modules/integrations/repository"
	opsrepo "github.com/company/auto-healing/internal/modules/ops/repository"
	opsservice "github.com/company/auto-healing/internal/modules/ops/service"
	secretsrepo "github.com/company/auto-healing/internal/modules/secrets/repository"
	platformlifecycle "github.com/company/auto-healing/internal/platform/lifecycle"
	cmdbrepo "github.com/company/auto-healing/internal/platform/repository/cmdb"
	incidentrepo "github.com/company/auto-healing/internal/platform/repository/incident"
	"gorm.io/gorm"
)

// Module 聚合 automation 域处理器构造。
type Module struct {
	Execution *automationhttp.ExecutionHandler
	Healing   *automationhttp.HealingHandler
	Schedule  *automationhttp.ScheduleHandler
}

type ModuleDeps struct {
	ExecutionRepo    *automationrepo.ExecutionRepository
	FlowRepo         *automationrepo.HealingFlowRepository
	RuleRepo         *automationrepo.HealingRuleRepository
	InstanceRepo     *automationrepo.FlowInstanceRepository
	ApprovalRepo     *automationrepo.ApprovalTaskRepository
	ScheduleRepo     *automationrepo.ScheduleRepository
	IncidentRepo     *incidentrepo.IncidentRepository
	NotificationRepo *engagementrepo.NotificationRepository
	ExecutionService *executionSvc.Service
	ScheduleService  *scheduleSvc.Service
	FlowExecutor     *healingSvc.FlowExecutor
	HealingScheduler *healingSvc.Scheduler
}

func DefaultModuleDepsWithDB(db *gorm.DB) ModuleDeps {
	executionRepo := automationrepo.NewExecutionRepositoryWithDB(db)
	flowRepo := automationrepo.NewHealingFlowRepositoryWithDB(db)
	ruleRepo := automationrepo.NewHealingRuleRepositoryWithDB(db)
	instanceRepo := automationrepo.NewFlowInstanceRepositoryWithDB(db)
	approvalRepo := automationrepo.NewApprovalTaskRepositoryWithDB(db)
	scheduleRepo := automationrepo.NewScheduleRepositoryWithDB(db)
	incidentRepo := incidentrepo.NewIncidentRepositoryWithDB(db)
	notificationRepo := engagementrepo.NewNotificationRepository(db)
	notificationService := notification.NewConfiguredService(db)
	executionService := executionSvc.NewServiceWithDeps(executionSvc.ServiceDeps{
		Repo:             executionRepo,
		GitRepo:          integrationrepo.NewGitRepositoryRepositoryWithDB(db),
		SecretsRepo:      secretsrepo.NewSecretsSourceRepositoryWithDB(db),
		CMDBRepo:         cmdbrepo.NewCMDBItemRepositoryWithDB(db),
		HealingFlowRepo:  flowRepo,
		WorkspaceManager: ansible.NewWorkspaceManager(),
		LocalExecutor:    ansible.NewLocalExecutor(),
		DockerExecutor:   ansible.NewDockerExecutor(),
		NotificationSvc:  notificationService,
		BlacklistSvc: opsservice.NewCommandBlacklistServiceWithDeps(opsservice.CommandBlacklistServiceDeps{
			Repo: opsrepo.NewCommandBlacklistRepositoryWithDB(db),
		}),
		ExemptionSvc: opsservice.NewBlacklistExemptionServiceWithDeps(opsservice.BlacklistExemptionServiceDeps{
			Repo: opsrepo.NewBlacklistExemptionRepository(db),
		}),
	})
	scheduleService := scheduleSvc.NewServiceWithDeps(scheduleSvc.ServiceDeps{
		Repo:     scheduleRepo,
		ExecRepo: executionRepo,
	})
	flowExecutor := healingSvc.NewFlowExecutorWithDeps(healingSvc.DefaultFlowExecutorDepsWithDB(db, executionService, notificationService))
	return ModuleDeps{
		ExecutionRepo:    executionRepo,
		FlowRepo:         flowRepo,
		RuleRepo:         ruleRepo,
		InstanceRepo:     instanceRepo,
		ApprovalRepo:     approvalRepo,
		ScheduleRepo:     scheduleRepo,
		IncidentRepo:     incidentRepo,
		NotificationRepo: notificationRepo,
		ExecutionService: executionService,
		ScheduleService:  scheduleService,
		FlowExecutor:     flowExecutor,
		HealingScheduler: healingSvc.NewSchedulerWithDeps(healingSvc.DefaultSchedulerDepsWithDB(db, flowExecutor)),
	}
}

func NewWithDB(db *gorm.DB) *Module {
	return NewWithDeps(DefaultModuleDepsWithDB(db))
}

func NewWithDeps(deps ModuleDeps) *Module {
	module := &Module{
		Execution: automationhttp.NewExecutionHandlerWithDeps(automationhttp.ExecutionHandlerDeps{
			Service: deps.ExecutionService,
		}),
		Healing: automationhttp.NewHealingHandlerWithDeps(automationhttp.HealingHandlerDeps{
			FlowRepo:         deps.FlowRepo,
			RuleRepo:         deps.RuleRepo,
			InstanceRepo:     deps.InstanceRepo,
			ApprovalRepo:     deps.ApprovalRepo,
			IncidentRepo:     deps.IncidentRepo,
			NotificationRepo: deps.NotificationRepo,
			Executor:         deps.FlowExecutor,
			Scheduler:        deps.HealingScheduler,
		}),
		Schedule: automationhttp.NewScheduleHandlerWithDeps(automationhttp.ScheduleHandlerDeps{
			Service: deps.ScheduleService,
		}),
	}
	platformlifecycle.RegisterCleanup(module.Execution.Shutdown)
	platformlifecycle.RegisterCleanup(module.Healing.Shutdown)
	return module
}
