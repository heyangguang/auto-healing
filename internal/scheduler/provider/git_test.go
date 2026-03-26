package provider

import (
	"context"
	"testing"
	"time"

	"github.com/company/auto-healing/internal/model"
	"github.com/google/uuid"
)

func TestGitSchedulerStopWaitsForSyncWorker(t *testing.T) {
	scheduler := NewGitScheduler()
	scheduler.interval = time.Hour

	workerStarted := make(chan struct{})
	workerStopped := make(chan struct{})
	scheduler.loadReposNeedSync = func(context.Context) ([]model.GitRepository, error) {
		return []model.GitRepository{{ID: uuid.New()}}, nil
	}
	scheduler.runRepoSync = func(ctx context.Context, repo model.GitRepository) {
		close(workerStarted)
		<-ctx.Done()
		close(workerStopped)
	}
	scheduler.claimRepoSync = func(context.Context, model.GitRepository) (bool, error) {
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

func TestGitSchedulerDispatchAfterStopDoesNotRunWorker(t *testing.T) {
	scheduler := NewGitScheduler()
	scheduler.lifecycle = newSchedulerLifecycle()
	scheduler.running = true

	scheduler.Stop()

	workerStarted := make(chan struct{}, 1)
	scheduler.runRepoSync = func(context.Context, model.GitRepository) {
		workerStarted <- struct{}{}
	}

	scheduler.dispatchRepoSync(scheduler.lifecycleSnapshot(), model.GitRepository{ID: uuid.New()})

	select {
	case <-workerStarted:
		t.Fatal("sync worker should not start after Stop")
	case <-time.After(50 * time.Millisecond):
	}
}

func TestGitSchedulerDoesNotDispatchSameRepoTwiceWhileInFlight(t *testing.T) {
	scheduler := NewGitScheduler()
	scheduler.lifecycle = newSchedulerLifecycle()
	defer scheduler.lifecycle.Stop()

	repoID := uuid.New()
	release := make(chan struct{})
	started := make(chan struct{}, 2)
	scheduler.runRepoSync = func(context.Context, model.GitRepository) {
		started <- struct{}{}
		<-release
	}

	scheduler.dispatchRepoSync(scheduler.lifecycleSnapshot(), model.GitRepository{ID: repoID})
	scheduler.dispatchRepoSync(scheduler.lifecycleSnapshot(), model.GitRepository{ID: repoID})

	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("first sync worker did not start")
	}

	select {
	case <-started:
		t.Fatal("same repo should not be dispatched twice while in flight")
	case <-time.After(100 * time.Millisecond):
	}

	close(release)
}

func TestGitSchedulerSyncUsesCompletionTimeForNextSyncAt(t *testing.T) {
	scheduler := NewGitScheduler()
	repoID := uuid.New()
	base := time.Date(2026, 3, 26, 10, 0, 0, 0, time.UTC)
	repo := model.GitRepository{
		ID:            repoID,
		Name:          "repo-a",
		SyncInterval:  "5m",
		DefaultBranch: "main",
	}

	current := base
	scheduler.now = func() time.Time { return current }
	scheduler.syncRepoWithTrigger = func(context.Context, uuid.UUID, string) error {
		current = base.Add(10 * time.Minute)
		return nil
	}
	scheduler.updateRepoState = func(_ context.Context, id interface{}, updates map[string]interface{}) error {
		if id != repoID {
			t.Fatalf("unexpected repo id: %v", id)
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

	scheduler.syncRepo(context.Background(), repo)
}

func TestFilterDueReposSkipsRecentlySyncedRepoWithStaleNextSyncAt(t *testing.T) {
	now := time.Date(2026, 3, 26, 10, 0, 0, 0, time.UTC)
	lastSyncAt := now.Add(-2 * time.Minute)
	due := filterDueRepos([]model.GitRepository{{
		ID:           uuid.New(),
		SyncInterval: "5m",
		LastSyncAt:   &lastSyncAt,
	}}, now)
	if len(due) != 0 {
		t.Fatalf("expected stale next_sync_at candidate to be filtered, got %d item(s)", len(due))
	}
}
