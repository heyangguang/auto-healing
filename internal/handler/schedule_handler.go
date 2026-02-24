package handler

import (
	"strconv"
	"time"

	"github.com/company/auto-healing/internal/model"
	"github.com/company/auto-healing/internal/pkg/response"
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

// Create 创建定时任务调度
func (h *ScheduleHandler) Create(c *gin.Context) {
	var req CreateScheduleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	schedule, err := h.service.Create(c.Request.Context(), req.ToModel())
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	response.Created(c, schedule)
}

// List 列出定时任务调度（支持多条件筛选）
func (h *ScheduleHandler) List(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))

	opts := &repository.ScheduleListOptions{
		Name:      GetStringFilter(c, "name"),
		SortBy:    c.Query("sort_by"),
		SortOrder: c.Query("sort_order"),
		Page:      page,
		PageSize:  pageSize,
	}

	// 解析 task_id
	if taskIDStr := c.Query("task_id"); taskIDStr != "" {
		id, err := uuid.Parse(taskIDStr)
		if err == nil {
			opts.TaskID = &id
		}
	}

	// 解析 enabled
	if enabledStr := c.Query("enabled"); enabledStr != "" {
		e := enabledStr == "true"
		opts.Enabled = &e
	}

	// 解析 schedule_type
	if scheduleType := c.Query("schedule_type"); scheduleType != "" {
		opts.ScheduleType = &scheduleType
	}

	// 解析 status
	if status := c.Query("status"); status != "" {
		opts.Status = &status
	}

	// 解析 skip_notification
	if skipStr := c.Query("skip_notification"); skipStr != "" {
		skip := skipStr == "true"
		opts.SkipNotification = &skip
	}

	// 解析 has_overrides
	if hasOverridesStr := c.Query("has_overrides"); hasOverridesStr != "" {
		has := hasOverridesStr == "true"
		opts.HasOverrides = &has
	}

	// 解析 created_from / created_to
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

	schedules, total, err := h.service.List(c.Request.Context(), opts)
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}

	response.List(c, schedules, total, page, pageSize)
}

// Get 获取定时任务调度详情
func (h *ScheduleHandler) Get(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "无效的调度ID")
		return
	}

	schedule, err := h.service.Get(c.Request.Context(), id)
	if err != nil {
		response.NotFound(c, "调度不存在")
		return
	}

	response.Success(c, schedule)
}

// Update 更新定时任务调度
func (h *ScheduleHandler) Update(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "无效的调度ID")
		return
	}

	var req UpdateScheduleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	schedule, err := h.service.Update(c.Request.Context(), id, req.ToModel())
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	response.Success(c, schedule)
}

// Delete 删除定时任务调度
func (h *ScheduleHandler) Delete(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "无效的调度ID")
		return
	}

	if err := h.service.Delete(c.Request.Context(), id); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	response.Message(c, "删除成功")
}

// Enable 启用定时任务调度
func (h *ScheduleHandler) Enable(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "无效的调度ID")
		return
	}

	if err := h.service.Enable(c.Request.Context(), id); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	response.Message(c, "已启用")
}

// Disable 禁用定时任务调度
func (h *ScheduleHandler) Disable(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "无效的调度ID")
		return
	}

	if err := h.service.Disable(c.Request.Context(), id); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	response.Message(c, "已禁用")
}

// ==================== 统计 ====================

// GetStats 获取定时任务调度统计信息
func (h *ScheduleHandler) GetStats(c *gin.Context) {
	stats, err := h.service.GetStats(c.Request.Context())
	if err != nil {
		response.InternalError(c, "获取统计信息失败:"+err.Error())
		return
	}
	response.Success(c, stats)
}

// GetTimeline 获取调度时间线（轻量接口，用于可视化）
func (h *ScheduleHandler) GetTimeline(c *gin.Context) {
	// 解析 date 参数，默认今天（Asia/Shanghai）
	loc, _ := time.LoadLocation("Asia/Shanghai")
	date := time.Now().In(loc)
	if dateStr := c.Query("date"); dateStr != "" {
		if parsed, err := time.ParseInLocation("2006-01-02", dateStr, loc); err == nil {
			date = parsed
		} else {
			response.BadRequest(c, "date 参数格式错误，应为 YYYY-MM-DD")
			return
		}
	}

	var enabled *bool
	if enabledStr := c.Query("enabled"); enabledStr != "" {
		v := enabledStr == "true"
		enabled = &v
	}
	scheduleType := c.Query("schedule_type")

	items, err := h.service.ListTimeline(c.Request.Context(), date, enabled, scheduleType)
	if err != nil {
		response.InternalError(c, "获取时间线失败:"+err.Error())
		return
	}
	response.Success(c, items)
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
