package repository

import (
	"strings"
	"testing"
	"time"

	projection "github.com/company/auto-healing/internal/modules/engagement/projection"
	"github.com/google/uuid"
	"github.com/robfig/cron/v3"
)

func TestAppendCronScheduleEntriesReturnsErrorForInvalidCron(t *testing.T) {
	repo := &WorkbenchRepository{}
	scheduleExpr := "not-a-cron"
	err := repo.appendCronScheduleEntries(
		map[string][]CalendarTask{},
		cron.NewParser(cron.Minute|cron.Hour|cron.Dom|cron.Month|cron.Dow),
		projection.ExecutionSchedule{
			ID:           uuid.New(),
			Name:         "broken",
			ScheduleExpr: &scheduleExpr,
		},
		time.Date(2026, time.March, 1, 0, 0, 0, 0, time.UTC),
		time.Date(2026, time.April, 1, 0, 0, 0, 0, time.UTC),
	)
	if err == nil {
		t.Fatal("appendCronScheduleEntries() error = nil, want invalid cron error")
	}
	if !strings.Contains(err.Error(), "broken") {
		t.Fatalf("appendCronScheduleEntries() error = %v, want schedule name in error", err)
	}
}
