package execution

import (
	"context"
	"testing"
	"time"
)

func TestAsyncLifecycleStopWaitsForTrackedWorker(t *testing.T) {
	lifecycle := newAsyncLifecycle(1)
	started := make(chan struct{})
	stopped := make(chan struct{})

	lifecycle.Go(func(ctx context.Context) {
		if !lifecycle.Acquire(ctx) {
			return
		}
		defer lifecycle.Release()
		close(started)
		<-ctx.Done()
		close(stopped)
	})

	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("worker did not start")
	}

	lifecycle.Stop()

	select {
	case <-stopped:
	case <-time.After(time.Second):
		t.Fatal("worker did not stop before Stop returned")
	}
}
