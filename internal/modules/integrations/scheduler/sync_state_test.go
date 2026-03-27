package scheduler

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/company/auto-healing/internal/modules/integrations/model"
	"github.com/google/uuid"
)

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
	scheduler := NewPluginScheduler()
	scheduler.updateSyncState = func(context.Context, interface{}, map[string]interface{}) error {
		return errors.New("persist failed")
	}

	scheduler.handlePluginSyncResult(context.Background(), model.Plugin{
		ID:   uuid.New(),
		Name: "plugin",
	}, &model.PluginSyncLog{Status: "success"}, "abcd1234", zeroTime(), zeroDuration())
}

func zeroTime() time.Time {
	return time.Time{}
}

func zeroDuration() time.Duration {
	return 0
}
