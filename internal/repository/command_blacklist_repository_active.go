package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/company/auto-healing/internal/model"
	"github.com/google/uuid"
)

// GetActiveRules 获取当前租户所有启用的规则（用于 Playbook 扫描，带缓存）
func (r *CommandBlacklistRepository) GetActiveRules(ctx context.Context) ([]model.CommandBlacklist, error) {
	tenantID, err := RequireTenantID(ctx)
	if err != nil {
		return nil, err
	}

	var tenantRules []model.CommandBlacklist
	if err := r.db.WithContext(ctx).Where("tenant_id = ? AND is_active = true", tenantID).Find(&tenantRules).Error; err != nil {
		return nil, fmt.Errorf("查询租户启用规则失败: %w", err)
	}

	var systemRules []model.CommandBlacklist
	if err := r.db.WithContext(ctx).
		Joins("JOIN tenant_blacklist_overrides tbo ON tbo.rule_id = command_blacklist.id AND tbo.tenant_id = ? AND tbo.is_active = true", tenantID).
		Where("command_blacklist.tenant_id IS NULL").
		Find(&systemRules).Error; err != nil {
		return nil, fmt.Errorf("查询系统启用规则失败: %w", err)
	}
	return append(tenantRules, systemRules...), nil
}

func (r *CommandBlacklistRepository) applyOverrides(ctx context.Context, tenantID uuid.UUID, rules []model.CommandBlacklist) []model.CommandBlacklist {
	var systemRuleIDs []uuid.UUID
	for _, rule := range rules {
		if rule.TenantID == nil {
			systemRuleIDs = append(systemRuleIDs, rule.ID)
		}
	}
	if len(systemRuleIDs) == 0 {
		return rules
	}

	var overrides []model.TenantBlacklistOverride
	r.db.WithContext(ctx).Where("tenant_id = ? AND rule_id IN ?", tenantID, systemRuleIDs).Find(&overrides)

	overrideMap := make(map[uuid.UUID]bool, len(overrides))
	overrideExists := make(map[uuid.UUID]bool, len(overrides))
	for _, override := range overrides {
		overrideMap[override.RuleID] = override.IsActive
		overrideExists[override.RuleID] = true
	}
	for i := range rules {
		if rules[i].TenantID == nil && overrideExists[rules[i].ID] {
			rules[i].IsActive = overrideMap[rules[i].ID]
		}
	}
	return rules
}

func (r *CommandBlacklistRepository) invalidateCache() {
	r.mu.Lock()
	r.cache = make(map[string][]model.CommandBlacklist)
	r.cacheTime = make(map[string]time.Time)
	r.mu.Unlock()
}
