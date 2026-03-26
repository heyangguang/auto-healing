package provider

import (
	"context"
	"testing"
	"time"

	"github.com/company/auto-healing/internal/model"
	"github.com/google/uuid"
)

func TestRunStatusCountsAsSuccess(t *testing.T) {
	t.Helper()

	cases := map[string]bool{
		"success":   true,
		"partial":   true,
		"failed":    false,
		"cancelled": false,
		"running":   false,
	}

	for status, want := range cases {
		if got := runStatusCountsAsSuccess(status); got != want {
			t.Fatalf("runStatusCountsAsSuccess(%q) = %v, want %v", status, got, want)
		}
	}
}

func TestExecutionSchedulerStopWaitsForScheduledWorker(t *testing.T) {
	scheduler := NewExecutionScheduler()
	scheduler.interval = time.Hour

	workerStarted := make(chan struct{})
	workerStopped := make(chan struct{})
	scheduler.loadDueSchedules = func(context.Context) ([]model.ExecutionSchedule, error) {
		nextRunAt := time.Now()
		scheduleExpr := "* * * * *"
		return []model.ExecutionSchedule{{
			ID:           uuid.New(),
			Enabled:      true,
			ScheduleType: model.ScheduleTypeCron,
			ScheduleExpr: &scheduleExpr,
			NextRunAt:    &nextRunAt,
		}}, nil
	}
	scheduler.runScheduledExecution = func(ctx context.Context, sched model.ExecutionSchedule) {
		close(workerStarted)
		<-ctx.Done()
		close(workerStopped)
	}
	scheduler.claimDueSchedule = func(context.Context, model.ExecutionSchedule) (bool, error) {
		return true, nil
	}

	scheduler.Start()

	select {
	case <-workerStarted:
	case <-time.After(time.Second):
		t.Fatal("scheduled worker did not start")
	}

	scheduler.Stop()

	select {
	case <-workerStopped:
	case <-time.After(time.Second):
		t.Fatal("scheduled worker did not stop before Stop returned")
	}
}

func TestExecutionSchedulerCheckAndExecuteSkipsUnclaimedSchedule(t *testing.T) {
	scheduler := NewExecutionScheduler()
	executed := 0

	scheduler.loadDueSchedules = func(context.Context) ([]model.ExecutionSchedule, error) {
		expr := "* * * * *"
		next := time.Now()
		return []model.ExecutionSchedule{{
			ID:           uuid.New(),
			Name:         "job",
			Enabled:      true,
			ScheduleType: model.ScheduleTypeCron,
			ScheduleExpr: &expr,
			NextRunAt:    &next,
		}}, nil
	}
	scheduler.claimDueSchedule = func(context.Context, model.ExecutionSchedule) (bool, error) {
		return false, nil
	}
	scheduler.runScheduledExecution = func(context.Context, model.ExecutionSchedule) {
		executed++
	}

	scheduler.checkAndExecute(context.Background())
	if executed != 0 {
		t.Fatalf("executed = %d, want 0 for unclaimed schedule", executed)
	}
}

func TestExecutionSchedulerDispatchReturnsFalseWhenLifecycleUnavailable(t *testing.T) {
	scheduler := NewExecutionScheduler()
	if ok := scheduler.dispatchScheduledExecution(model.ExecutionSchedule{ID: uuid.New()}); ok {
		t.Fatal("dispatchScheduledExecution() = true, want false when scheduler not running")
	}
}
