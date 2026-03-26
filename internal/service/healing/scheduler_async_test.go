package healing

import (
	"context"
	"testing"
	"time"

	"github.com/company/auto-healing/internal/model"
	"github.com/google/uuid"
)

func TestSchedulerStopWaitsForTrackedFlowWorker(t *testing.T) {
	scheduler := NewScheduler()
	scheduler.interval = time.Hour

	started := make(chan struct{})
	stopped := make(chan struct{})

	scheduler.recoverOrphans = func(context.Context) {}
	scheduler.scanNow = func(ctx context.Context) {
		scheduler.scheduleAutoFlowExecution(&model.FlowInstance{ID: uuid.New()}, uuid.New())
	}
	scheduler.runFlow = func(ctx context.Context, instance *model.FlowInstance) error {
		close(started)
		<-ctx.Done()
		close(stopped)
		return nil
	}

	scheduler.Start()

	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("flow worker did not start")
	}

	scheduler.Stop()

	select {
	case <-stopped:
	case <-time.After(time.Second):
		t.Fatal("flow worker did not stop before Stop returned")
	}
}
