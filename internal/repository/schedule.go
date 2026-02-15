package repository

import (
	"context"
	"time"

	"github.com/company/auto-healing/internal/database"
	"github.com/company/auto-healing/internal/model"
	"github.com/google/uuid"
)

// ScheduleRepository 定时任务调度仓库
type ScheduleRepository struct{}

// ScheduleListOptions 调度列表筛选选项
type ScheduleListOptions struct {
	TaskID       *uuid.UUID
	Enabled      *bool
	Search       string  // 模糊搜索（匹配 name 或 description）
	ScheduleType *string // 调度类型：cron/once
	Status       *string // 状态筛选
	Page         int
	PageSize     int
}

// NewScheduleRepository 创建定时任务调度仓库
func NewScheduleRepository() *ScheduleRepository {
	return &ScheduleRepository{}
}

// Create 创建定时任务调度
func (r *ScheduleRepository) Create(ctx context.Context, schedule *model.ExecutionSchedule) error {
	return database.DB.WithContext(ctx).
		Select("id", "name", "task_id", "schedule_type", "schedule_expr", "scheduled_at", "status",
			"next_run_at", "last_run_at", "enabled", "description",
			"max_failures", "consecutive_failures", "pause_reason",
			"target_hosts_override", "extra_vars_override", "secrets_source_ids",
			"skip_notification", "created_at", "updated_at").
		Create(schedule).Error
}

// GetByID 根据 ID 获取定时任务调度
func (r *ScheduleRepository) GetByID(ctx context.Context, id uuid.UUID) (*model.ExecutionSchedule, error) {
	var schedule model.ExecutionSchedule
	err := database.DB.WithContext(ctx).
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

	query := database.DB.WithContext(ctx).Model(&model.ExecutionSchedule{})

	// 任务 ID 筛选
	if opts.TaskID != nil {
		query = query.Where("task_id = ?", *opts.TaskID)
	}

	// 启用状态筛选
	if opts.Enabled != nil {
		query = query.Where("enabled = ?", *opts.Enabled)
	}

	// 模糊搜索（匹配 name 或 description）
	if opts.Search != "" {
		searchPattern := "%" + opts.Search + "%"
		query = query.Where("name ILIKE ? OR description ILIKE ?", searchPattern, searchPattern)
	}

	// 调度类型筛选
	if opts.ScheduleType != nil {
		query = query.Where("schedule_type = ?", *opts.ScheduleType)
	}

	// 状态筛选
	if opts.Status != nil {
		query = query.Where("status = ?", *opts.Status)
	}

	query.Count(&total)

	offset := (opts.Page - 1) * opts.PageSize
	err := query.
		Preload("Task").
		Order("created_at DESC").
		Offset(offset).
		Limit(opts.PageSize).
		Find(&schedules).Error

	return schedules, total, err
}

// Update 更新定时任务调度
func (r *ScheduleRepository) Update(ctx context.Context, schedule *model.ExecutionSchedule) error {
	return database.DB.WithContext(ctx).
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
	return database.DB.WithContext(ctx).Delete(&model.ExecutionSchedule{}, "id = ?", id).Error
}

// GetDueSchedules 获取到期需要执行的调度
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
	return database.DB.WithContext(ctx).
		Model(&model.ExecutionSchedule{}).
		Where("id = ?", id).
		Update("next_run_at", nextRunAt).Error
}

// SetEnabled 设置启用状态
func (r *ScheduleRepository) SetEnabled(ctx context.Context, id uuid.UUID, enabled bool) error {
	return database.DB.WithContext(ctx).
		Model(&model.ExecutionSchedule{}).
		Where("id = ?", id).
		Update("enabled", enabled).Error
}

// ListByTaskID 根据任务 ID 列出调度
func (r *ScheduleRepository) ListByTaskID(ctx context.Context, taskID uuid.UUID) ([]model.ExecutionSchedule, error) {
	var schedules []model.ExecutionSchedule
	err := database.DB.WithContext(ctx).
		Where("task_id = ?", taskID).
		Find(&schedules).Error
	return schedules, err
}

// UpdateLastRunAt 更新上次执行时间
func (r *ScheduleRepository) UpdateLastRunAt(ctx context.Context, id uuid.UUID) error {
	return database.DB.WithContext(ctx).
		Model(&model.ExecutionSchedule{}).
		Where("id = ?", id).
		Update("last_run_at", time.Now()).Error
}
