package repository

import (
	"context"
	"time"

	"github.com/company/auto-healing/internal/database"
	"github.com/company/auto-healing/internal/model"
	"github.com/company/auto-healing/internal/pkg/query"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// ScheduleRepository 定时任务调度仓库
type ScheduleRepository struct{}

// ScheduleListOptions 调度列表筛选选项
type ScheduleListOptions struct {
	TaskID           *uuid.UUID
	Enabled          *bool
	Name             query.StringFilter // 名称搜索（支持精确/模糊匹配）
	ScheduleType     *string            // 调度类型：cron/once
	Status           *string            // 状态筛选
	SkipNotification *bool              // 是否跳过通知
	HasOverrides     *bool              // 是否有执行覆盖参数
	// 时间范围
	CreatedFrom *time.Time // 创建时间范围起始
	CreatedTo   *time.Time // 创建时间范围结束
	// 排序
	SortBy    string // 排序字段：name / created_at / next_run_at / last_run_at
	SortOrder string // 排序方向：asc / desc
	Page      int
	PageSize  int
}

// NewScheduleRepository 创建定时任务调度仓库
func NewScheduleRepository() *ScheduleRepository {
	return &ScheduleRepository{}
}

// Create 创建定时任务调度
func (r *ScheduleRepository) Create(ctx context.Context, schedule *model.ExecutionSchedule) error {
	// 自动填充租户 ID
	if schedule.TenantID == nil {
		tenantID := TenantIDFromContext(ctx)
		schedule.TenantID = &tenantID
	}
	return database.DB.WithContext(ctx).
		Select("id", "tenant_id", "name", "task_id", "schedule_type", "schedule_expr", "scheduled_at", "status",
			"next_run_at", "last_run_at", "enabled", "description",
			"max_failures", "consecutive_failures", "pause_reason",
			"target_hosts_override", "extra_vars_override", "secrets_source_ids",
			"skip_notification", "created_at", "updated_at").
		Create(schedule).Error
}

// GetByID 根据 ID 获取定时任务调度
func (r *ScheduleRepository) GetByID(ctx context.Context, id uuid.UUID) (*model.ExecutionSchedule, error) {
	var schedule model.ExecutionSchedule
	err := TenantDB(database.DB, ctx).
		Preload("Task").
		Preload("Task.Playbook").
		First(&schedule, "id = ?", id).Error
	if err != nil {
		return nil, err
	}
	return &schedule, nil
}

// List 列出定时任务调度（支持多条件筛选）
func (r *ScheduleRepository) List(ctx context.Context, opts *ScheduleListOptions) ([]model.ExecutionSchedule, int64, error) {
	var schedules []model.ExecutionSchedule
	var total int64

	q := TenantDB(database.DB, ctx).Model(&model.ExecutionSchedule{})

	// 任务 ID 筛选
	if opts.TaskID != nil {
		q = q.Where("task_id = ?", *opts.TaskID)
	}

	// 启用状态筛选
	if opts.Enabled != nil {
		q = q.Where("enabled = ?", *opts.Enabled)
	}

	// 名称搜索（支持精确/模糊匹配）
	if !opts.Name.IsEmpty() {
		if opts.Name.Exact {
			q = q.Where("name = ?", opts.Name.Value)
		} else {
			q = q.Where("name ILIKE ?", "%"+opts.Name.Value+"%")
		}
	}

	// 调度类型筛选
	if opts.ScheduleType != nil {
		q = q.Where("schedule_type = ?", *opts.ScheduleType)
	}

	// 状态筛选
	if opts.Status != nil {
		q = q.Where("status = ?", *opts.Status)
	}

	// 跳过通知筛选
	if opts.SkipNotification != nil {
		q = q.Where("skip_notification = ?", *opts.SkipNotification)
	}

	// 是否有执行覆盖参数
	if opts.HasOverrides != nil {
		if *opts.HasOverrides {
			q = q.Where("(target_hosts_override != '' AND target_hosts_override IS NOT NULL) OR (extra_vars_override IS NOT NULL AND extra_vars_override != '{}' AND extra_vars_override != 'null') OR (secrets_source_ids IS NOT NULL AND secrets_source_ids != '[]' AND secrets_source_ids != 'null')")
		} else {
			q = q.Where("(target_hosts_override = '' OR target_hosts_override IS NULL) AND (extra_vars_override IS NULL OR extra_vars_override = '{}' OR extra_vars_override = 'null') AND (secrets_source_ids IS NULL OR secrets_source_ids = '[]' OR secrets_source_ids = 'null')")
		}
	}

	// 创建时间范围过滤
	if opts.CreatedFrom != nil {
		q = q.Where("created_at >= ?", *opts.CreatedFrom)
	}
	if opts.CreatedTo != nil {
		q = q.Where("created_at <= ?", *opts.CreatedTo)
	}

	q.Session(&gorm.Session{}).Count(&total)

	// 排序
	orderClause := "created_at DESC" // 默认排序
	if opts.SortBy != "" {
		dir := "ASC"
		if opts.SortOrder == "desc" {
			dir = "DESC"
		}
		switch opts.SortBy {
		case "name":
			orderClause = "name " + dir
		case "created_at":
			orderClause = "created_at " + dir
		case "next_run_at":
			orderClause = "next_run_at " + dir
		case "last_run_at":
			orderClause = "last_run_at " + dir
		}
	}

	offset := (opts.Page - 1) * opts.PageSize
	err := q.
		Preload("Task").
		Order(orderClause).
		Offset(offset).
		Limit(opts.PageSize).
		Find(&schedules).Error

	return schedules, total, err
}

// Update 更新定时任务调度
func (r *ScheduleRepository) Update(ctx context.Context, schedule *model.ExecutionSchedule) error {
	return TenantDB(database.DB, ctx).
		Model(schedule).
		Select("name", "task_id", "schedule_type", "schedule_expr", "scheduled_at", "status",
			"next_run_at", "last_run_at", "enabled", "description",
			"max_failures", "consecutive_failures", "pause_reason",
			"target_hosts_override", "extra_vars_override", "secrets_source_ids",
			"skip_notification", "updated_at").
		Updates(schedule).Error
}

// Delete 删除定时任务调度
func (r *ScheduleRepository) Delete(ctx context.Context, id uuid.UUID) error {
	return TenantDB(database.DB, ctx).Delete(&model.ExecutionSchedule{}, "id = ?", id).Error
}

// GetDueSchedules 获取到期需要执行的调度（跨租户，调度器专用）
// 注意：此函数不使用 TenantDB，因为调度器需要处理所有租户的调度
func (r *ScheduleRepository) GetDueSchedules(ctx context.Context) ([]model.ExecutionSchedule, error) {
	var schedules []model.ExecutionSchedule
	err := database.DB.WithContext(ctx).
		Preload("Task").
		Preload("Task.Playbook").
		Where("enabled = ? AND next_run_at <= ?", true, time.Now()).
		Find(&schedules).Error
	return schedules, err
}

// UpdateNextRunAt 更新下次执行时间
func (r *ScheduleRepository) UpdateNextRunAt(ctx context.Context, id uuid.UUID, nextRunAt time.Time) error {
	return TenantDB(database.DB, ctx).
		Model(&model.ExecutionSchedule{}).
		Where("id = ?", id).
		Update("next_run_at", nextRunAt).Error
}

// SetEnabled 设置启用状态
func (r *ScheduleRepository) SetEnabled(ctx context.Context, id uuid.UUID, enabled bool) error {
	return TenantDB(database.DB, ctx).
		Model(&model.ExecutionSchedule{}).
		Where("id = ?", id).
		Update("enabled", enabled).Error
}

// ListByTaskID 根据任务 ID 列出调度
func (r *ScheduleRepository) ListByTaskID(ctx context.Context, taskID uuid.UUID) ([]model.ExecutionSchedule, error) {
	var schedules []model.ExecutionSchedule
	err := TenantDB(database.DB, ctx).
		Where("task_id = ?", taskID).
		Find(&schedules).Error
	return schedules, err
}

// UpdateLastRunAt 更新上次执行时间
func (r *ScheduleRepository) UpdateLastRunAt(ctx context.Context, id uuid.UUID) error {
	return TenantDB(database.DB, ctx).
		Model(&model.ExecutionSchedule{}).
		Where("id = ?", id).
		Update("last_run_at", time.Now()).Error
}

// ==================== 统计 ====================

// GetStats 获取定时任务调度统计信息
func (r *ScheduleRepository) GetStats(ctx context.Context) (map[string]interface{}, error) {
	stats := make(map[string]interface{})

	// 总数
	var total int64
	// 每次查询使用新的 TenantDB 实例，避免 GORM session WHERE 条件累积
	newDB := func() *gorm.DB { return TenantDB(database.DB, ctx) }
	if err := newDB().Model(&model.ExecutionSchedule{}).Count(&total).Error; err != nil {
		return nil, err
	}
	stats["total"] = total

	// 按状态统计
	type StatusCount struct {
		Status string `json:"status"`
		Count  int64  `json:"count"`
	}
	var statusCounts []StatusCount
	newDB().Model(&model.ExecutionSchedule{}).
		Select("status, count(*) as count").
		Group("status").
		Scan(&statusCounts)
	stats["by_status"] = statusCounts

	// 按调度类型统计
	type ScheduleTypeCount struct {
		ScheduleType string `json:"schedule_type"`
		Count        int64  `json:"count"`
	}
	var scheduleTypeCounts []ScheduleTypeCount
	newDB().Model(&model.ExecutionSchedule{}).
		Select("schedule_type, count(*) as count").
		Group("schedule_type").
		Scan(&scheduleTypeCounts)
	stats["by_schedule_type"] = scheduleTypeCounts

	// 启用/禁用统计
	var enabledCount int64
	newDB().Model(&model.ExecutionSchedule{}).
		Where("enabled = ?", true).
		Count(&enabledCount)
	stats["enabled_count"] = enabledCount
	stats["disabled_count"] = total - enabledCount

	return stats, nil
}

// ScheduleTimelineItem 调度时间线项（轻量）
type ScheduleTimelineItem struct {
	ID           uuid.UUID  `json:"id"`
	Name         string     `json:"name"`
	ScheduleType string     `json:"schedule_type"`
	ScheduleExpr *string    `json:"schedule_expr,omitempty"`
	ScheduledAt  *time.Time `json:"scheduled_at,omitempty"`
	Status       string     `json:"status"`
	Enabled      bool       `json:"enabled"`
	NextRunAt    *time.Time `json:"next_run_at,omitempty"`
	LastRunAt    *time.Time `json:"last_run_at,omitempty"`
	TaskID       uuid.UUID  `json:"task_id"`
	TaskName     string     `json:"task_name"`
}

// ListTimeline 获取调度时间线（轻量接口，用于可视化）
func (r *ScheduleRepository) ListTimeline(ctx context.Context, date time.Time, enabled *bool, scheduleType string) ([]ScheduleTimelineItem, error) {
	var items []ScheduleTimelineItem

	// 计算日期范围：[dayStart, dayEnd)
	dayStart := time.Date(date.Year(), date.Month(), date.Day(), 0, 0, 0, 0, date.Location())
	dayEnd := dayStart.AddDate(0, 0, 1)

	tenantID := TenantIDFromContext(ctx)
	query := database.DB.WithContext(ctx).Where("s.tenant_id = ?", tenantID).
		Table("execution_schedules AS s").
		Select("s.id, s.name, s.schedule_type, s.schedule_expr, s.scheduled_at, s.status, s.enabled, s.next_run_at, s.last_run_at, s.task_id, t.name AS task_name").
		Joins("LEFT JOIN execution_tasks t ON t.id = s.task_id").
		Where("(s.next_run_at >= ? AND s.next_run_at < ?) OR (s.last_run_at >= ? AND s.last_run_at < ?)",
			dayStart, dayEnd, dayStart, dayEnd)

	if enabled != nil {
		query = query.Where("s.enabled = ?", *enabled)
	}
	if scheduleType != "" {
		query = query.Where("s.schedule_type = ?", scheduleType)
	}

	err := query.Order("s.next_run_at ASC NULLS LAST").Find(&items).Error
	return items, err
}
