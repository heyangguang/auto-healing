package repository

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/company/auto-healing/internal/database"
	"github.com/company/auto-healing/internal/model"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// CommandBlacklistRepository 高危指令黑名单仓库
type CommandBlacklistRepository struct {
	db *gorm.DB

	// 缓存（按租户）
	mu        sync.RWMutex
	cache     map[string][]model.CommandBlacklist
	cacheTime map[string]time.Time
	cacheTTL  time.Duration
}

// NewCommandBlacklistRepository 创建仓库
func NewCommandBlacklistRepository() *CommandBlacklistRepository {
	return &CommandBlacklistRepository{
		db:        database.DB,
		cacheTTL:  60 * time.Second,
		cache:     make(map[string][]model.CommandBlacklist),
		cacheTime: make(map[string]time.Time),
	}
}

// Create 创建规则（只能创建租户自有规则）
func (r *CommandBlacklistRepository) Create(ctx context.Context, rule *model.CommandBlacklist) error {
	if err := r.db.WithContext(ctx).Create(rule).Error; err != nil {
		return fmt.Errorf("创建黑名单规则失败: %w", err)
	}
	r.invalidateCache()
	return nil
}

// GetByID 获取规则
// 系统规则（tenant_id=NULL）对所有租户可见；租户规则仅对本租户可见
func (r *CommandBlacklistRepository) GetByID(ctx context.Context, id uuid.UUID) (*model.CommandBlacklist, error) {
	var rule model.CommandBlacklist
	err := r.db.WithContext(ctx).
		Where("id = ? AND (tenant_id = ? OR tenant_id IS NULL)", id, TenantIDFromContext(ctx)).
		First(&rule).Error
	if err != nil {
		return nil, fmt.Errorf("黑名单规则不存在: %w", err)
	}
	return &rule, nil
}

// CommandBlacklistListOptions 列表查询参数
type CommandBlacklistListOptions struct {
	Page         int
	PageSize     int
	Name         string
	NameExact    string
	Category     string
	Severity     string
	IsActive     *bool
	Pattern      string
	PatternExact string
}

// List 列表查询
// 返回：租户自有规则 + 系统规则（tenant_id=NULL），并将系统规则的 is_active 替换为租户的 override 值
func (r *CommandBlacklistRepository) List(ctx context.Context, opts *CommandBlacklistListOptions) ([]model.CommandBlacklist, int64, error) {
	tenantID := TenantIDFromContext(ctx)

	// 1. 查询所有相关规则（租户自有 OR 系统规则）
	query := r.db.WithContext(ctx).Model(&model.CommandBlacklist{}).
		Where("tenant_id = ? OR tenant_id IS NULL", tenantID)

	if opts.Name != "" {
		query = query.Where("name ILIKE ?", "%"+opts.Name+"%")
	}
	if opts.NameExact != "" {
		query = query.Where("name = ?", opts.NameExact)
	}
	if opts.Category != "" {
		query = query.Where("category = ?", opts.Category)
	}
	if opts.Severity != "" {
		query = query.Where("severity = ?", opts.Severity)
	}
	if opts.Pattern != "" {
		query = query.Where("pattern ILIKE ?", "%"+opts.Pattern+"%")
	}
	if opts.PatternExact != "" {
		query = query.Where("pattern = ?", opts.PatternExact)
	}

	var total int64
	query.Count(&total)

	var rules []model.CommandBlacklist
	offset := (opts.Page - 1) * opts.PageSize
	if err := query.Order("is_system DESC, severity ASC, created_at DESC").
		Offset(offset).Limit(opts.PageSize).Find(&rules).Error; err != nil {
		return nil, 0, err
	}

	// 2. 加载该租户对系统规则的 overrides，合并 is_active
	rules = r.applyOverrides(ctx, tenantID, rules)

	// 3. 如果 IsActive 过滤，在内存中过滤（因为 is_active 已被 override 替换）
	if opts.IsActive != nil {
		filtered := rules[:0]
		for _, rule := range rules {
			if rule.IsActive == *opts.IsActive {
				filtered = append(filtered, rule)
			}
		}
		rules = filtered
	}

	return rules, total, nil
}

// Update 更新租户自有规则
func (r *CommandBlacklistRepository) Update(ctx context.Context, rule *model.CommandBlacklist) error {
	if err := r.db.WithContext(ctx).Save(rule).Error; err != nil {
		return fmt.Errorf("更新黑名单规则失败: %w", err)
	}
	r.invalidateCache()
	return nil
}

// Delete 删除租户自有规则（系统规则不可删除，由 service 层校验）
func (r *CommandBlacklistRepository) Delete(ctx context.Context, id uuid.UUID) error {
	tenantID := TenantIDFromContext(ctx)
	if err := r.db.WithContext(ctx).
		Where("id = ? AND tenant_id = ?", id, tenantID).
		Delete(&model.CommandBlacklist{}).Error; err != nil {
		return fmt.Errorf("删除黑名单规则失败: %w", err)
	}
	r.invalidateCache()
	return nil
}

