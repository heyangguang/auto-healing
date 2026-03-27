package repository

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/company/auto-healing/internal/database"
	"github.com/company/auto-healing/internal/modules/integrations/model"
	"github.com/company/auto-healing/internal/pkg/query"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

var ErrGitRepositoryNotFound = errors.New("仓库不存在")

// GitRepositoryRepository Git 仓库仓储
type GitRepositoryRepository struct {
	db *gorm.DB
}

// NewGitRepositoryRepository 创建 Git 仓库仓储
func NewGitRepositoryRepository() *GitRepositoryRepository {
	return &GitRepositoryRepository{
		db: database.DB,
	}
}

// Create 创建仓库
func (r *GitRepositoryRepository) Create(ctx context.Context, repo *model.GitRepository) error {
	if err := FillTenantID(ctx, &repo.TenantID); err != nil {
		return err
	}
	return r.db.WithContext(ctx).Create(repo).Error
}

// GetByID 根据ID获取仓库
func (r *GitRepositoryRepository) GetByID(ctx context.Context, id uuid.UUID) (*model.GitRepository, error) {
	var repo model.GitRepository
	err := TenantDB(r.db, ctx).Where("id = ?", id).First(&repo).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrGitRepositoryNotFound
		}
		return nil, err
	}
	return &repo, nil
}

// GetByName 根据名称获取仓库
func (r *GitRepositoryRepository) GetByName(ctx context.Context, name string) (*model.GitRepository, error) {
	var repo model.GitRepository
	err := TenantDB(r.db, ctx).Where("name = ?", name).First(&repo).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrGitRepositoryNotFound
		}
		return nil, err
	}
	return &repo, nil
}

// Delete 删除仓库
func (r *GitRepositoryRepository) Delete(ctx context.Context, id uuid.UUID) error {
	return TenantDB(r.db, ctx).Delete(&model.GitRepository{}, id).Error
}

// GitRepoListOptions Git 仓库列表查询选项
type GitRepoListOptions struct {
	// 分页
	Page     int
	PageSize int

	// 搜索（支持精确/模糊匹配）
	Name query.StringFilter // 按名称搜索
	URL  query.StringFilter // 按 URL 搜索

	// 过滤
	Status      string // ready / pending / error / syncing
	AuthType    string // none / token / password / ssh_key
	SyncEnabled *bool  // 是否开启定时同步

	// 排序
	SortField string // name / status / created_at / updated_at / last_sync_at
	SortOrder string // asc / desc

	// 时间范围
	CreatedFrom *time.Time
	CreatedTo   *time.Time
}

// List 获取仓库列表（向后兼容）
func (r *GitRepositoryRepository) List(ctx context.Context, status string) ([]model.GitRepository, error) {
	repos, _, err := r.ListWithOptions(ctx, &GitRepoListOptions{
		Status: status,
	})
	return repos, err
}

// ListWithOptions 获取仓库列表（支持完整查询参数）
func (r *GitRepositoryRepository) ListWithOptions(ctx context.Context, opts *GitRepoListOptions) ([]model.GitRepository, int64, error) {
	var repos []model.GitRepository
	var total int64

	q := applyGitRepoListFilters(TenantDB(r.db, ctx).Model(&model.GitRepository{}), opts)
	if err := q.Session(&gorm.Session{}).Count(&total).Error; err != nil {
		return repos, total, err
	}
	q = applyGitRepoListOrder(q, opts)
	q = applyGitRepoListPagination(q, opts)
	if err := q.Find(&repos).Error; err != nil {
		return repos, total, err
	}
	if err := r.attachGitRepoPlaybookCounts(ctx, repos); err != nil {
		return repos, total, err
	}
	return repos, total, nil
}

func applyGitRepoListFilters(q *gorm.DB, opts *GitRepoListOptions) *gorm.DB {
	if !opts.Name.IsEmpty() {
		q = query.ApplyStringFilter(q, "name", opts.Name)
	}
	if !opts.URL.IsEmpty() {
		q = query.ApplyStringFilter(q, "url", opts.URL)
	}
	if opts.Status != "" {
		q = q.Where("status = ?", opts.Status)
	}
	if opts.AuthType != "" {
		q = q.Where("auth_type = ?", opts.AuthType)
	}
	if opts.SyncEnabled != nil {
		q = q.Where("sync_enabled = ?", *opts.SyncEnabled)
	}
	if opts.CreatedFrom != nil {
		q = q.Where("created_at >= ?", *opts.CreatedFrom)
	}
	if opts.CreatedTo != nil {
		q = q.Where("created_at <= ?", *opts.CreatedTo)
	}
	return q
}

func applyGitRepoListOrder(q *gorm.DB, opts *GitRepoListOptions) *gorm.DB {
	allowedSortFields := map[string]bool{
		"name": true, "status": true, "created_at": true,
		"updated_at": true, "last_sync_at": true,
	}
	if opts.SortField != "" && allowedSortFields[opts.SortField] {
		order := "ASC"
		if strings.ToLower(opts.SortOrder) == "desc" {
			order = "DESC"
		}
		return q.Order(fmt.Sprintf("%s %s", opts.SortField, order))
	}
	return q.Order("created_at DESC")
}

