package repository

import (
	"context"
	"time"

	"github.com/company/auto-healing/internal/database"
	"github.com/company/auto-healing/internal/model"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// ExecutionRepository 执行任务仓库
type ExecutionRepository struct{}

// TaskListOptions 任务列表筛选选项
type TaskListOptions struct {
	PlaybookID     *uuid.UUID
	Search         string // 模糊搜索（匹配 name 或 description）
	ExecutorType   string // 执行器类型（local / docker）
	Status         string // 状态（pending_review / ready）
	TargetHosts    string // 目标主机模糊匹配
	PlaybookName   string // Playbook 名称模糊匹配
	RepositoryName string // Git 仓库名称模糊匹配
	// 基于执行记录的过滤
	HasRuns       *bool  // 是否有执行记录
	MinRunCount   *int   // 最小执行次数
	LastRunStatus string // 最后执行状态
	Page          int
	PageSize      int
}

// RunListOptions 执行记录列表筛选选项
type RunListOptions struct {
	TaskID        *uuid.UUID
	Search        string     // 全局搜索（匹配 ID、triggered_by、task.name）
	Status        string     // 状态精确匹配
	TriggeredBy   string     // 触发来源精确匹配
	StartedAfter  *time.Time // 开始时间范围
	StartedBefore *time.Time // 结束时间范围
	Page          int
	PageSize      int
}

// NewExecutionRepository 创建执行任务仓库
func NewExecutionRepository() *ExecutionRepository {
	return &ExecutionRepository{}
}

// ==================== 任务模板 CRUD ====================

// CreateTask 创建任务模板
func (r *ExecutionRepository) CreateTask(ctx context.Context, task *model.ExecutionTask) error {
	return database.DB.WithContext(ctx).Create(task).Error
}

// GetTaskByID 根据 ID 获取任务模板
func (r *ExecutionRepository) GetTaskByID(ctx context.Context, id uuid.UUID) (*model.ExecutionTask, error) {
	var task model.ExecutionTask
	err := database.DB.WithContext(ctx).
		Preload("Playbook").
		Preload("Playbook.Repository").
		First(&task, "id = ?", id).Error
	if err != nil {
		return nil, err
	}
	return &task, nil
}

// GetPlaybookByID 根据 ID 获取 Playbook
func (r *ExecutionRepository) GetPlaybookByID(ctx context.Context, id uuid.UUID) (*model.Playbook, error) {
	var playbook model.Playbook
	err := database.DB.WithContext(ctx).
		Preload("Repository").
		First(&playbook, "id = ?", id).Error
	if err != nil {
		return nil, err
	}
	return &playbook, nil
}

// ListTasks 列出任务模板（支持多条件筛选）
func (r *ExecutionRepository) ListTasks(ctx context.Context, opts *TaskListOptions) ([]model.ExecutionTask, int64, error) {
	var tasks []model.ExecutionTask
	var total int64

	query := database.DB.WithContext(ctx).Model(&model.ExecutionTask{})

	// 需要 JOIN 的筛选条件
	needsPlaybookJoin := opts.PlaybookName != "" || opts.RepositoryName != ""
	if needsPlaybookJoin {
		query = query.Joins("LEFT JOIN playbooks ON playbooks.id = execution_tasks.playbook_id")
	}
	if opts.RepositoryName != "" {
		query = query.Joins("LEFT JOIN git_repositories ON git_repositories.id = playbooks.repository_id")
	}

	// Playbook ID 筛选
	if opts.PlaybookID != nil {
		query = query.Where("execution_tasks.playbook_id = ?", *opts.PlaybookID)
	}

	// 模糊搜索（匹配 name 或 description）
	if opts.Search != "" {
		searchPattern := "%" + opts.Search + "%"
		query = query.Where("execution_tasks.name ILIKE ? OR execution_tasks.description ILIKE ?", searchPattern, searchPattern)
	}

	// 执行器类型筛选
	if opts.ExecutorType != "" {
		query = query.Where("execution_tasks.executor_type = ?", opts.ExecutorType)
	}

	// 状态筛选（映射到 needs_review 字段）
	if opts.Status != "" {
		switch opts.Status {
		case "pending_review":
			query = query.Where("execution_tasks.needs_review = ?", true)
		case "ready":
			query = query.Where("execution_tasks.needs_review = ?", false)
		}
	}

	// 目标主机模糊匹配
	if opts.TargetHosts != "" {
		query = query.Where("execution_tasks.target_hosts ILIKE ?", "%"+opts.TargetHosts+"%")
	}

	// Playbook 名称模糊匹配
	if opts.PlaybookName != "" {
		query = query.Where("playbooks.name ILIKE ?", "%"+opts.PlaybookName+"%")
	}

	// Git 仓库名称模糊匹配
	if opts.RepositoryName != "" {
		query = query.Where("git_repositories.name ILIKE ?", "%"+opts.RepositoryName+"%")
	}

	// ==================== 基于执行记录的过滤 ====================

	// has_runs: 是否有执行记录
	if opts.HasRuns != nil {
		if *opts.HasRuns {
			query = query.Where("EXISTS (SELECT 1 FROM execution_runs WHERE task_id = execution_tasks.id)")
		} else {
			query = query.Where("NOT EXISTS (SELECT 1 FROM execution_runs WHERE task_id = execution_tasks.id)")
		}
	}

	// min_run_count: 最小执行次数
	if opts.MinRunCount != nil && *opts.MinRunCount > 0 {
		query = query.Where("(SELECT COUNT(*) FROM execution_runs WHERE task_id = execution_tasks.id) >= ?", *opts.MinRunCount)
	}

	// last_run_status: 最后执行状态筛选
	if opts.LastRunStatus != "" {
		query = query.Where(`EXISTS (
			SELECT 1 FROM execution_runs r 
			WHERE r.task_id = execution_tasks.id 
			AND r.status = ?
			AND r.created_at = (SELECT MAX(created_at) FROM execution_runs WHERE task_id = execution_tasks.id)
		)`, opts.LastRunStatus)
	}

	query.Count(&total)

	offset := (opts.Page - 1) * opts.PageSize
	err := query.
		Preload("Playbook").
		Order("execution_tasks.created_at DESC").
		Offset(offset).
		Limit(opts.PageSize).
		Find(&tasks).Error

	return tasks, total, err
}

// UpdateTask 更新任务模板
func (r *ExecutionRepository) UpdateTask(ctx context.Context, task *model.ExecutionTask) error {
	// 使用 Select 明确指定要更新的列，避免 GORM 忽略某些字段
	return database.DB.WithContext(ctx).
		Model(task).
		Select("name", "playbook_id", "target_hosts", "extra_vars", "executor_type", "description", "secrets_source_ids", "notification_config", "playbook_variables_snapshot", "needs_review", "changed_variables", "updated_at").
		Updates(task).Error
}

// DeleteTask 删除任务模板（级联删除 runs、logs、notification_logs）
func (r *ExecutionRepository) DeleteTask(ctx context.Context, id uuid.UUID) error {
	return database.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// 1. 获取所有关联的 execution runs
		var runIDs []uuid.UUID
		if err := tx.Model(&model.ExecutionRun{}).
			Where("task_id = ?", id).
			Pluck("id", &runIDs).Error; err != nil {
			return err
		}

		if len(runIDs) > 0 {
			// 2. 删除通知日志（引用 execution_run_id）
			if err := tx.Where("execution_run_id IN ?", runIDs).
				Delete(&model.NotificationLog{}).Error; err != nil {
				return err
			}

			// 3. 删除执行日志
			if err := tx.Where("run_id IN ?", runIDs).
				Delete(&model.ExecutionLog{}).Error; err != nil {
				return err
			}

			// 4. 删除执行记录
			if err := tx.Where("task_id = ?", id).
				Delete(&model.ExecutionRun{}).Error; err != nil {
				return err
			}
		}

		// 5. 删除任务
		return tx.Delete(&model.ExecutionTask{}, "id = ?", id).Error
	})
}

