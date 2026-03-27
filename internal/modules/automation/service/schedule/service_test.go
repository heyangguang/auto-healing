package schedule

import (
	"testing"
	"time"

	"github.com/company/auto-healing/internal/modules/automation/model"
)

func TestValidateAndSetNextRunCronSetsNextRunAt(t *testing.T) {
	svc := &Service{}
	expr := "*/5 * * * *"
	schedule := &model.ExecutionSchedule{
		ScheduleType: model.ScheduleTypeCron,
		ScheduleExpr: &expr,
	}

	err := svc.validateAndSetNextRun(schedule)

	if err != nil {
		t.Fatalf("validateAndSetNextRun() error = %v", err)
	}
	if schedule.NextRunAt == nil {
		t.Fatal("schedule.NextRunAt = nil, want non-nil")
	}
	if !schedule.NextRunAt.After(time.Now()) {
		t.Fatalf("schedule.NextRunAt = %v, want future time", schedule.NextRunAt)
	}
}

func TestValidateAndSetNextRunRejectsInvalidCronExpression(t *testing.T) {
	svc := &Service{}
	expr := "not-a-cron"
	schedule := &model.ExecutionSchedule{
		ScheduleType: model.ScheduleTypeCron,
		ScheduleExpr: &expr,
	}

	err := svc.validateAndSetNextRun(schedule)

	if err == nil {
		t.Fatal("validateAndSetNextRun() error = nil, want invalid cron error")
	}
}

func TestValidateAndSetNextRunOnceSetsNextRunAt(t *testing.T) {
	svc := &Service{}
	scheduledAt := time.Now().Add(10 * time.Minute)
	schedule := &model.ExecutionSchedule{
		ScheduleType: model.ScheduleTypeOnce,
		ScheduledAt:  &scheduledAt,
	}

	err := svc.validateAndSetNextRun(schedule)

	if err != nil {
		t.Fatalf("validateAndSetNextRun() error = %v", err)
	}
	if schedule.NextRunAt == nil || !schedule.NextRunAt.Equal(scheduledAt) {
		t.Fatalf("schedule.NextRunAt = %v, want %v", schedule.NextRunAt, scheduledAt)
	}
}

func TestValidateAndSetNextRunRejectsPastOnceExecution(t *testing.T) {
	svc := &Service{}
	scheduledAt := time.Now().Add(-time.Minute)
	schedule := &model.ExecutionSchedule{
		ScheduleType: model.ScheduleTypeOnce,
		ScheduledAt:  &scheduledAt,
	}

	err := svc.validateAndSetNextRun(schedule)

	if err == nil {
		t.Fatal("validateAndSetNextRun() error = nil, want past scheduled_at error")
	}
}

func TestValidateAndSetNextRunRejectsUnknownType(t *testing.T) {
	svc := &Service{}
	schedule := &model.ExecutionSchedule{ScheduleType: "manual"}

	err := svc.validateAndSetNextRun(schedule)

	if err == nil {
		t.Fatal("validateAndSetNextRun() error = nil, want invalid type error")
	}
}
