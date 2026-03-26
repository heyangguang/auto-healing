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
		nextSyncAt := time.Now()
		return []model.GitRepository{{ID: uuid.New(), NextSyncAt: &nextSyncAt}}, nil
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

func TestGitSchedulerCheckAndSyncSkipsUnclaimedRepo(t *testing.T) {
	scheduler := NewGitScheduler()
	executed := 0
	next := time.Now()
	scheduler.loadReposNeedSync = func(context.Context) ([]model.GitRepository, error) {
		return []model.GitRepository{{ID: uuid.New(), Name: "repo", NextSyncAt: &next}}, nil
	}
	scheduler.claimRepoSync = func(context.Context, model.GitRepository) (bool, error) {
		return false, nil
	}
	scheduler.runRepoSync = func(context.Context, model.GitRepository) {
		executed++
	}

	scheduler.checkAndSync(context.Background())
	if executed != 0 {
		t.Fatalf("executed = %d, want 0 for unclaimed repo", executed)
	}
}

func TestGitSchedulerDispatchReturnsFalseWhenLifecycleUnavailable(t *testing.T) {
	scheduler := NewGitScheduler()
	if ok := scheduler.dispatchRepoSync(model.GitRepository{ID: uuid.New()}); ok {
		t.Fatal("dispatchRepoSync() = true, want false when scheduler not running")
	}
}
