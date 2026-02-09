package repository

import (
	"context"
	"time"

	"github.com/company/auto-healing/internal/database"
	"github.com/company/auto-healing/internal/model"
	"github.com/google/uuid"
)

// PlaybookRepository Playbook 仓库
type PlaybookRepository struct{}

// NewPlaybookRepository 创建 Playbook 仓库
func NewPlaybookRepository() *PlaybookRepository {
	return &PlaybookRepository{}
}

// ==================== Playbook CRUD ====================

// Create 创建 Playbook
func (r *PlaybookRepository) Create(ctx context.Context, playbook *model.Playbook) error {
	return database.DB.WithContext(ctx).Create(playbook).Error
}

// GetByID 根据 ID 获取 Playbook
func (r *PlaybookRepository) GetByID(ctx context.Context, id uuid.UUID) (*model.Playbook, error) {
	var playbook model.Playbook
	err := database.DB.WithContext(ctx).
		Preload("Repository").
		First(&playbook, "id = ?", id).Error
	if err != nil {
		return nil, err
	}
	return &playbook, nil
}

// List 列出 Playbooks
func (r *PlaybookRepository) List(ctx context.Context, repositoryID *uuid.UUID, status string, page, pageSize int) ([]model.Playbook, int64, error) {
	var playbooks []model.Playbook
	var total int64

	query := database.DB.WithContext(ctx).Model(&model.Playbook{})

	if repositoryID != nil {
		query = query.Where("repository_id = ?", *repositoryID)
	}
	if status != "" {
		query = query.Where("status = ?", status)
	}

	query.Count(&total)

	offset := (page - 1) * pageSize
	err := query.
		Preload("Repository").
		Order("created_at DESC").
		Offset(offset).
		Limit(pageSize).
		Find(&playbooks).Error

	return playbooks, total, err
}

// ListByRepositoryID 根据仓库 ID 列出 Playbooks
func (r *PlaybookRepository) ListByRepositoryID(ctx context.Context, repositoryID uuid.UUID) ([]model.Playbook, error) {
	var playbooks []model.Playbook
	err := database.DB.WithContext(ctx).
		Where("repository_id = ?", repositoryID).
		Find(&playbooks).Error
	return playbooks, err
}

// Update 更新 Playbook
func (r *PlaybookRepository) Update(ctx context.Context, playbook *model.Playbook) error {
	playbook.UpdatedAt = time.Now()
	// 使用 Select 明确指定要更新的列，避免 GORM 忽略外键字段
	return database.DB.WithContext(ctx).
		Model(playbook).
		Select("name", "file_path", "repository_id", "status", "config_mode", "variables", "scanned_variables", "default_extra_vars", "default_timeout_minutes", "tags", "updated_at").
		Updates(playbook).Error
}

// UpdateStatus 更新 Playbook 状态
func (r *PlaybookRepository) UpdateStatus(ctx context.Context, id uuid.UUID, status string) error {
	return database.DB.WithContext(ctx).
		Model(&model.Playbook{}).
		Where("id = ?", id).
		Updates(map[string]any{
			"status":     status,
			"updated_at": time.Now(),
		}).Error
}

// UpdateVariables 更新 Playbook 变量
func (r *PlaybookRepository) UpdateVariables(ctx context.Context, id uuid.UUID, variables model.JSONArray, scannedVariables model.JSONArray) error {
	now := time.Now()
	return database.DB.WithContext(ctx).
		Model(&model.Playbook{}).
		Where("id = ?", id).
		Updates(map[string]any{
			"variables":         variables,
			"scanned_variables": scannedVariables,
			"last_scanned_at":   now,
			"updated_at":        now,
		}).Error
}

// Delete 删除 Playbook
func (r *PlaybookRepository) Delete(ctx context.Context, id uuid.UUID) error {
	return database.DB.WithContext(ctx).Delete(&model.Playbook{}, "id = ?", id).Error
}

// CountByRepositoryID 统计仓库关联的 Playbook 数量
func (r *PlaybookRepository) CountByRepositoryID(ctx context.Context, repositoryID uuid.UUID) (int64, error) {
	var count int64
	err := database.DB.WithContext(ctx).
		Model(&model.Playbook{}).
		Where("repository_id = ?", repositoryID).
		Count(&count).Error
	return count, err
}

// ==================== 扫描日志 ====================

// CreateScanLog 创建扫描日志
func (r *PlaybookRepository) CreateScanLog(ctx context.Context, log *model.PlaybookScanLog) error {
	return database.DB.WithContext(ctx).Create(log).Error
}

// ListScanLogs 列出扫描日志
func (r *PlaybookRepository) ListScanLogs(ctx context.Context, playbookID uuid.UUID, page, pageSize int) ([]model.PlaybookScanLog, int64, error) {
	var logs []model.PlaybookScanLog
	var total int64

	query := database.DB.WithContext(ctx).Model(&model.PlaybookScanLog{}).Where("playbook_id = ?", playbookID)
	query.Count(&total)

	offset := (page - 1) * pageSize
	err := query.Order("created_at DESC").Offset(offset).Limit(pageSize).Find(&logs).Error
	return logs, total, err
}
