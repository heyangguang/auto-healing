package httpapi

import (
	"github.com/company/auto-healing/internal/pkg/response"
	"github.com/gin-gonic/gin"
)

// GetRunStats 获取执行记录统计概览
func (h *ExecutionHandler) GetRunStats(c *gin.Context) {
	stats, err := h.service.GetRunStats(c.Request.Context())
	if err != nil {
		respondInternalError(c, "EXEC", "获取执行记录统计失败", err)
		return
	}
	response.Success(c, stats)
}

// GetRunTrend 获取执行趋势
func (h *ExecutionHandler) GetRunTrend(c *gin.Context) {
	days := parsePositiveIntQuery(c, "days", 7, 365)
	items, err := h.service.GetRunTrend(c.Request.Context(), days)
	if err != nil {
		respondInternalError(c, "EXEC", "获取执行趋势失败", err)
		return
	}
	response.Success(c, items)
}

// GetTriggerDistribution 获取触发方式分布
func (h *ExecutionHandler) GetTriggerDistribution(c *gin.Context) {
	items, err := h.service.GetTriggerDistribution(c.Request.Context())
	if err != nil {
		respondInternalError(c, "EXEC", "获取触发方式分布失败", err)
		return
	}
	response.Success(c, items)
}

// GetTopFailedTasks 获取失败率最高的任务
func (h *ExecutionHandler) GetTopFailedTasks(c *gin.Context) {
	limit := parsePositiveIntQuery(c, "limit", 5, 100)
	items, err := h.service.GetTopFailedTasks(c.Request.Context(), limit)
	if err != nil {
		respondInternalError(c, "EXEC", "获取失败率最高任务失败", err)
		return
	}
	response.Success(c, items)
}

// GetTopActiveTasks 获取最活跃的任务
func (h *ExecutionHandler) GetTopActiveTasks(c *gin.Context) {
	limit := parsePositiveIntQuery(c, "limit", 5, 100)
	items, err := h.service.GetTopActiveTasks(c.Request.Context(), limit)
	if err != nil {
		respondInternalError(c, "EXEC", "获取最活跃任务失败", err)
		return
	}
	response.Success(c, items)
}

// GetTaskStats 获取任务模板统计概览
func (h *ExecutionHandler) GetTaskStats(c *gin.Context) {
	stats, err := h.service.GetTaskStats(c.Request.Context())
	if err != nil {
		respondInternalError(c, "EXEC", "获取任务模板统计失败", err)
		return
	}
	response.Success(c, stats)
}
