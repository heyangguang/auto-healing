package schedulerx

import (
	"context"
	"testing"
	"time"
)

func TestLifecycleStopWaitsForWorker(t *testing.T) {
	lifecycle := NewLifecycle()
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

func TestLifecycleGoAfterStopDoesNotStartWorker(t *testing.T) {
	lifecycle := NewLifecycle()
	lifecycle.Stop()

	started := make(chan struct{}, 1)
	if ok := lifecycle.Go(func(context.Context) {
		started <- struct{}{}
	}); ok {
		t.Fatal("expected lifecycle.Go to reject new worker after Stop")
	}

	select {
	case <-started:
		t.Fatal("worker should not start after lifecycle.Stop")
	case <-time.After(50 * time.Millisecond):
	}
}
