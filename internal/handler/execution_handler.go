package handler

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/company/auto-healing/internal/pkg/response"
	"github.com/company/auto-healing/internal/repository"
	"github.com/company/auto-healing/internal/service/execution"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// ExecutionHandler 执行任务 Handler
type ExecutionHandler struct {
	service *execution.Service
}

// NewExecutionHandler 创建 ExecutionHandler
func NewExecutionHandler() *ExecutionHandler {
	return &ExecutionHandler{
		service: execution.NewService(),
	}
}

// ==================== Search Schema 声明 ====================

var taskSearchSchema = []SearchableField{
	{Key: "name", Label: "模板名称", Type: "text", MatchModes: []string{"fuzzy", "exact"}, DefaultMode: "fuzzy", Placeholder: "输入模板名称", Column: "name"},
	{Key: "description", Label: "任务描述", Type: "text", MatchModes: []string{"fuzzy", "exact"}, DefaultMode: "fuzzy", Placeholder: "输入任务描述", Column: "description"},
	{Key: "executor_type", Label: "执行器类型", Type: "enum", MatchModes: []string{"exact"}, DefaultMode: "exact", Options: []FilterOption{
		{Label: "Ansible", Value: "ansible"}, {Label: "Shell", Value: "shell"},
	}},
	{Key: "status", Label: "模板状态", Type: "enum", MatchModes: []string{"exact"}, DefaultMode: "exact", Options: []FilterOption{
		{Label: "就绪", Value: "ready"}, {Label: "草稿", Value: "draft"},
		{Label: "待审核", Value: "pending_review"}, {Label: "已下线", Value: "offline"},
	}},
	{Key: "needs_review", Label: "需要审核", Type: "boolean", MatchModes: []string{"exact"}, DefaultMode: "exact"},
	{Key: "has_runs", Label: "有执行记录", Type: "boolean", MatchModes: []string{"exact"}, DefaultMode: "exact"},
}

var runSearchSchema = []SearchableField{
	{Key: "run_id", Label: "执行记录 ID", Type: "text", MatchModes: []string{"exact"}, DefaultMode: "exact", Placeholder: "输入完整 UUID 或前 8 位短 ID"},
	{Key: "task_name", Label: "任务名称", Type: "text", MatchModes: []string{"fuzzy", "exact"}, DefaultMode: "fuzzy", Placeholder: "输入任务名称"},
	{Key: "status", Label: "执行状态", Type: "enum", MatchModes: []string{"exact"}, DefaultMode: "exact", Options: []FilterOption{
		{Label: "运行中", Value: "running"}, {Label: "成功", Value: "success"},
		{Label: "失败", Value: "failed"}, {Label: "已取消", Value: "cancelled"},
	}},
	{Key: "triggered_by", Label: "触发方式", Type: "enum", MatchModes: []string{"exact"}, DefaultMode: "exact", Options: []FilterOption{
		{Label: "手动", Value: "manual"}, {Label: "定时", Value: "schedule"},
		{Label: "自愈", Value: "healing"}, {Label: "API", Value: "api"},
	}},
}

// GetTaskSearchSchema 返回任务模板搜索 schema
func (h *ExecutionHandler) GetTaskSearchSchema(c *gin.Context) {
	response.Success(c, gin.H{"fields": taskSearchSchema})
}

// GetRunSearchSchema 返回执行记录搜索 schema
func (h *ExecutionHandler) GetRunSearchSchema(c *gin.Context) {
	response.Success(c, gin.H{"fields": runSearchSchema})
}

