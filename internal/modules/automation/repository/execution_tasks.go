package repository

import (
	"context"
	"time"

	"github.com/company/auto-healing/internal/model"
	qf "github.com/company/auto-healing/internal/pkg/query"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// CreateTask 创建任务模板
func (r *ExecutionRepository) CreateTask(ctx context.Context, task *model.ExecutionTask) error {
	if err := FillTenantID(ctx, &task.TenantID); err != nil {
		return err
	}
	return r.db.WithContext(ctx).Create(task).Error
}

// GetTaskByID 根据 ID 获取任务模板
func (r *ExecutionRepository) GetTaskByID(ctx context.Context, id uuid.UUID) (*model.ExecutionTask, error) {
	var task model.ExecutionTask
	err := r.tenantDB(ctx).
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
	err := r.tenantDB(ctx).
		Preload("Repository").
		First(&playbook, "id = ?", id).Error
	if err != nil {
		return nil, err
	}
	return &playbook, nil
}

// ListTasks 列出任务模板（支持多条件筛选）
func (r *ExecutionRepository) ListTasks(ctx context.Context, opts *TaskListOptions) ([]model.ExecutionTask, int64, error) {
	query, err := r.buildTaskListQuery(ctx, opts)
	if err != nil {
		return nil, 0, err
	}

	total, err := countWithSession(query)
	if err != nil {
		return nil, 0, err
	}

	tasks, err := r.fetchTaskListPage(query, opts)
	if err != nil {
		return nil, 0, err
	}
	if err := r.fillTaskScheduleCounts(ctx, tasks); err != nil {
		return nil, 0, err
	}

	return tasks, total, nil
}

func (r *ExecutionRepository) buildTaskListQuery(ctx context.Context, opts *TaskListOptions) (*gorm.DB, error) {
	tenantID, err := RequireTenantID(ctx)
	if err != nil {
		return nil, err
	}

	query := r.db.WithContext(ctx).Model(&model.ExecutionTask{}).
		Where("execution_tasks.tenant_id = ?", tenantID)
	query = applyTaskListJoins(query, opts)
	query = applyTaskListFilters(query, opts)
	return query, nil
}

func applyTaskListJoins(query *gorm.DB, opts *TaskListOptions) *gorm.DB {
	if opts.PlaybookName != "" || opts.RepositoryName != "" {
		query = query.Joins("LEFT JOIN playbooks ON playbooks.id = execution_tasks.playbook_id")
	}
	if opts.RepositoryName != "" {
		query = query.Joins("LEFT JOIN git_repositories ON git_repositories.id = playbooks.repository_id")
	}
	return query
}

func applyTaskListFilters(query *gorm.DB, opts *TaskListOptions) *gorm.DB {
	query = applyTaskIdentityFilters(query, opts)
	query = applyTaskMetadataFilters(query, opts)
	query = applyTaskTimeFilters(query, opts)
	query = applyTaskExecutionFilters(query, opts)
	return query
}

func applyTaskIdentityFilters(query *gorm.DB, opts *TaskListOptions) *gorm.DB {
	if opts.PlaybookID != nil {
		query = query.Where("execution_tasks.playbook_id = ?", *opts.PlaybookID)
	}
	if !opts.Name.IsEmpty() {
		query = qf.ApplyStringFilter(query, "execution_tasks.name", opts.Name)
	}
	if !opts.Description.IsEmpty() {
		query = qf.ApplyStringFilter(query, "execution_tasks.description", opts.Description)
	}
	if opts.ExecutorType != "" {
		query = query.Where("execution_tasks.executor_type = ?", opts.ExecutorType)
	}
	if opts.TargetHosts != "" {
		query = query.Where("execution_tasks.target_hosts ILIKE ?", "%"+opts.TargetHosts+"%")
	}
	if opts.PlaybookName != "" {
		query = query.Where("playbooks.name ILIKE ?", "%"+opts.PlaybookName+"%")
	}
	if opts.RepositoryName != "" {
		query = query.Where("git_repositories.name ILIKE ?", "%"+opts.RepositoryName+"%")
	}
	return query
}

func applyTaskMetadataFilters(query *gorm.DB, opts *TaskListOptions) *gorm.DB {
	if opts.Status != "" {
		switch opts.Status {
		case "pending_review":
			query = query.Where("execution_tasks.needs_review = ?", true)
		case "ready":
			query = query.Where("execution_tasks.needs_review = ?", false)
		}
	}
	if opts.NeedsReview != nil {
		query = query.Where("execution_tasks.needs_review = ?", *opts.NeedsReview)
	}
	return query
}

func applyTaskTimeFilters(query *gorm.DB, opts *TaskListOptions) *gorm.DB {
	if opts.CreatedFrom != nil {
		query = query.Where("execution_tasks.created_at >= ?", *opts.CreatedFrom)
	}
	if opts.CreatedTo != nil {
		query = query.Where("execution_tasks.created_at <= ?", *opts.CreatedTo)
	}
	return query
}

func applyTaskExecutionFilters(query *gorm.DB, opts *TaskListOptions) *gorm.DB {
	if opts.HasRuns != nil {
		if *opts.HasRuns {
			query = query.Where("EXISTS (SELECT 1 FROM execution_runs WHERE task_id = execution_tasks.id)")
		} else {
			query = query.Where("NOT EXISTS (SELECT 1 FROM execution_runs WHERE task_id = execution_tasks.id)")
		}
	}
	if opts.MinRunCount != nil && *opts.MinRunCount > 0 {
		query = query.Where("(SELECT COUNT(*) FROM execution_runs WHERE task_id = execution_tasks.id) >= ?", *opts.MinRunCount)
	}
	if opts.LastRunStatus != "" {
		query = query.Where(`EXISTS (
			SELECT 1 FROM execution_runs r
			WHERE r.task_id = execution_tasks.id
			AND r.status = ?
			AND r.created_at = (SELECT MAX(created_at) FROM execution_runs WHERE task_id = execution_tasks.id)
		)`, opts.LastRunStatus)
	}
	return query
}

func (r *ExecutionRepository) fetchTaskListPage(query *gorm.DB, opts *TaskListOptions) ([]model.ExecutionTask, error) {
	var tasks []model.ExecutionTask
	err := query.
		Preload("Playbook").
		Order(taskOrderClause(opts)).
		Offset((opts.Page - 1) * opts.PageSize).
		Limit(opts.PageSize).
		Find(&tasks).Error
	return tasks, err
}

func taskOrderClause(opts *TaskListOptions) string {
	direction := "ASC"
	if opts.SortOrder == "desc" {
		direction = "DESC"
	}
	switch opts.SortBy {
	case "name":
		return "execution_tasks.name " + direction
	case "created_at":
		return "execution_tasks.created_at " + direction
	case "updated_at":
		return "execution_tasks.updated_at " + direction
	default:
		return "execution_tasks.created_at DESC"
	}
}

func (r *ExecutionRepository) fillTaskScheduleCounts(ctx context.Context, tasks []model.ExecutionTask) error {
	if len(tasks) == 0 {
		return nil
	}

	taskIDs := make([]uuid.UUID, len(tasks))
	for i, task := range tasks {
		taskIDs[i] = task.ID
	}

	type scheduleCount struct {
		TaskID uuid.UUID `gorm:"column:task_id"`
		Count  int       `gorm:"column:count"`
	}
	var counts []scheduleCount
	if err := r.tenantDB(ctx).
		Model(&model.ExecutionSchedule{}).
		Select("task_id, COUNT(*) as count").
		Where("task_id IN ?", taskIDs).
		Group("task_id").
		Scan(&counts).Error; err != nil {
		return err
	}

	countMap := make(map[uuid.UUID]int, len(counts))
	for _, count := range counts {
		countMap[count.TaskID] = count.Count
	}
	for i := range tasks {
		tasks[i].ScheduleCount = countMap[tasks[i].ID]
	}
	return nil
}

// UpdateNextRunAt 更新下次执行时间
func (r *ExecutionRepository) UpdateNextRunAt(ctx context.Context, id uuid.UUID, nextRunAt time.Time) error {
	return r.tenantDB(ctx).
		Model(&model.ExecutionTask{}).
		Where("id = ?", id).
		Update("next_run_at", nextRunAt).Error
}

// GetScheduledTasks 获取需要执行的定时任务
func (r *ExecutionRepository) GetScheduledTasks(ctx context.Context) ([]model.ExecutionTask, error) {
	var tasks []model.ExecutionTask
	err := r.tenantDB(ctx).
		Where("is_recurring = ? AND next_run_at <= ?", true, time.Now()).
		Find(&tasks).Error
	return tasks, err
}

// ListTasksByPlaybookID 获取关联指定 Playbook 的所有任务模板
func (r *ExecutionRepository) ListTasksByPlaybookID(ctx context.Context, playbookID uuid.UUID) ([]model.ExecutionTask, error) {
	var tasks []model.ExecutionTask
	err := r.tenantDB(ctx).Where("playbook_id = ?", playbookID).Find(&tasks).Error
	return tasks, err
}

// CountTasksByPlaybookID 统计指定 Playbook 下的任务模板数量
func (r *ExecutionRepository) CountTasksByPlaybookID(ctx context.Context, playbookID uuid.UUID) (int64, error) {
	return r.countTasks(ctx, &model.ExecutionTask{}, "playbook_id = ?", playbookID)
}

// CountSchedulesByTaskID 统计指定任务模板下的调度任务数量
func (r *ExecutionRepository) CountSchedulesByTaskID(ctx context.Context, taskID uuid.UUID) (int64, error) {
	return r.countTasks(ctx, &model.ExecutionSchedule{}, "task_id = ?", taskID)
}

func (r *ExecutionRepository) countTasks(ctx context.Context, model any, predicate string, value any) (int64, error) {
	var count int64
	err := r.tenantDB(ctx).Model(model).Where(predicate, value).Count(&count).Error
	return count, err
}
