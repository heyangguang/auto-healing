package repository

import (
	"context"
	"errors"

	"github.com/company/auto-healing/internal/database"
	"github.com/company/auto-healing/internal/modules/automation/model"
	"github.com/company/auto-healing/internal/pkg/query"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// HealingRuleRepository 自愈规则仓库
type HealingRuleRepository struct {
	db *gorm.DB
}

// NewHealingRuleRepository 创建自愈规则仓库
func NewHealingRuleRepository() *HealingRuleRepository {
	return NewHealingRuleRepositoryWithDB(database.DB)
}

func NewHealingRuleRepositoryWithDB(db *gorm.DB) *HealingRuleRepository {
	return &HealingRuleRepository{db: db}
}

// Create 创建自愈规则
func (r *HealingRuleRepository) Create(ctx context.Context, rule *model.HealingRule) error {
	if err := FillTenantID(ctx, &rule.TenantID); err != nil {
		return err
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
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return ErrHealingRuleNotFound
		}
		return nil
	})
}

// List 获取自愈规则列表
// 支持 triggerMode, priority, matchMode, hasFlow, createdFrom/To 过滤、sortBy/sortOrder 排序（含 conditions_count）
func (r *HealingRuleRepository) List(ctx context.Context, page, pageSize int, isActive *bool, flowID *uuid.UUID, search query.StringFilter, triggerMode, sortBy, sortOrder string, priority *int, matchMode string, hasFlow *bool, createdFrom, createdTo string, scopes ...func(*gorm.DB) *gorm.DB) ([]model.HealingRule, int64, error) {
	var rules []model.HealingRule
	var total int64

	q := applyHealingRuleListFilters(TenantDB(r.db, ctx).Model(&model.HealingRule{}), isActive, flowID, search, triggerMode, priority, matchMode, hasFlow, createdFrom, createdTo, scopes...)
	if err := q.Session(&gorm.Session{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	err := q.Preload("Flow").Preload("Creator").
		Offset((page - 1) * pageSize).
		Limit(pageSize).
		Order(healingRuleOrderClause(sortBy, sortOrder)).
		Find(&rules).Error
	return rules, total, err
}

func applyHealingRuleListFilters(q *gorm.DB, isActive *bool, flowID *uuid.UUID, search query.StringFilter, triggerMode string, priority *int, matchMode string, hasFlow *bool, createdFrom, createdTo string, scopes ...func(*gorm.DB) *gorm.DB) *gorm.DB {
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
	return q
}

func healingRuleOrderClause(sortBy, sortOrder string) string {
	if sortOrder != "asc" && sortOrder != "desc" {
		sortOrder = "desc"
	}
	switch sortBy {
	case "priority":
		return "priority " + sortOrder
	case "created_at":
		return "created_at " + sortOrder
	case "updated_at":
		return "updated_at " + sortOrder
	case "name":
		return "name " + sortOrder
	case "conditions_count":
		return "jsonb_array_length(conditions) " + sortOrder
	default:
		return "priority DESC, created_at DESC"
	}
}

// ListActiveByPriority 获取所有启用的规则，按优先级排序（跨租户，调度器专用）
// 注意：不使用 TenantDB，调度器需要处理所有租户的规则
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