// ==================== 任务模板接口 ====================

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
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))

	opts := &repository.TaskListOptions{
		Name:           GetStringFilter(c, "name"),
		Description:    GetStringFilter(c, "description"),
		ExecutorType:   c.Query("executor_type"),
		Status:         c.Query("status"),
		TargetHosts:    c.Query("target_hosts"),
		PlaybookName:   c.Query("playbook_name"),
		RepositoryName: c.Query("repository_name"),
		SortBy:         c.Query("sort_by"),
		SortOrder:      c.Query("sort_order"),
		Page:           page,
		PageSize:       pageSize,
	}

	// 解析 playbook_id
	if playbookIDStr := c.Query("playbook_id"); playbookIDStr != "" {
		id, err := uuid.Parse(playbookIDStr)
		if err == nil {
			opts.PlaybookID = &id
		}
	}

	// 解析 needs_review (bool)
	if needsReviewStr := c.Query("needs_review"); needsReviewStr != "" {
		needsReview := needsReviewStr == "true"
		opts.NeedsReview = &needsReview
	}

	// 解析 created_from / created_to (时间范围)
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

	// 解析 has_runs (bool)
	if hasRunsStr := c.Query("has_runs"); hasRunsStr != "" {
		hasRuns := hasRunsStr == "true"
		opts.HasRuns = &hasRuns
	}

	// 解析 min_run_count (int)
	if minRunCountStr := c.Query("min_run_count"); minRunCountStr != "" {
		if count, err := strconv.Atoi(minRunCountStr); err == nil && count > 0 {
			opts.MinRunCount = &count
		}
	}

	// 解析 last_run_status (string)
	opts.LastRunStatus = c.Query("last_run_status")

	tasks, total, err := h.service.ListTasks(c.Request.Context(), opts)
	if err != nil {
		response.InternalError(c, err.Error())
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

// ==================== 执行接口 ====================

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

	opts := &execution.ExecuteOptions{
		TriggeredBy:      req.GetTriggeredBy(),
		SecretsSourceIDs: req.GetSecretsSourceIDs(),
		ExtraVars:        req.ExtraVars,
		TargetHosts:      req.TargetHosts,
		SkipNotification: req.SkipNotification,
	}

	run, err := h.service.ExecuteTask(c.Request.Context(), id, opts)
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

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))

	runs, total, err := h.service.GetRunsByTaskID(c.Request.Context(), taskID, page, pageSize)
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}

	response.List(c, runs, total, page, pageSize)
}

// ==================== 执行记录接口 ====================

// ListAllRuns 获取所有执行记录列表
func (h *ExecutionHandler) ListAllRuns(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))

	opts := &repository.RunListOptions{
		RunID:       c.Query("run_id"),
		TaskName:    GetStringFilter(c, "task_name"),
		Status:      c.Query("status"),
		TriggeredBy: c.Query("triggered_by"),
		Page:        page,
		PageSize:    pageSize,
	}

	// 解析 task_id
	if taskIDStr := c.Query("task_id"); taskIDStr != "" {
		id, err := uuid.Parse(taskIDStr)
		if err == nil {
			opts.TaskID = &id
		}
	}

	// 解析时间范围
	if startedAfter := c.Query("started_after"); startedAfter != "" {
		t, err := time.Parse(time.RFC3339, startedAfter)
		if err == nil {
			opts.StartedAfter = &t
		}
	}
	if startedBefore := c.Query("started_before"); startedBefore != "" {
		t, err := time.Parse(time.RFC3339, startedBefore)
		if err == nil {
			opts.StartedBefore = &t
		}
	}

	runs, total, err := h.service.ListAllRuns(c.Request.Context(), opts)
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}

	response.List(c, runs, total, page, pageSize)
}

// GetRun 获取执行记录详情
func (h *ExecutionHandler) GetRun(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "无效的执行记录ID")
		return
	}

	run, err := h.service.GetRun(c.Request.Context(), id)
	if err != nil {
		response.NotFound(c, "执行记录不存在")
		return
	}

	response.Success(c, run)
}

// GetRunLogs 获取执行日志
func (h *ExecutionHandler) GetRunLogs(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "无效的执行记录ID")
		return
	}

	logs, err := h.service.GetRunLogs(c.Request.Context(), id)
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}

	response.Success(c, logs)
}

// CancelRun 取消执行
func (h *ExecutionHandler) CancelRun(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "无效的执行记录ID")
		return
	}

	if err := h.service.CancelRun(c.Request.Context(), id); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	response.Message(c, "执行已取消")
}