// UpdateNextRunAt 更新下次执行时间
func (r *ExecutionRepository) UpdateNextRunAt(ctx context.Context, id uuid.UUID, nextRunAt time.Time) error {
	return database.DB.WithContext(ctx).
		Model(&model.ExecutionTask{}).
		Where("id = ?", id).
		Update("next_run_at", nextRunAt).Error
}

// GetScheduledTasks 获取需要执行的定时任务
func (r *ExecutionRepository) GetScheduledTasks(ctx context.Context) ([]model.ExecutionTask, error) {
	var tasks []model.ExecutionTask
	err := database.DB.WithContext(ctx).
		Where("is_recurring = ? AND next_run_at <= ?", true, time.Now()).
		Find(&tasks).Error
	return tasks, err
}

// ListTasksByPlaybookID 获取关联指定 Playbook 的所有任务模板
func (r *ExecutionRepository) ListTasksByPlaybookID(ctx context.Context, playbookID uuid.UUID) ([]model.ExecutionTask, error) {
	var tasks []model.ExecutionTask
	err := database.DB.WithContext(ctx).
		Where("playbook_id = ?", playbookID).
		Find(&tasks).Error
	return tasks, err
}

// CountTasksByPlaybookID 统计指定 Playbook 下的任务模板数量
func (r *ExecutionRepository) CountTasksByPlaybookID(ctx context.Context, playbookID uuid.UUID) (int64, error) {
	var count int64
	err := database.DB.WithContext(ctx).
		Model(&model.ExecutionTask{}).
		Where("playbook_id = ?", playbookID).
		Count(&count).Error
	return count, err
}

