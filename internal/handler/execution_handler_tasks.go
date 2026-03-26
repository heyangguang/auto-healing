package handler

import (
	"github.com/company/auto-healing/internal/pkg/response"
	"github.com/company/auto-healing/internal/service/execution"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// CreateTask 创建任务模板
func (h *ExecutionHandler) CreateTask(c *gin.Context) {
	var req CreateTaskRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	task, err := h.service.CreateTask(c.Request.Context(), req.ToModel())
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.Created(c, task)
}

// ListTasks 列出任务模板（支持多条件筛选）
func (h *ExecutionHandler) ListTasks(c *gin.Context) {
	page, pageSize := parsePagination(c, 20)
	tasks, total, err := h.service.ListTasks(c.Request.Context(), buildTaskListOptions(c, page, pageSize))
	if err != nil {
		respondInternalError(c, "EXEC", "获取任务模板列表失败", err)
		return
	}
	response.List(c, tasks, total, page, pageSize)
}

// GetTask 获取任务模板详情
func (h *ExecutionHandler) GetTask(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "无效的任务ID")
		return
	}
	task, err := h.service.GetTask(c.Request.Context(), id)
	if err != nil {
		response.NotFound(c, "任务不存在")
		return
	}
	response.Success(c, task)
}

// DeleteTask 删除任务模板
func (h *ExecutionHandler) DeleteTask(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "无效的任务ID")
		return
	}
	if err := h.service.DeleteTask(c.Request.Context(), id); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.Message(c, "删除成功")
}

// UpdateTask 更新任务模板
func (h *ExecutionHandler) UpdateTask(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "无效的任务ID")
		return
	}

	var req CreateTaskRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	task, err := h.service.UpdateTask(c.Request.Context(), id, req.ToModel())
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.Success(c, task)
}

// ConfirmReview 确认审核变量变更
func (h *ExecutionHandler) ConfirmReview(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "无效的任务ID")
		return
	}
	task, err := h.service.ConfirmReview(c.Request.Context(), id)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.Success(c, task)
}

// ExecuteTask 执行任务
func (h *ExecutionHandler) ExecuteTask(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "无效的任务ID")
		return
	}

	var req ExecuteTaskRequest
	if err := c.ShouldBindJSON(&req); err != nil && c.Request.ContentLength > 0 {
		response.BadRequest(c, "请求参数错误: "+err.Error())
		return
	}
	run, err := h.service.ExecuteTask(c.Request.Context(), id, &execution.ExecuteOptions{
		TriggeredBy:      req.GetTriggeredBy(),
		SecretsSourceIDs: req.GetSecretsSourceIDs(),
		ExtraVars:        req.ExtraVars,
		TargetHosts:      req.TargetHosts,
		SkipNotification: req.SkipNotification,
	})
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.Success(c, run)
}

// ListRuns 列出任务的执行历史
func (h *ExecutionHandler) ListRuns(c *gin.Context) {
	taskID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "无效的任务ID")
		return
	}
	page, pageSize := parsePagination(c, 20)
	runs, total, err := h.service.GetRunsByTaskID(c.Request.Context(), taskID, page, pageSize)
	if err != nil {
		respondInternalError(c, "EXEC", "获取任务执行历史失败", err)
		return
	}
	response.List(c, runs, total, page, pageSize)
}

// BatchConfirmReview 批量确认审核
func (h *ExecutionHandler) BatchConfirmReview(c *gin.Context) {
	var req execution.BatchConfirmReviewRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "请求参数无效: "+err.Error())
		return
	}
	result, err := h.service.BatchConfirmReview(c.Request.Context(), &req)
	if err != nil {
		respondInternalError(c, "EXEC", "批量确认审核失败", err)
		return
	}
	response.Success(c, result)
}
