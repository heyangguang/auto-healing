package repository

import (
	"context"
	"time"

	"github.com/company/auto-healing/internal/model"
	"github.com/google/uuid"
	"gorm.io/gorm/clause"
)

// BatchToggle 批量启用/禁用租户自有规则
func (r *CommandBlacklistRepository) BatchToggle(ctx context.Context, ids []uuid.UUID, isActive bool) (int64, error) {
	tenantID, err := RequireTenantID(ctx)
	if err != nil {
		return 0, err
	}
	systemIDs, tenantIDs := r.splitSystemAndTenantRuleIDs(ctx, tenantID, ids)

	var affected int64
	if len(tenantIDs) > 0 {
		result := r.db.WithContext(ctx).Model(&model.CommandBlacklist{}).Where("id IN ?", tenantIDs).Update("is_active", isActive)
		if result.Error != nil {
			return 0, result.Error
		}
		affected += result.RowsAffected
	}
	for _, ruleID := range systemIDs {
		if err := r.upsertOverride(ctx, tenantID, ruleID, isActive); err != nil {
			return affected, err
		}
		affected++
	}
	r.invalidateCache()
	return affected, nil
}

func (r *CommandBlacklistRepository) splitSystemAndTenantRuleIDs(ctx context.Context, tenantID uuid.UUID, ids []uuid.UUID) ([]uuid.UUID, []uuid.UUID) {
	var rules []model.CommandBlacklist
	r.db.WithContext(ctx).Where("id IN ? AND (tenant_id = ? OR tenant_id IS NULL)", ids, tenantID).Find(&rules)

	systemIDs := make([]uuid.UUID, 0, len(rules))
	tenantIDs := make([]uuid.UUID, 0, len(rules))
	for _, rule := range rules {
		if rule.IsSystem || rule.TenantID == nil {
			systemIDs = append(systemIDs, rule.ID)
		} else {
			tenantIDs = append(tenantIDs, rule.ID)
		}
	}
	return systemIDs, tenantIDs
}

// ToggleSystemRule 为当前租户 upsert 系统规则的 override
func (r *CommandBlacklistRepository) ToggleSystemRule(ctx context.Context, ruleID uuid.UUID, isActive bool) error {
	tenantID, err := RequireTenantID(ctx)
	if err != nil {
		return err
	}
	if err := r.upsertOverride(ctx, tenantID, ruleID, isActive); err != nil {
		return err
	}
	r.invalidateCache()
	return nil
}

func (r *CommandBlacklistRepository) upsertOverride(ctx context.Context, tenantID, ruleID uuid.UUID, isActive bool) error {
	override := model.TenantBlacklistOverride{
		TenantID:  tenantID,
		RuleID:    ruleID,
		IsActive:  isActive,
		UpdatedAt: time.Now(),
	}
	return r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "tenant_id"}, {Name: "rule_id"}},
		DoUpdates: clause.AssignmentColumns([]string{"is_active", "updated_at"}),
	}).Create(&override).Error
}
