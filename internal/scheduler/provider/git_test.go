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
