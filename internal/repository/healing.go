package repository

import (
	"context"
	"errors"
	"time"

	"github.com/company/auto-healing/internal/database"
	"github.com/company/auto-healing/internal/model"
	"github.com/company/auto-healing/internal/pkg/query"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

var (
	ErrHealingFlowNotFound  = errors.New("自愈流程不存在")
	ErrHealingRuleNotFound  = errors.New("自愈规则不存在")
	ErrFlowInstanceNotFound = errors.New("流程实例不存在")
	ErrApprovalTaskNotFound = errors.New("审批任务不存在")
)

// HealingFlowRepository 自愈流程仓库
type HealingFlowRepository struct {
	db *gorm.DB
}

// NewHealingFlowRepository 创建自愈流程仓库
func NewHealingFlowRepository() *HealingFlowRepository {
	return &HealingFlowRepository{db: database.DB}
}

// Create 创建自愈流程
func (r *HealingFlowRepository) Create(ctx context.Context, flow *model.HealingFlow) error {
	if flow.TenantID == nil {
		tenantID := TenantIDFromContext(ctx)
		flow.TenantID = &tenantID
	}
	return r.db.WithContext(ctx).Create(flow).Error
}

// GetByID 根据 ID 获取自愈流程
func (r *HealingFlowRepository) GetByID(ctx context.Context, id uuid.UUID) (*model.HealingFlow, error) {
	var flow model.HealingFlow
	err := TenantDB(r.db, ctx).Preload("Creator").First(&flow, "id = ?", id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrHealingFlowNotFound
	}
	return &flow, err
}

// Update 更新自愈流程
func (r *HealingFlowRepository) Update(ctx context.Context, flow *model.HealingFlow) error {
	return r.db.WithContext(ctx).Save(flow).Error
}

// Delete 删除自愈流程
func (r *HealingFlowRepository) Delete(ctx context.Context, id uuid.UUID) error {
	result := TenantDB(r.db, ctx).Delete(&model.HealingFlow{}, "id = ?", id)
	if result.RowsAffected == 0 {
		return ErrHealingFlowNotFound
	}
	return result.Error
}

// List 获取自愈流程列表
// 支持排序（name/created_at/updated_at/nodes_count）、高级过滤（name精确/description模糊/node_type/nodes数量/时间范围）
func (r *HealingFlowRepository) List(ctx context.Context, page, pageSize int, isActive *bool, search query.StringFilter, name, description query.StringFilter, nodeType string, minNodes, maxNodes *int, createdFrom, createdTo, updatedFrom, updatedTo, sortBy, sortOrder string) ([]model.HealingFlow, int64, error) {
	var flows []model.HealingFlow
	var total int64

	q := TenantDB(r.db, ctx).Model(&model.HealingFlow{})

	if isActive != nil {
		q = q.Where("is_active = ?", *isActive)
	}
	if !search.IsEmpty() {
		q = query.ApplyMultiStringFilter(q, []string{"name", "description"}, search)
	}
	if !name.IsEmpty() {
		q = query.ApplyStringFilter(q, "name", name)
	}
	if !description.IsEmpty() {
		q = query.ApplyStringFilter(q, "description", description)
	}
	if nodeType != "" {
		q = q.Where("nodes @> ?", `[{"type": "`+nodeType+`"}]`)
	}
	if minNodes != nil {
		q = q.Where("jsonb_array_length(nodes) >= ?", *minNodes)
	}
	if maxNodes != nil {
		q = q.Where("jsonb_array_length(nodes) <= ?", *maxNodes)
	}
	if createdFrom != "" {
		q = q.Where("created_at >= ?", createdFrom)
	}
	if createdTo != "" {
		q = q.Where("created_at <= ?", createdTo)
	}
	if updatedFrom != "" {
		q = q.Where("updated_at >= ?", updatedFrom)
	}
	if updatedTo != "" {
		q = q.Where("updated_at <= ?", updatedTo)
	}

	if err := q.Session(&gorm.Session{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}

	// 排序方向校验
	if sortOrder != "asc" && sortOrder != "desc" {
		sortOrder = "desc"
	}

	// 排序字段白名单
	var orderClause string
	switch sortBy {
	case "name":
		orderClause = "name " + sortOrder
	case "created_at":
		orderClause = "created_at " + sortOrder
	case "updated_at":
		orderClause = "updated_at " + sortOrder
	case "nodes_count":
		orderClause = "jsonb_array_length(nodes) " + sortOrder
	default:
		orderClause = "created_at DESC"
	}

	offset := (page - 1) * pageSize
	err := q.Preload("Creator").Offset(offset).Limit(pageSize).Order(orderClause).Find(&flows).Error
	return flows, total, err
}

// CountFlowsUsingTaskTemplate 统计使用指定任务模板的自愈流程数量
// 检查 nodes JSONB 数组中是否有 execution 节点引用该 task_template_id
func (r *HealingFlowRepository) CountFlowsUsingTaskTemplate(ctx context.Context, taskTemplateID string) (int64, error) {
	var count int64
	// 使用 PostgreSQL JSONB 查询：检查 nodes 数组中是否包含 task_template_id
	err := TenantDB(r.db, ctx).Model(&model.HealingFlow{}).
		Where("nodes @> ?", `[{"config": {"task_template_id": "`+taskTemplateID+`"}}]`).
		Count(&count).Error
	return count, err
}

// CountFlowsUsingChannel 统计使用指定通知渠道的自愈流程数量
// 检查 nodes JSONB 数组中是否有 notification 节点引用该 channel_id
func (r *HealingFlowRepository) CountFlowsUsingChannel(ctx context.Context, channelID string) (int64, error) {
	var count int64
	// 检查 channel_id（单个渠道）
	err := TenantDB(r.db, ctx).Model(&model.HealingFlow{}).
		Where("nodes @> ?", `[{"config": {"channel_id": "`+channelID+`"}}]`).
		Count(&count).Error
	if err != nil {
		return 0, err
	}

	// 也检查 channel_ids 数组（多个渠道）
	var count2 int64
	err = TenantDB(r.db, ctx).Model(&model.HealingFlow{}).
		Where("nodes::text LIKE ?", "%"+channelID+"%").
		Where("nodes @> ?", `[{"type": "notification"}]`).
		Count(&count2).Error
	if err != nil {
		return count, nil // 第一个查询结果仍然有效
	}

	// 返回较大的值（避免重复计数，两个查询可能有交集）
	if count2 > count {
		return count2, nil
	}
	return count, nil
}

// CountRulesUsingFlow 统计引用指定流程的规则数量
func (r *HealingFlowRepository) CountRulesUsingFlow(ctx context.Context, flowID uuid.UUID) (int64, error) {
	var count int64
	err := TenantDB(r.db, ctx).Model(&model.HealingRule{}).
		Where("flow_id = ?", flowID).
		Count(&count).Error
	return count, err
}

// CountActiveInstancesByFlowID 统计指定流程的运行中/待审批实例数量
func (r *HealingFlowRepository) CountActiveInstancesByFlowID(ctx context.Context, flowID uuid.UUID) (int64, error) {
	var count int64
	err := TenantDB(r.db, ctx).Model(&model.FlowInstance{}).
		Where("flow_id = ?", flowID).
		Where("status IN ?", []string{model.FlowInstanceStatusPending, model.FlowInstanceStatusRunning, model.FlowInstanceStatusWaitingApproval}).
		Count(&count).Error
	return count, err
}

// CountFlowsUsingTemplate 统计使用指定通知模板的自愈流程数量
// 检查 nodes JSONB 数组中是否有 notification 节点引用该 template_id
func (r *HealingFlowRepository) CountFlowsUsingTemplate(ctx context.Context, templateID string) (int64, error) {
	var count int64
	err := TenantDB(r.db, ctx).Model(&model.HealingFlow{}).
		Where("nodes @> ?", `[{"config": {"template_id": "`+templateID+`"}}]`).
		Count(&count).Error
	return count, err
}

// GetStats 获取自愈流程统计信息
func (r *HealingFlowRepository) GetStats(ctx context.Context) (map[string]interface{}, error) {
	stats := make(map[string]interface{})

	// 总数
	var total int64
	// 每次查询使用新的 TenantDB 实例，避免 GORM session WHERE 条件累积
	newDB := func() *gorm.DB { return TenantDB(r.db, ctx) }
	if err := newDB().Model(&model.HealingFlow{}).Count(&total).Error; err != nil {
		return nil, err
	}
	stats["total"] = total

	// 启用/禁用统计
	var activeCount int64
	newDB().Model(&model.HealingFlow{}).
		Where("is_active = ?", true).
		Count(&activeCount)
	stats["active_count"] = activeCount
	stats["inactive_count"] = total - activeCount

	return stats, nil
}

// HealingRuleRepository 自愈规则仓库
type HealingRuleRepository struct {
	db *gorm.DB
}

// NewHealingRuleRepository 创建自愈规则仓库
func NewHealingRuleRepository() *HealingRuleRepository {
	return &HealingRuleRepository{db: database.DB}
}

// Create 创建自愈规则
func (r *HealingRuleRepository) Create(ctx context.Context, rule *model.HealingRule) error {
	if rule.TenantID == nil {
		tenantID := TenantIDFromContext(ctx)
		rule.TenantID = &tenantID
	}
	return r.db.WithContext(ctx).Create(rule).Error
}

// GetByID 根据 ID 获取自愈规则
func (r *HealingRuleRepository) GetByID(ctx context.Context, id uuid.UUID) (*model.HealingRule, error) {
	var rule model.HealingRule
	err := TenantDB(r.db, ctx).Preload("Flow").Preload("Creator").First(&rule, "id = ?", id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrHealingRuleNotFound
	}
	return &rule, err
}

// Update 更新自愈规则
func (r *HealingRuleRepository) Update(ctx context.Context, rule *model.HealingRule) error {
	// 使用 Select 明确指定要更新的列，避免 GORM 忽略外键字段
	return TenantDB(r.db, ctx).
		Model(rule).
		Select("name", "description", "flow_id", "trigger_mode", "conditions", "match_mode", "priority", "is_active", "updated_at").
		Updates(rule).Error
}

// Delete 删除自愈规则
// force=false: 检查是否有关联的流程实例，有则拒绝删除
// force=true: 自动解除关联后删除
func (r *HealingRuleRepository) Delete(ctx context.Context, id uuid.UUID, force bool) error {
	// 检查是否有关联的流程实例
	var count int64
	if err := TenantDB(r.db, ctx).Model(&model.FlowInstance{}).Where("rule_id = ?", id).Count(&count).Error; err != nil {
		return err
	}

	if count > 0 && !force {
		return errors.New("规则存在关联的执行记录，请使用 force=true 强制删除")
	}

	return TenantDB(r.db, ctx).Transaction(func(tx *gorm.DB) error {
		// 如果有关联且 force=true，先解除关联
		if count > 0 {
			if err := tx.Model(&model.FlowInstance{}).Where("rule_id = ?", id).Update("rule_id", nil).Error; err != nil {
				return err
			}
		}

		// 删除规则
		result := tx.Delete(&model.HealingRule{}, "id = ?", id)
		if result.RowsAffected == 0 {
			return ErrHealingRuleNotFound
		}
		return result.Error
	})
}

// List 获取自愈规则列表
// 支持 triggerMode, priority, matchMode, hasFlow, createdFrom/To 过滤、sortBy/sortOrder 排序（含 conditions_count）
func (r *HealingRuleRepository) List(ctx context.Context, page, pageSize int, isActive *bool, flowID *uuid.UUID, search query.StringFilter, triggerMode, sortBy, sortOrder string, priority *int, matchMode string, hasFlow *bool, createdFrom, createdTo string, scopes ...func(*gorm.DB) *gorm.DB) ([]model.HealingRule, int64, error) {
	var rules []model.HealingRule
	var total int64

	q := TenantDB(r.db, ctx).Model(&model.HealingRule{})

	if isActive != nil {
		q = q.Where("is_active = ?", *isActive)
	}
	if flowID != nil {
		q = q.Where("flow_id = ?", *flowID)
	}
	if !search.IsEmpty() {
		q = query.ApplyMultiStringFilter(q, []string{"name", "description"}, search)
	}
	for _, scope := range scopes {
		q = scope(q)
	}
	if triggerMode != "" {
		q = q.Where("trigger_mode = ?", triggerMode)
	}
	if priority != nil {
		q = q.Where("priority = ?", *priority)
	}
	if matchMode != "" {
		q = q.Where("match_mode = ?", matchMode)
	}
	if hasFlow != nil {
		if *hasFlow {
			q = q.Where("flow_id IS NOT NULL")
		} else {
			q = q.Where("flow_id IS NULL")
		}
	}
	if createdFrom != "" {
		q = q.Where("created_at >= ?", createdFrom)
	}
	if createdTo != "" {
		q = q.Where("created_at <= ?", createdTo)
	}

	if err := q.Session(&gorm.Session{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}

	// 排序方向校验
	if sortOrder != "asc" && sortOrder != "desc" {
		sortOrder = "desc"
	}

	// 排序字段白名单
	var orderClause string
	switch sortBy {
	case "priority":
		orderClause = "priority " + sortOrder
	case "created_at":
		orderClause = "created_at " + sortOrder
	case "updated_at":
		orderClause = "updated_at " + sortOrder
	case "name":
		orderClause = "name " + sortOrder
	case "conditions_count":
		orderClause = "jsonb_array_length(conditions) " + sortOrder
	default:
		orderClause = "priority DESC, created_at DESC"
	}

	offset := (page - 1) * pageSize
	err := q.Preload("Flow").Preload("Creator").Offset(offset).Limit(pageSize).Order(orderClause).Find(&rules).Error
	return rules, total, err
}

// ListActiveByPriority 获取所有启用的规则，按优先级排序
func (r *HealingRuleRepository) ListActiveByPriority(ctx context.Context) ([]model.HealingRule, error) {
	var rules []model.HealingRule
	err := TenantDB(r.db, ctx).
		Where("is_active = ?", true).
		Preload("Flow").
		Order("priority DESC").
		Find(&rules).Error
	return rules, err
}

// UpdateLastRunAt 更新规则最后运行时间
func (r *HealingRuleRepository) UpdateLastRunAt(ctx context.Context, id uuid.UUID) error {
	return TenantDB(r.db, ctx).Model(&model.HealingRule{}).
		Where("id = ?", id).
		Update("last_run_at", gorm.Expr("NOW()")).Error
}

// Activate 启用规则
func (r *HealingRuleRepository) Activate(ctx context.Context, id uuid.UUID) error {
	return TenantDB(r.db, ctx).Model(&model.HealingRule{}).
		Where("id = ?", id).
		Update("is_active", true).Error
}

// Deactivate 停用规则
func (r *HealingRuleRepository) Deactivate(ctx context.Context, id uuid.UUID) error {
	return TenantDB(r.db, ctx).Model(&model.HealingRule{}).
		Where("id = ?", id).
		Update("is_active", false).Error
}

// GetStats 获取自愈规则统计信息
func (r *HealingRuleRepository) GetStats(ctx context.Context) (map[string]interface{}, error) {
	stats := make(map[string]interface{})

	// 总数
	var total int64
	// 每次查询使用新的 TenantDB 实例，避免 GORM session WHERE 条件累积
	newDB := func() *gorm.DB { return TenantDB(r.db, ctx) }
	if err := newDB().Model(&model.HealingRule{}).Count(&total).Error; err != nil {
		return nil, err
	}
	stats["total"] = total

	// 启用/禁用统计
	var activeCount int64
	newDB().Model(&model.HealingRule{}).
		Where("is_active = ?", true).
		Count(&activeCount)
	stats["active_count"] = activeCount
	stats["inactive_count"] = total - activeCount

	// 按触发模式统计
	type TriggerModeCount struct {
		TriggerMode string `json:"trigger_mode"`
		Count       int64  `json:"count"`
	}
	var triggerModeCounts []TriggerModeCount
	newDB().Model(&model.HealingRule{}).
		Select("trigger_mode, count(*) as count").
		Group("trigger_mode").
		Scan(&triggerModeCounts)
	stats["by_trigger_mode"] = triggerModeCounts

	return stats, nil
}

// FlowInstanceRepository 流程实例仓库
type FlowInstanceRepository struct {
	db *gorm.DB
}

// NewFlowInstanceRepository 创建流程实例仓库
func NewFlowInstanceRepository() *FlowInstanceRepository {
	return &FlowInstanceRepository{db: database.DB}
}

// Create 创建流程实例
func (r *FlowInstanceRepository) Create(ctx context.Context, instance *model.FlowInstance) error {
	if instance.TenantID == nil {
		tenantID := TenantIDFromContext(ctx)
		instance.TenantID = &tenantID
	}
	return r.db.WithContext(ctx).Create(instance).Error
}

// GetByID 根据 ID 获取流程实例
// 如果实例的 flow_nodes/flow_edges 快照为空，自动从关联的流程定义中回填
func (r *FlowInstanceRepository) GetByID(ctx context.Context, id uuid.UUID) (*model.FlowInstance, error) {
	var instance model.FlowInstance
	err := TenantDB(r.db, ctx).
		Preload("Rule").
		Preload("Incident").
		First(&instance, "id = ?", id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrFlowInstanceNotFound
	}
	if err != nil {
		return nil, err
	}

	// 回填：如果快照的 flow_nodes/flow_edges 为空，从关联的流程定义中获取
	if len(instance.FlowNodes) == 0 || len(instance.FlowEdges) == 0 {
		var flow model.HealingFlow
		if fetchErr := r.db.WithContext(ctx).First(&flow, "id = ?", instance.FlowID).Error; fetchErr == nil {
			if len(instance.FlowNodes) == 0 {
				instance.FlowNodes = flow.Nodes
			}
			if len(instance.FlowEdges) == 0 {
				instance.FlowEdges = flow.Edges
			}
			if instance.FlowName == "" {
				instance.FlowName = flow.Name
			}
		}
	}

	return &instance, nil
}

// Update 更新流程实例
func (r *FlowInstanceRepository) Update(ctx context.Context, instance *model.FlowInstance) error {
	return r.db.WithContext(ctx).Save(instance).Error
}

// UpdateStatus 更新流程实例状态
func (r *FlowInstanceRepository) UpdateStatus(ctx context.Context, id uuid.UUID, status string, errorMsg string) error {
	updates := map[string]interface{}{
		"status": status,
	}
	if errorMsg != "" {
		updates["error_message"] = errorMsg
	}
	if status == model.FlowInstanceStatusCompleted || status == model.FlowInstanceStatusFailed || status == model.FlowInstanceStatusCancelled {
		updates["completed_at"] = gorm.Expr("NOW()")
	}
	return TenantDB(r.db, ctx).Model(&model.FlowInstance{}).Where("id = ?", id).Updates(updates).Error
}

// UpdateNodeStates 更新节点状态
func (r *FlowInstanceRepository) UpdateNodeStates(ctx context.Context, id uuid.UUID, nodeStates model.JSON) error {
	return TenantDB(r.db, ctx).Model(&model.FlowInstance{}).Where("id = ?", id).Update("node_states", nodeStates).Error
}

// ListStaleRunning 查询所有停滞的 running/pending 实例（跨租户）
// 用于服务启动时恢复因进程终止而遗留的孤儿实例
func (r *FlowInstanceRepository) ListStaleRunning(ctx context.Context, staleThreshold time.Duration) ([]model.FlowInstance, error) {
	var instances []model.FlowInstance
	cutoff := time.Now().Add(-staleThreshold)
	err := r.db.WithContext(ctx).
		Where("status IN ?", []string{model.FlowInstanceStatusRunning, model.FlowInstanceStatusPending}).
		Where("updated_at < ?", cutoff).
		Find(&instances).Error
	return instances, err
}

// UpdateCurrentNodeAndStates 更新当前节点和节点状态
func (r *FlowInstanceRepository) UpdateCurrentNodeAndStates(ctx context.Context, id uuid.UUID, currentNodeID string, nodeStates model.JSON) error {
	return TenantDB(r.db, ctx).Model(&model.FlowInstance{}).Where("id = ?", id).Updates(map[string]interface{}{
		"current_node_id": currentNodeID,
		"node_states":     nodeStates,
	}).Error
}

// List 获取流程实例列表
func (r *FlowInstanceRepository) List(ctx context.Context, page, pageSize int, flowID, ruleID *uuid.UUID, incidentID *uuid.UUID, status string, search string) ([]model.FlowInstance, int64, error) {
	var instances []model.FlowInstance
	var total int64

	query := r.db.WithContext(ctx).Model(&model.FlowInstance{}).
		Where("flow_instances.tenant_id = ?", TenantIDFromContext(ctx))

	if flowID != nil {
		query = query.Where("flow_instances.flow_id = ?", *flowID)
	}
	if ruleID != nil {
		query = query.Where("flow_instances.rule_id = ?", *ruleID)
	}
	if incidentID != nil {
		query = query.Where("flow_instances.incident_id = ?", *incidentID)
	}
	if status != "" {
		query = query.Where("flow_instances.status = ?", status)
	}
	if search != "" {
		pattern := "%" + search + "%"
		query = query.
			Joins("LEFT JOIN healing_flows ON healing_flows.id = flow_instances.flow_id").
			Joins("LEFT JOIN healing_rules ON healing_rules.id = flow_instances.rule_id").
			Joins("LEFT JOIN incidents ON incidents.id = flow_instances.incident_id").
			Where("(flow_instances.id::text ILIKE ? OR healing_flows.name ILIKE ? OR healing_rules.name ILIKE ? OR incidents.title ILIKE ?)",
				pattern, pattern, pattern, pattern)
	}

	if err := query.Session(&gorm.Session{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * pageSize
	err := query.
		Preload("Rule").
		Preload("Incident").
		Offset(offset).Limit(pageSize).Order("flow_instances.created_at DESC").Find(&instances).Error
	return instances, total, err
}

// FlowInstanceListOptions 实例列表查询选项
type FlowInstanceListOptions struct {
	// 分页
	Page     int
	PageSize int

	// 搜索
	Search query.StringFilter // 全文模糊/精确搜索（流程名/规则名/工单标题/实例ID）

	// 排序
	SortBy    string // created_at, started_at, completed_at, status, flow_name, rule_name
	SortOrder string // asc / desc

	// 精确/模糊过滤
	Status         string             // 状态精确匹配
	FlowID         *uuid.UUID         // 流程 ID 精确
	FlowName       query.StringFilter // 流程名称
	RuleID         *uuid.UUID         // 规则 ID 精确
	RuleName       query.StringFilter // 规则名称
	IncidentID     *uuid.UUID         // 工单 ID 精确
	IncidentTitle  query.StringFilter // 工单标题
	CurrentNodeID  string             // 当前节点 ID
	ErrorMessage   query.StringFilter // 错误信息
	HasError       *bool              // 是否有错误
	ApprovalStatus string             // 包含对应审批结果的节点（approved 或 rejected）

	// 时间范围
	CreatedFrom   *time.Time
	CreatedTo     *time.Time
	StartedFrom   *time.Time
	StartedTo     *time.Time
	CompletedFrom *time.Time
	CompletedTo   *time.Time

	// 数量范围
	MinNodes       *int
	MaxNodes       *int
	MinFailedNodes *int
	MaxFailedNodes *int
}

// FlowInstanceSummary 列表接口的精简 DTO
type FlowInstanceSummary struct {
	ID                uuid.UUID  `json:"id"`
	Status            string     `json:"status"`
	FlowID            uuid.UUID  `json:"flow_id"`
	FlowName          string     `json:"flow_name"`
	RuleID            *uuid.UUID `json:"rule_id,omitempty"`
	RuleName          *string    `json:"rule_name,omitempty"`
	IncidentID        *uuid.UUID `json:"incident_id,omitempty"`
	IncidentTitle     *string    `json:"incident_title,omitempty"`
	CurrentNodeID     string     `json:"current_node_id,omitempty"`
	ErrorMessage      string     `json:"error_message,omitempty"`
	NodeCount         int        `json:"node_count"`
	FailedNodeCount   int        `json:"failed_node_count"`
	RejectedNodeCount int        `json:"rejected_node_count"`
	StartedAt         *time.Time `json:"started_at,omitempty"`
	CompletedAt       *time.Time `json:"completed_at,omitempty"`
	CreatedAt         time.Time  `json:"created_at"`
}

// ListSummary 获取流程实例精简列表（瘦身版）
func (r *FlowInstanceRepository) ListSummary(ctx context.Context, page, pageSize int, flowID, ruleID *uuid.UUID, incidentID *uuid.UUID, status string, search string) ([]FlowInstanceSummary, int64, error) {
	var total int64

	// === 构建 count 查询 ===
	tenantID := TenantIDFromContext(ctx)
	countQuery := r.db.WithContext(ctx).Where("flow_instances.tenant_id = ?", tenantID).Model(&model.FlowInstance{})
	if flowID != nil {
		countQuery = countQuery.Where("flow_instances.flow_id = ?", *flowID)
	}
	if ruleID != nil {
		countQuery = countQuery.Where("flow_instances.rule_id = ?", *ruleID)
	}
	if incidentID != nil {
		countQuery = countQuery.Where("flow_instances.incident_id = ?", *incidentID)
	}
	if status != "" {
		countQuery = countQuery.Where("flow_instances.status = ?", status)
	}
	if search != "" {
		pattern := "%" + search + "%"
		countQuery = countQuery.
			Joins("LEFT JOIN healing_flows ON healing_flows.id = flow_instances.flow_id").
			Joins("LEFT JOIN healing_rules ON healing_rules.id = flow_instances.rule_id").
			Joins("LEFT JOIN incidents ON incidents.id = flow_instances.incident_id").
			Where("(flow_instances.id::text ILIKE ? OR healing_flows.name ILIKE ? OR healing_rules.name ILIKE ? OR incidents.title ILIKE ?)",
				pattern, pattern, pattern, pattern)
	}
	if err := countQuery.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	// === 构建数据查询 ===
	offset := (page - 1) * pageSize
	var results []FlowInstanceSummary

	dataQuery := r.db.WithContext(ctx).Where("flow_instances.tenant_id = ?", tenantID).
		Table("flow_instances").
		Select(`
			flow_instances.id,
			flow_instances.status,
			flow_instances.flow_id,
			COALESCE(flow_instances.flow_name, healing_flows.name, '') AS flow_name,
			flow_instances.rule_id,
			healing_rules.name AS rule_name,
			flow_instances.incident_id,
			incidents.title AS incident_title,
			flow_instances.current_node_id,
			flow_instances.error_message,
			COALESCE(jsonb_array_length(healing_flows.nodes), 0) AS node_count,
			(SELECT COUNT(*) FROM jsonb_each(flow_instances.node_states) AS ns(key, value) WHERE ns.value->>'status' IN ('failed', 'error')) AS failed_node_count,
			(SELECT COUNT(*) FROM jsonb_each(flow_instances.node_states) AS ns(key, value) WHERE ns.value->>'status' = 'rejected') AS rejected_node_count,
			flow_instances.started_at,
			flow_instances.completed_at,
			flow_instances.created_at
		`).
		Joins("LEFT JOIN healing_flows ON healing_flows.id = flow_instances.flow_id").
		Joins("LEFT JOIN healing_rules ON healing_rules.id = flow_instances.rule_id").
		Joins("LEFT JOIN incidents ON incidents.id = flow_instances.incident_id")

	if flowID != nil {
		dataQuery = dataQuery.Where("flow_instances.flow_id = ?", *flowID)
	}
	if ruleID != nil {
		dataQuery = dataQuery.Where("flow_instances.rule_id = ?", *ruleID)
	}
	if incidentID != nil {
		dataQuery = dataQuery.Where("flow_instances.incident_id = ?", *incidentID)
	}
	if status != "" {
		dataQuery = dataQuery.Where("flow_instances.status = ?", status)
	}
	if search != "" {
		pattern := "%" + search + "%"
		dataQuery = dataQuery.Where("(flow_instances.id::text ILIKE ? OR healing_flows.name ILIKE ? OR healing_rules.name ILIKE ? OR incidents.title ILIKE ?)",
			pattern, pattern, pattern, pattern)
	}

	err := dataQuery.
		Offset(offset).Limit(pageSize).
		Order("flow_instances.created_at DESC").
		Scan(&results).Error

	return results, total, err
}

// ListSummaryWithOptions 增强版列表查询，支持排序/过滤/时间范围/数量范围
func (r *FlowInstanceRepository) ListSummaryWithOptions(ctx context.Context, opts FlowInstanceListOptions) ([]FlowInstanceSummary, int64, error) {
	// ===== 构建通用条件 =====
	applyFilters := func(q *gorm.DB, needJoins bool) *gorm.DB {
		if needJoins {
			q = q.
				Joins("LEFT JOIN healing_flows ON healing_flows.id = flow_instances.flow_id").
				Joins("LEFT JOIN healing_rules ON healing_rules.id = flow_instances.rule_id").
				Joins("LEFT JOIN incidents ON incidents.id = flow_instances.incident_id")
		}

		// 状态
		if opts.Status != "" {
			q = q.Where("flow_instances.status = ?", opts.Status)
		}
		// 精确 ID
		if opts.FlowID != nil {
			q = q.Where("flow_instances.flow_id = ?", *opts.FlowID)
		}
		if opts.RuleID != nil {
			q = q.Where("flow_instances.rule_id = ?", *opts.RuleID)
		}
		if opts.IncidentID != nil {
			q = q.Where("flow_instances.incident_id = ?", *opts.IncidentID)
		}
		// 模糊/精确搜索
		if !opts.FlowName.IsEmpty() {
			q = query.ApplyStringFilter(q, "COALESCE(flow_instances.flow_name, healing_flows.name, '')", opts.FlowName)
		}
		if !opts.RuleName.IsEmpty() {
			q = query.ApplyStringFilter(q, "healing_rules.name", opts.RuleName)
		}
		if !opts.IncidentTitle.IsEmpty() {
			q = query.ApplyStringFilter(q, "incidents.title", opts.IncidentTitle)
		}
		if opts.CurrentNodeID != "" {
			q = q.Where("flow_instances.current_node_id = ?", opts.CurrentNodeID)
		}
		if !opts.ErrorMessage.IsEmpty() {
			q = query.ApplyStringFilter(q, "flow_instances.error_message", opts.ErrorMessage)
		}
		if opts.HasError != nil {
			if *opts.HasError {
				q = q.Where("flow_instances.error_message IS NOT NULL AND flow_instances.error_message != ''")
			} else {
				q = q.Where("(flow_instances.error_message IS NULL OR flow_instances.error_message = '')")
			}
		}
		if opts.ApprovalStatus != "" {
			if opts.ApprovalStatus == "approved" {
				q = q.Where("EXISTS (SELECT 1 FROM jsonb_each(flow_instances.node_states) AS ns(key, value) WHERE ns.value->>'status' = 'approved')")
			} else if opts.ApprovalStatus == "rejected" {
				q = q.Where("EXISTS (SELECT 1 FROM jsonb_each(flow_instances.node_states) AS ns(key, value) WHERE ns.value->>'status' = 'rejected')")
			}
		}
		// 全文搜索
		if !opts.Search.IsEmpty() {
			q = query.ApplyMultiStringFilter(q, []string{"flow_instances.id::text", "COALESCE(flow_instances.flow_name, healing_flows.name, '')", "healing_rules.name", "incidents.title"}, opts.Search)
		}
		// 时间范围
		if opts.CreatedFrom != nil {
			q = q.Where("flow_instances.created_at >= ?", *opts.CreatedFrom)
		}
		if opts.CreatedTo != nil {
			q = q.Where("flow_instances.created_at <= ?", *opts.CreatedTo)
		}
		if opts.StartedFrom != nil {
			q = q.Where("flow_instances.started_at >= ?", *opts.StartedFrom)
		}
		if opts.StartedTo != nil {
			q = q.Where("flow_instances.started_at <= ?", *opts.StartedTo)
		}
		if opts.CompletedFrom != nil {
			q = q.Where("flow_instances.completed_at >= ?", *opts.CompletedFrom)
		}
		if opts.CompletedTo != nil {
			q = q.Where("flow_instances.completed_at <= ?", *opts.CompletedTo)
		}
		return q
	}

	// ===== COUNT 查询 =====
	var total int64
	tenantID := TenantIDFromContext(ctx)
	countQuery := r.db.WithContext(ctx).Where("flow_instances.tenant_id = ?", tenantID).Model(&model.FlowInstance{})
	// count 查询在存在 name 模糊搜索或全文搜索时才需要 join
	needJoins := !opts.FlowName.IsEmpty() || !opts.RuleName.IsEmpty() || !opts.IncidentTitle.IsEmpty() || !opts.Search.IsEmpty()
	countQuery = applyFilters(countQuery, needJoins)

	// 数量范围需要子查询过滤
	if opts.MinNodes != nil || opts.MaxNodes != nil || opts.MinFailedNodes != nil || opts.MaxFailedNodes != nil {
		// 使用子查询包装，因为 node_count/failed_node_count 是计算列
		subQuery := r.db.WithContext(ctx).Where("flow_instances.tenant_id = ?", tenantID).
			Table("flow_instances").
			Select("flow_instances.id").
			Joins("LEFT JOIN healing_flows ON healing_flows.id = flow_instances.flow_id")
		subQuery = applyFilters(subQuery, needJoins)

		if opts.MinNodes != nil {
			subQuery = subQuery.Where("COALESCE(jsonb_array_length(healing_flows.nodes), 0) >= ?", *opts.MinNodes)
		}
		if opts.MaxNodes != nil {
			subQuery = subQuery.Where("COALESCE(jsonb_array_length(healing_flows.nodes), 0) <= ?", *opts.MaxNodes)
		}
		if opts.MinFailedNodes != nil {
			subQuery = subQuery.Where("(SELECT COUNT(*) FROM jsonb_each(flow_instances.node_states) AS ns(key, value) WHERE ns.value->>'status' IN ('failed', 'error')) >= ?", *opts.MinFailedNodes)
		}
		if opts.MaxFailedNodes != nil {
			subQuery = subQuery.Where("(SELECT COUNT(*) FROM jsonb_each(flow_instances.node_states) AS ns(key, value) WHERE ns.value->>'status' IN ('failed', 'error')) <= ?", *opts.MaxFailedNodes)
		}

		countQuery = r.db.WithContext(ctx).Where("flow_instances.tenant_id = ?", tenantID).Model(&model.FlowInstance{}).
			Where("flow_instances.id IN (?)", subQuery)
	}

	if err := countQuery.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	// ===== DATA 查询 =====
	offset := (opts.Page - 1) * opts.PageSize
	var results []FlowInstanceSummary

	dataQuery := r.db.WithContext(ctx).Where("flow_instances.tenant_id = ?", tenantID).
		Table("flow_instances").
		Select(`
			flow_instances.id,
			flow_instances.status,
			flow_instances.flow_id,
			COALESCE(flow_instances.flow_name, healing_flows.name, '') AS flow_name,
			flow_instances.rule_id,
			healing_rules.name AS rule_name,
			flow_instances.incident_id,
			incidents.title AS incident_title,
			flow_instances.current_node_id,
			flow_instances.error_message,
			COALESCE(jsonb_array_length(healing_flows.nodes), 0) AS node_count,
			(SELECT COUNT(*) FROM jsonb_each(flow_instances.node_states) AS ns(key, value) WHERE ns.value->>'status' IN ('failed', 'error')) AS failed_node_count,
			(SELECT COUNT(*) FROM jsonb_each(flow_instances.node_states) AS ns(key, value) WHERE ns.value->>'status' = 'rejected') AS rejected_node_count,
			flow_instances.started_at,
			flow_instances.completed_at,
			flow_instances.created_at
		`).
		Joins("LEFT JOIN healing_flows ON healing_flows.id = flow_instances.flow_id").
		Joins("LEFT JOIN healing_rules ON healing_rules.id = flow_instances.rule_id").
		Joins("LEFT JOIN incidents ON incidents.id = flow_instances.incident_id")

	dataQuery = applyFilters(dataQuery, false) // joins 已在上面加了

	// 数量范围过滤
	if opts.MinNodes != nil {
		dataQuery = dataQuery.Where("COALESCE(jsonb_array_length(healing_flows.nodes), 0) >= ?", *opts.MinNodes)
	}
	if opts.MaxNodes != nil {
		dataQuery = dataQuery.Where("COALESCE(jsonb_array_length(healing_flows.nodes), 0) <= ?", *opts.MaxNodes)
	}
	if opts.MinFailedNodes != nil {
		dataQuery = dataQuery.Where("(SELECT COUNT(*) FROM jsonb_each(flow_instances.node_states) AS ns(key, value) WHERE ns.value->>'status' IN ('failed', 'error')) >= ?", *opts.MinFailedNodes)
	}
	if opts.MaxFailedNodes != nil {
		dataQuery = dataQuery.Where("(SELECT COUNT(*) FROM jsonb_each(flow_instances.node_states) AS ns(key, value) WHERE ns.value->>'status' IN ('failed', 'error')) <= ?", *opts.MaxFailedNodes)
	}

	// 排序
	orderClause := "flow_instances.created_at DESC" // 默认
	if opts.SortBy != "" {
		// 白名单校验
		sortColumnMap := map[string]string{
			"created_at":   "flow_instances.created_at",
			"started_at":   "flow_instances.started_at",
			"completed_at": "flow_instances.completed_at",
			"status":       "flow_instances.status",
			"flow_name":    "flow_name",
			"rule_name":    "rule_name",
		}
		if col, ok := sortColumnMap[opts.SortBy]; ok {
			direction := "DESC"
			if opts.SortOrder == "asc" {
				direction = "ASC"
			}
			orderClause = col + " " + direction
		}
	}

	err := dataQuery.
		Offset(offset).Limit(opts.PageSize).
		Order(orderClause).
		Scan(&results).Error

	return results, total, err
}

// GetStats 获取流程实例统计信息
func (r *FlowInstanceRepository) GetStats(ctx context.Context) (map[string]interface{}, error) {
	stats := make(map[string]interface{})

	// 总数
	var total int64
	// 每次查询使用新的 TenantDB 实例，避免 GORM session WHERE 条件累积
	newDB := func() *gorm.DB { return TenantDB(r.db, ctx) }
	if err := newDB().Model(&model.FlowInstance{}).Count(&total).Error; err != nil {
		return nil, err
	}
	stats["total"] = total

	// 按状态统计
	type StatusCount struct {
		Status string `json:"status"`
		Count  int64  `json:"count"`
	}
	var statusCounts []StatusCount
	newDB().Model(&model.FlowInstance{}).
		Select("status, count(*) as count").
		Group("status").
		Scan(&statusCounts)
	stats["by_status"] = statusCounts

	return stats, nil
}

// ApprovalTaskRepository 审批任务仓库
type ApprovalTaskRepository struct {
	db *gorm.DB
}

// NewApprovalTaskRepository 创建审批任务仓库
func NewApprovalTaskRepository() *ApprovalTaskRepository {
	return &ApprovalTaskRepository{db: database.DB}
}

// Create 创建审批任务
func (r *ApprovalTaskRepository) Create(ctx context.Context, task *model.ApprovalTask) error {
	if task.TenantID == nil {
		tenantID := TenantIDFromContext(ctx)
		task.TenantID = &tenantID
	}
	return r.db.WithContext(ctx).Create(task).Error
}

// GetByID 根据 ID 获取审批任务
func (r *ApprovalTaskRepository) GetByID(ctx context.Context, id uuid.UUID) (*model.ApprovalTask, error) {
	var task model.ApprovalTask
	err := TenantDB(r.db, ctx).
		Preload("FlowInstance").
		Preload("FlowInstance.Incident").
		Preload("Initiator").
		Preload("Decider").
		First(&task, "id = ?", id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrApprovalTaskNotFound
	}
	return &task, err
}

// Update 更新审批任务
func (r *ApprovalTaskRepository) Update(ctx context.Context, task *model.ApprovalTask) error {
	return r.db.WithContext(ctx).Save(task).Error
}

// Approve 批准审批任务
func (r *ApprovalTaskRepository) Approve(ctx context.Context, id uuid.UUID, decidedBy uuid.UUID, comment string) error {
	return TenantDB(r.db, ctx).Model(&model.ApprovalTask{}).
		Where("id = ? AND status = ?", id, model.ApprovalTaskStatusPending).
		Updates(map[string]interface{}{
			"status":           model.ApprovalTaskStatusApproved,
			"decided_by":       decidedBy,
			"decided_at":       gorm.Expr("NOW()"),
			"decision_comment": comment,
		}).Error
}

// Reject 拒绝审批任务
func (r *ApprovalTaskRepository) Reject(ctx context.Context, id uuid.UUID, decidedBy uuid.UUID, comment string) error {
	return TenantDB(r.db, ctx).Model(&model.ApprovalTask{}).
		Where("id = ? AND status = ?", id, model.ApprovalTaskStatusPending).
		Updates(map[string]interface{}{
			"status":           model.ApprovalTaskStatusRejected,
			"decided_by":       decidedBy,
			"decided_at":       gorm.Expr("NOW()"),
			"decision_comment": comment,
		}).Error
}

// ExpireTimedOut 将超时的审批任务标记为过期
func (r *ApprovalTaskRepository) ExpireTimedOut(ctx context.Context) (int64, error) {
	result := TenantDB(r.db, ctx).Model(&model.ApprovalTask{}).
		Where("status = ? AND timeout_at < NOW()", model.ApprovalTaskStatusPending).
		Updates(map[string]interface{}{
			"status":     model.ApprovalTaskStatusExpired,
			"decided_at": gorm.Expr("NOW()"),
		})
	return result.RowsAffected, result.Error
}

// ListPending 获取待审批列表
// 支持搜索和过滤：nodeName（模糊匹配 node_id, flow_instance_id）、dateFrom、dateTo
func (r *ApprovalTaskRepository) ListPending(ctx context.Context, page, pageSize int, nodeName, dateFrom, dateTo string) ([]model.ApprovalTask, int64, error) {
	var tasks []model.ApprovalTask
	var total int64

	query := TenantDB(r.db, ctx).Model(&model.ApprovalTask{}).Where("status = ?", model.ApprovalTaskStatusPending)

	// 模糊搜索：node_id 或 flow_instance_id
	if nodeName != "" {
		searchPattern := "%" + nodeName + "%"
		query = query.Where("(node_id ILIKE ? OR flow_instance_id::text ILIKE ?)", searchPattern, searchPattern)
	}

	// 日期范围过滤
	if dateFrom != "" {
		query = query.Where("created_at >= ?", dateFrom+" 00:00:00")
	}
	if dateTo != "" {
		query = query.Where("created_at <= ?", dateTo+" 23:59:59")
	}

	if err := query.Session(&gorm.Session{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * pageSize
	err := query.
		Preload("FlowInstance").
		Preload("FlowInstance.Incident").
		Preload("Initiator").
		Offset(offset).Limit(pageSize).Order("created_at DESC").Find(&tasks).Error
	return tasks, total, err
}

// List 获取审批任务列表
func (r *ApprovalTaskRepository) List(ctx context.Context, page, pageSize int, flowInstanceID *uuid.UUID, status string) ([]model.ApprovalTask, int64, error) {
	var tasks []model.ApprovalTask
	var total int64

	query := TenantDB(r.db, ctx).Model(&model.ApprovalTask{})

	if flowInstanceID != nil {
		query = query.Where("flow_instance_id = ?", *flowInstanceID)
	}
	if status != "" {
		query = query.Where("status = ?", status)
	}

	if err := query.Session(&gorm.Session{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * pageSize
	err := query.
		Preload("FlowInstance").
		Preload("Initiator").
		Preload("Decider").
		Offset(offset).Limit(pageSize).Order("created_at DESC").Find(&tasks).Error
	return tasks, total, err
}

// GetByFlowInstanceAndNode 根据流程实例ID和节点ID获取审批任务
func (r *ApprovalTaskRepository) GetByFlowInstanceAndNode(ctx context.Context, flowInstanceID uuid.UUID, nodeID string) (*model.ApprovalTask, error) {
	var task model.ApprovalTask
	err := TenantDB(r.db, ctx).
		Where("flow_instance_id = ? AND node_id = ?", flowInstanceID, nodeID).
		First(&task).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrApprovalTaskNotFound
	}
	return &task, err
}

// =============================================================================
// FlowLogRepository - 流程执行日志仓库
// =============================================================================

// FlowLogRepository 流程执行日志仓库
type FlowLogRepository struct {
	db *gorm.DB
}

// NewFlowLogRepository 创建流程执行日志仓库
func NewFlowLogRepository() *FlowLogRepository {
	return &FlowLogRepository{db: database.DB}
}

// Create 创建日志
func (r *FlowLogRepository) Create(ctx context.Context, log *model.FlowExecutionLog) error {
	if log.TenantID == nil {
		tenantID := TenantIDFromContext(ctx)
		log.TenantID = &tenantID
	}
	return r.db.WithContext(ctx).Create(log).Error
}

// CreateBatch 批量创建日志
func (r *FlowLogRepository) CreateBatch(ctx context.Context, logs []*model.FlowExecutionLog) error {
	tenantID := TenantIDFromContext(ctx)
	for _, log := range logs {
		if log.TenantID == nil {
			log.TenantID = &tenantID
		}
	}
	return r.db.WithContext(ctx).Create(&logs).Error
}

// GetByInstanceID 根据流程实例ID获取所有日志
func (r *FlowLogRepository) GetByInstanceID(ctx context.Context, instanceID uuid.UUID) ([]*model.FlowExecutionLog, error) {
	var logs []*model.FlowExecutionLog
	err := TenantDB(r.db, ctx).
		Where("flow_instance_id = ?", instanceID).
		Order("created_at ASC").
		Find(&logs).Error
	return logs, err
}

// GetByInstanceAndNode 根据流程实例ID和节点ID获取日志
func (r *FlowLogRepository) GetByInstanceAndNode(ctx context.Context, instanceID uuid.UUID, nodeID string) ([]*model.FlowExecutionLog, error) {
	var logs []*model.FlowExecutionLog
	err := TenantDB(r.db, ctx).
		Where("flow_instance_id = ? AND node_id = ?", instanceID, nodeID).
		Order("created_at ASC").
		Find(&logs).Error
	return logs, err
}

// GetByLevel 根据日志级别获取日志
func (r *FlowLogRepository) GetByLevel(ctx context.Context, instanceID uuid.UUID, level string) ([]*model.FlowExecutionLog, error) {
	var logs []*model.FlowExecutionLog
	err := TenantDB(r.db, ctx).
		Where("flow_instance_id = ? AND level = ?", instanceID, level).
		Order("created_at ASC").
		Find(&logs).Error
	return logs, err
}

// DeleteByInstanceID 删除流程实例的所有日志
func (r *FlowLogRepository) DeleteByInstanceID(ctx context.Context, instanceID uuid.UUID) error {
	return TenantDB(r.db, ctx).
		Where("flow_instance_id = ?", instanceID).
		Delete(&model.FlowExecutionLog{}).Error
}

// ListPaginated 分页获取日志
func (r *FlowLogRepository) ListPaginated(ctx context.Context, instanceID uuid.UUID, page, pageSize int) ([]*model.FlowExecutionLog, int64, error) {
	var logs []*model.FlowExecutionLog
	var total int64

	query := TenantDB(r.db, ctx).Model(&model.FlowExecutionLog{}).Where("flow_instance_id = ?", instanceID)

	if err := query.Session(&gorm.Session{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * pageSize
	err := query.Offset(offset).Limit(pageSize).Order("created_at ASC").Find(&logs).Error
	return logs, total, err
}
