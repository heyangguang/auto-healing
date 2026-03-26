package repository

import (
	"context"
	"time"

	"github.com/company/auto-healing/internal/model"
	qf "github.com/company/auto-healing/internal/pkg/query"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// CreateRun 创建执行记录
func (r *ExecutionRepository) CreateRun(ctx context.Context, run *model.ExecutionRun) error {
	if err := FillTenantID(ctx, &run.TenantID); err != nil {
		return err
	}
	return r.db.WithContext(ctx).Create(run).Error
}

// GetRunByID 根据 ID 获取执行记录
func (r *ExecutionRepository) GetRunByID(ctx context.Context, id uuid.UUID) (*model.ExecutionRun, error) {
	var run model.ExecutionRun
	err := r.tenantDB(ctx).
		Preload("Task").
		Preload("Task.Playbook").
		First(&run, "id = ?", id).Error
	if err != nil {
		return nil, err
	}
	return &run, nil
}

// GetRunStatus 获取执行记录状态（轻量查询，供取消轮询使用）
func (r *ExecutionRepository) GetRunStatus(ctx context.Context, id uuid.UUID) (string, error) {
	var status string
	err := r.tenantDB(ctx).
		Model(&model.ExecutionRun{}).
		Where("id = ?", id).
		Pluck("status", &status).Error
	return status, err
}

// ListRunsByTaskID 列出任务的执行记录
func (r *ExecutionRepository) ListRunsByTaskID(ctx context.Context, taskID uuid.UUID, page, pageSize int) ([]model.ExecutionRun, int64, error) {
	query := r.tenantDB(ctx).Model(&model.ExecutionRun{}).Where("task_id = ?", taskID)
	total, err := countWithSession(query)
	if err != nil {
		return nil, 0, err
	}

	var runs []model.ExecutionRun
	err = query.Order("created_at DESC").
		Offset((page - 1) * pageSize).
		Limit(pageSize).
		Find(&runs).Error
	return runs, total, err
}

// ListAllRuns 列出所有执行记录（跨任务模板，支持多条件筛选）
func (r *ExecutionRepository) ListAllRuns(ctx context.Context, opts *RunListOptions) ([]model.ExecutionRun, int64, error) {
	query, err := r.buildRunListQuery(ctx, opts)
	if err != nil {
		return nil, 0, err
	}

	total, err := countWithSession(query)
	if err != nil {
		return nil, 0, err
	}

	var runs []model.ExecutionRun
	err = query.Preload("Task").
		Order("execution_runs.created_at DESC").
		Offset((opts.Page - 1) * opts.PageSize).
		Limit(opts.PageSize).
		Find(&runs).Error
	return runs, total, err
}

func (r *ExecutionRepository) buildRunListQuery(ctx context.Context, opts *RunListOptions) (*gorm.DB, error) {
	tenantID, err := RequireTenantID(ctx)
	if err != nil {
		return nil, err
	}

	query := r.db.WithContext(ctx).Model(&model.ExecutionRun{}).
		Where("execution_runs.tenant_id = ?", tenantID)
	if !opts.TaskName.IsEmpty() {
		query = query.Joins("LEFT JOIN execution_tasks ON execution_tasks.id = execution_runs.task_id")
		query = qf.ApplyStringFilter(query, "execution_tasks.name", opts.TaskName)
	}
	query = applyRunListFilters(query, opts)
	return query, nil
}

func applyRunListFilters(query *gorm.DB, opts *RunListOptions) *gorm.DB {
	if opts.RunID != "" {
		query = query.Where("execution_runs.id::text ILIKE ?", opts.RunID+"%")
	}
	if opts.TaskID != nil {
		query = query.Where("execution_runs.task_id = ?", *opts.TaskID)
	}
	if opts.Status != "" {
		query = query.Where("execution_runs.status = ?", opts.Status)
	}
	if opts.TriggeredBy != "" {
		query = query.Where("execution_runs.triggered_by = ?", opts.TriggeredBy)
	}
	if opts.StartedAfter != nil {
		query = query.Where("execution_runs.started_at >= ?", *opts.StartedAfter)
	}
	if opts.StartedBefore != nil {
		query = query.Where("execution_runs.started_at <= ?", *opts.StartedBefore)
	}
	return query
}

// UpdateRunStatus 更新执行状态
func (r *ExecutionRepository) UpdateRunStatus(ctx context.Context, id uuid.UUID, status string) (bool, error) {
	updates := map[string]any{"status": status}
	if status == "cancelled" {
		updates["completed_at"] = time.Now()
	}

	result := executionRunStatusScope(r.tenantDB(ctx).Model(&model.ExecutionRun{}), status).
		Where("id = ?", id).
		Updates(updates)
	return result.RowsAffected > 0, result.Error
}

func executionRunStatusScope(query *gorm.DB, status string) *gorm.DB {
	if status == "cancelled" {
		return query.Where("status IN ?", []string{"pending", "running"})
	}
	return query
}

// UpdateRunStarted 更新执行开始
func (r *ExecutionRepository) UpdateRunStarted(ctx context.Context, id uuid.UUID) (bool, error) {
	result := r.tenantDB(ctx).
		Model(&model.ExecutionRun{}).
		Where("id = ? AND status = ?", id, "pending").
		Updates(map[string]any{
			"status":     "running",
			"started_at": time.Now(),
		})
	return result.RowsAffected > 0, result.Error
}

// UpdateRunResult 更新执行结果
func (r *ExecutionRepository) UpdateRunResult(ctx context.Context, id uuid.UUID, exitCode int, stdout, stderr string, stats model.JSON) error {
	_, err := r.UpdateRunResultIfCurrent(ctx, id, nil, exitCode, stdout, stderr, stats)
	return err
}

func (r *ExecutionRepository) UpdateRunResultIfCurrent(ctx context.Context, id uuid.UUID, currentStatuses []string, exitCode int, stdout, stderr string, stats model.JSON) (bool, error) {
	query := r.tenantDB(ctx).Model(&model.ExecutionRun{}).Where("id = ?", id)
	if len(currentStatuses) > 0 {
		query = query.Where("status IN ?", currentStatuses)
	} else {
		query = query.Where("status <> ?", "cancelled")
	}

	result := query.Updates(map[string]any{
		"status":       resolveRunStatus(exitCode, stats),
		"exit_code":    exitCode,
		"stdout":       stdout,
		"stderr":       stderr,
		"stats":        stats,
		"completed_at": time.Now(),
	})
	return result.RowsAffected > 0, result.Error
}

func resolveRunStatus(exitCode int, stats model.JSON) string {
	if stats == nil {
		if exitCode == 0 {
			return "success"
		}
		return "failed"
	}

	okCount := getStatValue(stats, "ok")
	failed := getStatValue(stats, "failed")
	unreachable := getStatValue(stats, "unreachable")
	if okCount == 0 {
		return "failed"
	}
	if failed == 0 && unreachable == 0 {
		return "success"
	}
	return "partial"
}

func getStatValue(stats model.JSON, key string) float64 {
	if value, ok := stats[key].(float64); ok {
		return value
	}
	if value, ok := stats[key].(int); ok {
		return float64(value)
	}
	return 0
}
