package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/company/auto-healing/internal/database"
	"github.com/company/auto-healing/internal/model"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// PlatformAuditLogRepository 平台级审计日志仓库（无租户隔离）
type PlatformAuditLogRepository struct {
	db *gorm.DB
}

// NewPlatformAuditLogRepository 创建平台级审计日志仓库
func NewPlatformAuditLogRepository() *PlatformAuditLogRepository {
	return &PlatformAuditLogRepository{db: database.DB}
}

// Create 创建平台级审计日志
func (r *PlatformAuditLogRepository) Create(ctx context.Context, log *model.PlatformAuditLog) error {
	return r.db.WithContext(ctx).Create(log).Error
}

// GetByID 根据 ID 获取平台级审计日志
func (r *PlatformAuditLogRepository) GetByID(ctx context.Context, id uuid.UUID) (*model.PlatformAuditLog, error) {
	var log model.PlatformAuditLog
	err := r.db.WithContext(ctx).First(&log, "id = ?", id).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &log, nil
}

// PlatformAuditListOptions 平台审计日志查询选项
type PlatformAuditListOptions struct {
	Page          int
	PageSize      int
	Search        string
	Category      string // login | operation
	Action        string
	ResourceType  string
	Username      string
	UserID        *uuid.UUID
	Status        string
	CreatedAfter  *time.Time
	CreatedBefore *time.Time
	SortBy        string
	SortOrder     string
}

