package execution

import (
	"context"
	"fmt"

	"github.com/company/auto-healing/internal/model"
	"github.com/company/auto-healing/internal/pkg/logger"
	"github.com/company/auto-healing/internal/repository"
	"github.com/google/uuid"
)

// ==================== 执行操作 ====================

// ExecuteOptions 执行选项
type ExecuteOptions struct {
	TriggeredBy      string
	SecretsSourceIDs []uuid.UUID
	ExtraVars        map[string]any
	TargetHosts      string
	SkipNotification bool
}

type executeParams struct {
	targetHosts      string
	extraVars        map[string]any
	secretsSourceIDs []uuid.UUID
	skipNotification bool
}

// ExecuteTask 异步执行任务（立即返回 RunID，后台执行）
func (s *Service) ExecuteTask(ctx context.Context, taskID uuid.UUID, opts *ExecuteOptions) (*model.ExecutionRun, error) {
	if opts == nil {
		opts = &ExecuteOptions{}
	}
	task, err := s.repo.GetTaskByID(ctx, taskID)
	if err != nil {
		return nil, fmt.Errorf("任务不存在: %w", err)
	}
	if task.NeedsReview {
		return nil, fmt.Errorf("任务模板需要审核变量变更后才能执行，变更字段: %v", task.ChangedVariables)
	}

	targetHosts := task.TargetHosts
	if opts.TargetHosts != "" {
		targetHosts = opts.TargetHosts
		logger.Exec("TASK").Info("使用运行时目标主机: %s", targetHosts)
	}

	playbook, err := s.repo.GetPlaybookByID(ctx, task.PlaybookID)
	if err != nil {
		return nil, fmt.Errorf("Playbook 不存在: %w", err)
	}
	if playbook.Status != "ready" {
		return nil, fmt.Errorf("Playbook 未上线，无法执行 (当前状态: %s)", playbook.Status)
	}

	gitRepo, err := s.gitRepo.GetByID(ctx, playbook.RepositoryID)
	if err != nil {
		return nil, fmt.Errorf("仓库不存在: %w", err)
	}

	secretsSourceIDs := resolveSecretsSourceIDs(task, opts)
	run := &model.ExecutionRun{
		TaskID:                  taskID,
		Status:                  "pending",
		TriggeredBy:             defaultTriggeredBy(opts.TriggeredBy),
		RuntimeTargetHosts:      targetHosts,
		RuntimeSecretsSourceIDs: uuidsToStrings(secretsSourceIDs),
		RuntimeExtraVars:        toJSON(opts.ExtraVars),
		RuntimeSkipNotification: opts.SkipNotification,
	}
	if err := s.repo.CreateRun(ctx, run); err != nil {
		return nil, fmt.Errorf("创建执行记录失败: %w", err)
	}

	s.scheduleExecution(run.ID, task, playbook, gitRepo, &executeParams{
		targetHosts:      targetHosts,
		extraVars:        opts.ExtraVars,
		secretsSourceIDs: secretsSourceIDs,
		skipNotification: opts.SkipNotification,
	})
	return run, nil
}

func defaultTriggeredBy(triggeredBy string) string {
	if triggeredBy == "" {
		return "manual"
	}
	return triggeredBy
}

func resolveSecretsSourceIDs(task *model.ExecutionTask, opts *ExecuteOptions) []uuid.UUID {
	if len(opts.SecretsSourceIDs) > 0 {
		return opts.SecretsSourceIDs
	}
	if len(task.SecretsSourceIDs) == 0 {
		return nil
	}

	result := make([]uuid.UUID, 0, len(task.SecretsSourceIDs))
	for _, idStr := range task.SecretsSourceIDs {
		if id, err := uuid.Parse(idStr); err == nil {
			result = append(result, id)
		}
	}
	return result
}

func (s *Service) scheduleExecution(runID uuid.UUID, task *model.ExecutionTask, playbook *model.Playbook, gitRepo *model.GitRepository, params *executeParams) {
	lifecycle := s.ensureLifecycle()
	lifecycle.Go(func(rootCtx context.Context) {
		if !lifecycle.Acquire(rootCtx) {
			logger.Exec("RUN").Warn("[%s] 执行未启动，服务正在停止", shortRunID(runID))
			return
		}
		defer lifecycle.Release()

		s.executeInBackground(withTenantContext(rootCtx, task.TenantID), runID, task, playbook, gitRepo, params)
	})
}

func (s *Service) ensureLifecycle() *asyncLifecycle {
	if s.lifecycle == nil || s.lifecycle.ctx.Err() != nil {
		s.lifecycle = newAsyncLifecycle(maxExecutionWorkers)
	}
	return s.lifecycle
}

// GetRun 获取执行记录
func (s *Service) GetRun(ctx context.Context, id uuid.UUID) (*model.ExecutionRun, error) {
	return s.repo.GetRunByID(ctx, id)
}

// GetRunsByTaskID 获取任务的执行历史
func (s *Service) GetRunsByTaskID(ctx context.Context, taskID uuid.UUID, page, pageSize int) ([]model.ExecutionRun, int64, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	return s.repo.ListRunsByTaskID(ctx, taskID, page, pageSize)
}

// ListAllRuns 获取所有执行记录列表（跨任务模板）
func (s *Service) ListAllRuns(ctx context.Context, opts *repository.RunListOptions) ([]model.ExecutionRun, int64, error) {
	if opts.Page < 1 {
		opts.Page = 1
	}
	if opts.PageSize < 1 || opts.PageSize > 100 {
		opts.PageSize = 20
	}
	return s.repo.ListAllRuns(ctx, opts)
}

