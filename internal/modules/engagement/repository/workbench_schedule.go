package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/company/auto-healing/internal/modules/engagement/model"
	projection "github.com/company/auto-healing/internal/modules/engagement/projection"
	"github.com/robfig/cron/v3"
)

// CalendarTask 日历任务项
type CalendarTask struct {
	Name       string `json:"name"`
	Time       string `json:"time"`
	ScheduleID string `json:"schedule_id"`
}

// GetScheduleCalendar 获取指定月份的定时任务日历
func (r *WorkbenchRepository) GetScheduleCalendar(ctx context.Context, year, month int) (map[string][]CalendarTask, error) {
	result := make(map[string][]CalendarTask)
	location := time.Now().Location()
	startOfMonth := time.Date(year, time.Month(month), 1, 0, 0, 0, 0, location)
	endOfMonth := startOfMonth.AddDate(0, 1, 0)

	if err := r.appendCronScheduleCalendar(ctx, result, startOfMonth, endOfMonth); err != nil {
		return nil, err
	}
	if err := r.appendOnceScheduleCalendar(ctx, result, startOfMonth, endOfMonth, location); err != nil {
		return nil, err
	}

	return result, nil
}

func (r *WorkbenchRepository) appendCronScheduleCalendar(ctx context.Context, result map[string][]CalendarTask, startOfMonth, endOfMonth time.Time) error {
	var schedules []projection.ExecutionSchedule
	err := r.tenantDB(ctx).
		Where("enabled = ? AND schedule_type = ?", true, model.ScheduleTypeCron).
		Where("schedule_expr IS NOT NULL AND schedule_expr != ''").
		Preload("Task").
		Find(&schedules).Error
	if err != nil {
		return err
	}

	parser := cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)
	for _, schedule := range schedules {
		if err := r.appendCronScheduleEntries(result, parser, schedule, startOfMonth, endOfMonth); err != nil {
			return err
		}
	}
	return nil
}

func (r *WorkbenchRepository) appendOnceScheduleCalendar(ctx context.Context, result map[string][]CalendarTask, startOfMonth, endOfMonth time.Time, location *time.Location) error {
	var schedules []projection.ExecutionSchedule
	err := r.tenantDB(ctx).
		Where("enabled = ? AND schedule_type = ?", true, model.ScheduleTypeOnce).
		Where("scheduled_at IS NOT NULL AND scheduled_at >= ? AND scheduled_at < ?", startOfMonth, endOfMonth).
		Find(&schedules).Error
	if err != nil {
		return err
	}

	for _, schedule := range schedules {
		if schedule.ScheduledAt == nil {
			continue
		}

		runAt := schedule.ScheduledAt.In(location)
		dateKey := runAt.Format("2006-01-02")
		result[dateKey] = append(result[dateKey], CalendarTask{
			Name:       schedule.Name,
			Time:       runAt.Format("15:04"),
			ScheduleID: schedule.ID.String(),
		})
	}
	return nil
}

func (r *WorkbenchRepository) appendCronScheduleEntries(result map[string][]CalendarTask, parser cron.Parser, schedule projection.ExecutionSchedule, startOfMonth, endOfMonth time.Time) error {
	if schedule.ScheduleExpr == nil || *schedule.ScheduleExpr == "" {
		return nil
	}

	parsed, err := parser.Parse(*schedule.ScheduleExpr)
	if err != nil {
		return fmt.Errorf("invalid cron schedule %s (%s): %w", schedule.ID, schedule.Name, err)
	}

	current := startOfMonth.Add(-time.Second)
	for {
		nextRun := parsed.Next(current)
		if !nextRun.Before(endOfMonth) {
			return nil
		}

		dateKey := nextRun.Format("2006-01-02")
		result[dateKey] = append(result[dateKey], CalendarTask{
			Name:       schedule.Name,
			Time:       nextRun.Format("15:04"),
			ScheduleID: schedule.ID.String(),
		})
		current = nextRun
	}
}
