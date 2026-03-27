package healing

import (
	"context"

	"github.com/company/auto-healing/internal/database"
	"github.com/company/auto-healing/internal/modules/automation/model"
	automationrepo "github.com/company/auto-healing/internal/modules/automation/repository"
	engagementrepo "github.com/company/auto-healing/internal/modules/engagement/repository"
	platformmodel "github.com/company/auto-healing/internal/platform/model"
	cmdbrepo "github.com/company/auto-healing/internal/platform/repository/cmdb"
	"gorm.io/gorm"
)

// DryRunResult Dry-Run 执行结果
type DryRunResult struct {
	Success bool               `json:"success"`
	Message string             `json:"message,omitempty"`
	Nodes   []DryRunNodeResult `json:"nodes"`
}

// DryRunNodeResult 节点执行结果
type DryRunNodeResult struct {
	NodeID   string                 `json:"node_id"`
	NodeType string                 `json:"node_type"`
	NodeName string                 `json:"node_name,omitempty"`
	Status   string                 `json:"status"` // success, failed, error (与真实执行一致)
	Message  string                 `json:"message,omitempty"`
	Input    map[string]interface{} `json:"input,omitempty"`   // 节点输入（上游数据+全局上下文）
	Process  []string               `json:"process,omitempty"` // 执行过程日志
	Output   map[string]interface{} `json:"output,omitempty"`  // 节点输出（传给下游）
	Config   map[string]interface{} `json:"config,omitempty"`
}

// DryRunExecutor Dry-Run 执行器
type DryRunExecutor struct {
	taskRepo         *automationrepo.ExecutionRepository
	cmdbRepo         *cmdbrepo.CMDBItemRepository
	notificationRepo *engagementrepo.NotificationRepository
}

type DryRunExecutorDeps struct {
	TaskRepo         *automationrepo.ExecutionRepository
	CMDBRepo         *cmdbrepo.CMDBItemRepository
	NotificationRepo *engagementrepo.NotificationRepository
}

// NewDryRunExecutor 创建 Dry-Run 执行器
func NewDryRunExecutor() *DryRunExecutor {
	return NewDryRunExecutorWithDB(database.DB)
}

func NewDryRunExecutorWithDB(db *gorm.DB) *DryRunExecutor {
	return NewDryRunExecutorWithDeps(DryRunExecutorDeps{
		TaskRepo:         automationrepo.NewExecutionRepositoryWithDB(db),
		CMDBRepo:         cmdbrepo.NewCMDBItemRepositoryWithDB(db),
		NotificationRepo: engagementrepo.NewNotificationRepository(db),
	})
}

func NewDryRunExecutorWithDependencies(taskRepo *automationrepo.ExecutionRepository, cmdbRepo *cmdbrepo.CMDBItemRepository, notificationRepo *engagementrepo.NotificationRepository) *DryRunExecutor {
	return NewDryRunExecutorWithDeps(DryRunExecutorDeps{
		TaskRepo:         taskRepo,
		CMDBRepo:         cmdbRepo,
		NotificationRepo: notificationRepo,
	})
}

func NewDryRunExecutorWithDeps(deps DryRunExecutorDeps) *DryRunExecutor {
	return &DryRunExecutor{
		taskRepo:         deps.TaskRepo,
		cmdbRepo:         deps.CMDBRepo,
		notificationRepo: deps.NotificationRepo,
	}
}

// MockIncident 模拟工单数据
type MockIncident struct {
	Title           string                 `json:"title"`
	Description     string                 `json:"description,omitempty"`
	Severity        string                 `json:"severity,omitempty"`
	Priority        string                 `json:"priority,omitempty"`
	Status          string                 `json:"status,omitempty"`
	Category        string                 `json:"category,omitempty"`
	AffectedCI      string                 `json:"affected_ci,omitempty"`
	AffectedService string                 `json:"affected_service,omitempty"`
	Assignee        string                 `json:"assignee,omitempty"`
	Reporter        string                 `json:"reporter,omitempty"`
	RawData         map[string]interface{} `json:"raw_data,omitempty"`
}

// ToIncident 转换为 Incident 模型
func (m *MockIncident) ToIncident() *platformmodel.Incident {
	return &platformmodel.Incident{
		Title:           m.Title,
		Description:     m.Description,
		Severity:        m.Severity,
		Priority:        m.Priority,
		Status:          m.Status,
		Category:        m.Category,
		AffectedCI:      m.AffectedCI,
		AffectedService: m.AffectedService,
		Assignee:        m.Assignee,
		Reporter:        m.Reporter,
		RawData:         m.RawData,
	}
}

// NodeCallback 节点执行回调函数
// eventType: flow_start, node_start, node_log, node_complete, flow_complete
type NodeCallback func(eventType string, data map[string]interface{})

// Execute 执行 Dry-Run 测试
// fromNodeID: 从哪个节点开始（用于重试），为空则从 start 节点开始
// initialContext: 初始上下文（用于重试），为空则使用默认上下文
// mockApprovals: 模拟审批结果，node_id -> "approved" | "rejected"
func (e *DryRunExecutor) Execute(ctx context.Context, flow *model.HealingFlow, mockIncident *MockIncident, fromNodeID string, initialContext map[string]interface{}, mockApprovals map[string]string) *DryRunResult {
	return e.ExecuteWithCallback(ctx, flow, mockIncident, fromNodeID, initialContext, mockApprovals, nil)
}

// ExecuteWithCallback 执行 Dry-Run 测试（带回调，用于 SSE）
func (e *DryRunExecutor) ExecuteWithCallback(ctx context.Context, flow *model.HealingFlow, mockIncident *MockIncident, fromNodeID string, initialContext map[string]interface{}, mockApprovals map[string]string, callback NodeCallback) *DryRunResult {
	result := &DryRunResult{Success: true, Nodes: []DryRunNodeResult{}}
	emit := dryRunEmitter(callback)
	nodes := e.parseNodes(flow.Nodes)
	edges := e.parseEdges(flow.Edges)

	startNode, flowContext, ok := e.prepareDryRunStart(flow, mockIncident, fromNodeID, initialContext, mockApprovals, nodes, result, emit)
	if !ok {
		return result
	}

	emit(model.SSEEventFlowStart, map[string]interface{}{
		"flow_id":   flow.ID.String(),
		"flow_name": flow.Name,
	})
	e.runDryRunLoop(ctx, nodes, edges, startNode, flowContext, result, emit)
	e.finishDryRunResult(result, emit)
	return result
}
