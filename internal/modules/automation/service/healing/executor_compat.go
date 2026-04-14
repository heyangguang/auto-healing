package healing

import (
	"context"

	"github.com/company/auto-healing/internal/database"
	"github.com/company/auto-healing/internal/modules/automation/engine/provider/ansible"
	automationrepo "github.com/company/auto-healing/internal/modules/automation/repository"
	"github.com/company/auto-healing/internal/modules/automation/service/execution"
	notificationSvc "github.com/company/auto-healing/internal/modules/engagement/service/notification"
	integrationrepo "github.com/company/auto-healing/internal/modules/integrations/repository"
	pluginservice "github.com/company/auto-healing/internal/modules/integrations/service/plugin"
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
	incidentCloser := pluginservice.NewIncidentServiceWithDeps(pluginservice.IncidentServiceDeps{
		IncidentRepo:     incidentrepo.NewIncidentRepositoryWithDB(db),
		WritebackLogRepo: incidentrepo.NewIncidentWritebackLogRepositoryWithDB(db),
		PluginRepo:       integrationrepo.NewPluginRepositoryWithDB(db),
		HTTPClient:       pluginservice.NewHTTPClient(),
	})
	return FlowExecutorDeps{
		InstanceRepo:    automationrepo.NewFlowInstanceRepositoryWithDB(db),
		ApprovalRepo:    automationrepo.NewApprovalTaskRepositoryWithDB(db),
		FlowRepo:        automationrepo.NewHealingFlowRepositoryWithDB(db),
		FlowLogRepo:     automationrepo.NewFlowLogRepositoryWithDB(db),
		CMDBRepo:        cmdbrepo.NewCMDBItemRepositoryWithDB(db),
		GitRepoRepo:     integrationrepo.NewGitRepositoryRepositoryWithDB(db),
		ExecutionRepo:   automationrepo.NewExecutionRepositoryWithDB(db),
		IncidentRepo:    incidentrepo.NewIncidentRepositoryWithDB(db),
		IncidentCloser:  &incidentCloserAdapter{svc: incidentCloser},
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

type incidentCloserAdapter struct {
	svc *pluginservice.IncidentService
}

func (a *incidentCloserAdapter) CloseIncident(ctx context.Context, params IncidentCloseParams) (*IncidentCloseResult, error) {
	result, err := a.svc.CloseIncident(ctx, pluginservice.CloseIncidentParams{
		IncidentID:     params.IncidentID,
		Resolution:     params.Resolution,
		WorkNotes:      params.WorkNotes,
		CloseCode:      params.CloseCode,
		CloseStatus:    params.CloseStatus,
		TriggerSource:  params.TriggerSource,
		OperatorUserID: params.OperatorUserID,
		OperatorName:   params.OperatorName,
		FlowInstanceID: params.FlowInstanceID,
		ExecutionRunID: params.ExecutionRunID,
	})
	if err != nil {
		return nil, err
	}
	return &IncidentCloseResult{
		LocalStatus:    result.LocalStatus,
		SourceUpdated:  result.SourceUpdated,
		WritebackLogID: result.WritebackLogID,
	}, nil
}
