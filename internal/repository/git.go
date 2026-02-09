package repository

import (
	"context"

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

// List 获取仓库列表
func (r *GitRepositoryRepository) List(ctx context.Context, status string) ([]model.GitRepository, error) {
	var repos []model.GitRepository
	query := r.db.WithContext(ctx)

	if status != "" {
		query = query.Where("status = ?", status)
	}

	err := query.Order("created_at DESC").Find(&repos).Error
	return repos, err
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
