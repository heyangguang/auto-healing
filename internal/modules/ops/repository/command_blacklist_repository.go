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
	tenantID, err := RequireTenantID(ctx)
	if err != nil {
		return nil, err
	}
	var rule model.CommandBlacklist
	err = r.db.WithContext(ctx).
		Where("id = ? AND (tenant_id = ? OR tenant_id IS NULL)", id, tenantID).
		First(&rule).Error
	if err != nil {
		return nil, fmt.Errorf("黑名单规则不存在: %w", err)
	}
	if rule.IsSystem || rule.TenantID == nil {
		rules, err := r.applyOverrides(ctx, tenantID, []model.CommandBlacklist{rule})
		if err != nil {
			return nil, err
		}
		if len(rules) > 0 {
			rule = rules[0]
		}
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
	tenantID, err := RequireTenantID(ctx)
	if err != nil {
		return nil, 0, err
	}
	var rules []model.CommandBlacklist
	if err := applyCommandBlacklistListFilters(r.db.WithContext(ctx).Model(&model.CommandBlacklist{}), tenantID, opts).
		Order("is_system DESC, severity ASC, created_at DESC").
		Find(&rules).Error; err != nil {
		return nil, 0, err
	}
	rules, err = r.applyOverrides(ctx, tenantID, rules)
	if err != nil {
		return nil, 0, err
	}
	rules = applyCommandBlacklistActiveFilter(rules, opts.IsActive)
	total := int64(len(rules))
	return paginateCommandBlacklistRules(rules, total, opts), total, nil
}

func applyCommandBlacklistListFilters(query *gorm.DB, tenantID uuid.UUID, opts *CommandBlacklistListOptions) *gorm.DB {
	query = query.Where("tenant_id = ? OR tenant_id IS NULL", tenantID)
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
	return query
}

func applyCommandBlacklistActiveFilter(rules []model.CommandBlacklist, isActive *bool) []model.CommandBlacklist {
	if isActive == nil {
		return rules
	}
	filtered := make([]model.CommandBlacklist, 0, len(rules))
	for _, rule := range rules {
		if rule.IsActive == *isActive {
			filtered = append(filtered, rule)
		}
	}
	return filtered
}

func paginateCommandBlacklistRules(rules []model.CommandBlacklist, total int64, opts *CommandBlacklistListOptions) []model.CommandBlacklist {
	if opts.PageSize <= 0 {
		return rules
	}
	offset := (opts.Page - 1) * opts.PageSize
	if offset >= int(total) {
		return []model.CommandBlacklist{}
	}
	end := offset + opts.PageSize
	if end > int(total) {
		end = int(total)
	}
	return rules[offset:end]
}

// Update 更新租户自有规则
func (r *CommandBlacklistRepository) Update(ctx context.Context, rule *model.CommandBlacklist) error {
	if err := UpdateTenantScopedModel(r.db, ctx, rule.ID, rule); err != nil {
		return fmt.Errorf("更新黑名单规则失败: %w", err)
	}
	r.invalidateCache()
	return nil
}

// Delete 删除租户自有规则（系统规则不可删除，由 service 层校验）
func (r *CommandBlacklistRepository) Delete(ctx context.Context, id uuid.UUID) error {
	tenantID, err := RequireTenantID(ctx)
	if err != nil {
		return err
	}
	if err := r.db.WithContext(ctx).
		Where("id = ? AND tenant_id = ?", id, tenantID).
		Delete(&model.CommandBlacklist{}).Error; err != nil {
		return fmt.Errorf("删除黑名单规则失败: %w", err)
	}
	r.invalidateCache()
	return nil
}
