package scheduler

import (
	"context"
	"testing"
	"time"

	opsservice "github.com/company/auto-healing/internal/modules/ops/service"
)

func TestBlacklistExemptionSchedulerStopWaitsForExpireRun(t *testing.T) {
	scheduler := NewBlacklistExemptionSchedulerWithDeps(BlacklistExemptionSchedulerDeps{
		Service:  &opsservice.BlacklistExemptionService{},
		Interval: time.Hour,
	})

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