// BatchToggle 批量启用/禁用租户自有规则
func (r *CommandBlacklistRepository) BatchToggle(ctx context.Context, ids []uuid.UUID, isActive bool) (int64, error) {
	tenantID := TenantIDFromContext(ctx)

	// 分为系统规则和租户规则分别处理
	var systemIDs, tenantIDs []uuid.UUID
	var rules []model.CommandBlacklist
	r.db.WithContext(ctx).Where("id IN ? AND (tenant_id = ? OR tenant_id IS NULL)", ids, tenantID).Find(&rules)
	for _, rule := range rules {
		if rule.IsSystem || rule.TenantID == nil {
			systemIDs = append(systemIDs, rule.ID)
		} else {
			tenantIDs = append(tenantIDs, rule.ID)
		}
	}

	var affected int64

	// 租户自有规则：直接更新
	if len(tenantIDs) > 0 {
		result := r.db.WithContext(ctx).Model(&model.CommandBlacklist{}).
			Where("id IN ?", tenantIDs).
			Update("is_active", isActive)
		if result.Error != nil {
			return 0, result.Error
		}
		affected += result.RowsAffected
	}

	// 系统规则：upsert override
	for _, ruleID := range systemIDs {
		if err := r.upsertOverride(ctx, tenantID, ruleID, isActive); err != nil {
			return affected, err
		}
		affected++
	}

	r.invalidateCache()
	return affected, nil
}

// ToggleSystemRule 为当前租户 upsert 系统规则的 override
func (r *CommandBlacklistRepository) ToggleSystemRule(ctx context.Context, ruleID uuid.UUID, isActive bool) error {
	tenantID := TenantIDFromContext(ctx)
	if err := r.upsertOverride(ctx, tenantID, ruleID, isActive); err != nil {
		return err
	}
	r.invalidateCache()
	return nil
}

// upsertOverride 内部方法：INSERT ... ON CONFLICT DO UPDATE
func (r *CommandBlacklistRepository) upsertOverride(ctx context.Context, tenantID, ruleID uuid.UUID, isActive bool) error {
	override := model.TenantBlacklistOverride{
		TenantID:  tenantID,
		RuleID:    ruleID,
		IsActive:  isActive,
		UpdatedAt: time.Now(),
	}
	return r.db.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "tenant_id"}, {Name: "rule_id"}},
			DoUpdates: clause.AssignmentColumns([]string{"is_active", "updated_at"}),
		}).
		Create(&override).Error
}

// GetActiveRules 获取当前租户所有启用的规则（用于 Playbook 扫描，带缓存）
// 包含：租户自有启用规则 + 系统规则中该租户 override 为 true 的
func (r *CommandBlacklistRepository) GetActiveRules(ctx context.Context) ([]model.CommandBlacklist, error) {
	tenantID := TenantIDFromContext(ctx)
	cacheKey := tenantID.String()

	r.mu.RLock()
	if cached, ok := r.cache[cacheKey]; ok && time.Since(r.cacheTime[cacheKey]) < r.cacheTTL {
		result := make([]model.CommandBlacklist, len(cached))
		copy(result, cached)
		r.mu.RUnlock()
		return result, nil
	}
	r.mu.RUnlock()

	// 1. 租户自有启用规则
	var tenantRules []model.CommandBlacklist
	if err := r.db.WithContext(ctx).
		Where("tenant_id = ? AND is_active = true", tenantID).
		Find(&tenantRules).Error; err != nil {
		return nil, fmt.Errorf("查询租户启用规则失败: %w", err)
	}

	// 2. 系统规则中该租户 override 为 true 的
	var systemRules []model.CommandBlacklist
	if err := r.db.WithContext(ctx).
		Joins("JOIN tenant_blacklist_overrides tbo ON tbo.rule_id = command_blacklist.id "+
			"AND tbo.tenant_id = ? AND tbo.is_active = true", tenantID).
		Where("command_blacklist.tenant_id IS NULL").
		Find(&systemRules).Error; err != nil {
		return nil, fmt.Errorf("查询系统启用规则失败: %w", err)
	}

	rules := append(tenantRules, systemRules...)

	// 更新缓存
	r.mu.Lock()
	r.cache[cacheKey] = make([]model.CommandBlacklist, len(rules))
	copy(r.cache[cacheKey], rules)
	r.cacheTime[cacheKey] = time.Now()
	r.mu.Unlock()

	return rules, nil
}

// applyOverrides 将系统规则的 is_active 替换为该租户的 override 值（list 展示用）
func (r *CommandBlacklistRepository) applyOverrides(ctx context.Context, tenantID uuid.UUID, rules []model.CommandBlacklist) []model.CommandBlacklist {
	// 收集系统规则 ID
	var systemRuleIDs []uuid.UUID
	for _, rule := range rules {
		if rule.TenantID == nil {
			systemRuleIDs = append(systemRuleIDs, rule.ID)
		}
	}
	if len(systemRuleIDs) == 0 {
		return rules
	}

	// 查询该租户对这些系统规则的 overrides
	var overrides []model.TenantBlacklistOverride
	r.db.WithContext(ctx).
		Where("tenant_id = ? AND rule_id IN ?", tenantID, systemRuleIDs).
		Find(&overrides)

	overrideMap := make(map[uuid.UUID]bool, len(overrides))
	overrideExists := make(map[uuid.UUID]bool, len(overrides))
	for _, o := range overrides {
		overrideMap[o.RuleID] = o.IsActive
		overrideExists[o.RuleID] = true
	}

	// 合并：有 override 用 override 值，否则用规则自身默认值
	for i := range rules {
		if rules[i].TenantID == nil {
			if exists := overrideExists[rules[i].ID]; exists {
				rules[i].IsActive = overrideMap[rules[i].ID]
			}
			// 没有 override 则保持规则原始的 is_active（默认 false）
		}
	}
	return rules
}

// invalidateCache 使所有租户缓存失效
func (r *CommandBlacklistRepository) invalidateCache() {
	r.mu.Lock()
	r.cache = make(map[string][]model.CommandBlacklist)
	r.cacheTime = make(map[string]time.Time)
	r.mu.Unlock()
}
