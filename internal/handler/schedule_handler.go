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
		Search:   c.Query("search"),
		Page:     page,
		PageSize: pageSize,
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

// ==================== DTO ====================

// CreateScheduleRequest 创建调度请求
type CreateScheduleRequest struct {
	Name         string     `json:"name" binding:"required"`
	TaskID       uuid.UUID  `json:"task_id" binding:"required"`
	ScheduleType string     `json:"schedule_type" binding:"required"` // cron 或 once
	ScheduleExpr *string    `json:"schedule_expr"`                    // Cron 表达式（cron 模式必填）
	ScheduledAt  *time.Time `json:"scheduled_at"`                     // 执行时间（once 模式必填）
	Description  string     `json:"description"`
	Enabled      *bool      `json:"enabled"` // 默认 true

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

	return &model.ExecutionSchedule{
		Name:                r.Name,
		TaskID:              r.TaskID,
		ScheduleType:        r.ScheduleType,
		ScheduleExpr:        r.ScheduleExpr,
		ScheduledAt:         r.ScheduledAt,
		Description:         r.Description,
		Enabled:             enabled,
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

	return &model.ExecutionSchedule{
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
}