// GetRunLogs 获取执行日志
func (s *Service) GetRunLogs(ctx context.Context, runID uuid.UUID) ([]model.ExecutionLog, error) {
	return s.repo.GetLogsByRunID(ctx, runID)
}

// CancelRun 取消执行
func (s *Service) CancelRun(ctx context.Context, id uuid.UUID) error {
	run, err := s.repo.GetRunByID(ctx, id)
	if err != nil {
		return err
	}
	if run.Status != "pending" && run.Status != "running" {
		return fmt.Errorf("执行状态不允许取消: %s", run.Status)
	}
	if err := s.repo.UpdateRunStatus(ctx, id, "cancelled"); err != nil {
		return err
	}

	s.cancelRunningExecution(id)
	s.appendLog(ctx, id, "warn", "control", "执行已被取消", nil)
	logger.Exec("RUN").Warn("已取消: %s", id)
	return nil
}

func (s *Service) cancelRunningExecution(id uuid.UUID) {
	cancelFunc, ok := s.runningExecutions.Load(id)
	if !ok {
		return
	}
	if cancel, ok := cancelFunc.(context.CancelFunc); ok {
		cancel()
		logger.Exec("RUN").Warn("已发送取消信号: %s", id)
	}
	s.runningExecutions.Delete(id)
}

func (s *Service) Shutdown() {
	if s.lifecycle != nil {
		s.lifecycle.Stop()
	}
}

// ==================== 统计方法 ====================

func (s *Service) GetRunStats(ctx context.Context) (*repository.RunStats, error) {
	return s.repo.GetRunStats(ctx)
}

func (s *Service) GetRunTrend(ctx context.Context, days int) ([]repository.RunTrendItem, error) {
	return s.repo.GetRunTrend(ctx, days)
}

func (s *Service) GetTriggerDistribution(ctx context.Context) ([]repository.TriggerDistItem, error) {
	return s.repo.GetTriggerDistribution(ctx)
}

func (s *Service) GetTopFailedTasks(ctx context.Context, limit int) ([]repository.TaskFailRate, error) {
	return s.repo.GetTopFailedTasks(ctx, limit)
}

func (s *Service) GetTopActiveTasks(ctx context.Context, limit int) ([]repository.TaskActivity, error) {
	return s.repo.GetTopActiveTasks(ctx, limit)
}

func (s *Service) GetTaskStats(ctx context.Context) (*repository.TaskStats, error) {
	return s.repo.GetTaskStats(ctx)
}

// BatchConfirmReviewRequest 批量审核请求
type BatchConfirmReviewRequest struct {
	TaskIDs    []uuid.UUID `json:"task_ids"`
	PlaybookID *uuid.UUID  `json:"playbook_id"`
}

// BatchConfirmReviewResponse 批量审核响应
type BatchConfirmReviewResponse struct {
	ConfirmedCount int64  `json:"confirmed_count"`
	Message        string `json:"message"`
}

// BatchConfirmReview 批量确认审核（同时更新快照）
func (s *Service) BatchConfirmReview(ctx context.Context, req *BatchConfirmReviewRequest) (*BatchConfirmReviewResponse, error) {
	tasks, err := s.listTasksPendingReview(ctx, req)
	if err != nil {
		return nil, err
	}
	if len(tasks) == 0 {
		return &BatchConfirmReviewResponse{ConfirmedCount: 0, Message: "没有需要确认的任务"}, nil
	}

	playbookVars := s.loadPlaybookVariableSnapshots(ctx, tasks)
	var count int64
	for _, task := range tasks {
		if s.confirmTaskReview(ctx, &task, playbookVars[task.PlaybookID]) {
			count++
		}
	}

	logger.Exec("TASK").Info("批量审核确认: %d 个任务模板（快照已同步）", count)
	return &BatchConfirmReviewResponse{
		ConfirmedCount: count,
		Message:        fmt.Sprintf("已确认 %d 个任务模板", count),
	}, nil
}

func (s *Service) listTasksPendingReview(ctx context.Context, req *BatchConfirmReviewRequest) ([]model.ExecutionTask, error) {
	if req.PlaybookID != nil {
		return s.repo.ListTasksByPlaybookIDAndReview(ctx, *req.PlaybookID)
	}
	if len(req.TaskIDs) > 0 {
		return s.repo.ListTasksByIDsAndReview(ctx, req.TaskIDs)
	}
	return nil, fmt.Errorf("必须提供 task_ids 或 playbook_id")
}

func (s *Service) loadPlaybookVariableSnapshots(ctx context.Context, tasks []model.ExecutionTask) map[uuid.UUID]model.JSONArray {
	cache := make(map[uuid.UUID]model.JSONArray, len(tasks))
	for _, task := range tasks {
		if _, ok := cache[task.PlaybookID]; ok {
			continue
		}
		playbook, err := s.repo.GetPlaybookByID(ctx, task.PlaybookID)
		if err != nil {
			logger.Exec("TASK").Warn("获取 Playbook %s 失败: %v", task.PlaybookID, err)
			cache[task.PlaybookID] = nil
			continue
		}
		cache[task.PlaybookID] = playbook.Variables
	}
	return cache
}

func (s *Service) confirmTaskReview(ctx context.Context, task *model.ExecutionTask, vars model.JSONArray) bool {
	if vars != nil {
		task.PlaybookVariablesSnapshot = vars
	}
	task.NeedsReview = false
	task.ChangedVariables = model.JSONArray{}
	if err := s.repo.UpdateTask(ctx, task); err != nil {
		logger.Exec("TASK").Warn("批量审核更新任务 %s 失败: %v", task.ID, err)
		return false
	}
	return true
}
