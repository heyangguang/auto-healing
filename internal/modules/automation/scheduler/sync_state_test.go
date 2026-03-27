package scheduler

import (
	"context"
	"errors"
	"testing"

	"github.com/company/auto-healing/internal/model"
	"github.com/google/uuid"
)

func TestExecutionSchedulerAfterScheduleTriggeredReturnsStateUpdateError(t *testing.T) {
	scheduler := NewExecutionScheduler()
	scheduler.updateScheduleLastRun = func(context.Context, uuid.UUID) error { return nil }
	scheduler.updateScheduleNextRun = func(context.Context, uuid.UUID, string) error {
		return errors.New("next run failed")
	}
	scheduler.getRun = func(context.Context, uuid.UUID) (*model.ExecutionRun, error) {
		return &model.ExecutionRun{Status: "success"}, nil
	}

	expr := "* * * * *"
	_, err := scheduler.afterScheduleTriggered(context.Background(), model.ExecutionSchedule{
		ID:           uuid.New(),
		ScheduleType: model.ScheduleTypeCron,
		ScheduleExpr: &expr,
	}, uuid.New())
	if err == nil {
		t.Fatal("afterScheduleTriggered() error = nil, want state update error")
	}
}
