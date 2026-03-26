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
		return []model.ExecutionSchedule{{ID: uuid.New()}}, nil
	}
	scheduler.runScheduledExecution = func(ctx context.Context, sched model.ExecutionSchedule) {
		close(workerStarted)
		<-ctx.Done()
		close(workerStopped)
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
