package scheduler

import (
	"context"
	"testing"
	"time"

	"github.com/company/auto-healing/internal/modules/automation/model"
	platformsched "github.com/company/auto-healing/internal/platform/schedulerx"
	"github.com/google/uuid"
)

func TestExecutionSchedulerStopWaitsForScheduledWorker(t *testing.T) {
	scheduler := newExecutionSchedulerForTest()
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

func TestExecutionSchedulerDoesNotDispatchSameScheduleTwiceWhileInFlight(t *testing.T) {
	scheduler := newExecutionSchedulerForTest()
	scheduler.lifecycle = platformsched.NewLifecycle()
	defer scheduler.lifecycle.Stop()

	scheduleID := uuid.New()
	started := make(chan struct{}, 2)
	release := make(chan struct{})
	scheduler.loadDueSchedules = func(context.Context) ([]model.ExecutionSchedule, error) {
		return []model.ExecutionSchedule{{
			ID:   scheduleID,
			Name: "dedupe",
		}}, nil
	}
	scheduler.runScheduledExecution = func(context.Context, model.ExecutionSchedule) {
		started <- struct{}{}
		<-release
	}
	scheduler.claimDueSchedule = func(context.Context, model.ExecutionSchedule) (bool, error) {
		return true, nil
	}

	scheduler.checkAndExecute(context.Background())

	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("scheduled worker did not start")
	}

	scheduler.checkAndExecute(context.Background())

	select {
	case <-started:
		t.Fatal("duplicate schedule dispatch started while the first run was still in flight")
	case <-time.After(100 * time.Millisecond):
	}

	close(release)
}

func TestExecutionSchedulerAfterScheduleTriggeredDetachesOnSchedulerStop(t *testing.T) {
	scheduler := newExecutionSchedulerForTest()
	expr := "*/5 * * * *"
	runID := uuid.New()
	detached := make(chan struct{}, 1)

	scheduler.updateLastRunAt = func(context.Context, uuid.UUID) error { return nil }
	scheduler.updateNextRunAt = func(context.Context, uuid.UUID, string) error { return nil }
	scheduler.getRun = func(context.Context, uuid.UUID) (*model.ExecutionRun, error) {
		return &model.ExecutionRun{Status: "running"}, nil
	}
	scheduler.followRunAfterStop = func(ctx context.Context, sched model.ExecutionSchedule, gotRunID uuid.UUID) {
		if gotRunID != runID {
			t.Fatalf("followRunAfterStop runID = %s, want %s", gotRunID, runID)
		}
		select {
		case detached <- struct{}{}:
		default:
		}
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	result, err := scheduler.afterScheduleTriggered(ctx, model.ExecutionSchedule{
		ID:           uuid.New(),
		ScheduleType: model.ScheduleTypeCron,
		ScheduleExpr: &expr,
	}, runID)
	if err != nil {
		t.Fatalf("afterScheduleTriggered() error = %v", err)
	}
	if result != scheduleTriggerDetached {
		t.Fatalf("afterScheduleTriggered() result = %v, want detached", result)
	}

	select {
	case <-detached:
	case <-time.After(time.Second):
		t.Fatal("expected detached follow-up tracking to be scheduled")
	}
}

func TestExecutionSchedulerAfterScheduleTriggeredTreatsCanceledGetRunAsStop(t *testing.T) {
	scheduler := newExecutionSchedulerForTest()
	expr := "*/5 * * * *"
	runID := uuid.New()
	detached := make(chan struct{}, 1)

	scheduler.updateLastRunAt = func(context.Context, uuid.UUID) error { return nil }
	scheduler.updateNextRunAt = func(context.Context, uuid.UUID, string) error { return nil }
	scheduler.getRun = func(ctx context.Context, id uuid.UUID) (*model.ExecutionRun, error) {
		return nil, ctx.Err()
	}
	scheduler.followRunAfterStop = func(ctx context.Context, sched model.ExecutionSchedule, gotRunID uuid.UUID) {
		if gotRunID != runID {
			t.Fatalf("followRunAfterStop runID = %s, want %s", gotRunID, runID)
		}
		detached <- struct{}{}
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	result, err := scheduler.afterScheduleTriggered(ctx, model.ExecutionSchedule{
		ID:           uuid.New(),
		ScheduleType: model.ScheduleTypeCron,
		ScheduleExpr: &expr,
	}, runID)
	if err != nil {
		t.Fatalf("afterScheduleTriggered() error = %v", err)
	}
	if result != scheduleTriggerDetached {
		t.Fatalf("afterScheduleTriggered() result = %v, want detached", result)
	}

	select {
	case <-detached:
	case <-time.After(time.Second):
		t.Fatal("expected detached follow-up tracking when ctx was canceled")
	}
}

func TestExecutionSchedulerAfterScheduleTriggeredRetainsInFlightUntilDetachedFollowUpFinishes(t *testing.T) {
	scheduler := newExecutionSchedulerForTest()
	expr := "*/5 * * * *"
	scheduleID := uuid.New()
	runID := uuid.New()

	if !scheduler.inFlight.Start(scheduleID) {
		t.Fatal("expected initial in-flight start")
	}
	scheduler.updateLastRunAt = func(context.Context, uuid.UUID) error { return nil }
	scheduler.updateNextRunAt = func(context.Context, uuid.UUID, string) error { return nil }
	scheduler.getRun = func(ctx context.Context, id uuid.UUID) (*model.ExecutionRun, error) {
		return nil, ctx.Err()
	}
	scheduler.followRunAfterStop = func(context.Context, model.ExecutionSchedule, uuid.UUID) {}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	result, err := scheduler.afterScheduleTriggered(ctx, model.ExecutionSchedule{
		ID:           scheduleID,
		ScheduleType: model.ScheduleTypeCron,
		ScheduleExpr: &expr,
	}, runID)
	if err != nil {
		t.Fatalf("afterScheduleTriggered() error = %v", err)
	}
	if result != scheduleTriggerDetached {
		t.Fatalf("afterScheduleTriggered() result = %v, want detached", result)
	}

	scheduler.inFlight.Finish(scheduleID)
	if scheduler.inFlight.Start(scheduleID) {
		t.Fatal("expected detached run to keep in-flight hold after worker release")
	}
	scheduler.inFlight.Finish(scheduleID)
}
