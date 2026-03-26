package handler

import (
	"time"

	"github.com/company/auto-healing/internal/model"
	"github.com/company/auto-healing/internal/repository"
	"github.com/company/auto-healing/internal/service/schedule"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// ScheduleHandler 定时任务调度 Handler
type ScheduleHandler struct {
	service *schedule.Service
}

// NewScheduleHandler 创建 ScheduleHandler
func NewScheduleHandler() *ScheduleHandler {
	return &ScheduleHandler{
		service: schedule.NewService(),
	}
}

// ==================== DTO ====================

// CreateScheduleRequest 创建调度请求
type CreateScheduleRequest struct {
	Name         string     `json:"name" binding:"required"`
	TaskID       uuid.UUID  `json:"task_id" binding:"required"`
	ScheduleType string     `json:"schedule_type" binding:"required"` // cron 或 once
	ScheduleExpr *string    `json:"schedule_expr"`                    // Cron 表达式（cron 模式必填）
	ScheduledAt  *time.Time `json:"scheduled_at"`                     // 执行时间（once 模式必填）
	Description  string     `json:"description"`
	Enabled      *bool      `json:"enabled"`      // 默认 true
	MaxFailures  *int       `json:"max_failures"` // 最大连续失败次数，默认 5

	// 执行参数覆盖（可选）
	TargetHostsOverride string         `json:"target_hosts_override"`
	ExtraVarsOverride   map[string]any `json:"extra_vars_override"`
	SecretsSourceIDs    []uuid.UUID    `json:"secrets_source_ids"`
	SkipNotification    bool           `json:"skip_notification"`
}

// ToModel 转换为 Model
func (r *CreateScheduleRequest) ToModel() *model.ExecutionSchedule {
	var secretIDs model.StringArray
	if len(r.SecretsSourceIDs) > 0 {
		secretIDs = make(model.StringArray, len(r.SecretsSourceIDs))
		for i, id := range r.SecretsSourceIDs {
			secretIDs[i] = id.String()
		}
	}

	enabled := true
	if r.Enabled != nil {
		enabled = *r.Enabled
	}

	maxFailures := 5
	if r.MaxFailures != nil {
		maxFailures = *r.MaxFailures
	}

	return &model.ExecutionSchedule{
		Name:                r.Name,
		TaskID:              r.TaskID,
		ScheduleType:        r.ScheduleType,
		ScheduleExpr:        r.ScheduleExpr,
		ScheduledAt:         r.ScheduledAt,
		Description:         r.Description,
		Enabled:             enabled,
		MaxFailures:         maxFailures,
		TargetHostsOverride: r.TargetHostsOverride,
		ExtraVarsOverride:   model.JSON(r.ExtraVarsOverride),
		SecretsSourceIDs:    secretIDs,
		SkipNotification:    r.SkipNotification,
	}
}

// UpdateScheduleRequest 更新调度请求
type UpdateScheduleRequest struct {
	Name         string     `json:"name"`
	ScheduleType string     `json:"schedule_type"`
	ScheduleExpr *string    `json:"schedule_expr"`
	ScheduledAt  *time.Time `json:"scheduled_at"`
	Description  string     `json:"description"`
	MaxFailures  *int       `json:"max_failures"` // 最大连续失败次数

	// 执行参数覆盖（可选）
	TargetHostsOverride string         `json:"target_hosts_override"`
	ExtraVarsOverride   map[string]any `json:"extra_vars_override"`
	SecretsSourceIDs    []uuid.UUID    `json:"secrets_source_ids"`
	SkipNotification    bool           `json:"skip_notification"`
}

// ToModel 转换为 Model
func (r *UpdateScheduleRequest) ToModel() *model.ExecutionSchedule {
	var secretIDs model.StringArray
	if len(r.SecretsSourceIDs) > 0 {
		secretIDs = make(model.StringArray, len(r.SecretsSourceIDs))
		for i, id := range r.SecretsSourceIDs {
			secretIDs[i] = id.String()
		}
	}

	schedule := &model.ExecutionSchedule{
		Name:                r.Name,
		ScheduleType:        r.ScheduleType,
		ScheduleExpr:        r.ScheduleExpr,
		ScheduledAt:         r.ScheduledAt,
		Description:         r.Description,
		TargetHostsOverride: r.TargetHostsOverride,
		ExtraVarsOverride:   model.JSON(r.ExtraVarsOverride),
		SecretsSourceIDs:    secretIDs,
		SkipNotification:    r.SkipNotification,
	}
	if r.MaxFailures != nil {
		schedule.MaxFailures = *r.MaxFailures
	}
	return schedule
}

func buildScheduleListOptions(c *gin.Context, page, pageSize int) *repository.ScheduleListOptions {
	opts := &repository.ScheduleListOptions{
		Name:      GetStringFilter(c, "name"),
		SortBy:    c.Query("sort_by"),
		SortOrder: c.Query("sort_order"),
		Page:      page,
		PageSize:  pageSize,
	}
	parseScheduleUUIDFilters(c, opts)
	parseScheduleBoolFilters(c, opts)
	parseScheduleStringFilters(c, opts)
	parseScheduleDateFilters(c, opts)
	return opts
}

func parseScheduleUUIDFilters(c *gin.Context, opts *repository.ScheduleListOptions) {
	if taskIDStr := c.Query("task_id"); taskIDStr != "" {
		if id, err := uuid.Parse(taskIDStr); err == nil {
			opts.TaskID = &id
		}
	}
}

func parseScheduleBoolFilters(c *gin.Context, opts *repository.ScheduleListOptions) {
	if enabledStr := c.Query("enabled"); enabledStr != "" {
		enabled := enabledStr == "true"
		opts.Enabled = &enabled
	}
	if skipStr := c.Query("skip_notification"); skipStr != "" {
		skip := skipStr == "true"
		opts.SkipNotification = &skip
	}
	if hasOverridesStr := c.Query("has_overrides"); hasOverridesStr != "" {
		hasOverrides := hasOverridesStr == "true"
		opts.HasOverrides = &hasOverrides
	}
}

func parseScheduleStringFilters(c *gin.Context, opts *repository.ScheduleListOptions) {
	if scheduleType := c.Query("schedule_type"); scheduleType != "" {
		opts.ScheduleType = &scheduleType
	}
	if status := c.Query("status"); status != "" {
		opts.Status = &status
	}
}

func parseScheduleDateFilters(c *gin.Context, opts *repository.ScheduleListOptions) {
	if createdFromStr := c.Query("created_from"); createdFromStr != "" {
		if t, err := time.Parse(time.RFC3339, createdFromStr); err == nil {
			opts.CreatedFrom = &t
		}
	}
	if createdToStr := c.Query("created_to"); createdToStr != "" {
		if t, err := time.Parse(time.RFC3339, createdToStr); err == nil {
			opts.CreatedTo = &t
		}
	}
}

func buildScheduleTimelineDate(c *gin.Context) (time.Time, error) {
	loc, _ := time.LoadLocation("Asia/Shanghai")
	date := time.Now().In(loc)
	if dateStr := c.Query("date"); dateStr != "" {
		parsed, err := time.ParseInLocation("2006-01-02", dateStr, loc)
		if err != nil {
			return time.Time{}, err
		}
		date = parsed
	}
	return date, nil
}
