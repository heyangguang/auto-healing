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
type ScheduleRepository struct {
	db *gorm.DB
}

// ScheduleListOptions 调度列表筛选选项
type ScheduleListOptions struct {
	TaskID           *uuid.UUID
	Enabled          *bool
	Name             query.StringFilter
	ScheduleType     *string
	Status           *string
	SkipNotification *bool
	HasOverrides     *bool
	CreatedFrom      *time.Time
	CreatedTo        *time.Time
	SortBy           string
	SortOrder        string
	Page             int
	PageSize         int
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

// NewScheduleRepository 创建定时任务调度仓库
func NewScheduleRepository() *ScheduleRepository {
	return &ScheduleRepository{db: database.DB}
}

func (r *ScheduleRepository) tenantDB(ctx context.Context) *gorm.DB {
	return TenantDB(r.db, ctx)
}

// Create 创建定时任务调度
func (r *ScheduleRepository) Create(ctx context.Context, schedule *model.ExecutionSchedule) error {
	if err := FillTenantID(ctx, &schedule.TenantID); err != nil {
		return err
	}
	return r.db.WithContext(ctx).
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
	err := r.tenantDB(ctx).
		Preload("Task").
		Preload("Task.Playbook").
		First(&schedule, "id = ?", id).Error
	if err != nil {
		return nil, err
	}
	return &schedule, nil
}

// Update 更新定时任务调度
func (r *ScheduleRepository) Update(ctx context.Context, schedule *model.ExecutionSchedule) error {
	return r.tenantDB(ctx).
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
	return r.tenantDB(ctx).Delete(&model.ExecutionSchedule{}, "id = ?", id).Error
}

// GetDueSchedules 获取到期需要执行的调度（跨租户，调度器专用）
func (r *ScheduleRepository) GetDueSchedules(ctx context.Context) ([]model.ExecutionSchedule, error) {
	var schedules []model.ExecutionSchedule
	err := r.db.WithContext(ctx).
		Preload("Task").
		Preload("Task.Playbook").
		Where("enabled = ? AND next_run_at <= ?", true, time.Now()).
		Find(&schedules).Error
	return schedules, err
}

// UpdateNextRunAt 更新下次执行时间
func (r *ScheduleRepository) UpdateNextRunAt(ctx context.Context, id uuid.UUID, nextRunAt time.Time) error {
	return r.tenantDB(ctx).Model(&model.ExecutionSchedule{}).Where("id = ?", id).Update("next_run_at", nextRunAt).Error
}

// SetEnabled 设置启用状态
func (r *ScheduleRepository) SetEnabled(ctx context.Context, id uuid.UUID, enabled bool) error {
	return r.tenantDB(ctx).Model(&model.ExecutionSchedule{}).Where("id = ?", id).Update("enabled", enabled).Error
}

// ListByTaskID 根据任务 ID 列出调度
func (r *ScheduleRepository) ListByTaskID(ctx context.Context, taskID uuid.UUID) ([]model.ExecutionSchedule, error) {
	var schedules []model.ExecutionSchedule
	err := r.tenantDB(ctx).Where("task_id = ?", taskID).Find(&schedules).Error
	return schedules, err
}

// UpdateLastRunAt 更新上次执行时间
func (r *ScheduleRepository) UpdateLastRunAt(ctx context.Context, id uuid.UUID) error {
	return r.tenantDB(ctx).Model(&model.ExecutionSchedule{}).Where("id = ?", id).Update("last_run_at", time.Now()).Error
}
