package repository

import (
	"context"
	"time"

	"github.com/company/auto-healing/internal/modules/automation/model"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

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

// GetRunStats 获取执行记录统计概览
func (r *ExecutionRepository) GetRunStats(ctx context.Context) (*RunStats, error) {
	stats := &RunStats{}
	newDB := func() *gorm.DB { return r.tenantDB(ctx) }

	if err := newDB().Model(&model.ExecutionRun{}).Count(&stats.TotalCount).Error; err != nil {
		return nil, err
	}
	if err := r.countRunStatus(newDB, "success", &stats.SuccessCount); err != nil {
		return nil, err
	}
	if err := r.countRunStatus(newDB, "failed", &stats.FailedCount); err != nil {
		return nil, err
	}
	if err := r.countRunStatus(newDB, "partial", &stats.PartialCount); err != nil {
		return nil, err
	}
	if err := r.countRunStatus(newDB, "cancelled", &stats.CancelledCount); err != nil {
		return nil, err
	}
	if stats.TotalCount > 0 {
		stats.SuccessRate = float64(stats.SuccessCount) / float64(stats.TotalCount) * 100
	}
	if err := r.loadAvgDuration(newDB, &stats.AvgDurationSec); err != nil {
		return nil, err
	}
	if err := r.loadTodayRunCount(newDB, &stats.TodayCount); err != nil {
		return nil, err
	}
	return stats, nil
}

func (r *ExecutionRepository) countRunStatus(newDB func() *gorm.DB, status string, dest *int64) error {
	return newDB().Model(&model.ExecutionRun{}).Where("status = ?", status).Count(dest).Error
}

func (r *ExecutionRepository) loadAvgDuration(newDB func() *gorm.DB, dest *float64) error {
	var avgDuration *float64
	err := newDB().Model(&model.ExecutionRun{}).
		Where("completed_at IS NOT NULL AND started_at IS NOT NULL").
		Select("EXTRACT(EPOCH FROM AVG(completed_at - started_at))").
		Scan(&avgDuration).Error
	if err != nil {
		return err
	}
	if avgDuration != nil {
		*dest = *avgDuration
	}
	return nil
}

func (r *ExecutionRepository) loadTodayRunCount(newDB func() *gorm.DB, dest *int64) error {
	now := time.Now()
	todayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	return newDB().Model(&model.ExecutionRun{}).Where("created_at >= ?", todayStart).Count(dest).Error
}

// GetRunTrend 获取执行趋势（按天+状态分组）
func (r *ExecutionRepository) GetRunTrend(ctx context.Context, days int) ([]RunTrendItem, error) {
	var items []RunTrendItem
	tenantID, err := RequireTenantID(ctx)
	if err != nil {
		return nil, err
	}
	err = r.db.WithContext(ctx).
		Model(&model.ExecutionRun{}).
		Where("tenant_id = ?", tenantID).
		Where("created_at >= ?", time.Now().AddDate(0, 0, -days)).
		Select("TO_CHAR(created_at, 'YYYY-MM-DD') as date, status, COUNT(*) as count").
		Group("date, status").
		Order("date ASC").
		Scan(&items).Error
	return items, err
}

// GetTriggerDistribution 获取触发方式分布
func (r *ExecutionRepository) GetTriggerDistribution(ctx context.Context) ([]TriggerDistItem, error) {
	var items []TriggerDistItem
	err := r.tenantDB(ctx).
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
	tenantID, err := RequireTenantID(ctx)
	if err != nil {
		return nil, err
	}
	err = r.db.WithContext(ctx).
		Table("execution_runs AS r").
		Select(`
			r.task_id::text as task_id,
			COALESCE(t.name, '已删除任务') as task_name,
			COUNT(*) as total,
			SUM(CASE WHEN r.status = 'failed' THEN 1 ELSE 0 END) as failed,
			ROUND(SUM(CASE WHEN r.status = 'failed' THEN 1 ELSE 0 END)::numeric / COUNT(*)::numeric * 100, 2) as fail_rate
		`).
		Joins("LEFT JOIN execution_tasks t ON t.id = r.task_id").
		Where("r.tenant_id = ?", tenantID).
		Group("r.task_id, t.name").
		Having("COUNT(*) >= 2").
		Order("fail_rate DESC").
		Limit(limit).
		Scan(&items).Error
	return items, err
}

// GetTopActiveTasks 获取最活跃的 Top N 任务
func (r *ExecutionRepository) GetTopActiveTasks(ctx context.Context, limit int) ([]TaskActivity, error) {
	var items []TaskActivity
	tenantID, err := RequireTenantID(ctx)
	if err != nil {
		return nil, err
	}
	err = r.db.WithContext(ctx).
		Table("execution_runs AS r").
		Select(`
			r.task_id::text as task_id,
			COALESCE(t.name, '已删除任务') as task_name,
			COUNT(*) as total
		`).
		Joins("LEFT JOIN execution_tasks t ON t.id = r.task_id").
		Where("r.tenant_id = ?", tenantID).
		Group("r.task_id, t.name").
		Order("total DESC").
		Limit(limit).
		Scan(&items).Error
	return items, err
}

// GetTaskStats 获取任务模板统计概览
func (r *ExecutionRepository) GetTaskStats(ctx context.Context) (*TaskStats, error) {
	stats := &TaskStats{}
	newDB := func() *gorm.DB { return r.tenantDB(ctx) }
	tenantID, err := RequireTenantID(ctx)
	if err != nil {
		return nil, err
	}

	if err := newDB().Model(&model.ExecutionTask{}).Count(&stats.Total).Error; err != nil {
		return nil, err
	}
	if err := newDB().Model(&model.ExecutionTask{}).Where("executor_type = ?", "docker").Count(&stats.Docker).Error; err != nil {
		return nil, err
	}
	if err := newDB().Model(&model.ExecutionTask{}).Where("executor_type != ?", "docker").Count(&stats.Local).Error; err != nil {
		return nil, err
	}
	if err := newDB().Model(&model.ExecutionTask{}).Where("needs_review = ?", true).Count(&stats.NeedsReview).Error; err != nil {
		return nil, err
	}
	if err := newDB().Model(&model.ExecutionTask{}).Where("needs_review = ?", true).Distinct("playbook_id").Count(&stats.ChangedPlaybooks).Error; err != nil {
		return nil, err
	}
	if err := r.loadReadyTaskCount(ctx, tenantID, &stats.Ready); err != nil {
		return nil, err
	}
	if err := newDB().Model(&model.ExecutionTask{}).
		Where("NOT EXISTS (SELECT 1 FROM execution_runs WHERE execution_runs.task_id = execution_tasks.id)").
		Count(&stats.NeverExecuted).Error; err != nil {
		return nil, err
	}
	if err := newDB().Model(&model.ExecutionTask{}).
		Where("EXISTS (SELECT 1 FROM execution_runs r1 WHERE r1.task_id = execution_tasks.id AND r1.status = 'failed' AND r1.created_at = (SELECT MAX(r2.created_at) FROM execution_runs r2 WHERE r2.task_id = execution_tasks.id))").
		Count(&stats.LastRunFailed).Error; err != nil {
		return nil, err
	}
	return stats, nil
}

func (r *ExecutionRepository) loadReadyTaskCount(ctx context.Context, tenantID uuid.UUID, dest *int64) error {
	return r.db.WithContext(ctx).Model(&model.ExecutionTask{}).
		Joins("JOIN playbooks ON playbooks.id = execution_tasks.playbook_id").
		Where("execution_tasks.tenant_id = ? AND execution_tasks.needs_review = ? AND playbooks.status = ?", tenantID, false, "ready").
		Count(dest).Error
}
