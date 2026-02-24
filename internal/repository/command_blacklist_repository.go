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
	cache     map[string][]model.CommandBlacklist // key = tenantID string
	cacheTime map[string]time.Time
	cacheTTL  time.Duration
}

// NewCommandBlacklistRepository 创建仓库
func NewCommandBlacklistRepository() *CommandBlacklistRepository {
	return &CommandBlacklistRepository{
		db:        database.DB,
		cacheTTL:  60 * time.Second, // 60秒缓存
		cache:     make(map[string][]model.CommandBlacklist),
		cacheTime: make(map[string]time.Time),
	}
}

// Create 创建规则
func (r *CommandBlacklistRepository) Create(ctx context.Context, rule *model.CommandBlacklist) error {
	if err := r.db.WithContext(ctx).Create(rule).Error; err != nil {
		return fmt.Errorf("创建黑名单规则失败: %w", err)
	}
	r.invalidateCache()
	return nil
}

// GetByID 获取规则（带租户过滤）
func (r *CommandBlacklistRepository) GetByID(ctx context.Context, id uuid.UUID) (*model.CommandBlacklist, error) {
	var rule model.CommandBlacklist
	if err := TenantDB(r.db, ctx).First(&rule, "id = ?", id).Error; err != nil {
		return nil, fmt.Errorf("黑名单规则不存在: %w", err)
	}
	return &rule, nil
}

// CommandBlacklistListOptions 列表查询参数
type CommandBlacklistListOptions struct {
	Page         int
	PageSize     int
	Name         string // 名称搜索
	NameExact    string // 名称精确匹配
	Category     string // 分类筛选
	Severity     string // 严重级别筛选
	IsActive     *bool  // 启用状态筛选
	Pattern      string // 模式搜索
	PatternExact string // 模式精确匹配
}

// List 列表查询（带租户过滤）
func (r *CommandBlacklistRepository) List(ctx context.Context, opts *CommandBlacklistListOptions) ([]model.CommandBlacklist, int64, error) {
	query := TenantDB(r.db, ctx).Model(&model.CommandBlacklist{})

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
	if opts.IsActive != nil {
		query = query.Where("is_active = ?", *opts.IsActive)
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

	return rules, total, nil
}

// Update 更新规则
func (r *CommandBlacklistRepository) Update(ctx context.Context, rule *model.CommandBlacklist) error {
	if err := r.db.WithContext(ctx).Save(rule).Error; err != nil {
		return fmt.Errorf("更新黑名单规则失败: %w", err)
	}
	r.invalidateCache()
	return nil
}

// Delete 删除规则（带租户过滤）
func (r *CommandBlacklistRepository) Delete(ctx context.Context, id uuid.UUID) error {
	if err := TenantDB(r.db, ctx).Delete(&model.CommandBlacklist{}, "id = ?", id).Error; err != nil {
		return fmt.Errorf("删除黑名单规则失败: %w", err)
	}
	r.invalidateCache()
	return nil
}

// BatchToggle 批量启用/禁用（带租户过滤）
func (r *CommandBlacklistRepository) BatchToggle(ctx context.Context, ids []uuid.UUID, isActive bool) (int64, error) {
	result := TenantDB(r.db, ctx).Model(&model.CommandBlacklist{}).
		Where("id IN ?", ids).
		Update("is_active", isActive)
	if result.Error != nil {
		return 0, fmt.Errorf("批量更新失败: %w", result.Error)
	}
	r.invalidateCache()
	return result.RowsAffected, nil
}

// GetActiveRules 获取当前租户所有启用的规则（带缓存）
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

	// 从数据库加载（按租户过滤）
	var rules []model.CommandBlacklist
	if err := TenantDB(r.db, ctx).Where("is_active = ?", true).
		Order("severity ASC").Find(&rules).Error; err != nil {
		return nil, fmt.Errorf("查询启用的黑名单规则失败: %w", err)
	}

	// 更新缓存
	r.mu.Lock()
	r.cache[cacheKey] = make([]model.CommandBlacklist, len(rules))
	copy(r.cache[cacheKey], rules)
	r.cacheTime[cacheKey] = time.Now()
	r.mu.Unlock()

	return rules, nil
}

// invalidateCache 使所有租户缓存失效
func (r *CommandBlacklistRepository) invalidateCache() {
	r.mu.Lock()
	r.cache = make(map[string][]model.CommandBlacklist)
	r.cacheTime = make(map[string]time.Time)
	r.mu.Unlock()
}