// List 分页查询平台级审计日志
func (r *PlatformAuditLogRepository) List(ctx context.Context, opts *PlatformAuditListOptions) ([]model.PlatformAuditLog, int64, error) {
	var logs []model.PlatformAuditLog
	var total int64

	query := r.db.WithContext(ctx).Model(&model.PlatformAuditLog{})

	if opts.Category != "" {
		query = query.Where("category = ?", opts.Category)
	}
	if opts.Action != "" {
		query = query.Where("action = ?", opts.Action)
	}
	if opts.ResourceType != "" {
		query = query.Where("resource_type = ?", opts.ResourceType)
	}
	if opts.Username != "" {
		query = query.Where("username = ?", opts.Username)
	}
	if opts.UserID != nil {
		query = query.Where("user_id = ?", *opts.UserID)
	}
	if opts.Status != "" {
		query = query.Where("status = ?", opts.Status)
	}
	if opts.CreatedAfter != nil {
		query = query.Where("created_at >= ?", *opts.CreatedAfter)
	}
	if opts.CreatedBefore != nil {
		query = query.Where("created_at <= ?", *opts.CreatedBefore)
	}
	if opts.Search != "" {
		searchTerm := "%" + opts.Search + "%"
		query = query.Where(
			"username ILIKE ? OR resource_name ILIKE ? OR request_path ILIKE ?",
			searchTerm, searchTerm, searchTerm,
		)
	}

	if err := query.Session(&gorm.Session{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}

	// 排序
	sortBy := "created_at"
	sortOrder := "DESC"
	allowedSortFields := map[string]bool{
		"created_at":    true,
		"action":        true,
		"resource_type": true,
		"username":      true,
		"status":        true,
		"category":      true,
	}
	if opts.SortBy != "" && allowedSortFields[opts.SortBy] {
		sortBy = opts.SortBy
	}
	if opts.SortOrder == "asc" || opts.SortOrder == "ASC" {
		sortOrder = "ASC"
	}
	query = query.Order(fmt.Sprintf("%s %s", sortBy, sortOrder))

	offset := (opts.Page - 1) * opts.PageSize
	if err := query.Offset(offset).Limit(opts.PageSize).Find(&logs).Error; err != nil {
		return nil, 0, err
	}

	return logs, total, nil
}

// PlatformAuditStats 平台审计统计
type PlatformAuditStats struct {
	TotalCount   int64        `json:"total_count"`
	LoginCount   int64        `json:"login_count"`
	OperCount    int64        `json:"operation_count"`
	SuccessCount int64        `json:"success_count"`
	FailedCount  int64        `json:"failed_count"`
	TodayCount   int64        `json:"today_count"`
	WeekCount    int64        `json:"week_count"`
	ActionStats  []ActionStat `json:"action_stats"`
}

// GetStats 获取平台审计统计
func (r *PlatformAuditLogRepository) GetStats(ctx context.Context) (*PlatformAuditStats, error) {
	stats := &PlatformAuditStats{}

	newDB := func() *gorm.DB { return r.db.WithContext(ctx) }
	newDB().Model(&model.PlatformAuditLog{}).Count(&stats.TotalCount)
	newDB().Model(&model.PlatformAuditLog{}).Where("category = ?", "login").Count(&stats.LoginCount)
	newDB().Model(&model.PlatformAuditLog{}).Where("category = ?", "operation").Count(&stats.OperCount)
	newDB().Model(&model.PlatformAuditLog{}).Where("status = ?", "success").Count(&stats.SuccessCount)
	newDB().Model(&model.PlatformAuditLog{}).Where("status = ?", "failed").Count(&stats.FailedCount)

	now := time.Now()
	todayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	weekStart := todayStart.AddDate(0, 0, -int(now.Weekday()))
	newDB().Model(&model.PlatformAuditLog{}).Where("created_at >= ?", todayStart).Count(&stats.TodayCount)
	newDB().Model(&model.PlatformAuditLog{}).Where("created_at >= ?", weekStart).Count(&stats.WeekCount)

	newDB().Model(&model.PlatformAuditLog{}).
		Select("action, count(*) as count").
		Group("action").Order("count DESC").
		Scan(&stats.ActionStats)

	return stats, nil
}

// GetTrend 获取平台审计趋势
func (r *PlatformAuditLogRepository) GetTrend(ctx context.Context, days int) ([]TrendItem, error) {
	var items []TrendItem
	since := time.Now().AddDate(0, 0, -days)

	err := r.db.WithContext(ctx).Model(&model.PlatformAuditLog{}).
		Select("TO_CHAR(created_at, 'YYYY-MM-DD') as date, count(*) as count").
		Where("created_at >= ?", since).
		Group("TO_CHAR(created_at, 'YYYY-MM-DD')").
		Order("date ASC").
		Scan(&items).Error

	return items, err
}

// GetUserRanking 获取平台用户操作排行
func (r *PlatformAuditLogRepository) GetUserRanking(ctx context.Context, limit int, days int) ([]UserRanking, error) {
	var rankings []UserRanking

	query := r.db.WithContext(ctx).Model(&model.PlatformAuditLog{}).
		Select("user_id, username, count(*) as count")

	if days > 0 {
		since := time.Now().AddDate(0, 0, -days)
		query = query.Where("created_at >= ?", since)
	}

	err := query.
		Where("user_id IS NOT NULL").
		Group("user_id, username").
		Order("count DESC").
		Limit(limit).
		Scan(&rankings).Error

	return rankings, err
}

// GetHighRiskLogs 获取平台高危操作日志
func (r *PlatformAuditLogRepository) GetHighRiskLogs(ctx context.Context, page, pageSize int) ([]model.PlatformAuditLog, int64, error) {
	var logs []model.PlatformAuditLog
	var total int64

	condition := buildHighRiskCondition()
	query := r.db.WithContext(ctx).Model(&model.PlatformAuditLog{}).Where(condition)

	if err := query.Session(&gorm.Session{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * pageSize
	err := query.Order("created_at DESC").
		Offset(offset).Limit(pageSize).
		Find(&logs).Error

	return logs, total, err
}

// GetResourceTypeStats 按资源类型统计
func (r *PlatformAuditLogRepository) GetResourceTypeStats(ctx context.Context, days int) ([]ResourceTypeGroupItem, error) {
	var items []ResourceTypeGroupItem

	query := r.db.WithContext(ctx).Model(&model.PlatformAuditLog{}).
		Select("resource_type, count(*) as count")

	if days > 0 {
		since := time.Now().AddDate(0, 0, -days)
		query = query.Where("created_at >= ?", since)
	}

	err := query.Group("resource_type").Order("count DESC").Scan(&items).Error
	return items, err
}

// GetActionGrouping 按操作类型分组
func (r *PlatformAuditLogRepository) GetActionGrouping(ctx context.Context, action string, days int) ([]ActionGroupItem, error) {
	var items []ActionGroupItem

	query := r.db.WithContext(ctx).Model(&model.PlatformAuditLog{}).
		Select("action, resource_type, username, count(*) as count")

	if action != "" {
		query = query.Where("action = ?", action)
	}
	if days > 0 {
		since := time.Now().AddDate(0, 0, -days)
		query = query.Where("created_at >= ?", since)
	}

	err := query.Group("action, resource_type, username").Order("count DESC").Scan(&items).Error
	return items, err
}
