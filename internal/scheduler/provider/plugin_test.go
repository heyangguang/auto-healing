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
		nextSyncAt := time.Now()
		return []model.Plugin{{ID: uuid.New(), Status: "active", NextSyncAt: &nextSyncAt}}, nil
	}
	scheduler.runPluginSync = func(ctx context.Context, plugin model.Plugin) {
		close(workerStarted)
		<-ctx.Done()
		close(workerStopped)
	}
	scheduler.claimPluginSync = func(context.Context, model.Plugin) (bool, error) {
		return true, nil
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

func TestPluginSchedulerCheckAndSyncSkipsUnclaimedPlugin(t *testing.T) {
	scheduler := NewScheduler()
	executed := 0
	next := time.Now()
	scheduler.checkExpiredMaintenance = func(context.Context) (int, error) { return 0, nil }
	scheduler.loadPluginsNeedSync = func(context.Context) ([]model.Plugin, error) {
		return []model.Plugin{{ID: uuid.New(), Name: "plugin", Status: "active", NextSyncAt: &next}}, nil
	}
	scheduler.claimPluginSync = func(context.Context, model.Plugin) (bool, error) {
		return false, nil
	}
	scheduler.runPluginSync = func(context.Context, model.Plugin) {
		executed++
	}

	scheduler.checkAndSync(context.Background())
	if executed != 0 {
		t.Fatalf("executed = %d, want 0 for unclaimed plugin", executed)
	}
}

func TestPluginSchedulerDispatchReturnsFalseWhenLifecycleUnavailable(t *testing.T) {
	scheduler := NewScheduler()
	if ok := scheduler.dispatchPluginSync(model.Plugin{ID: uuid.New()}); ok {
		t.Fatal("dispatchPluginSync() = true, want false when scheduler not running")
	}
}
