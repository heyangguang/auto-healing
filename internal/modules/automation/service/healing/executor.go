package healing

import (
	"strconv"
	"sync"

	"github.com/company/auto-healing/internal/database"
	"github.com/company/auto-healing/internal/modules/automation/engine/provider/ansible"
	"github.com/company/auto-healing/internal/model"
	automationrepo "github.com/company/auto-healing/internal/modules/automation/repository"
	notificationSvc "github.com/company/auto-healing/internal/modules/engagement/service/notification"
	"github.com/company/auto-healing/internal/modules/automation/service/execution"
	integrationrepo "github.com/company/auto-healing/internal/modules/integrations/repository"
	cmdbrepo "github.com/company/auto-healing/internal/platform/repository/cmdb"
	incidentrepo "github.com/company/auto-healing/internal/platform/repository/incident"
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

// NewFlowExecutor 创建流程执行器
func NewFlowExecutor() *FlowExecutor {
	return NewFlowExecutorWithDependencies(execution.NewService(), notificationSvc.NewConfiguredService(database.DB))
}

func NewFlowExecutorWithDependencies(executionSvc *execution.Service, notificationService *notificationSvc.Service) *FlowExecutor {
	return &FlowExecutor{
		instanceRepo:    automationrepo.NewFlowInstanceRepository(),
		approvalRepo:    automationrepo.NewApprovalTaskRepository(),
		flowRepo:        automationrepo.NewHealingFlowRepository(),
		flowLogRepo:     automationrepo.NewFlowLogRepository(),
		cmdbRepo:        cmdbrepo.NewCMDBItemRepository(),
		gitRepoRepo:     integrationrepo.NewGitRepositoryRepository(),
		executionRepo:   automationrepo.NewExecutionRepository(),
		incidentRepo:    incidentrepo.NewIncidentRepository(),
		executionSvc:    executionSvc,
		notificationSvc: notificationService,
		ansibleExecutor: ansible.NewLocalExecutor(),
		eventBus:        GetEventBus(),
		lifecycle:       newAsyncLifecycle(),
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
