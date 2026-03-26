package repository

import (
	"context"
	"time"

	"github.com/company/auto-healing/internal/model"
	"github.com/company/auto-healing/internal/pkg/query"
	"gorm.io/gorm"
)

// List 列出定时任务调度（支持多条件筛选）
func (r *ScheduleRepository) List(ctx context.Context, opts *ScheduleListOptions) ([]model.ExecutionSchedule, int64, error) {
	query := r.buildScheduleListQuery(ctx, opts)
	total, err := countWithSession(query)
	if err != nil {
		return nil, 0, err
	}

	var schedules []model.ExecutionSchedule
	err = query.Preload("Task").
		Order(scheduleOrderClause(opts)).
		Offset((opts.Page - 1) * opts.PageSize).
		Limit(opts.PageSize).
		Find(&schedules).Error
	return schedules, total, err
}

func (r *ScheduleRepository) buildScheduleListQuery(ctx context.Context, opts *ScheduleListOptions) *gorm.DB {
	queryBuilder := r.tenantDB(ctx).Model(&model.ExecutionSchedule{})
	if opts.TaskID != nil {
		queryBuilder = queryBuilder.Where("task_id = ?", *opts.TaskID)
	}
	if opts.Enabled != nil {
		queryBuilder = queryBuilder.Where("enabled = ?", *opts.Enabled)
	}
	if !opts.Name.IsEmpty() {
		queryBuilder = query.ApplyStringFilter(queryBuilder, "name", opts.Name)
	}
	if opts.ScheduleType != nil {
		queryBuilder = queryBuilder.Where("schedule_type = ?", *opts.ScheduleType)
	}
	if opts.Status != nil {
		queryBuilder = queryBuilder.Where("status = ?", *opts.Status)
	}
	if opts.SkipNotification != nil {
		queryBuilder = queryBuilder.Where("skip_notification = ?", *opts.SkipNotification)
	}
	if opts.HasOverrides != nil {
		queryBuilder = applyScheduleOverrideFilter(queryBuilder, *opts.HasOverrides)
	}
	if opts.CreatedFrom != nil {
		queryBuilder = queryBuilder.Where("created_at >= ?", *opts.CreatedFrom)
	}
	if opts.CreatedTo != nil {
		queryBuilder = queryBuilder.Where("created_at <= ?", *opts.CreatedTo)
	}
	return queryBuilder
}

func applyScheduleOverrideFilter(queryBuilder *gorm.DB, hasOverrides bool) *gorm.DB {
	if hasOverrides {
		return queryBuilder.Where("(target_hosts_override != '' AND target_hosts_override IS NOT NULL) OR (extra_vars_override IS NOT NULL AND extra_vars_override != '{}' AND extra_vars_override != 'null') OR (secrets_source_ids IS NOT NULL AND secrets_source_ids != '[]' AND secrets_source_ids != 'null')")
	}
	return queryBuilder.Where("(target_hosts_override = '' OR target_hosts_override IS NULL) AND (extra_vars_override IS NULL OR extra_vars_override = '{}' OR extra_vars_override = 'null') AND (secrets_source_ids IS NULL OR secrets_source_ids = '[]' OR secrets_source_ids = 'null')")
}

func scheduleOrderClause(opts *ScheduleListOptions) string {
	dir := "ASC"
	if opts.SortOrder == "desc" {
		dir = "DESC"
	}
	switch opts.SortBy {
	case "name":
		return "name " + dir
	case "created_at":
		return "created_at " + dir
	case "next_run_at":
		return "next_run_at " + dir
	case "last_run_at":
		return "last_run_at " + dir
	default:
		return "created_at DESC"
	}
}

// ListTimeline 获取调度时间线（轻量接口，用于可视化）
func (r *ScheduleRepository) ListTimeline(ctx context.Context, date time.Time, enabled *bool, scheduleType string) ([]ScheduleTimelineItem, error) {
	var items []ScheduleTimelineItem
	dayStart := time.Date(date.Year(), date.Month(), date.Day(), 0, 0, 0, 0, date.Location())
	dayEnd := dayStart.AddDate(0, 0, 1)
	tenantID, err := RequireTenantID(ctx)
	if err != nil {
		return nil, err
	}

	queryBuilder := r.db.WithContext(ctx).Where("s.tenant_id = ?", tenantID).
		Table("execution_schedules AS s").
		Select("s.id, s.name, s.schedule_type, s.schedule_expr, s.scheduled_at, s.status, s.enabled, s.next_run_at, s.last_run_at, s.task_id, t.name AS task_name").
		Joins("LEFT JOIN execution_tasks t ON t.id = s.task_id").
		Where("(s.next_run_at >= ? AND s.next_run_at < ?) OR (s.last_run_at >= ? AND s.last_run_at < ?)", dayStart, dayEnd, dayStart, dayEnd)
	if enabled != nil {
		queryBuilder = queryBuilder.Where("s.enabled = ?", *enabled)
	}
	if scheduleType != "" {
		queryBuilder = queryBuilder.Where("s.schedule_type = ?", scheduleType)
	}
	err = queryBuilder.Order("s.next_run_at ASC NULLS LAST").Find(&items).Error
	return items, err
}
