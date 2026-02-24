package repository

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/company/auto-healing/internal/database"
	"github.com/company/auto-healing/internal/model"
	"github.com/company/auto-healing/internal/pkg/query"
	"github.com/google/uuid"
	"gorm.io/gorm"
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
	if playbook.TenantID == nil {
		tenantID := TenantIDFromContext(ctx)
		playbook.TenantID = &tenantID
	}
	return database.DB.WithContext(ctx).Create(playbook).Error
}

// GetByID 根据 ID 获取 Playbook
func (r *PlaybookRepository) GetByID(ctx context.Context, id uuid.UUID) (*model.Playbook, error) {
	var playbook model.Playbook
	err := TenantDB(database.DB, ctx).
		Preload("Repository").
		First(&playbook, "id = ?", id).Error
	if err != nil {
		return nil, err
	}
	return &playbook, nil
}

// PlaybookListOptions Playbook 列表查询选项
type PlaybookListOptions struct {
	// 分页
	Page     int
	PageSize int

	// 搜索（支持精确/模糊匹配）
	Name     query.StringFilter // 按名称搜索
	FilePath query.StringFilter // 按入口文件路径搜索

	// 过滤
	RepositoryID    *uuid.UUID // 按仓库 ID
	Status          string     // ready / pending / error / outdated
	ConfigMode      string     // auto / enhanced
	HasVariables    *bool      // 是否包含变量
	MinVariables    *int       // 最小变量数量
	MaxVariables    *int       // 最大变量数量
	HasRequiredVars *bool      // 是否包含必填变量

	// 排序
	SortField string // name / status / config_mode / file_path / created_at / updated_at / last_scanned_at
	SortOrder string // asc / desc

	// 时间范围
	CreatedFrom *time.Time
	CreatedTo   *time.Time
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

// ListWithOptions 列出 Playbooks（支持完整查询参数）
func (r *PlaybookRepository) ListWithOptions(ctx context.Context, opts *PlaybookListOptions) ([]model.Playbook, int64, error) {
	var playbooks []model.Playbook
	var total int64

	q := TenantDB(database.DB, ctx).Model(&model.Playbook{})

	// 按名称搜索（支持精确/模糊匹配）
	if !opts.Name.IsEmpty() {
		q = query.ApplyStringFilter(q, "name", opts.Name)
	}

	// 按入口文件路径搜索（支持精确/模糊匹配）
	if !opts.FilePath.IsEmpty() {
		q = query.ApplyStringFilter(q, "file_path", opts.FilePath)
	}

	// 仓库 ID 过滤
	if opts.RepositoryID != nil {
		q = q.Where("repository_id = ?", *opts.RepositoryID)
	}

	// 状态过滤
	if opts.Status != "" {
		q = q.Where("status = ?", opts.Status)
	}

	// 配置模式过滤
	if opts.ConfigMode != "" {
		q = q.Where("config_mode = ?", opts.ConfigMode)
	}

	// 变量数量过滤
	if opts.HasVariables != nil {
		if *opts.HasVariables {
			q = q.Where("jsonb_array_length(COALESCE(variables, '[]'::jsonb)) > 0")
		} else {
			q = q.Where("jsonb_array_length(COALESCE(variables, '[]'::jsonb)) = 0")
		}
	}
	if opts.MinVariables != nil {
		q = q.Where("jsonb_array_length(COALESCE(variables, '[]'::jsonb)) >= ?", *opts.MinVariables)
	}
	if opts.MaxVariables != nil {
		q = q.Where("jsonb_array_length(COALESCE(variables, '[]'::jsonb)) <= ?", *opts.MaxVariables)
	}

	// 必填变量过滤
	if opts.HasRequiredVars != nil {
		if *opts.HasRequiredVars {
			q = q.Where("EXISTS (SELECT 1 FROM jsonb_array_elements(COALESCE(variables, '[]'::jsonb)) AS v WHERE (v->>'required')::boolean = true)")
		} else {
			q = q.Where("NOT EXISTS (SELECT 1 FROM jsonb_array_elements(COALESCE(variables, '[]'::jsonb)) AS v WHERE (v->>'required')::boolean = true)")
		}
	}

	// 时间范围
	if opts.CreatedFrom != nil {
		q = q.Where("created_at >= ?", *opts.CreatedFrom)
	}
	if opts.CreatedTo != nil {
		q = q.Where("created_at <= ?", *opts.CreatedTo)
	}

	// 计数
	q.Session(&gorm.Session{}).Count(&total)

	// 排序
	allowedSortFields := map[string]bool{
		"name": true, "status": true, "config_mode": true, "file_path": true,
		"created_at": true, "updated_at": true, "last_scanned_at": true,
	}
	if opts.SortField != "" && allowedSortFields[opts.SortField] {
		order := "ASC"
		if strings.ToLower(opts.SortOrder) == "desc" {
			order = "DESC"
		}
		q = q.Order(fmt.Sprintf("%s %s", opts.SortField, order))
	} else {
		q = q.Order("created_at DESC")
	}

	// 分页
	if opts.Page > 0 && opts.PageSize > 0 {
		q = q.Offset((opts.Page - 1) * opts.PageSize).Limit(opts.PageSize)
	}

	err := q.Preload("Repository").Find(&playbooks).Error
	return playbooks, total, err
}

// ListByRepositoryID 根据仓库 ID 列出 Playbooks
func (r *PlaybookRepository) ListByRepositoryID(ctx context.Context, repositoryID uuid.UUID) ([]model.Playbook, error) {
	var playbooks []model.Playbook
	err := TenantDB(database.DB, ctx).
		Where("repository_id = ?", repositoryID).
		Find(&playbooks).Error
	return playbooks, err
}

// Update 更新 Playbook
func (r *PlaybookRepository) Update(ctx context.Context, playbook *model.Playbook) error {
	playbook.UpdatedAt = time.Now()
	// 使用 Select 明确指定要更新的列，避免 GORM 忽略外键字段
	return TenantDB(database.DB, ctx).
		Model(playbook).
		Select("name", "file_path", "repository_id", "status", "config_mode", "variables", "scanned_variables", "default_extra_vars", "default_timeout_minutes", "tags", "updated_at").
		Updates(playbook).Error
}

// UpdateStatus 更新 Playbook 状态
func (r *PlaybookRepository) UpdateStatus(ctx context.Context, id uuid.UUID, status string) error {
	return TenantDB(database.DB, ctx).
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
	return TenantDB(database.DB, ctx).
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
	return TenantDB(database.DB, ctx).Delete(&model.Playbook{}, "id = ?", id).Error
}

// CountByRepositoryID 统计仓库关联的 Playbook 数量
func (r *PlaybookRepository) CountByRepositoryID(ctx context.Context, repositoryID uuid.UUID) (int64, error) {
	var count int64
	err := TenantDB(database.DB, ctx).
		Model(&model.Playbook{}).
		Where("repository_id = ?", repositoryID).
		Count(&count).Error
	return count, err
}

// ==================== 扫描日志 ====================

// CreateScanLog 创建扫描日志
func (r *PlaybookRepository) CreateScanLog(ctx context.Context, log *model.PlaybookScanLog) error {
	if log.TenantID == nil {
		tenantID := TenantIDFromContext(ctx)
		log.TenantID = &tenantID
	}
	return database.DB.WithContext(ctx).Create(log).Error
}

// ListScanLogs 列出扫描日志
func (r *PlaybookRepository) ListScanLogs(ctx context.Context, playbookID uuid.UUID, page, pageSize int) ([]model.PlaybookScanLog, int64, error) {
	var logs []model.PlaybookScanLog
	var total int64

	query := TenantDB(database.DB, ctx).Model(&model.PlaybookScanLog{}).Where("playbook_id = ?", playbookID)
	query.Session(&gorm.Session{}).Count(&total)

	offset := (page - 1) * pageSize
	err := query.Order("created_at DESC").Offset(offset).Limit(pageSize).Find(&logs).Error
	return logs, total, err
}

// ==================== 统计 ====================

// GetStats 获取 Playbook 统计信息
func (r *PlaybookRepository) GetStats(ctx context.Context) (map[string]interface{}, error) {
	stats := make(map[string]interface{})

	// 总数
	var total int64
	// 每次查询使用新的 TenantDB 实例，避免 GORM session WHERE 条件累积
	newDB := func() *gorm.DB { return TenantDB(database.DB, ctx) }
	if err := newDB().Model(&model.Playbook{}).Count(&total).Error; err != nil {
		return nil, err
	}
	stats["total"] = total

	// 按状态统计
	type StatusCount struct {
		Status string `json:"status"`
		Count  int64  `json:"count"`
	}
	var statusCounts []StatusCount
	newDB().Model(&model.Playbook{}).
		Select("status, count(*) as count").
		Group("status").
		Scan(&statusCounts)
	stats["by_status"] = statusCounts

	// 按配置模式统计
	type ConfigModeCount struct {
		ConfigMode string `json:"config_mode"`
		Count      int64  `json:"count"`
	}
	var configModeCounts []ConfigModeCount
	newDB().Model(&model.Playbook{}).
		Select("config_mode, count(*) as count").
		Group("config_mode").
		Scan(&configModeCounts)
	stats["by_config_mode"] = configModeCounts

	return stats, nil
}