// StreamLogs SSE 实时日志流
func (h *ExecutionHandler) StreamLogs(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "无效的执行记录ID")
		return
	}

	// 设置 SSE 头 - 必须在任何写入之前设置
	c.Writer.Header().Set("Content-Type", "text/event-stream")
	c.Writer.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
	c.Writer.Header().Set("Connection", "keep-alive")
	c.Writer.Header().Set("X-Accel-Buffering", "no") // 禁用 Nginx 缓冲
	c.Writer.Header().Set("Access-Control-Allow-Origin", "*")

	// 获取初始执行状态
	run, err := h.service.GetRun(c.Request.Context(), id)
	if err != nil {
		fmt.Fprintf(c.Writer, "event: error\ndata: {\"message\":\"执行记录不存在\"}\n\n")
		c.Writer.Flush()
		return
	}

	// 立即发送一个心跳确保连接建立
	fmt.Fprintf(c.Writer, ": ping\n\n")
	c.Writer.Flush()

	lastSeq := 0
	ctx := c.Request.Context()

	// 使用 Gin 的 Stream 功能
	c.Stream(func(w io.Writer) bool {
		select {
		case <-ctx.Done():
			return false
		default:
			// 获取新日志
			logs, _ := h.service.GetRunLogs(ctx, id)
			for _, log := range logs {
				if log.Sequence > lastSeq {
					// 手动格式化 SSE 事件（确保立即发送）
					data, _ := json.Marshal(log)
					fmt.Fprintf(w, "event: log\ndata: %s\n\n", string(data))
					if f, ok := w.(http.Flusher); ok {
						f.Flush()
					}
					lastSeq = log.Sequence
				}
			}

			// 检查执行状态
			run, _ = h.service.GetRun(ctx, id)
			if run != nil && (run.Status == "success" || run.Status == "failed" || run.Status == "cancelled") {
				doneData, _ := json.Marshal(map[string]any{
					"status":    run.Status,
					"exit_code": run.ExitCode,
					"stats":     run.Stats,
				})
				fmt.Fprintf(w, "event: done\ndata: %s\n\n", string(doneData))
				if f, ok := w.(http.Flusher); ok {
					f.Flush()
				}
				return false
			}

			// 缩短轮询间隔到 200ms，更快响应
			time.Sleep(200 * time.Millisecond)
			return true
		}
	})
}

// ==================== 执行记录统计接口 ====================

// GetRunStats 获取执行记录统计概览
func (h *ExecutionHandler) GetRunStats(c *gin.Context) {
	stats, err := h.service.GetRunStats(c.Request.Context())
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}

	response.Success(c, stats)
}

// GetRunTrend 获取执行趋势
func (h *ExecutionHandler) GetRunTrend(c *gin.Context) {
	days, _ := strconv.Atoi(c.DefaultQuery("days", "7"))
	if days <= 0 {
		days = 7
	}

	items, err := h.service.GetRunTrend(c.Request.Context(), days)
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}

	response.Success(c, gin.H{
		"items": items,
		"days":  days,
	})
}

// GetTriggerDistribution 获取触发方式分布
func (h *ExecutionHandler) GetTriggerDistribution(c *gin.Context) {
	items, err := h.service.GetTriggerDistribution(c.Request.Context())
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}

	response.Success(c, items)
}

// GetTopFailedTasks 获取失败率最高的任务
func (h *ExecutionHandler) GetTopFailedTasks(c *gin.Context) {
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "5"))
	if limit <= 0 {
		limit = 5
	}

	items, err := h.service.GetTopFailedTasks(c.Request.Context(), limit)
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}

	response.Success(c, items)
}

// GetTopActiveTasks 获取最活跃的任务
func (h *ExecutionHandler) GetTopActiveTasks(c *gin.Context) {
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "5"))
	if limit <= 0 {
		limit = 5
	}

	items, err := h.service.GetTopActiveTasks(c.Request.Context(), limit)
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}

	response.Success(c, items)
}

// GetTaskStats 获取任务模板统计概览
func (h *ExecutionHandler) GetTaskStats(c *gin.Context) {
	stats, err := h.service.GetTaskStats(c.Request.Context())
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}

	response.Success(c, stats)
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
		response.InternalError(c, err.Error())
		return
	}

	response.Success(c, result)
}
