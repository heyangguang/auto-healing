package repository

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/company/auto-healing/internal/database"
	"github.com/company/auto-healing/internal/model"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

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
	return r.db.WithContext(ctx).Create(repo).Error
}

// GetByID 根据ID获取仓库
func (r *GitRepositoryRepository) GetByID(ctx context.Context, id uuid.UUID) (*model.GitRepository, error) {
	var repo model.GitRepository
	err := r.db.WithContext(ctx).Where("id = ?", id).First(&repo).Error
	if err != nil {
		return nil, err
	}
	return &repo, nil
}

// GetByName 根据名称获取仓库
func (r *GitRepositoryRepository) GetByName(ctx context.Context, name string) (*model.GitRepository, error) {
	var repo model.GitRepository
	err := r.db.WithContext(ctx).Where("name = ?", name).First(&repo).Error
	if err != nil {
		return nil, err
	}
	return &repo, nil
}

// Update 更新仓库
func (r *GitRepositoryRepository) Update(ctx context.Context, repo *model.GitRepository) error {
	return r.db.WithContext(ctx).Save(repo).Error
}

// Delete 删除仓库
func (r *GitRepositoryRepository) Delete(ctx context.Context, id uuid.UUID) error {
	return r.db.WithContext(ctx).Delete(&model.GitRepository{}, id).Error
}

// GitRepoListOptions Git 仓库列表查询选项
type GitRepoListOptions struct {
	// 分页
	Page     int
	PageSize int

	// 搜索
	Search string // 全文搜索（匹配 name + url）
	Name   string // 按名称模糊搜索
	URL    string // 按 URL 模糊搜索

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

	query := r.db.WithContext(ctx).Model(&model.GitRepository{})

	// 全文搜索（name + url）
	if opts.Search != "" {
		search := "%" + strings.ToLower(opts.Search) + "%"
		query = query.Where("LOWER(name) LIKE ? OR LOWER(url) LIKE ?", search, search)
	}

	// 按名称模糊搜索
	if opts.Name != "" {
		query = query.Where("LOWER(name) LIKE ?", "%"+strings.ToLower(opts.Name)+"%")
	}

	// 按 URL 模糊搜索
	if opts.URL != "" {
		query = query.Where("LOWER(url) LIKE ?", "%"+strings.ToLower(opts.URL)+"%")
	}

	// 状态过滤
	if opts.Status != "" {
		query = query.Where("status = ?", opts.Status)
	}

	// 认证方式过滤
	if opts.AuthType != "" {
		query = query.Where("auth_type = ?", opts.AuthType)
	}

	// 定时同步过滤
	if opts.SyncEnabled != nil {
		query = query.Where("sync_enabled = ?", *opts.SyncEnabled)
	}

	// 时间范围
	if opts.CreatedFrom != nil {
		query = query.Where("created_at >= ?", *opts.CreatedFrom)
	}
	if opts.CreatedTo != nil {
		query = query.Where("created_at <= ?", *opts.CreatedTo)
	}

	// 计数
	query.Count(&total)

	// 排序
	allowedSortFields := map[string]bool{
		"name": true, "status": true, "created_at": true,
		"updated_at": true, "last_sync_at": true,
	}
	if opts.SortField != "" && allowedSortFields[opts.SortField] {
		order := "ASC"
		if strings.ToLower(opts.SortOrder) == "desc" {
			order = "DESC"
		}
		query = query.Order(fmt.Sprintf("%s %s", opts.SortField, order))
	} else {
		query = query.Order("created_at DESC")
	}

	// 分页
	if opts.Page > 0 && opts.PageSize > 0 {
		query = query.Offset((opts.Page - 1) * opts.PageSize).Limit(opts.PageSize)
	}

	err := query.Find(&repos).Error
	return repos, total, err
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
	return r.db.WithContext(ctx).Model(&model.GitRepository{}).Where("id = ?", id).Updates(updates).Error
}

// UpdateBranches 更新分支列表
func (r *GitRepositoryRepository) UpdateBranches(ctx context.Context, id uuid.UUID, branches []string) error {
	// 将 []string 转换为 JSON 数组
	jsonBranches := make(model.JSONArray, len(branches))
	for i, b := range branches {
		jsonBranches[i] = b
	}
	return r.db.WithContext(ctx).Model(&model.GitRepository{}).Where("id = ?", id).Update("branches", jsonBranches).Error
}

// UpdateLastSync 更新最后同步时间
func (r *GitRepositoryRepository) UpdateLastSync(ctx context.Context, id uuid.UUID) error {
	return r.db.WithContext(ctx).Model(&model.GitRepository{}).Where("id = ?", id).
		Update("last_sync_at", gorm.Expr("NOW()")).Error
}

// CreateSyncLog 创建同步日志
func (r *GitRepositoryRepository) CreateSyncLog(ctx context.Context, log *model.GitSyncLog) error {
	return r.db.WithContext(ctx).Create(log).Error
}

// ListSyncLogs 获取同步日志列表
func (r *GitRepositoryRepository) ListSyncLogs(ctx context.Context, repoID uuid.UUID, page, pageSize int) ([]model.GitSyncLog, int64, error) {
	var logs []model.GitSyncLog
	var total int64

	query := r.db.WithContext(ctx).Model(&model.GitSyncLog{}).Where("repository_id = ?", repoID)
	query.Count(&total)

	err := query.Order("created_at DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&logs).Error
	return logs, total, err
}
