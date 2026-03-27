package scheduler

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/company/auto-healing/internal/modules/automation/model"
	executionService "github.com/company/auto-healing/internal/modules/automation/service/execution"
	platformsched "github.com/company/auto-healing/internal/platform/schedulerx"
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

func TestBuildExecutionOptionsRejectsInvalidSecretID(t *testing.T) {
	_, err := buildExecutionOptions(model.ExecutionSchedule{
		SecretsSourceIDs: model.StringArray{"not-a-uuid"},
	})
	if err == nil || !strings.Contains(err.Error(), "无效的密钥源 ID") {
		t.Fatalf("expected invalid secret id error, got %v", err)
	}
}

func TestExecutionSchedulerWaitForRunTerminalStatusTreatsTimeoutAsTerminal(t *testing.T) {
	scheduler := newExecutionSchedulerForTest()
	scheduler.getRun = func(context.Context, uuid.UUID) (*model.ExecutionRun, error) {
		return &model.ExecutionRun{Status: "timeout"}, nil
	}

	status, err := scheduler.waitForRunTerminalStatus(context.Background(), uuid.New())
	if err != nil {
		t.Fatalf("waitForRunTerminalStatus() error = %v", err)
	}
	if status != "timeout" {
		t.Fatalf("waitForRunTerminalStatus() status = %q, want timeout", status)
	}
}

func TestExecutionSchedulerAfterScheduleTriggeredReturnsPersistenceError(t *testing.T) {
	scheduler := newExecutionSchedulerForTest()
	expr := "*/5 * * * *"
	scheduler.updateLastRunAt = func(context.Context, uuid.UUID) error {
		return errors.New("write failed")
	}
	scheduler.getRun = func(context.Context, uuid.UUID) (*model.ExecutionRun, error) {
		return &model.ExecutionRun{Status: "success"}, nil
	}

	_, err := scheduler.afterScheduleTriggered(context.Background(), model.ExecutionSchedule{
		ID:           uuid.New(),
		ScheduleType: model.ScheduleTypeCron,
		ScheduleExpr: &expr,
	}, uuid.New())
	if err == nil || !strings.Contains(err.Error(), "更新上次执行时间失败") {
		t.Fatalf("expected persistence error, got %v", err)
	}
}

func TestExecutionSchedulerOnceFailureBeforeRunDoesNotMarkCompleted(t *testing.T) {
	scheduler := newExecutionSchedulerForTest()
	scheduleID := uuid.New()
	updated := make(map[string]interface{})
	markCompletedCalled := false

	scheduler.executeTask = func(context.Context, uuid.UUID, *executionService.ExecuteOptions) (*model.ExecutionRun, error) {
		return nil, errors.New("start failed")
	}
	scheduler.updateScheduleState = func(ctx context.Context, id uuid.UUID, updates map[string]interface{}) error {
		if id != scheduleID {
			t.Fatalf("updateScheduleState id = %s, want %s", id, scheduleID)
		}
		for k, v := range updates {
			updated[k] = v
		}
		return nil
	}
	scheduler.markCompleted = func(context.Context, uuid.UUID) error {
		markCompletedCalled = true
		return nil
	}

	scheduler.executeSchedule(context.Background(), model.ExecutionSchedule{
		ID:           scheduleID,
		Name:         "once",
		TaskID:       uuid.New(),
		ScheduleType: model.ScheduleTypeOnce,
	})

	if markCompletedCalled {
		t.Fatal("expected once schedule without a started run to avoid MarkCompleted")
	}
	if updated["status"] != model.ScheduleStatusDisabled {
		t.Fatalf("status = %v, want %s", updated["status"], model.ScheduleStatusDisabled)
	}
	if updated["enabled"] != false {
		t.Fatalf("enabled = %v, want false", updated["enabled"])
	}
	if updated["next_run_at"] != nil {
		t.Fatalf("next_run_at = %v, want nil", updated["next_run_at"])
	}
}

func TestExecutionSchedulerDispatchAfterStopDoesNotRunWorkerOrLeakSemaphore(t *testing.T) {
	scheduler := newExecutionSchedulerForTest()
	scheduler.lifecycle = platformsched.NewLifecycle()
	scheduler.running = true

	scheduler.Stop()

	workerStarted := make(chan struct{}, 1)
	scheduler.runScheduledExecution = func(context.Context, model.ExecutionSchedule) {
		workerStarted <- struct{}{}
	}

	scheduler.sem <- struct{}{}
	scheduler.dispatchScheduledExecution(scheduler.lifecycleSnapshot(), model.ExecutionSchedule{ID: uuid.New()})

	select {
	case <-workerStarted:
		t.Fatal("scheduled worker should not start after Stop")
	case <-time.After(50 * time.Millisecond):
	}

	if len(scheduler.sem) != 0 {
		t.Fatalf("expected semaphore token to be released, got len=%d", len(scheduler.sem))
	}
}
