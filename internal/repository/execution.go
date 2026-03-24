package repository

import (
	"context"
	"time"

	"github.com/company/auto-healing/internal/database"
	"github.com/company/auto-healing/internal/model"
	qf "github.com/company/auto-healing/internal/pkg/query"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// ExecutionRepository 执行任务仓库
type ExecutionRepository struct{}

// TaskListOptions 任务列表筛选选项
type TaskListOptions struct {
	PlaybookID     *uuid.UUID
	Name           qf.StringFilter // 独立字段：名称
	Description    qf.StringFilter // 独立字段：描述
	ExecutorType   string          // 执行器类型（local / docker）
	Status         string          // 状态（pending_review / ready）
	NeedsReview    *bool           // 直接按 needs_review 布尔值过滤
	TargetHosts    string          // 目标主机模糊匹配
	PlaybookName   string          // Playbook 名称模糊匹配
	RepositoryName string          // Git 仓库名称模糊匹配
	// 时间范围
	CreatedFrom *time.Time // 创建时间范围起始
	CreatedTo   *time.Time // 创建时间范围结束
	// 排序
	SortBy    string // 排序字段：name / created_at / updated_at
	SortOrder string // 排序方向：asc / desc
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
	RunID         string          // 执行记录 ID（支持完整 UUID 或前缀匹配）
	TaskName      qf.StringFilter // 独立字段：任务名称
	Status        string          // 状态精确匹配
	TriggeredBy   string          // 触发来源精确匹配
	StartedAfter  *time.Time      // 开始时间范围
	StartedBefore *time.Time      // 结束时间范围
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
	if task.TenantID == nil {
		tenantID := TenantIDFromContext(ctx)
		task.TenantID = &tenantID
	}
	return database.DB.WithContext(ctx).Create(task).Error
}

// GetTaskByID 根据 ID 获取任务模板
func (r *ExecutionRepository) GetTaskByID(ctx context.Context, id uuid.UUID) (*model.ExecutionTask, error) {
	var task model.ExecutionTask
	err := TenantDB(database.DB, ctx).
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
	err := TenantDB(database.DB, ctx).
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

	query := database.DB.WithContext(ctx).Model(&model.ExecutionTask{}).
		Where("execution_tasks.tenant_id = ?", TenantIDFromContext(ctx))

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

	// 独立字段查询
	if !opts.Name.IsEmpty() {
		query = qf.ApplyStringFilter(query, "execution_tasks.name", opts.Name)
	}
	if !opts.Description.IsEmpty() {
		query = qf.ApplyStringFilter(query, "execution_tasks.description", opts.Description)
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

	// needs_review 直接布尔过滤
	if opts.NeedsReview != nil {
		query = query.Where("execution_tasks.needs_review = ?", *opts.NeedsReview)
	}

	// 创建时间范围过滤
	if opts.CreatedFrom != nil {
		query = query.Where("execution_tasks.created_at >= ?", *opts.CreatedFrom)
	}
	if opts.CreatedTo != nil {
		query = query.Where("execution_tasks.created_at <= ?", *opts.CreatedTo)
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

	query.Session(&gorm.Session{}).Count(&total)

	// 排序
	orderClause := "execution_tasks.created_at DESC" // 默认排序
	if opts.SortBy != "" {
		dir := "ASC"
		if opts.SortOrder == "desc" {
			dir = "DESC"
		}
		switch opts.SortBy {
		case "name":
			orderClause = "execution_tasks.name " + dir
		case "created_at":
			orderClause = "execution_tasks.created_at " + dir
		case "updated_at":
			orderClause = "execution_tasks.updated_at " + dir
		}
	}

	offset := (opts.Page - 1) * opts.PageSize
	err := query.
		Preload("Playbook").
		Order(orderClause).
		Offset(offset).
		Limit(opts.PageSize).
		Find(&tasks).Error
	if err != nil {
		return nil, 0, err
	}

	// 批量填充 schedule_count（避免 N+1）
	if len(tasks) > 0 {
		taskIDs := make([]uuid.UUID, len(tasks))
		for i, t := range tasks {
			taskIDs[i] = t.ID
		}

		type scheduleCount struct {
			TaskID uuid.UUID `gorm:"column:task_id"`
			Count  int       `gorm:"column:count"`
		}
		var counts []scheduleCount
		TenantDB(database.DB, ctx).
			Model(&model.ExecutionSchedule{}).
			Select("task_id, COUNT(*) as count").
			Where("task_id IN ?", taskIDs).
			Group("task_id").
			Scan(&counts)

		countMap := make(map[uuid.UUID]int)
		for _, c := range counts {
			countMap[c.TaskID] = c.Count
		}
		for i := range tasks {
			tasks[i].ScheduleCount = countMap[tasks[i].ID]
		}
	}

	return tasks, total, nil
}

// BatchConfirmReviewByIDs 按任务 ID 列表批量确认审核
func (r *ExecutionRepository) BatchConfirmReviewByIDs(ctx context.Context, taskIDs []uuid.UUID) (int64, error) {
	result := TenantDB(database.DB, ctx).
		Model(&model.ExecutionTask{}).
		Where("id IN ? AND needs_review = ?", taskIDs, true).
		Updates(map[string]any{
			"needs_review":      false,
			"changed_variables": "[]",
		})
	return result.RowsAffected, result.Error
}

// BatchConfirmReviewByPlaybookID 按 Playbook ID 批量确认审核
func (r *ExecutionRepository) BatchConfirmReviewByPlaybookID(ctx context.Context, playbookID uuid.UUID) (int64, error) {
	result := TenantDB(database.DB, ctx).
		Model(&model.ExecutionTask{}).
		Where("playbook_id = ? AND needs_review = ?", playbookID, true).
		Updates(map[string]any{
			"needs_review":      false,
			"changed_variables": "[]",
		})
	return result.RowsAffected, result.Error
}

// ListTasksByPlaybookIDAndReview 查询指定 Playbook 下需要审核的任务
func (r *ExecutionRepository) ListTasksByPlaybookIDAndReview(ctx context.Context, playbookID uuid.UUID) ([]model.ExecutionTask, error) {
	var tasks []model.ExecutionTask
	err := TenantDB(database.DB, ctx).
		Where("playbook_id = ? AND needs_review = ?", playbookID, true).
		Find(&tasks).Error
	return tasks, err
}

// ListTasksByIDsAndReview 查询指定 ID 列表中需要审核的任务
func (r *ExecutionRepository) ListTasksByIDsAndReview(ctx context.Context, taskIDs []uuid.UUID) ([]model.ExecutionTask, error) {
	var tasks []model.ExecutionTask
	err := TenantDB(database.DB, ctx).
		Where("id IN ? AND needs_review = ?", taskIDs, true).
		Find(&tasks).Error
	return tasks, err
}

// UpdateTask 更新任务模板
func (r *ExecutionRepository) UpdateTask(ctx context.Context, task *model.ExecutionTask) error {
	// 使用 Select 明确指定要更新的列，避免 GORM 忽略某些字段
	return TenantDB(database.DB, ctx).
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
	return TenantDB(database.DB, ctx).
		Model(&model.ExecutionTask{}).
		Where("id = ?", id).
		Update("next_run_at", nextRunAt).Error
}

// GetScheduledTasks 获取需要执行的定时任务
func (r *ExecutionRepository) GetScheduledTasks(ctx context.Context) ([]model.ExecutionTask, error) {
	var tasks []model.ExecutionTask
	err := TenantDB(database.DB, ctx).
		Where("is_recurring = ? AND next_run_at <= ?", true, time.Now()).
		Find(&tasks).Error
	return tasks, err
}

// ListTasksByPlaybookID 获取关联指定 Playbook 的所有任务模板
func (r *ExecutionRepository) ListTasksByPlaybookID(ctx context.Context, playbookID uuid.UUID) ([]model.ExecutionTask, error) {
	var tasks []model.ExecutionTask
	err := TenantDB(database.DB, ctx).
		Where("playbook_id = ?", playbookID).
		Find(&tasks).Error
	return tasks, err
}

// CountTasksByPlaybookID 统计指定 Playbook 下的任务模板数量
func (r *ExecutionRepository) CountTasksByPlaybookID(ctx context.Context, playbookID uuid.UUID) (int64, error) {
	var count int64
	err := TenantDB(database.DB, ctx).
		Model(&model.ExecutionTask{}).
		Where("playbook_id = ?", playbookID).
		Count(&count).Error
	return count, err
}

// CountSchedulesByTaskID 统计指定任务模板下的调度任务数量
func (r *ExecutionRepository) CountSchedulesByTaskID(ctx context.Context, taskID uuid.UUID) (int64, error) {
	var count int64
	err := TenantDB(database.DB, ctx).
		Model(&model.ExecutionSchedule{}).
		Where("task_id = ?", taskID).
		Count(&count).Error
	return count, err
}

// UpdateTaskReviewStatus 更新任务模板的 review 状态
func (r *ExecutionRepository) UpdateTaskReviewStatus(ctx context.Context, taskID uuid.UUID, needsReview bool, changedVariables model.JSONArray) error {
	return TenantDB(database.DB, ctx).
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
	if run.TenantID == nil {
		tenantID := TenantIDFromContext(ctx)
		run.TenantID = &tenantID
	}
	return database.DB.WithContext(ctx).Create(run).Error
}

// GetRunByID 根据 ID 获取执行记录
func (r *ExecutionRepository) GetRunByID(ctx context.Context, id uuid.UUID) (*model.ExecutionRun, error) {
	var run model.ExecutionRun
	err := TenantDB(database.DB, ctx).
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

	query := TenantDB(database.DB, ctx).Model(&model.ExecutionRun{}).Where("task_id = ?", taskID)
	query.Session(&gorm.Session{}).Count(&total)

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

	// 使用显式表前缀的 tenant_id 条件，避免 JOIN 后 ambiguous
	query := database.DB.WithContext(ctx).Model(&model.ExecutionRun{}).
		Where("execution_runs.tenant_id = ?", TenantIDFromContext(ctx))

	// 独立字段搜索需要 JOIN task 表
	needsTaskJoin := !opts.TaskName.IsEmpty()
	if needsTaskJoin {
		query = query.Joins("LEFT JOIN execution_tasks ON execution_tasks.id = execution_runs.task_id")
	}
	// 独立字段：任务名称
	if !opts.TaskName.IsEmpty() {
		query = qf.ApplyStringFilter(query, "execution_tasks.name", opts.TaskName)
	}

	// Run ID 搜索（支持完整 UUID 或前缀匹配）
	if opts.RunID != "" {
		query = query.Where("execution_runs.id::text ILIKE ?", opts.RunID+"%")
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

	query.Session(&gorm.Session{}).Count(&total)

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
	return TenantDB(database.DB, ctx).
		Model(&model.ExecutionRun{}).
		Where("id = ?", id).
		Update("status", status).Error
}

// UpdateRunStarted 更新执行开始
func (r *ExecutionRepository) UpdateRunStarted(ctx context.Context, id uuid.UUID) (bool, error) {
	now := time.Now()
	result := TenantDB(database.DB, ctx).
		Model(&model.ExecutionRun{}).
		Where("id = ? AND status = ?", id, "pending").
		Updates(map[string]any{
			"status":     "running",
			"started_at": now,
		})
	return result.RowsAffected > 0, result.Error
}

// UpdateRunResult 更新执行结果
func (r *ExecutionRepository) UpdateRunResult(ctx context.Context, id uuid.UUID, exitCode int, stdout, stderr string, stats model.JSON) error {
	now := time.Now()

	// 基于 stats 判断真实执行状态（Stats 优先）
	// Ansible 退出码规则：0=全成功, 2=有failed, 4=有unreachable, 6=两者都有
	// 因此 exitCode != 0 并不意味着全部失败，可能只是部分主机出问题
	// 正确规则：
	// 1. 优先看 stats（主机级别的真实结果）
	// 2. ok > 0 且 failed == 0 且 unreachable == 0 → success
	// 3. ok > 0 且有 failed 或 unreachable → partial（部分成功）
	// 4. ok == 0 → failed（全部失败）
	// 5. 无 stats 时 fallback 到 exitCode 判断
	status := "failed"
	if stats != nil {
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
	} else if exitCode == 0 {
		// 无 stats 但退出码为 0，视为成功
		status = "success"
	}

	return TenantDB(database.DB, ctx).
		Model(&model.ExecutionRun{}).
		Where("id = ? AND status <> ?", id, "cancelled").
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
	if log.TenantID == nil {
		tenantID := TenantIDFromContext(ctx)
		log.TenantID = &tenantID
	}
	return database.DB.WithContext(ctx).Create(log).Error
}

// GetLogsByRunID 获取执行记录的日志
func (r *ExecutionRepository) GetLogsByRunID(ctx context.Context, runID uuid.UUID) ([]model.ExecutionLog, error) {
	var logs []model.ExecutionLog
	err := TenantDB(database.DB, ctx).
		Where("run_id = ?", runID).
		Order("sequence ASC").
		Find(&logs).Error
	return logs, err
}

// GetNextLogSequence 获取下一个日志序号
func (r *ExecutionRepository) GetNextLogSequence(ctx context.Context, runID uuid.UUID) (int, error) {
	var maxSeq int
	err := TenantDB(database.DB, ctx).
		Model(&model.ExecutionLog{}).
		Where("run_id = ?", runID).
		Select("COALESCE(MAX(sequence), 0)").
		Scan(&maxSeq).Error
	return maxSeq + 1, err
}

// ==================== 执行记录统计 ====================

// RunStats 执行记录统计概览
type RunStats struct {
	TotalCount     int64   `json:"total_count"`
	SuccessCount   int64   `json:"success_count"`
	FailedCount    int64   `json:"failed_count"`
	PartialCount   int64   `json:"partial_count"`
	CancelledCount int64   `json:"cancelled_count"`
	SuccessRate    float64 `json:"success_rate"`
	AvgDurationSec float64 `json:"avg_duration_sec"`
	TodayCount     int64   `json:"today_count"`
}

// RunTrendItem 执行趋势数据（按天+状态分组）
type RunTrendItem struct {
	Date   string `json:"date" gorm:"column:date"`
	Status string `json:"status" gorm:"column:status"`
	Count  int64  `json:"count" gorm:"column:count"`
}

// TriggerDistItem 触发方式分布
type TriggerDistItem struct {
	TriggeredBy string `json:"triggered_by" gorm:"column:triggered_by"`
	Count       int64  `json:"count" gorm:"column:count"`
}

// TaskFailRate 任务失败率
type TaskFailRate struct {
	TaskID   string  `json:"task_id" gorm:"column:task_id"`
	TaskName string  `json:"task_name" gorm:"column:task_name"`
	Total    int64   `json:"total" gorm:"column:total"`
	Failed   int64   `json:"failed" gorm:"column:failed"`
	FailRate float64 `json:"fail_rate" gorm:"column:fail_rate"`
}

// TaskActivity 任务活跃度
type TaskActivity struct {
	TaskID   string `json:"task_id" gorm:"column:task_id"`
	TaskName string `json:"task_name" gorm:"column:task_name"`
	Total    int64  `json:"total" gorm:"column:total"`
}

// GetRunStats 获取执行记录统计概览
func (r *ExecutionRepository) GetRunStats(ctx context.Context) (*RunStats, error) {
	stats := &RunStats{}
	// 每次查询使用新的 TenantDB 实例，避免 GORM session WHERE 条件累积
	newDB := func() *gorm.DB { return TenantDB(database.DB, ctx) }

	// 总数
	newDB().Model(&model.ExecutionRun{}).Count(&stats.TotalCount)

	// 各状态数量
	newDB().Model(&model.ExecutionRun{}).Where("status = ?", "success").Count(&stats.SuccessCount)
	newDB().Model(&model.ExecutionRun{}).Where("status = ?", "failed").Count(&stats.FailedCount)
	newDB().Model(&model.ExecutionRun{}).Where("status = ?", "partial").Count(&stats.PartialCount)
	newDB().Model(&model.ExecutionRun{}).Where("status = ?", "cancelled").Count(&stats.CancelledCount)

	// 成功率
	if stats.TotalCount > 0 {
		stats.SuccessRate = float64(stats.SuccessCount) / float64(stats.TotalCount) * 100
	}

	// 平均执行时间（秒）
	var avgDuration *float64
	newDB().Model(&model.ExecutionRun{}).
		Where("completed_at IS NOT NULL AND started_at IS NOT NULL").
		Select("EXTRACT(EPOCH FROM AVG(completed_at - started_at))").
		Scan(&avgDuration)
	if avgDuration != nil {
		stats.AvgDurationSec = *avgDuration
	}

	// 今日执行数量
	now := time.Now()
	todayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	newDB().Model(&model.ExecutionRun{}).Where("created_at >= ?", todayStart).Count(&stats.TodayCount)

	return stats, nil
}

// GetRunTrend 获取执行趋势（按天+状态分组）
func (r *ExecutionRepository) GetRunTrend(ctx context.Context, days int) ([]RunTrendItem, error) {
	var items []RunTrendItem
	since := time.Now().AddDate(0, 0, -days)

	tenantID := TenantIDFromContext(ctx)
	err := database.DB.WithContext(ctx).
		Model(&model.ExecutionRun{}).
		Where("tenant_id = ?", tenantID).
		Where("created_at >= ?", since).
		Select("TO_CHAR(created_at, 'YYYY-MM-DD') as date, status, COUNT(*) as count").
		Group("date, status").
		Order("date ASC").
		Scan(&items).Error

	return items, err
}

// GetTriggerDistribution 获取触发方式分布
func (r *ExecutionRepository) GetTriggerDistribution(ctx context.Context) ([]TriggerDistItem, error) {
	var items []TriggerDistItem

	err := TenantDB(database.DB, ctx).
		Model(&model.ExecutionRun{}).
		Select("COALESCE(triggered_by, 'unknown') as triggered_by, COUNT(*) as count").
		Group("triggered_by").
		Order("count DESC").
		Scan(&items).Error

	return items, err
}

// GetTopFailedTasks 获取失败率最高的 Top N 任务
func (r *ExecutionRepository) GetTopFailedTasks(ctx context.Context, limit int) ([]TaskFailRate, error) {
	var items []TaskFailRate

	tenantID := TenantIDFromContext(ctx)
	err := database.DB.WithContext(ctx).
		Raw(`
			SELECT 
				r.task_id::text as task_id,
				COALESCE(t.name, '已删除任务') as task_name,
				COUNT(*) as total,
				SUM(CASE WHEN r.status = 'failed' THEN 1 ELSE 0 END) as failed,
				ROUND(SUM(CASE WHEN r.status = 'failed' THEN 1 ELSE 0 END)::numeric / COUNT(*)::numeric * 100, 2) as fail_rate
			FROM execution_runs r
			LEFT JOIN execution_tasks t ON t.id = r.task_id
			WHERE r.tenant_id = ?
			GROUP BY r.task_id, t.name
			HAVING COUNT(*) >= 2
			ORDER BY fail_rate DESC
			LIMIT ?
		`, tenantID, limit).
		Scan(&items).Error

	return items, err
}

// GetTopActiveTasks 获取最活跃的 Top N 任务
func (r *ExecutionRepository) GetTopActiveTasks(ctx context.Context, limit int) ([]TaskActivity, error) {
	var items []TaskActivity

	tenantID := TenantIDFromContext(ctx)
	err := database.DB.WithContext(ctx).
		Raw(`
			SELECT 
				r.task_id::text as task_id,
				COALESCE(t.name, '已删除任务') as task_name,
				COUNT(*) as total
			FROM execution_runs r
			LEFT JOIN execution_tasks t ON t.id = r.task_id
			WHERE r.tenant_id = ?
			GROUP BY r.task_id, t.name
			ORDER BY total DESC
			LIMIT ?
		`, tenantID, limit).
		Scan(&items).Error

	return items, err
}

// TaskStats 任务模板统计概览
type TaskStats struct {
	Total            int64 `json:"total"`
	Docker           int64 `json:"docker"`
	Local            int64 `json:"local"`
	NeedsReview      int64 `json:"needs_review"`
	ChangedPlaybooks int64 `json:"changed_playbooks"`
	Ready            int64 `json:"ready"`
	NeverExecuted    int64 `json:"never_executed"`
	LastRunFailed    int64 `json:"last_run_failed"`
}

// GetTaskStats 获取任务模板统计概览
func (r *ExecutionRepository) GetTaskStats(ctx context.Context) (*TaskStats, error) {
	stats := &TaskStats{}
	// 每次查询使用新的 TenantDB 实例，避免 GORM session WHERE 条件累积
	newDB := func() *gorm.DB { return TenantDB(database.DB, ctx) }

	// 全部模板数
	newDB().Model(&model.ExecutionTask{}).Count(&stats.Total)

	// executor_type='docker' 的模板数
	newDB().Model(&model.ExecutionTask{}).Where("executor_type = ?", "docker").Count(&stats.Docker)

	// executor_type!='docker' 的模板数（包括 local 和 ssh）
	newDB().Model(&model.ExecutionTask{}).Where("executor_type != ?", "docker").Count(&stats.Local)

	// needs_review=true 的模板数
	newDB().Model(&model.ExecutionTask{}).Where("needs_review = ?", true).Count(&stats.NeedsReview)

	// needs_review=true 的模板中，去重 playbook_id 的数量
	newDB().Model(&model.ExecutionTask{}).
		Where("needs_review = ?", true).
		Distinct("playbook_id").
		Count(&stats.ChangedPlaybooks)

	// 就绪可执行：needs_review=false 且 playbook status='ready'
	// 注意：JOIN 后两张表都有 tenant_id，需显式加表前缀避免 SQLSTATE 42702 歧义
	tenantID := TenantIDFromContext(ctx)
	database.DB.WithContext(ctx).Model(&model.ExecutionTask{}).
		Joins("JOIN playbooks ON playbooks.id = execution_tasks.playbook_id").
		Where("execution_tasks.tenant_id = ? AND execution_tasks.needs_review = ? AND playbooks.status = ?", tenantID, false, "ready").
		Count(&stats.Ready)

	// 从未执行过：没有任何 execution_runs 记录的模板
	newDB().Model(&model.ExecutionTask{}).
		Where("NOT EXISTS (SELECT 1 FROM execution_runs WHERE execution_runs.task_id = execution_tasks.id)").
		Count(&stats.NeverExecuted)

	// 最后执行失败：最新一次 run 的 status='failed'
	newDB().Model(&model.ExecutionTask{}).
		Where("EXISTS (SELECT 1 FROM execution_runs r1 WHERE r1.task_id = execution_tasks.id AND r1.status = 'failed' AND r1.created_at = (SELECT MAX(r2.created_at) FROM execution_runs r2 WHERE r2.task_id = execution_tasks.id))").
		Count(&stats.LastRunFailed)

	return stats, nil
}
