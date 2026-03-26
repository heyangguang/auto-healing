package provider

import (
	"context"
	"testing"
	"time"

	"github.com/company/auto-healing/internal/model"
	"github.com/google/uuid"
)

func TestPluginSchedulerStopWaitsForSyncWorker(t *testing.T) {
	scheduler := NewScheduler()
	scheduler.interval = time.Hour

	workerStarted := make(chan struct{})
	workerStopped := make(chan struct{})
	scheduler.checkExpiredMaintenance = func(context.Context) (int, error) { return 0, nil }
	scheduler.loadPluginsNeedSync = func(context.Context) ([]model.Plugin, error) {
		return []model.Plugin{{ID: uuid.New()}}, nil
	}
	scheduler.runPluginSync = func(ctx context.Context, plugin model.Plugin) {
		close(workerStarted)
		<-ctx.Done()
		close(workerStopped)
	}

	scheduler.Start()

	select {
	case <-workerStarted:
	case <-time.After(time.Second):
		t.Fatal("sync worker did not start")
	}

	scheduler.Stop()

	select {
	case <-workerStopped:
	case <-time.After(time.Second):
		t.Fatal("sync worker did not stop before Stop returned")
	}
}
