package repository

import (
	"context"
	"time"

	"github.com/company/auto-healing/internal/model"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type ExecutionSection struct {
	TasksTotal       int64         `json:"tasks_total"`
	RunsTotal        int64         `json:"runs_total"`
	SuccessRate      float64       `json:"success_rate"`
	Running          int64         `json:"running"`
	AvgDurationSec   float64       `json:"avg_duration_sec"`
	SchedulesTotal   int64         `json:"schedules_total"`
	SchedulesEnabled int64         `json:"schedules_enabled"`
	RunsByStatus     []StatusCount `json:"runs_by_status"`
	Trend7d          []TrendPoint  `json:"trend_7d"`
	Trend30d         []TrendPoint  `json:"trend_30d"`
	SchedulesByType  []StatusCount `json:"schedules_by_type"`
	TaskTop10        []RankItem    `json:"task_top10"`
	RecentRuns       []RunItem     `json:"recent_runs"`
	FailedRuns       []RunItem     `json:"failed_runs"`
}

type RunItem struct {
	ID          uuid.UUID  `json:"id"`
	TaskName    string     `json:"task_name"`
	Status      string     `json:"status"`
	StartedAt   *time.Time `json:"started_at"`
	CompletedAt *time.Time `json:"completed_at"`
	CreatedAt   time.Time  `json:"created_at"`
}

func (r *DashboardRepository) GetExecutionSection(ctx context.Context) (*ExecutionSection, error) {
	section := &ExecutionSection{}
	db := r.tenantDB(ctx)
	tenantID, err := RequireTenantID(ctx)
	if err != nil {
		return nil, err
	}
	now := time.Now()

	countModel(db, &model.ExecutionTask{}, &section.TasksTotal)
	countModel(db, &model.ExecutionRun{}, &section.RunsTotal)
	countModel(db.Where("status = ?", "running"), &model.ExecutionRun{}, &section.Running)
	section.SuccessRate = calculateExecutionSuccessRate(db, section.RunsTotal)
	db.Model(&model.ExecutionRun{}).
		Where("started_at IS NOT NULL AND completed_at IS NOT NULL").
		Select("COALESCE(AVG(EXTRACT(EPOCH FROM (completed_at - started_at))), 0)").
		Scan(&section.AvgDurationSec)
	countModel(db, &model.ExecutionSchedule{}, &section.SchedulesTotal)
	countModel(db.Where("enabled = ?", true), &model.ExecutionSchedule{}, &section.SchedulesEnabled)
	scanStatusCounts(db, &model.ExecutionRun{}, "status", &section.RunsByStatus)
	scanTrendPoints(db, &model.ExecutionRun{}, "created_at", now.AddDate(0, 0, -7), &section.Trend7d)
	scanTrendPoints(db, &model.ExecutionRun{}, "created_at", now.AddDate(0, 0, -30), &section.Trend30d)
	scanStatusCounts(db, &model.ExecutionSchedule{}, "schedule_type", &section.SchedulesByType)
	section.TaskTop10 = listTopExecutionTasks(r.db.WithContext(ctx), tenantID)
	section.RecentRuns = listRunItems(db.Preload("Task").Order("created_at DESC").Limit(10))
	section.FailedRuns = listRunItems(db.Preload("Task").Where("status = ?", "failed").Order("created_at DESC").Limit(10))
	return section, nil
}

func calculateExecutionSuccessRate(db *gorm.DB, total int64) float64 {
	if total == 0 {
		return 0
	}
	var success int64
	countModel(db.Where("status = ?", "success"), &model.ExecutionRun{}, &success)
	return float64(success) / float64(total) * 100
}

func listTopExecutionTasks(db *gorm.DB, tenantID uuid.UUID) []RankItem {
	var items []RankItem
	db.Where("er.tenant_id = ?", tenantID).
		Table("execution_runs er").
		Select("et.name as name, count(*) as count").
		Joins("JOIN execution_tasks et ON er.task_id = et.id").
		Group("et.name").
		Order("count DESC").
		Limit(10).
		Scan(&items)
	return items
}

func listRunItems(query *gorm.DB) []RunItem {
	var runs []model.ExecutionRun
	query.Find(&runs)
	items := make([]RunItem, 0, len(runs))
	for _, run := range runs {
		taskName := ""
		if run.Task != nil {
			taskName = run.Task.Name
		}
		items = append(items, RunItem{
			ID:          run.ID,
			TaskName:    taskName,
			Status:      run.Status,
			StartedAt:   run.StartedAt,
			CompletedAt: run.CompletedAt,
			CreatedAt:   run.CreatedAt,
		})
	}
	return items
}
