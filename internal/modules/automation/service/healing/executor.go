package healing

import (
	"strconv"
	"sync"

	"github.com/company/auto-healing/internal/database"
	"github.com/company/auto-healing/internal/engine/provider/ansible"
	"github.com/company/auto-healing/internal/model"
	"github.com/company/auto-healing/internal/modules/automation/service/execution"
	notificationSvc "github.com/company/auto-healing/internal/notification"
	"github.com/company/auto-healing/internal/repository"
)

// FlowExecutor 流程执行器
type FlowExecutor struct {
	instanceRepo    *repository.FlowInstanceRepository
	approvalRepo    *repository.ApprovalTaskRepository
	flowRepo        *repository.HealingFlowRepository
	flowLogRepo     *repository.FlowLogRepository
	cmdbRepo        *repository.CMDBItemRepository
	gitRepoRepo     *repository.GitRepositoryRepository
	executionRepo   *repository.ExecutionRepository
	incidentRepo    *repository.IncidentRepository
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
		instanceRepo:    repository.NewFlowInstanceRepository(),
		approvalRepo:    repository.NewApprovalTaskRepository(),
		flowRepo:        repository.NewHealingFlowRepository(),
		flowLogRepo:     repository.NewFlowLogRepository(),
		cmdbRepo:        repository.NewCMDBItemRepository(),
		gitRepoRepo:     repository.NewGitRepositoryRepository(),
		executionRepo:   repository.NewExecutionRepository(),
		incidentRepo:    repository.NewIncidentRepository(),
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
