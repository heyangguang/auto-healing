package repository

import (
	"context"
	"errors"

	"github.com/company/auto-healing/internal/database"
	"github.com/company/auto-healing/internal/model"
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
	return r.db.WithContext(ctx).Create(flow).Error
}

// GetByID 根据 ID 获取自愈流程
func (r *HealingFlowRepository) GetByID(ctx context.Context, id uuid.UUID) (*model.HealingFlow, error) {
	var flow model.HealingFlow
	err := r.db.WithContext(ctx).Preload("Creator").First(&flow, "id = ?", id).Error
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
	result := r.db.WithContext(ctx).Delete(&model.HealingFlow{}, "id = ?", id)
	if result.RowsAffected == 0 {
		return ErrHealingFlowNotFound
	}
	return result.Error
}

// List 获取自愈流程列表
func (r *HealingFlowRepository) List(ctx context.Context, page, pageSize int, isActive *bool, search string) ([]model.HealingFlow, int64, error) {
	var flows []model.HealingFlow
	var total int64

	query := r.db.WithContext(ctx).Model(&model.HealingFlow{})

	if isActive != nil {
		query = query.Where("is_active = ?", *isActive)
	}
	if search != "" {
		pattern := "%" + search + "%"
		query = query.Where("(name ILIKE ? OR description ILIKE ?)", pattern, pattern)
	}

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * pageSize
	err := query.Preload("Creator").Offset(offset).Limit(pageSize).Order("created_at DESC").Find(&flows).Error
	return flows, total, err
}

// CountFlowsUsingTaskTemplate 统计使用指定任务模板的自愈流程数量
// 检查 nodes JSONB 数组中是否有 execution 节点引用该 task_template_id
func (r *HealingFlowRepository) CountFlowsUsingTaskTemplate(ctx context.Context, taskTemplateID string) (int64, error) {
	var count int64
	// 使用 PostgreSQL JSONB 查询：检查 nodes 数组中是否包含 task_template_id
	err := r.db.WithContext(ctx).Model(&model.HealingFlow{}).
		Where("nodes @> ?", `[{"config": {"task_template_id": "`+taskTemplateID+`"}}]`).
		Count(&count).Error
	return count, err
}

