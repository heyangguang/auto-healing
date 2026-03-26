package provider

import (
	"context"
	"testing"
	"time"
)

func TestSchedulerLifecycleStopWaitsForWorker(t *testing.T) {
	lifecycle := newSchedulerLifecycle()
	started := make(chan struct{})
	stopped := make(chan struct{})

	lifecycle.Go(func(ctx context.Context) {
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
		t.Fatal("worker did not stop before lifecycle.Stop returned")
	}
}
