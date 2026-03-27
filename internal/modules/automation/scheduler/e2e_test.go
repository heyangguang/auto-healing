package scheduler

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/company/auto-healing/internal/modules/automation/model"
	executionService "github.com/company/auto-healing/internal/modules/automation/service/execution"
	platformsched "github.com/company/auto-healing/internal/platform/schedulerx"
	"github.com/google/uuid"
)

func TestExecutionSchedulerEndToEndCronRunCompletesAndReleasesInFlight(t *testing.T) {
	scheduler := NewExecutionScheduler()
	scheduler.lifecycle = platformsched.NewLifecycle()
	defer scheduler.lifecycle.Stop()

	scheduleID := uuid.New()
	taskID := uuid.New()
	runID := uuid.New()
	expr := "*/5 * * * *"

	var (
		mu               sync.Mutex
		updateLastRunAt  bool
		updateNextRunAt  bool
		successStateSeen bool
		getRunCalls      int
		successStateDone = make(chan struct{}, 1)
	)

	scheduler.loadDueSchedules = func(context.Context) ([]model.ExecutionSchedule, error) {
		return []model.ExecutionSchedule{{
			ID:           scheduleID,
			Name:         "cron-e2e",
			TaskID:       taskID,
			ScheduleType: model.ScheduleTypeCron,
			ScheduleExpr: &expr,
		}}, nil
	}
	scheduler.claimDueSchedule = func(context.Context, model.ExecutionSchedule) (bool, error) {
		return true, nil
	}
	scheduler.executeTask = func(context.Context, uuid.UUID, *executionService.ExecuteOptions) (*model.ExecutionRun, error) {
		return &model.ExecutionRun{ID: runID}, nil
	}
	scheduler.getRun = func(context.Context, uuid.UUID) (*model.ExecutionRun, error) {
		mu.Lock()
		defer mu.Unlock()
		getRunCalls++
		return &model.ExecutionRun{Status: "success"}, nil
	}
	scheduler.updateLastRunAt = func(context.Context, uuid.UUID) error {
		mu.Lock()
		updateLastRunAt = true
		mu.Unlock()
		return nil
	}
	scheduler.updateNextRunAt = func(context.Context, uuid.UUID, string) error {
		mu.Lock()
		updateNextRunAt = true
		mu.Unlock()
		return nil
	}
	scheduler.updateScheduleState = func(context.Context, uuid.UUID, map[string]interface{}) error {
		mu.Lock()
		successStateSeen = true
		mu.Unlock()
		select {
		case successStateDone <- struct{}{}:
		default:
		}
		return nil
	}
	scheduler.markCompleted = func(context.Context, uuid.UUID) error {
		t.Fatal("markCompleted should not be called for cron schedules")
		return nil
	}

	scheduler.checkAndExecute(context.Background())

	select {
	case <-successStateDone:
	case <-time.After(time.Second):
		t.Fatal("expected cron schedule to complete end-to-end")
	}

	mu.Lock()
	defer mu.Unlock()
	if !updateLastRunAt {
		t.Fatal("expected updateLastRunAt to be called")
	}
	if !updateNextRunAt {
		t.Fatal("expected updateNextRunAt to be called")
	}
	if !successStateSeen {
		t.Fatal("expected success state update to be called")
	}
	if getRunCalls < 1 {
		t.Fatalf("expected run status polling, got %d calls", getRunCalls)
	}
	deadline := time.Now().Add(time.Second)
	for {
		if scheduler.inFlight.Start(scheduleID) {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("expected in-flight slot to be released after cron completion")
		}
		time.Sleep(10 * time.Millisecond)
	}
}
