package plugin

import (
	"context"
	"testing"
	"time"
)

func TestServiceShutdownWaitsForBackgroundWorker(t *testing.T) {
	svc := &Service{lifecycle: newAsyncLifecycle()}
	started := make(chan struct{})
	stopped := make(chan struct{})

	svc.Go(func(ctx context.Context) {
		close(started)
		<-ctx.Done()
		close(stopped)
	})

	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("background worker did not start")
	}

	svc.Shutdown()

	select {
	case <-stopped:
	case <-time.After(time.Second):
		t.Fatal("background worker did not stop before Shutdown returned")
	}
}
