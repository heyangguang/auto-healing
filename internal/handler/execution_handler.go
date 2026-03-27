package handler

import (
	"time"

	"github.com/company/auto-healing/internal/modules/automation/service/execution"
	"github.com/company/auto-healing/internal/pkg/response"
	"github.com/company/auto-healing/internal/repository"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// ExecutionHandler 执行任务 Handler
type ExecutionHandler struct {
	service *execution.Service
}

type ExecutionHandlerDeps struct {
	Service *execution.Service
}

// NewExecutionHandler 创建 ExecutionHandler
func NewExecutionHandler() *ExecutionHandler {
	return NewExecutionHandlerWithDeps(ExecutionHandlerDeps{
		Service: execution.NewService(),
	})
}

func NewExecutionHandlerWithDeps(deps ExecutionHandlerDeps) *ExecutionHandler {
	return &ExecutionHandler{service: deps.Service}
}

func (h *ExecutionHandler) Shutdown() {
	if h != nil && h.service != nil {
		h.service.Shutdown()
	}
}

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

func buildTaskListOptions(c *gin.Context, page, pageSize int) *repository.TaskListOptions {
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
		LastRunStatus:  c.Query("last_run_status"),
	}
	if playbookIDStr := c.Query("playbook_id"); playbookIDStr != "" {
		if id, err := parseUUIDParam(playbookIDStr); err == nil {
			opts.PlaybookID = id
		}
	}
	if needsReviewStr := c.Query("needs_review"); needsReviewStr != "" {
		needsReview := needsReviewStr == "true"
		opts.NeedsReview = &needsReview
	}
	if hasRunsStr := c.Query("has_runs"); hasRunsStr != "" {
		hasRuns := hasRunsStr == "true"
		opts.HasRuns = &hasRuns
	}
	if minRunCountStr := c.Query("min_run_count"); minRunCountStr != "" {
		if count := parsePositiveIntQuery(c, "min_run_count", 0, 0); count > 0 {
			opts.MinRunCount = &count
		}
	}
	applyRFC3339Range(c, "created_from", &opts.CreatedFrom)
	applyRFC3339Range(c, "created_to", &opts.CreatedTo)
	return opts
}

func buildRunListOptions(c *gin.Context, page, pageSize int) *repository.RunListOptions {
	opts := &repository.RunListOptions{
		RunID:       c.Query("run_id"),
		TaskName:    GetStringFilter(c, "task_name"),
		Status:      c.Query("status"),
		TriggeredBy: c.Query("triggered_by"),
		Page:        page,
		PageSize:    pageSize,
	}
	if taskIDStr := c.Query("task_id"); taskIDStr != "" {
		if id, err := parseUUIDParam(taskIDStr); err == nil {
			opts.TaskID = id
		}
	}
	applyRFC3339Range(c, "started_after", &opts.StartedAfter)
	applyRFC3339Range(c, "started_before", &opts.StartedBefore)
	return opts
}

func parseUUIDParam(raw string) (*uuid.UUID, error) {
	id, err := uuid.Parse(raw)
	if err != nil {
		return nil, err
	}
	return &id, nil
}

func applyRFC3339Range(c *gin.Context, key string, target **time.Time) {
	if raw := c.Query(key); raw != "" {
		if parsed, err := time.Parse(time.RFC3339, raw); err == nil {
			*target = &parsed
		}
	}
}