// CountFlowsUsingChannel 统计使用指定通知渠道的自愈流程数量
// 检查 nodes JSONB 数组中是否有 notification 节点引用该 channel_id
func (r *HealingFlowRepository) CountFlowsUsingChannel(ctx context.Context, channelID string) (int64, error) {
	var count int64
	// 检查 channel_id（单个渠道）
	err := r.db.WithContext(ctx).Model(&model.HealingFlow{}).
		Where("nodes @> ?", `[{"config": {"channel_id": "`+channelID+`"}}]`).
		Count(&count).Error
	if err != nil {
		return 0, err
	}

	// 也检查 channel_ids 数组（多个渠道）
	var count2 int64
	err = r.db.WithContext(ctx).Model(&model.HealingFlow{}).
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

// GetStats 获取自愈流程统计信息
func (r *HealingFlowRepository) GetStats(ctx context.Context) (map[string]interface{}, error) {
	stats := make(map[string]interface{})

	// 总数
	var total int64
	if err := r.db.WithContext(ctx).Model(&model.HealingFlow{}).Count(&total).Error; err != nil {
		return nil, err
	}
	stats["total"] = total

	// 启用/禁用统计
	var activeCount int64
	r.db.WithContext(ctx).Model(&model.HealingFlow{}).
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
	return r.db.WithContext(ctx).Create(rule).Error
}

// GetByID 根据 ID 获取自愈规则
func (r *HealingRuleRepository) GetByID(ctx context.Context, id uuid.UUID) (*model.HealingRule, error) {
	var rule model.HealingRule
	err := r.db.WithContext(ctx).Preload("Flow").Preload("Creator").First(&rule, "id = ?", id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrHealingRuleNotFound
	}
	return &rule, err
}

// Update 更新自愈规则
func (r *HealingRuleRepository) Update(ctx context.Context, rule *model.HealingRule) error {
	// 使用 Select 明确指定要更新的列，避免 GORM 忽略外键字段
	return r.db.WithContext(ctx).
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
	if err := r.db.WithContext(ctx).Model(&model.FlowInstance{}).Where("rule_id = ?", id).Count(&count).Error; err != nil {
		return err
	}

	if count > 0 && !force {
		return errors.New("规则存在关联的执行记录，请使用 force=true 强制删除")
	}

	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
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
func (r *HealingRuleRepository) List(ctx context.Context, page, pageSize int, isActive *bool, flowID *uuid.UUID, search string) ([]model.HealingRule, int64, error) {
	var rules []model.HealingRule
	var total int64

	query := r.db.WithContext(ctx).Model(&model.HealingRule{})

	if isActive != nil {
		query = query.Where("is_active = ?", *isActive)
	}
	if flowID != nil {
		query = query.Where("flow_id = ?", *flowID)
	}
	if search != "" {
		pattern := "%" + search + "%"
		query = query.Where("(name ILIKE ? OR description ILIKE ?)", pattern, pattern)
	}

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * pageSize
	err := query.Preload("Flow").Preload("Creator").Offset(offset).Limit(pageSize).Order("priority DESC, created_at DESC").Find(&rules).Error
	return rules, total, err
}

// ListActiveByPriority 获取所有启用的规则，按优先级排序
func (r *HealingRuleRepository) ListActiveByPriority(ctx context.Context) ([]model.HealingRule, error) {
	var rules []model.HealingRule
	err := r.db.WithContext(ctx).
		Where("is_active = ?", true).
		Preload("Flow").
		Order("priority DESC").
		Find(&rules).Error
	return rules, err
}

// UpdateLastRunAt 更新规则最后运行时间
func (r *HealingRuleRepository) UpdateLastRunAt(ctx context.Context, id uuid.UUID) error {
	return r.db.WithContext(ctx).Model(&model.HealingRule{}).
		Where("id = ?", id).
		Update("last_run_at", gorm.Expr("NOW()")).Error
}

// Activate 启用规则
func (r *HealingRuleRepository) Activate(ctx context.Context, id uuid.UUID) error {
	return r.db.WithContext(ctx).Model(&model.HealingRule{}).
		Where("id = ?", id).
		Update("is_active", true).Error
}

// Deactivate 停用规则
func (r *HealingRuleRepository) Deactivate(ctx context.Context, id uuid.UUID) error {
	return r.db.WithContext(ctx).Model(&model.HealingRule{}).
		Where("id = ?", id).
		Update("is_active", false).Error
}

// GetStats 获取自愈规则统计信息
func (r *HealingRuleRepository) GetStats(ctx context.Context) (map[string]interface{}, error) {
	stats := make(map[string]interface{})

	// 总数
	var total int64
	if err := r.db.WithContext(ctx).Model(&model.HealingRule{}).Count(&total).Error; err != nil {
		return nil, err
	}
	stats["total"] = total

	// 启用/禁用统计
	var activeCount int64
	r.db.WithContext(ctx).Model(&model.HealingRule{}).
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
	r.db.WithContext(ctx).Model(&model.HealingRule{}).
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
	return r.db.WithContext(ctx).Create(instance).Error
}

// GetByID 根据 ID 获取流程实例
func (r *FlowInstanceRepository) GetByID(ctx context.Context, id uuid.UUID) (*model.FlowInstance, error) {
	var instance model.FlowInstance
	err := r.db.WithContext(ctx).
		Preload("Flow").
		Preload("Rule").
		Preload("Incident").
		First(&instance, "id = ?", id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrFlowInstanceNotFound
	}
	return &instance, err
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
	return r.db.WithContext(ctx).Model(&model.FlowInstance{}).Where("id = ?", id).Updates(updates).Error
}

// UpdateNodeStates 更新节点状态
func (r *FlowInstanceRepository) UpdateNodeStates(ctx context.Context, id uuid.UUID, nodeStates model.JSON) error {
	return r.db.WithContext(ctx).Model(&model.FlowInstance{}).Where("id = ?", id).Update("node_states", nodeStates).Error
}

// UpdateCurrentNodeAndStates 更新当前节点和节点状态
func (r *FlowInstanceRepository) UpdateCurrentNodeAndStates(ctx context.Context, id uuid.UUID, currentNodeID string, nodeStates model.JSON) error {
	return r.db.WithContext(ctx).Model(&model.FlowInstance{}).Where("id = ?", id).Updates(map[string]interface{}{
		"current_node_id": currentNodeID,
		"node_states":     nodeStates,
	}).Error
}

// List 获取流程实例列表
func (r *FlowInstanceRepository) List(ctx context.Context, page, pageSize int, flowID, ruleID *uuid.UUID, incidentID *uuid.UUID, status string, search string) ([]model.FlowInstance, int64, error) {
	var instances []model.FlowInstance
	var total int64

	query := r.db.WithContext(ctx).Model(&model.FlowInstance{})

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

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * pageSize
	err := query.
		Preload("Flow").
		Preload("Rule").
		Preload("Incident").
		Offset(offset).Limit(pageSize).Order("flow_instances.created_at DESC").Find(&instances).Error
	return instances, total, err
}

// GetStats 获取流程实例统计信息
func (r *FlowInstanceRepository) GetStats(ctx context.Context) (map[string]interface{}, error) {
	stats := make(map[string]interface{})

	// 总数
	var total int64
	if err := r.db.WithContext(ctx).Model(&model.FlowInstance{}).Count(&total).Error; err != nil {
		return nil, err
	}
	stats["total"] = total

	// 按状态统计
	type StatusCount struct {
		Status string `json:"status"`
		Count  int64  `json:"count"`
	}
	var statusCounts []StatusCount
	r.db.WithContext(ctx).Model(&model.FlowInstance{}).
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
	return r.db.WithContext(ctx).Create(task).Error
}

// GetByID 根据 ID 获取审批任务
func (r *ApprovalTaskRepository) GetByID(ctx context.Context, id uuid.UUID) (*model.ApprovalTask, error) {
	var task model.ApprovalTask
	err := r.db.WithContext(ctx).
		Preload("FlowInstance").
		Preload("FlowInstance.Flow").
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
	return r.db.WithContext(ctx).Model(&model.ApprovalTask{}).
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
	return r.db.WithContext(ctx).Model(&model.ApprovalTask{}).
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
	result := r.db.WithContext(ctx).Model(&model.ApprovalTask{}).
		Where("status = ? AND timeout_at < NOW()", model.ApprovalTaskStatusPending).
		Updates(map[string]interface{}{
			"status":     model.ApprovalTaskStatusExpired,
			"decided_at": gorm.Expr("NOW()"),
		})
	return result.RowsAffected, result.Error
}

// ListPending 获取待审批列表
// 支持搜索和过滤：search（模糊匹配 node_id, flow_instance_id）、dateFrom、dateTo
func (r *ApprovalTaskRepository) ListPending(ctx context.Context, page, pageSize int, search, dateFrom, dateTo string) ([]model.ApprovalTask, int64, error) {
	var tasks []model.ApprovalTask
	var total int64

	query := r.db.WithContext(ctx).Model(&model.ApprovalTask{}).Where("status = ?", model.ApprovalTaskStatusPending)

	// 模糊搜索：node_id 或 flow_instance_id
	if search != "" {
		searchPattern := "%" + search + "%"
		query = query.Where("(node_id ILIKE ? OR flow_instance_id::text ILIKE ?)", searchPattern, searchPattern)
	}

	// 日期范围过滤
	if dateFrom != "" {
		query = query.Where("created_at >= ?", dateFrom+" 00:00:00")
	}
	if dateTo != "" {
		query = query.Where("created_at <= ?", dateTo+" 23:59:59")
	}

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * pageSize
	err := query.
		Preload("FlowInstance").
		Preload("FlowInstance.Flow").
		Preload("FlowInstance.Incident").
		Preload("Initiator").
		Offset(offset).Limit(pageSize).Order("created_at DESC").Find(&tasks).Error
	return tasks, total, err
}

// List 获取审批任务列表
func (r *ApprovalTaskRepository) List(ctx context.Context, page, pageSize int, flowInstanceID *uuid.UUID, status string) ([]model.ApprovalTask, int64, error) {
	var tasks []model.ApprovalTask
	var total int64

	query := r.db.WithContext(ctx).Model(&model.ApprovalTask{})

	if flowInstanceID != nil {
		query = query.Where("flow_instance_id = ?", *flowInstanceID)
	}
	if status != "" {
		query = query.Where("status = ?", status)
	}

	if err := query.Count(&total).Error; err != nil {
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
	err := r.db.WithContext(ctx).
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
	return r.db.WithContext(ctx).Create(log).Error
}

// CreateBatch 批量创建日志
func (r *FlowLogRepository) CreateBatch(ctx context.Context, logs []*model.FlowExecutionLog) error {
	if len(logs) == 0 {
		return nil
	}
	return r.db.WithContext(ctx).Create(&logs).Error
}

// GetByInstanceID 根据流程实例ID获取所有日志
func (r *FlowLogRepository) GetByInstanceID(ctx context.Context, instanceID uuid.UUID) ([]*model.FlowExecutionLog, error) {
	var logs []*model.FlowExecutionLog
	err := r.db.WithContext(ctx).
		Where("flow_instance_id = ?", instanceID).
		Order("created_at ASC").
		Find(&logs).Error
	return logs, err
}

// GetByInstanceAndNode 根据流程实例ID和节点ID获取日志
func (r *FlowLogRepository) GetByInstanceAndNode(ctx context.Context, instanceID uuid.UUID, nodeID string) ([]*model.FlowExecutionLog, error) {
	var logs []*model.FlowExecutionLog
	err := r.db.WithContext(ctx).
		Where("flow_instance_id = ? AND node_id = ?", instanceID, nodeID).
		Order("created_at ASC").
		Find(&logs).Error
	return logs, err
}

// GetByLevel 根据日志级别获取日志
func (r *FlowLogRepository) GetByLevel(ctx context.Context, instanceID uuid.UUID, level string) ([]*model.FlowExecutionLog, error) {
	var logs []*model.FlowExecutionLog
	err := r.db.WithContext(ctx).
		Where("flow_instance_id = ? AND level = ?", instanceID, level).
		Order("created_at ASC").
		Find(&logs).Error
	return logs, err
}

// DeleteByInstanceID 删除流程实例的所有日志
func (r *FlowLogRepository) DeleteByInstanceID(ctx context.Context, instanceID uuid.UUID) error {
	return r.db.WithContext(ctx).
		Where("flow_instance_id = ?", instanceID).
		Delete(&model.FlowExecutionLog{}).Error
}

// ListPaginated 分页获取日志
func (r *FlowLogRepository) ListPaginated(ctx context.Context, instanceID uuid.UUID, page, pageSize int) ([]*model.FlowExecutionLog, int64, error) {
	var logs []*model.FlowExecutionLog
	var total int64

	query := r.db.WithContext(ctx).Model(&model.FlowExecutionLog{}).Where("flow_instance_id = ?", instanceID)

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * pageSize
	err := query.Offset(offset).Limit(pageSize).Order("created_at ASC").Find(&logs).Error
	return logs, total, err
}
