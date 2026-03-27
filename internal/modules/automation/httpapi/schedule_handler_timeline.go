package httpapi

import (
	"github.com/company/auto-healing/internal/pkg/response"
	"github.com/gin-gonic/gin"
)

// GetStats 获取定时任务调度统计信息
func (h *ScheduleHandler) GetStats(c *gin.Context) {
	stats, err := h.service.GetStats(c.Request.Context())
	if err != nil {
		respondInternalError(c, "SCHEDULE", "获取统计信息失败", err)
		return
	}
	response.Success(c, stats)
}

// GetTimeline 获取调度时间线（轻量接口，用于可视化）
func (h *ScheduleHandler) GetTimeline(c *gin.Context) {
	date, err := buildScheduleTimelineDate(c)
	if err != nil {
		response.BadRequest(c, "date 参数格式错误，应为 YYYY-MM-DD")
		return
	}

	var enabled *bool
	if enabledStr := c.Query("enabled"); enabledStr != "" {
		value := enabledStr == "true"
		enabled = &value
	}

	items, err := h.service.ListTimeline(c.Request.Context(), date, enabled, c.Query("schedule_type"))
	if err != nil {
		respondInternalError(c, "SCHEDULE", "获取时间线失败", err)
		return
	}
	response.Success(c, items)
}
