package httpapi

import (
	"github.com/company/auto-healing/internal/pkg/response"
	"github.com/company/auto-healing/internal/repository"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// Create 创建定时任务调度
func (h *ScheduleHandler) Create(c *gin.Context) {
	var req CreateScheduleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	if err := validateScheduleCreateRequest(&req); err != nil {
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

// List 列出定时任务调度
func (h *ScheduleHandler) List(c *gin.Context) {
	page, pageSize := parsePagination(c, 20)
	opts := buildScheduleListOptions(c, page, pageSize)

	schedules, total, err := h.service.List(c.Request.Context(), opts)
	if err != nil {
		respondInternalError(c, "SCHEDULE", "获取调度列表失败", err)
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
		respondResourceError(c, "SCHEDULE", "获取调度详情失败", "调度不存在", repository.ErrScheduleNotFound, resourceErrorModeInternal, err)
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
	if err := validateScheduleUpdateRequest(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	schedule, err := h.service.Update(c.Request.Context(), id, req.ToUpdateInput())
	if err != nil {
		respondResourceError(c, "SCHEDULE", "更新调度失败", "调度不存在", repository.ErrScheduleNotFound, resourceErrorModeBadRequest, err)
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
		respondResourceError(c, "SCHEDULE", "删除调度失败", "调度不存在", repository.ErrScheduleNotFound, resourceErrorModeBadRequest, err)
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
		respondResourceError(c, "SCHEDULE", "启用调度失败", "调度不存在", repository.ErrScheduleNotFound, resourceErrorModeBadRequest, err)
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
		respondResourceError(c, "SCHEDULE", "禁用调度失败", "调度不存在", repository.ErrScheduleNotFound, resourceErrorModeBadRequest, err)
		return
	}
	response.Message(c, "已禁用")
}