func applyGitRepoListPagination(q *gorm.DB, opts *GitRepoListOptions) *gorm.DB {
	if opts.Page > 0 && opts.PageSize > 0 {
		return q.Offset((opts.Page - 1) * opts.PageSize).Limit(opts.PageSize)
	}
	return q
}

func (r *GitRepositoryRepository) attachGitRepoPlaybookCounts(ctx context.Context, repos []model.GitRepository) error {
	if len(repos) == 0 {
		return nil
	}
	countMap, err := r.listGitRepoPlaybookCounts(ctx, repos)
	if err != nil {
		return err
	}
	for i := range repos {
		repos[i].PlaybookCount = countMap[repos[i].ID]
	}
	return nil
}

func (r *GitRepositoryRepository) listGitRepoPlaybookCounts(ctx context.Context, repos []model.GitRepository) (map[uuid.UUID]int64, error) {
	repoIDs := make([]uuid.UUID, 0, len(repos))
	for _, repo := range repos {
		repoIDs = append(repoIDs, repo.ID)
	}
	type playbookCountResult struct {
		RepositoryID uuid.UUID `gorm:"column:repository_id"`
		Count        int64     `gorm:"column:count"`
	}
	var counts []playbookCountResult
	err := TenantDB(r.db, ctx).
		Model(&model.Playbook{}).
		Select("repository_id, COUNT(*) as count").
		Where("repository_id IN ?", repoIDs).
		Group("repository_id").
		Find(&counts).Error
	if err != nil {
		return nil, err
	}
	countMap := make(map[uuid.UUID]int64, len(counts))
	for _, count := range counts {
		countMap[count.RepositoryID] = count.Count
	}
	return countMap, nil
}

// UpdateStatus 更新仓库状态
func (r *GitRepositoryRepository) UpdateStatus(ctx context.Context, id uuid.UUID, status, errorMsg string) error {
	updates := map[string]interface{}{
		"status": status,
	}
	if errorMsg != "" {
		updates["error_message"] = errorMsg
	} else {
		updates["error_message"] = ""
	}
	return TenantDB(r.db, ctx).Model(&model.GitRepository{}).Where("id = ?", id).Updates(updates).Error
}

// UpdateBranches 更新分支列表
func (r *GitRepositoryRepository) UpdateBranches(ctx context.Context, id uuid.UUID, branches []string) error {
	// 将 []string 转换为 JSON 数组
	jsonBranches := make(model.JSONArray, len(branches))
	for i, b := range branches {
		jsonBranches[i] = b
	}
	return TenantDB(r.db, ctx).Model(&model.GitRepository{}).Where("id = ?", id).Update("branches", jsonBranches).Error
}

// UpdateLastSync 更新最后同步时间
func (r *GitRepositoryRepository) UpdateLastSync(ctx context.Context, id uuid.UUID) error {
	return TenantDB(r.db, ctx).Model(&model.GitRepository{}).Where("id = ?", id).
		Update("last_sync_at", gorm.Expr("NOW()")).Error
}

// CreateSyncLog 创建同步日志
func (r *GitRepositoryRepository) CreateSyncLog(ctx context.Context, log *model.GitSyncLog) error {
	if err := FillTenantID(ctx, &log.TenantID); err != nil {
		return err
	}
	return r.db.WithContext(ctx).Create(log).Error
}

// ListSyncLogs 获取同步日志列表
func (r *GitRepositoryRepository) ListSyncLogs(ctx context.Context, repoID uuid.UUID, page, pageSize int) ([]model.GitSyncLog, int64, error) {
	var logs []model.GitSyncLog
	var total int64

	query := TenantDB(r.db, ctx).Model(&model.GitSyncLog{}).Where("repository_id = ?", repoID)
	if err := query.Session(&gorm.Session{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}

	err := query.Order("created_at DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&logs).Error
	return logs, total, err
}

// ==================== 统计 ====================

// GetStats 获取 Git 仓库统计信息
func (r *GitRepositoryRepository) GetStats(ctx context.Context) (map[string]interface{}, error) {
	stats := make(map[string]interface{})

	// 总数
	var total int64
	// 每次查询使用新的 TenantDB 实例，避免 GORM session WHERE 条件累积
	newDB := func() *gorm.DB { return TenantDB(r.db, ctx) }
	if err := newDB().Model(&model.GitRepository{}).Count(&total).Error; err != nil {
		return nil, err
	}
	stats["total"] = total

	// 按状态统计
	type StatusCount struct {
		Status string `json:"status"`
		Count  int64  `json:"count"`
	}
	var statusCounts []StatusCount
	if err := newDB().Model(&model.GitRepository{}).
		Select("status, count(*) as count").
		Group("status").
		Scan(&statusCounts).Error; err != nil {
		return nil, err
	}
	stats["by_status"] = statusCounts

	return stats, nil
}
