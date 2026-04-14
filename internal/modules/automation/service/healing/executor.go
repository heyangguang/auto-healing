package healing

import (
	"context"
	"strconv"
	"sync"

	"github.com/company/auto-healing/internal/modules/automation/engine/provider/ansible"
	"github.com/company/auto-healing/internal/modules/automation/model"
	automationrepo "github.com/company/auto-healing/internal/modules/automation/repository"
	"github.com/company/auto-healing/internal/modules/automation/service/execution"
	notificationSvc "github.com/company/auto-healing/internal/modules/engagement/service/notification"
	integrationrepo "github.com/company/auto-healing/internal/modules/integrations/repository"
	cmdbrepo "github.com/company/auto-healing/internal/platform/repository/cmdb"
	incidentrepo "github.com/company/auto-healing/internal/platform/repository/incident"
	"github.com/google/uuid"
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
	incidentCloser  IncidentCloser
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
	IncidentCloser  IncidentCloser
	ExecutionSvc    *execution.Service
	NotificationSvc *notificationSvc.Service
	AnsibleExecutor ansible.Executor
	EventBus        *EventBus
	Lifecycle       *asyncLifecycle
}

func NewFlowExecutorWithDeps(deps FlowExecutorDeps) *FlowExecutor {
	switch {
	case deps.InstanceRepo == nil:
		panic("automation flow executor requires instance repo")
	case deps.ApprovalRepo == nil:
		panic("automation flow executor requires approval repo")
	case deps.FlowRepo == nil:
		panic("automation flow executor requires flow repo")
	case deps.FlowLogRepo == nil:
		panic("automation flow executor requires flow log repo")
	case deps.CMDBRepo == nil:
		panic("automation flow executor requires cmdb repo")
	case deps.GitRepoRepo == nil:
		panic("automation flow executor requires git repo")
	case deps.ExecutionRepo == nil:
		panic("automation flow executor requires execution repo")
	case deps.IncidentRepo == nil:
		panic("automation flow executor requires incident repo")
	case deps.IncidentCloser == nil:
		panic("automation flow executor requires incident closer")
	case deps.ExecutionSvc == nil:
		panic("automation flow executor requires execution service")
	case deps.NotificationSvc == nil:
		panic("automation flow executor requires notification service")
	case deps.AnsibleExecutor == nil:
		panic("automation flow executor requires ansible executor")
	case deps.EventBus == nil:
		panic("automation flow executor requires event bus")
	}
	if deps.Lifecycle == nil {
		deps.Lifecycle = newAsyncLifecycle()
	}
	return &FlowExecutor{
		instanceRepo:    deps.InstanceRepo,
		approvalRepo:    deps.ApprovalRepo,
		flowRepo:        deps.FlowRepo,
		flowLogRepo:     deps.FlowLogRepo,
		cmdbRepo:        deps.CMDBRepo,
		gitRepoRepo:     deps.GitRepoRepo,
		executionRepo:   deps.ExecutionRepo,
		incidentRepo:    deps.IncidentRepo,
		incidentCloser:  deps.IncidentCloser,
		executionSvc:    deps.ExecutionSvc,
		notificationSvc: deps.NotificationSvc,
		ansibleExecutor: deps.AnsibleExecutor,
		eventBus:        deps.EventBus,
		lifecycle:       deps.Lifecycle,
	}
}

type IncidentCloser interface {
	CloseIncident(ctx context.Context, params IncidentCloseParams) (*IncidentCloseResult, error)
}

type IncidentCloseParams struct {
	IncidentID     uuid.UUID
	Resolution     string
	WorkNotes      string
	CloseCode      string
	CloseStatus    string
	TriggerSource  string
	OperatorUserID *uuid.UUID
	OperatorName   string
	FlowInstanceID *uuid.UUID
	ExecutionRunID *uuid.UUID
}

type IncidentCloseResult struct {
	LocalStatus    string
	SourceUpdated  bool
	WritebackLogID *uuid.UUID
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
