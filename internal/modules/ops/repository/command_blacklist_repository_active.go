package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/company/auto-healing/internal/modules/ops/model"
	"github.com/google/uuid"
)

// GetActiveRules 获取当前租户所有启用的规则（用于 Playbook 扫描，带缓存）
func (r *CommandBlacklistRepository) GetActiveRules(ctx context.Context) ([]model.CommandBlacklist, error) {
	tenantID, err := RequireTenantID(ctx)
	if err != nil {
		return nil, err
	}

	tenantRules, err := r.listTenantActiveRules(ctx, tenantID)
	if err != nil {
		return nil, fmt.Errorf("查询租户启用规则失败: %w", err)
	}

	systemRules, err := r.listSystemRules(ctx)
	if err != nil {
		return nil, fmt.Errorf("查询系统启用规则失败: %w", err)
	}

	systemRules, err = r.applyOverrides(ctx, tenantID, systemRules)
	if err != nil {
		return nil, err
	}

	active := true
	return append(tenantRules, applyCommandBlacklistActiveFilter(systemRules, &active)...), nil
}

func (r *CommandBlacklistRepository) listTenantActiveRules(ctx context.Context, tenantID uuid.UUID) ([]model.CommandBlacklist, error) {
	var tenantRules []model.CommandBlacklist
	err := r.db.WithContext(ctx).
		Where("tenant_id = ? AND is_active = true", tenantID).
		Find(&tenantRules).Error
	return tenantRules, err
}

func (r *CommandBlacklistRepository) listSystemRules(ctx context.Context) ([]model.CommandBlacklist, error) {
	var systemRules []model.CommandBlacklist
	err := r.db.WithContext(ctx).
		Where("tenant_id IS NULL").
		Find(&systemRules).Error
	return systemRules, err
}

func (r *CommandBlacklistRepository) applyOverrides(ctx context.Context, tenantID uuid.UUID, rules []model.CommandBlacklist) ([]model.CommandBlacklist, error) {
	systemRuleIDs := collectSystemRuleIDs(rules)
	if len(systemRuleIDs) == 0 {
		return rules, nil
	}

	overrides, err := r.listTenantBlacklistOverrides(ctx, tenantID, systemRuleIDs)
	if err != nil {
		return nil, fmt.Errorf("查询黑名单覆盖配置失败: %w", err)
	}
	return mergeCommandBlacklistOverrides(rules, overrides), nil
}

func collectSystemRuleIDs(rules []model.CommandBlacklist) []uuid.UUID {
	systemRuleIDs := make([]uuid.UUID, 0, len(rules))
	for _, rule := range rules {
		if rule.TenantID == nil {
			systemRuleIDs = append(systemRuleIDs, rule.ID)
		}
	}
	return systemRuleIDs
}

func (r *CommandBlacklistRepository) listTenantBlacklistOverrides(ctx context.Context, tenantID uuid.UUID, ruleIDs []uuid.UUID) ([]model.TenantBlacklistOverride, error) {
	var overrides []model.TenantBlacklistOverride
	err := r.db.WithContext(ctx).
		Where("tenant_id = ? AND rule_id IN ?", tenantID, ruleIDs).
		Find(&overrides).Error
	return overrides, err
}

func mergeCommandBlacklistOverrides(rules []model.CommandBlacklist, overrides []model.TenantBlacklistOverride) []model.CommandBlacklist {
	merged := append([]model.CommandBlacklist(nil), rules...)
	overrideMap := make(map[uuid.UUID]bool, len(overrides))
	overrideExists := make(map[uuid.UUID]bool, len(overrides))
	for _, override := range overrides {
		overrideMap[override.RuleID] = override.IsActive
		overrideExists[override.RuleID] = true
	}
	for i := range merged {
		if merged[i].TenantID == nil && overrideExists[merged[i].ID] {
			merged[i].IsActive = overrideMap[merged[i].ID]
		}
	}
	return merged
}

func (r *CommandBlacklistRepository) invalidateCache() {
	r.mu.Lock()
	r.cache = make(map[string][]model.CommandBlacklist)
	r.cacheTime = make(map[string]time.Time)
	r.mu.Unlock()
}