// CountSchedulesByTaskID 统计指定任务模板下的调度任务数量
func (r *ExecutionRepository) CountSchedulesByTaskID(ctx context.Context, taskID uuid.UUID) (int64, error) {
	var count int64
	err := database.DB.WithContext(ctx).
		Model(&model.ExecutionSchedule{}).
		Where("task_id = ?", taskID).
		Count(&count).Error
	return count, err
}

// UpdateTaskReviewStatus 更新任务模板的 review 状态
func (r *ExecutionRepository) UpdateTaskReviewStatus(ctx context.Context, taskID uuid.UUID, needsReview bool, changedVariables model.JSONArray) error {
	return database.DB.WithContext(ctx).
		Model(&model.ExecutionTask{}).
		Where("id = ?", taskID).
		Updates(map[string]any{
			"needs_review":      needsReview,
			"changed_variables": changedVariables,
			"updated_at":        time.Now(),
		}).Error
}

// ==================== 执行记录 CRUD ====================

// CreateRun 创建执行记录
func (r *ExecutionRepository) CreateRun(ctx context.Context, run *model.ExecutionRun) error {
	return database.DB.WithContext(ctx).Create(run).Error
}

// GetRunByID 根据 ID 获取执行记录
func (r *ExecutionRepository) GetRunByID(ctx context.Context, id uuid.UUID) (*model.ExecutionRun, error) {
	var run model.ExecutionRun
	err := database.DB.WithContext(ctx).
		Preload("Task").
		Preload("Task.Playbook").
		First(&run, "id = ?", id).Error
	if err != nil {
		return nil, err
	}
	return &run, nil
}

// ListRunsByTaskID 列出任务的执行记录
func (r *ExecutionRepository) ListRunsByTaskID(ctx context.Context, taskID uuid.UUID, page, pageSize int) ([]model.ExecutionRun, int64, error) {
	var runs []model.ExecutionRun
	var total int64

	query := database.DB.WithContext(ctx).Model(&model.ExecutionRun{}).Where("task_id = ?", taskID)
	query.Count(&total)

	offset := (page - 1) * pageSize
	err := query.
		Order("created_at DESC").
		Offset(offset).
		Limit(pageSize).
		Find(&runs).Error

	return runs, total, err
}

// ListAllRuns 列出所有执行记录（跨任务模板，支持多条件筛选）
func (r *ExecutionRepository) ListAllRuns(ctx context.Context, opts *RunListOptions) ([]model.ExecutionRun, int64, error) {
	var runs []model.ExecutionRun
	var total int64

	query := database.DB.WithContext(ctx).Model(&model.ExecutionRun{})

	// 全局搜索需要 JOIN task 表
	if opts.Search != "" {
		query = query.Joins("LEFT JOIN execution_tasks ON execution_tasks.id = execution_runs.task_id")
		searchPattern := "%" + opts.Search + "%"
		// OR 匹配：ID、triggered_by、task.name
		query = query.Where(
			"execution_runs.id::text ILIKE ? OR execution_runs.triggered_by ILIKE ? OR execution_tasks.name ILIKE ?",
			searchPattern, searchPattern, searchPattern,
		)
	}

	// Task ID 筛选
	if opts.TaskID != nil {
		query = query.Where("execution_runs.task_id = ?", *opts.TaskID)
	}

	// 状态精确匹配
	if opts.Status != "" {
		query = query.Where("execution_runs.status = ?", opts.Status)
	}

	// 触发来源精确匹配
	if opts.TriggeredBy != "" {
		query = query.Where("execution_runs.triggered_by = ?", opts.TriggeredBy)
	}

	// 时间范围筛选
	if opts.StartedAfter != nil {
		query = query.Where("execution_runs.started_at >= ?", *opts.StartedAfter)
	}
	if opts.StartedBefore != nil {
		query = query.Where("execution_runs.started_at <= ?", *opts.StartedBefore)
	}

	query.Count(&total)

	offset := (opts.Page - 1) * opts.PageSize
	err := query.
		Preload("Task").
		Order("execution_runs.created_at DESC").
		Offset(offset).
		Limit(opts.PageSize).
		Find(&runs).Error

	return runs, total, err
}

