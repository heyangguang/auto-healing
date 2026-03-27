package healing

import (
	"strconv"
	"sync"

	"github.com/company/auto-healing/internal/database"
	"github.com/company/auto-healing/internal/modules/automation/engine/provider/ansible"
	"github.com/company/auto-healing/internal/modules/automation/model"
	automationrepo "github.com/company/auto-healing/internal/modules/automation/repository"
	"github.com/company/auto-healing/internal/modules/automation/service/execution"
	notificationSvc "github.com/company/auto-healing/internal/modules/engagement/service/notification"
	integrationrepo "github.com/company/auto-healing/internal/modules/integrations/repository"
	cmdbrepo "github.com/company/auto-healing/internal/platform/repository/cmdb"
	incidentrepo "github.com/company/auto-healing/internal/platform/repository/incident"
	"gorm.io/gorm"
)

// FlowExecutor 流程执行器
type FlowExecutor struct {
	instanceRepo    *automationrepo.FlowInstanceRepository
	approvalRepo    *automationrepo.ApprovalTaskRepository
	flowRepo        *automationrepo.HealingFlowRepository
	flowLogRepo     *automationrepo.FlowLogRepository
	cmdbRepo        *cmdbrepo.CMDBItemRepository
	gitRepoRepo     *integrationrepo.GitRepositoryRepository
	executionRepo   *automationrepo.ExecutionRepository
	incidentRepo    *incidentrepo.IncidentRepository
	executionSvc    *execution.Service
	notificationSvc *notificationSvc.Service
	ansibleExecutor ansible.Executor
	eventBus        *EventBus
	lifecycle       *asyncLifecycle
}

var runningFlowCancels sync.Map // map[uuid.UUID]context.CancelFunc

type FlowExecutorDeps struct {
	InstanceRepo    *automationrepo.FlowInstanceRepository
	ApprovalRepo    *automationrepo.ApprovalTaskRepository
	FlowRepo        *automationrepo.HealingFlowRepository
	FlowLogRepo     *automationrepo.FlowLogRepository
	CMDBRepo        *cmdbrepo.CMDBItemRepository
	GitRepoRepo     *integrationrepo.GitRepositoryRepository
	ExecutionRepo   *automationrepo.ExecutionRepository
	IncidentRepo    *incidentrepo.IncidentRepository
	ExecutionSvc    *execution.Service
	NotificationSvc *notificationSvc.Service
	AnsibleExecutor ansible.Executor
	EventBus        *EventBus
	Lifecycle       *asyncLifecycle
}

func DefaultFlowExecutorDeps(executionSvc *execution.Service, notificationService *notificationSvc.Service) FlowExecutorDeps {
	return DefaultFlowExecutorDepsWithDB(database.DB, executionSvc, notificationService)
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

func DefaultFlowExecutorRuntimeDeps() FlowExecutorDeps {
	return DefaultFlowExecutorRuntimeDepsWithDB(database.DB)
}

func DefaultFlowExecutorRuntimeDepsWithDB(db *gorm.DB) FlowExecutorDeps {
	return DefaultFlowExecutorDepsWithDB(db, execution.NewServiceWithDB(db), notificationSvc.NewConfiguredService(db))
}

// NewFlowExecutor 创建流程执行器
func NewFlowExecutor() *FlowExecutor {
	return NewFlowExecutorWithDB(database.DB)
}

func NewFlowExecutorWithDB(db *gorm.DB) *FlowExecutor {
	return NewFlowExecutorWithDeps(DefaultFlowExecutorRuntimeDepsWithDB(db))
}

func NewFlowExecutorWithDependencies(executionSvc *execution.Service, notificationService *notificationSvc.Service) *FlowExecutor {
	return NewFlowExecutorWithDeps(DefaultFlowExecutorDeps(executionSvc, notificationService))
}

func NewFlowExecutorWithDeps(deps FlowExecutorDeps) *FlowExecutor {
	return &FlowExecutor{
		instanceRepo:    deps.InstanceRepo,
		approvalRepo:    deps.ApprovalRepo,
		flowRepo:        deps.FlowRepo,
		flowLogRepo:     deps.FlowLogRepo,
		cmdbRepo:        deps.CMDBRepo,
		gitRepoRepo:     deps.GitRepoRepo,
		executionRepo:   deps.ExecutionRepo,
		incidentRepo:    deps.IncidentRepo,
		executionSvc:    deps.ExecutionSvc,
		notificationSvc: deps.NotificationSvc,
		ansibleExecutor: deps.AnsibleExecutor,
		eventBus:        deps.EventBus,
		lifecycle:       deps.Lifecycle,
	}
}

// toFloat 将 interface{} 转换为 float64
func toFloat(v interface{}) float64 {
	switch val := v.(type) {
	case float64:
		return val
	case float32:
		return float64(val)
	case int:
		return float64(val)
	case int64:
		return float64(val)
	case int32:
		return float64(val)
	case string:
		if f, err := strconv.ParseFloat(val, 64); err == nil {
			return f
		}
		return 0
	default:
		return 0
	}
}

// shortID 返回实例ID的前8位，用于日志追踪
func shortID(instance *model.FlowInstance) string {
	return instance.ID.String()[:8]
}
