package scheduler

import (
	"context"
	"testing"
	"time"

	"github.com/company/auto-healing/internal/modules/integrations/model"
	schedulerx "github.com/company/auto-healing/internal/platform/schedulerx"
	"github.com/google/uuid"
)

func TestPluginSchedulerStopWaitsForSyncWorker(t *testing.T) {
	scheduler := NewPluginScheduler()
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

func TestPluginSchedulerDispatchAfterStopDoesNotRunWorker(t *testing.T) {
	scheduler := NewPluginScheduler()
	scheduler.lifecycle = schedulerx.NewLifecycle()
	scheduler.running = true

	scheduler.Stop()

	workerStarted := make(chan struct{}, 1)
	scheduler.runPluginSync = func(context.Context, model.Plugin) {
		workerStarted <- struct{}{}
	}

	scheduler.dispatchPluginSync(scheduler.lifecycleSnapshot(), model.Plugin{ID: uuid.New()})

	select {
	case <-workerStarted:
		t.Fatal("sync worker should not start after Stop")
	case <-time.After(50 * time.Millisecond):
	}
}

func TestPluginSchedulerDoesNotDispatchSamePluginTwiceWhileInFlight(t *testing.T) {
	scheduler := NewPluginScheduler()
	scheduler.lifecycle = schedulerx.NewLifecycle()
	defer scheduler.lifecycle.Stop()

	pluginID := uuid.New()
	release := make(chan struct{})
	started := make(chan struct{}, 2)
	scheduler.runPluginSync = func(context.Context, model.Plugin) {
		started <- struct{}{}
		<-release
	}

	scheduler.dispatchPluginSync(scheduler.lifecycleSnapshot(), model.Plugin{ID: pluginID})
	scheduler.dispatchPluginSync(scheduler.lifecycleSnapshot(), model.Plugin{ID: pluginID})

	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("first sync worker did not start")
	}

	select {
	case <-started:
		t.Fatal("same plugin should not be dispatched twice while in flight")
	case <-time.After(100 * time.Millisecond):
	}

	close(release)
}

func TestPluginSchedulerSyncUsesCompletionTimeForNextSyncAt(t *testing.T) {
	scheduler := NewPluginScheduler()
	pluginID := uuid.New()
	base := time.Date(2026, 3, 26, 10, 0, 0, 0, time.UTC)
	plugin := model.Plugin{
		ID:                  pluginID,
		Name:                "plugin-a",
		SyncIntervalMinutes: 5,
	}

	current := base
	scheduler.now = func() time.Time { return current }
	scheduler.triggerPluginSync = func(context.Context, uuid.UUID) (*model.PluginSyncLog, error) {
		current = base.Add(10 * time.Minute)
		return &model.PluginSyncLog{Status: "success"}, nil
	}
	scheduler.updatePluginState = func(_ context.Context, id interface{}, updates map[string]interface{}) error {
		if id != pluginID {
			t.Fatalf("unexpected plugin id: %v", id)
		}
		nextSyncAt, ok := updates["next_sync_at"].(time.Time)
		if !ok {
			t.Fatalf("next_sync_at type = %T, want time.Time", updates["next_sync_at"])
		}
		want := base.Add(15 * time.Minute)
		if !nextSyncAt.Equal(want) {
			t.Fatalf("next_sync_at = %v, want %v", nextSyncAt, want)
		}
		return nil
	}

	scheduler.syncPlugin(context.Background(), plugin)
}

func TestFilterDuePluginsSkipsRecentlySyncedPluginWithStaleNextSyncAt(t *testing.T) {
	now := time.Date(2026, 3, 26, 10, 0, 0, 0, time.UTC)
	lastSyncAt := now.Add(-2 * time.Minute)
	due := filterDuePlugins([]model.Plugin{{
		ID:                  uuid.New(),
		SyncIntervalMinutes: 5,
		LastSyncAt:          &lastSyncAt,
	}}, now)
	if len(due) != 0 {
		t.Fatalf("expected stale next_sync_at candidate to be filtered, got %d item(s)", len(due))
	}
}
