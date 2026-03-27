package scheduler

import (
	"context"
	"testing"
	"time"

	notification "github.com/company/auto-healing/internal/modules/engagement/service/notification"
)

func TestNotificationRetrySchedulerStopWaitsForRetryRun(t *testing.T) {
	scheduler := NewNotificationRetrySchedulerWithDeps(NotificationRetrySchedulerDeps{
		NotificationService: &notification.Service{},
		Interval:            time.Hour,
	})

	started := make(chan struct{})
	stopped := make(chan struct{})
	scheduler.retryFunc = func(ctx context.Context) error {
		close(started)
		<-ctx.Done()
		close(stopped)
		return nil
	}

	scheduler.Start()

	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("retry run did not start")
	}

	scheduler.Stop()

	select {
	case <-stopped:
	case <-time.After(time.Second):
		t.Fatal("retry run did not stop before Stop returned")
	}
}
