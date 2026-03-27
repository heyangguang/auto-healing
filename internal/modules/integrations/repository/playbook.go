package repository

import (
	"context"
	"errors"
	"time"

	"github.com/company/auto-healing/internal/database"
	"github.com/company/auto-healing/internal/modules/integrations/model"
	"github.com/company/auto-healing/internal/pkg/query"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

var ErrPlaybookNotFound = errors.New("Playbook不存在")

// PlaybookRepository Playbook 仓库
type PlaybookRepository struct {
	db *gorm.DB
}

// PlaybookListOptions Playbook 列表查询选项
type PlaybookListOptions struct {
	Page            int
	PageSize        int
	Name            query.StringFilter
	FilePath        query.StringFilter
	RepositoryID    *uuid.UUID
	Status          string
	ConfigMode      string
	HasVariables    *bool
	MinVariables    *int
	MaxVariables    *int
	HasRequiredVars *bool
	SortField       string
	SortOrder       string
	CreatedFrom     *time.Time
	CreatedTo       *time.Time
}

// NewPlaybookRepository 创建 Playbook 仓库
func NewPlaybookRepository() *PlaybookRepository {
	return &PlaybookRepository{db: database.DB}
}

func (r *PlaybookRepository) tenantDB(ctx context.Context) *gorm.DB {
	return TenantDB(r.db, ctx)
}

// Create 创建 Playbook
func (r *PlaybookRepository) Create(ctx context.Context, playbook *model.Playbook) error {
	if err := FillTenantID(ctx, &playbook.TenantID); err != nil {
		return err
	}
	return r.db.WithContext(ctx).Create(playbook).Error
}

// GetByID 根据 ID 获取 Playbook
func (r *PlaybookRepository) GetByID(ctx context.Context, id uuid.UUID) (*model.Playbook, error) {
	var playbook model.Playbook
	err := r.tenantDB(ctx).Preload("Repository").First(&playbook, "id = ?", id).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrPlaybookNotFound
		}
		return nil, err
	}
	return &playbook, nil
}

// List 列出 Playbooks（向后兼容）
func (r *PlaybookRepository) List(ctx context.Context, repositoryID *uuid.UUID, status string, page, pageSize int) ([]model.Playbook, int64, error) {
	return r.ListWithOptions(ctx, &PlaybookListOptions{
		RepositoryID: repositoryID,
		Status:       status,
		Page:         page,
		PageSize:     pageSize,
	})
}

// ListByRepositoryID 根据仓库 ID 列出 Playbooks
func (r *PlaybookRepository) ListByRepositoryID(ctx context.Context, repositoryID uuid.UUID) ([]model.Playbook, error) {
	var playbooks []model.Playbook
	err := r.tenantDB(ctx).Where("repository_id = ?", repositoryID).Find(&playbooks).Error
	return playbooks, err
}

func (r *PlaybookRepository) UpdateMetadata(ctx context.Context, id uuid.UUID, name, description string) error {
	return r.tenantDB(ctx).
		Model(&model.Playbook{}).
		Where("id = ?", id).
		Updates(map[string]any{
			"name":        name,
			"description": description,
			"updated_at":  time.Now(),
		}).Error
}

func (r *PlaybookRepository) UpdateConfirmedVariables(ctx context.Context, id uuid.UUID, variables model.JSONArray) error {
	return r.tenantDB(ctx).
		Model(&model.Playbook{}).
		Where("id = ?", id).
		Updates(map[string]any{
			"variables":  variables,
			"updated_at": time.Now(),
		}).Error
}

// UpdateStatus 更新 Playbook 状态
func (r *PlaybookRepository) UpdateStatus(ctx context.Context, id uuid.UUID, status string) error {
	return r.tenantDB(ctx).
		Model(&model.Playbook{}).
		Where("id = ?", id).
		Updates(map[string]any{"status": status, "updated_at": time.Now()}).Error
}

// UpdateVariables 更新 Playbook 变量
func (r *PlaybookRepository) UpdateVariables(ctx context.Context, id uuid.UUID, variables model.JSONArray, scannedVariables model.JSONArray) error {
	now := time.Now()
	return r.tenantDB(ctx).
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
	return r.tenantDB(ctx).Delete(&model.Playbook{}, "id = ?", id).Error
}

// CountByRepositoryID 统计仓库关联的 Playbook 数量
func (r *PlaybookRepository) CountByRepositoryID(ctx context.Context, repositoryID uuid.UUID) (int64, error) {
	var count int64
	err := r.tenantDB(ctx).Model(&model.Playbook{}).Where("repository_id = ?", repositoryID).Count(&count).Error
	return count, err
}

// CreateScanLog 创建扫描日志
func (r *PlaybookRepository) CreateScanLog(ctx context.Context, log *model.PlaybookScanLog) error {
	if err := FillTenantID(ctx, &log.TenantID); err != nil {
		return err
	}
	return r.db.WithContext(ctx).Create(log).Error
}

// ListScanLogs 列出扫描日志
func (r *PlaybookRepository) ListScanLogs(ctx context.Context, playbookID uuid.UUID, page, pageSize int) ([]model.PlaybookScanLog, int64, error) {
	var logs []model.PlaybookScanLog
	query := r.tenantDB(ctx).Model(&model.PlaybookScanLog{}).Where("playbook_id = ?", playbookID)
	total, err := countWithSession(query)
	if err != nil {
		return nil, 0, err
	}
	err = query.Order("created_at DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&logs).Error
	return logs, total, err
}
