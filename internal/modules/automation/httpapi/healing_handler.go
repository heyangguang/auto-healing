package httpapi

import (
	"github.com/company/auto-healing/internal/database"
	automationrepo "github.com/company/auto-healing/internal/modules/automation/repository"
	healing "github.com/company/auto-healing/internal/modules/automation/service/healing"
	engagementrepo "github.com/company/auto-healing/internal/modules/engagement/repository"
	"github.com/company/auto-healing/internal/pkg/response"
	incidentrepo "github.com/company/auto-healing/internal/platform/repository/incident"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// HealingHandler 自愈引擎处理器
type HealingHandler struct {
	flowRepo     *automationrepo.HealingFlowRepository
	ruleRepo     *automationrepo.HealingRuleRepository
	instanceRepo *automationrepo.FlowInstanceRepository
	approvalRepo *automationrepo.ApprovalTaskRepository
	incidentRepo *incidentrepo.IncidentRepository
	notifRepo    *engagementrepo.NotificationRepository
	executor     *healing.FlowExecutor
	scheduler    *healing.Scheduler
}

type HealingHandlerDeps struct {
	FlowRepo         *automationrepo.HealingFlowRepository
	RuleRepo         *automationrepo.HealingRuleRepository
	InstanceRepo     *automationrepo.FlowInstanceRepository
	ApprovalRepo     *automationrepo.ApprovalTaskRepository
	IncidentRepo     *incidentrepo.IncidentRepository
	NotificationRepo *engagementrepo.NotificationRepository
	Executor         *healing.FlowExecutor
	Scheduler        *healing.Scheduler
}

// NewHealingHandler 创建自愈引擎处理器
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

func NewHealingHandlerWithDeps(deps HealingHandlerDeps) *HealingHandler {
	return &HealingHandler{
		flowRepo:     deps.FlowRepo,
		ruleRepo:     deps.RuleRepo,
		instanceRepo: deps.InstanceRepo,
		approvalRepo: deps.ApprovalRepo,
		incidentRepo: deps.IncidentRepo,
		notifRepo:    deps.NotificationRepo,
		executor:     deps.Executor,
		scheduler:    deps.Scheduler,
	}
}

func (h *HealingHandler) Shutdown() {
	if h == nil {
		return
	}
	if h.scheduler != nil {
		h.scheduler.Stop()
	}
	if h.executor != nil {
		h.executor.Shutdown()
	}
}

// ========== HealingFlow 相关 ==========

// GetNodeSchema 获取节点类型的配置和变量定义
// 用于前端流程设计器，帮助用户了解每种节点的配置项和输入输出
func (h *HealingHandler) GetNodeSchema(c *gin.Context) {
	response.Success(c, healingNodeSchema())
}

// ========== Search Schema 声明 ==========

var flowSearchSchema = []SearchableField{
	{Key: "name", Label: "名称", Type: "text", MatchModes: []string{"fuzzy", "exact"}, DefaultMode: "fuzzy", Placeholder: "流程名称", Column: "name"},
	{Key: "description", Label: "描述", Type: "text", MatchModes: []string{"fuzzy", "exact"}, DefaultMode: "fuzzy", Placeholder: "流程描述", Column: "description"},
	{Key: "is_active", Label: "状态", Type: "enum", MatchModes: []string{"exact"}, DefaultMode: "exact", Options: []FilterOption{{Label: "已启用", Value: "true"}, {Label: "已停用", Value: "false"}}},
}

var ruleSearchSchema = []SearchableField{
	{Key: "name", Label: "名称", Type: "text", MatchModes: []string{"fuzzy", "exact"}, DefaultMode: "fuzzy", Placeholder: "规则名称", Column: "name"},
	{Key: "description", Label: "描述", Type: "text", MatchModes: []string{"fuzzy", "exact"}, DefaultMode: "fuzzy", Placeholder: "规则描述", Column: "description"},
	{Key: "trigger_mode", Label: "触发模式", Type: "enum", MatchModes: []string{"exact"}, DefaultMode: "exact", Options: []FilterOption{{Label: "自动", Value: "auto"}, {Label: "手动", Value: "manual"}}},
	{Key: "is_active", Label: "状态", Type: "enum", MatchModes: []string{"exact"}, DefaultMode: "exact", Options: []FilterOption{{Label: "已启用", Value: "true"}, {Label: "已停用", Value: "false"}}},
	{Key: "has_flow", Label: "关联流程", Type: "boolean", MatchModes: []string{"exact"}, DefaultMode: "exact"},
}

var instanceSearchSchema = []SearchableField{
	{Key: "status", Label: "状态", Type: "enum", MatchModes: []string{"exact"}, DefaultMode: "exact", Options: []FilterOption{
		{Label: "运行中", Value: "running"}, {Label: "已完成", Value: "completed"},
		{Label: "失败", Value: "failed"}, {Label: "等待审批", Value: "waiting_approval"},
		{Label: "已取消", Value: "cancelled"}, {Label: "已跳过", Value: "skipped"},
	}},
	{Key: "flow_name", Label: "流程名称", Type: "text", MatchModes: []string{"fuzzy", "exact"}, DefaultMode: "fuzzy"},
	{Key: "rule_name", Label: "规则名称", Type: "text", MatchModes: []string{"fuzzy", "exact"}, DefaultMode: "fuzzy"},
	{Key: "incident_title", Label: "工单标题", Type: "text", MatchModes: []string{"fuzzy", "exact"}, DefaultMode: "fuzzy"},
	{Key: "error_message", Label: "错误信息", Type: "text", MatchModes: []string{"fuzzy"}, DefaultMode: "fuzzy"},
}

// GetFlowSearchSchema 返回自愈流程搜索字段声明
func (h *HealingHandler) GetFlowSearchSchema(c *gin.Context) {
	response.Success(c, gin.H{"fields": flowSearchSchema})
}

// GetRuleSearchSchema 返回自愈规则搜索字段声明
func (h *HealingHandler) GetRuleSearchSchema(c *gin.Context) {
	response.Success(c, gin.H{"fields": ruleSearchSchema})
}

// GetInstanceSearchSchema 返回流程实例搜索字段声明
func (h *HealingHandler) GetInstanceSearchSchema(c *gin.Context) {
	response.Success(c, gin.H{"fields": instanceSearchSchema})
}
