package scheduler

import (
	"context"
	"testing"
	"time"
)

func TestBlacklistExemptionSchedulerStopWaitsForExpireRun(t *testing.T) {
	scheduler := NewBlacklistExemptionScheduler()
	scheduler.interval = time.Hour

	started := make(chan struct{})
	stopped := make(chan struct{})
	scheduler.expireFunc = func(ctx context.Context) (int64, error) {
		close(started)
		<-ctx.Done()
		close(stopped)
		return 0, nil
	}

	scheduler.Start()

	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("expire run did not start")
	}

	scheduler.Stop()

	select {
	case <-stopped:
	case <-time.After(time.Second):
		t.Fatal("expire run did not stop before Stop returned")
	}
}
