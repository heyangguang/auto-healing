package provider

import (
	"context"
	"errors"
	"testing"
	"time"

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

	_, err := scheduler.afterScheduleTriggered(context.Background(), model.ExecutionSchedule{
		ID:           uuid.New(),
		ScheduleType: model.ScheduleTypeCron,
		ScheduleExpr: stringPtr("* * * * *"),
	}, uuid.New())
	if err == nil {
		t.Fatal("afterScheduleTriggered() error = nil, want state update error")
	}
}

func TestGitSchedulerHandleSyncSuccessSkipsSuccessLogOnPersistError(t *testing.T) {
	scheduler := NewGitScheduler()
	scheduler.updateSyncState = func(context.Context, interface{}, map[string]interface{}) error {
		return errors.New("persist failed")
	}

	scheduler.handleGitSyncSuccess(context.Background(), model.GitRepository{
		ID:            uuid.New(),
		Name:          "repo",
		DefaultBranch: "main",
	}, "abcd1234", zeroTime(), zeroDuration())
}

func TestPluginSchedulerHandleSyncResultSkipsSuccessLogOnPersistError(t *testing.T) {
	scheduler := NewScheduler()
	scheduler.updateSyncState = func(context.Context, interface{}, map[string]interface{}) error {
		return errors.New("persist failed")
	}

	scheduler.handlePluginSyncResult(context.Background(), model.Plugin{
		ID:   uuid.New(),
		Name: "plugin",
	}, &model.PluginSyncLog{Status: "success"}, "abcd1234", zeroTime(), zeroDuration())
}

func stringPtr(value string) *string {
	return &value
}

func zeroTime() time.Time {
	return time.Time{}
}

func zeroDuration() time.Duration {
	return 0
}