// UpdateRunStatus 更新执行状态
func (r *ExecutionRepository) UpdateRunStatus(ctx context.Context, id uuid.UUID, status string) error {
	return database.DB.WithContext(ctx).
		Model(&model.ExecutionRun{}).
		Where("id = ?", id).
		Update("status", status).Error
}

// UpdateRunStarted 更新执行开始
func (r *ExecutionRepository) UpdateRunStarted(ctx context.Context, id uuid.UUID) error {
	now := time.Now()
	return database.DB.WithContext(ctx).
		Model(&model.ExecutionRun{}).
		Where("id = ?", id).
		Updates(map[string]any{
			"status":     "running",
			"started_at": now,
		}).Error
}

// UpdateRunResult 更新执行结果
func (r *ExecutionRepository) UpdateRunResult(ctx context.Context, id uuid.UUID, exitCode int, stdout, stderr string, stats model.JSON) error {
	now := time.Now()

	// 基于 stats 判断真实执行状态
	// 规则：
	// 1. 如果 exit code 非 0，一定是 failed
	// 2. 全部成功（ok > 0, failed == 0, unreachable == 0）→ success
	// 3. 全部失败（ok == 0）→ failed
	// 4. 部分成功部分失败（ok > 0 且有 failed 或 unreachable）→ partial
	status := "failed"
	if exitCode == 0 && stats != nil {
		ok := getStatValue(stats, "ok")
		failed := getStatValue(stats, "failed")
		unreachable := getStatValue(stats, "unreachable")

		if ok > 0 {
			if failed == 0 && unreachable == 0 {
				// 全部成功
				status = "success"
			} else {
				// 部分成功
				status = "partial"
			}
		}
		// 如果 ok == 0，保持 failed 状态
	}

	return database.DB.WithContext(ctx).
		Model(&model.ExecutionRun{}).
		Where("id = ?", id).
		Updates(map[string]any{
			"status":       status,
			"exit_code":    exitCode,
			"stdout":       stdout,
			"stderr":       stderr,
			"stats":        stats,
			"completed_at": now,
		}).Error
}

// getStatValue 从 stats 中获取数值（兼容 float64 和 int）
func getStatValue(stats model.JSON, key string) float64 {
	if v, ok := stats[key].(float64); ok {
		return v
	}
	if v, ok := stats[key].(int); ok {
		return float64(v)
	}
	return 0
}

// ==================== 执行日志 CRUD ====================

// AppendLog 追加执行日志
func (r *ExecutionRepository) AppendLog(ctx context.Context, log *model.ExecutionLog) error {
	return database.DB.WithContext(ctx).Create(log).Error
}

// GetLogsByRunID 获取执行记录的日志
func (r *ExecutionRepository) GetLogsByRunID(ctx context.Context, runID uuid.UUID) ([]model.ExecutionLog, error) {
	var logs []model.ExecutionLog
	err := database.DB.WithContext(ctx).
		Where("run_id = ?", runID).
		Order("sequence ASC").
		Find(&logs).Error
	return logs, err
}

// GetNextLogSequence 获取下一个日志序号
func (r *ExecutionRepository) GetNextLogSequence(ctx context.Context, runID uuid.UUID) (int, error) {
	var maxSeq int
	err := database.DB.WithContext(ctx).
		Model(&model.ExecutionLog{}).
		Where("run_id = ?", runID).
		Select("COALESCE(MAX(sequence), 0)").
		Scan(&maxSeq).Error
	return maxSeq + 1, err
}
