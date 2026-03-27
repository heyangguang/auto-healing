package repository

import (
	"context"
	"errors"

	"github.com/company/auto-healing/internal/modules/automation/model"
	"github.com/company/auto-healing/internal/pkg/query"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// HealingFlowRepository 自愈流程仓库
type HealingFlowRepository struct {
	db *gorm.DB
}

func NewHealingFlowRepositoryWithDB(db *gorm.DB) *HealingFlowRepository {
	return &HealingFlowRepository{db: db}
}

// Create 创建自愈流程
func (r *HealingFlowRepository) Create(ctx context.Context, flow *model.HealingFlow) error {
	if err := FillTenantID(ctx, &flow.TenantID); err != nil {
		return err
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
	return UpdateTenantScopedModel(r.db, ctx, flow.ID, flow)
}

// Delete 删除自愈流程
func (r *HealingFlowRepository) Delete(ctx context.Context, id uuid.UUID) error {
	result := TenantDB(r.db, ctx).Delete(&model.HealingFlow{}, "id = ?", id)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return ErrHealingFlowNotFound
	}
	return nil
}

// List 获取自愈流程列表
// 支持排序（name/created_at/updated_at/nodes_count）、高级过滤（name精确/description模糊/node_type/nodes数量/时间范围）
func (r *HealingFlowRepository) List(ctx context.Context, page, pageSize int, isActive *bool, search query.StringFilter, name, description query.StringFilter, nodeType string, minNodes, maxNodes *int, createdFrom, createdTo, updatedFrom, updatedTo, sortBy, sortOrder string) ([]model.HealingFlow, int64, error) {
	var flows []model.HealingFlow
	var total int64

	q := applyHealingFlowListFilters(TenantDB(r.db, ctx).Model(&model.HealingFlow{}), isActive, search, name, description, nodeType, minNodes, maxNodes, createdFrom, createdTo, updatedFrom, updatedTo)
	if err := q.Session(&gorm.Session{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	err := q.Preload("Creator").
		Offset((page - 1) * pageSize).
		Limit(pageSize).
		Order(healingFlowOrderClause(sortBy, sortOrder)).
		Find(&flows).Error
	return flows, total, err
}

func applyHealingFlowListFilters(q *gorm.DB, isActive *bool, search, name, description query.StringFilter, nodeType string, minNodes, maxNodes *int, createdFrom, createdTo, updatedFrom, updatedTo string) *gorm.DB {
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
	return q
}

func healingFlowOrderClause(sortBy, sortOrder string) string {
	if sortOrder != "asc" && sortOrder != "desc" {
		sortOrder = "desc"
	}
	switch sortBy {
	case "name":
		return "name " + sortOrder
	case "created_at":
		return "created_at " + sortOrder
	case "updated_at":
		return "updated_at " + sortOrder
	case "nodes_count":
		return "jsonb_array_length(nodes) " + sortOrder
	default:
		return "created_at DESC"
	}
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
