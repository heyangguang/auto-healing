package repository

import (
	"context"
	platformrepo "github.com/company/auto-healing/internal/platform/repositoryx"
	"time"

	projection "github.com/company/auto-healing/internal/modules/engagement/projection"
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
	newDB := func() *gorm.DB { return r.tenantDB(ctx) }
	tenantID, err := platformrepo.RequireTenantID(ctx)
	if err != nil {
		return nil, err
	}
	now := time.Now()

	if err := countModel(newDB(), &projection.ExecutionTask{}, &section.TasksTotal); err != nil {
		return nil, err
	}
	if err := countModel(newDB(), &projection.ExecutionRun{}, &section.RunsTotal); err != nil {
		return nil, err
	}
	if err := countModel(newDB().Where("status = ?", "running"), &projection.ExecutionRun{}, &section.Running); err != nil {
		return nil, err
	}
	rate, err := calculateExecutionSuccessRate(newDB(), section.RunsTotal)
	if err != nil {
		return nil, err
	}
	section.SuccessRate = rate
	avgDuration, err := executionAvgDurationSeconds(newDB())
	if err != nil {
		return nil, err
	}
	section.AvgDurationSec = avgDuration
	if err := countModel(newDB(), &projection.ExecutionSchedule{}, &section.SchedulesTotal); err != nil {
		return nil, err
	}
	if err := countModel(newDB().Where("enabled = ?", true), &projection.ExecutionSchedule{}, &section.SchedulesEnabled); err != nil {
		return nil, err
	}
	if err := scanStatusCounts(newDB(), &projection.ExecutionRun{}, "status", &section.RunsByStatus); err != nil {
		return nil, err
	}
	if err := scanTrendPoints(newDB(), &projection.ExecutionRun{}, "created_at", now.AddDate(0, 0, -7), &section.Trend7d); err != nil {
		return nil, err
	}
	if err := scanTrendPoints(newDB(), &projection.ExecutionRun{}, "created_at", now.AddDate(0, 0, -30), &section.Trend30d); err != nil {
		return nil, err
	}
	if err := scanStatusCounts(newDB(), &projection.ExecutionSchedule{}, "schedule_type", &section.SchedulesByType); err != nil {
		return nil, err
	}
	top10, err := listTopExecutionTasks(r.db.WithContext(ctx), tenantID)
	if err != nil {
		return nil, err
	}
	section.TaskTop10 = top10
	recent, err := listRunItems(newDB().Preload("Task", "tenant_id = ?", tenantID).Order("created_at DESC").Limit(10))
	if err != nil {
		return nil, err
	}
	section.RecentRuns = recent
	failed, err := listRunItems(newDB().Preload("Task", "tenant_id = ?", tenantID).Where("status = ?", "failed").Order("created_at DESC").Limit(10))
	if err != nil {
		return nil, err
	}
	section.FailedRuns = failed
	return section, nil
}

func calculateExecutionSuccessRate(db *gorm.DB, total int64) (float64, error) {
	if total == 0 {
		return 0, nil
	}
	var success int64
	if err := countModel(db.Where("status = ?", "success"), &projection.ExecutionRun{}, &success); err != nil {
		return 0, err
	}
	return float64(success) / float64(total) * 100, nil
}

func executionAvgDurationSeconds(db *gorm.DB) (float64, error) {
	var avgDuration float64
	query := db.Model(&projection.ExecutionRun{}).
		Where("started_at IS NOT NULL AND completed_at IS NOT NULL")
	if db.Dialector.Name() == "sqlite" {
		err := query.Select("COALESCE(AVG((julianday(completed_at) - julianday(started_at)) * 86400.0), 0)").
			Scan(&avgDuration).Error
		return avgDuration, err
	}
	err := query.Select("COALESCE(AVG(EXTRACT(EPOCH FROM (completed_at - started_at))), 0)").
		Scan(&avgDuration).Error
	return avgDuration, err
}

func listTopExecutionTasks(db *gorm.DB, tenantID uuid.UUID) ([]RankItem, error) {
	var items []RankItem
	if err := db.Where("er.tenant_id = ?", tenantID).
		Table("execution_runs er").
		Select("et.name as name, count(*) as count").
		Joins("JOIN execution_tasks et ON er.task_id = et.id AND et.tenant_id = ?", tenantID).
		Group("et.name").
		Order("count DESC").
		Limit(10).
		Scan(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}

func listRunItems(query *gorm.DB) ([]RunItem, error) {
	var runs []projection.ExecutionRun
	if err := query.Find(&runs).Error; err != nil {
		return nil, err
	}
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
	return items, nil
}
