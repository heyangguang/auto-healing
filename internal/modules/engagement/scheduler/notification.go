package scheduler

import (
	"context"
	"os"
	"sync"
	"time"

	notification "github.com/company/auto-healing/internal/modules/engagement/service/notification"
	"github.com/company/auto-healing/internal/pkg/logger"
	schedulerx "github.com/company/auto-healing/internal/platform/schedulerx"
)

// NotificationRetryScheduler 通知失败重试调度器
type NotificationRetryScheduler struct {
	notifSvc  *notification.Service
	interval  time.Duration
	lifecycle *schedulerx.Lifecycle
	running   bool
	mu        sync.Mutex
	retryFunc func(context.Context) error
}

type NotificationRetrySchedulerDeps struct {
	NotificationService *notification.Service
	Interval            time.Duration
}

func NewNotificationRetrySchedulerWithDeps(deps NotificationRetrySchedulerDeps) *NotificationRetryScheduler {
	if deps.NotificationService == nil {
		panic("notification retry scheduler requires notification service")
	}
	if deps.Interval == 0 {
		deps.Interval = notificationRetryInterval()
	}
	return &NotificationRetryScheduler{
		notifSvc:  deps.NotificationService,
		interval:  deps.Interval,
		lifecycle: schedulerx.NewLifecycle(),
		retryFunc: deps.NotificationService.RetryFailed,
	}
}

func notificationRetryInterval() time.Duration {
	interval := 30 * time.Second
	if value := os.Getenv("NOTIFICATION_RETRY_INTERVAL"); value != "" {
		if parsed, err := time.ParseDuration(value); err == nil && parsed > 0 {
			interval = parsed
		}
	}
	return interval
}

// Start 启动调度器
func (s *NotificationRetryScheduler) Start() {
	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		return
	}
	if s.lifecycle == nil || s.lifecycle.Context().Err() != nil {
		s.lifecycle = schedulerx.NewLifecycle()
	}
	lifecycle := s.lifecycle
	s.running = true
	s.mu.Unlock()

	lifecycle.Go(s.run)
	logger.Sched("NOTIFY").Info("通知重试调度器已启动 (检查间隔: %v)", s.interval)
}

// Stop 停止调度器
func (s *NotificationRetryScheduler) Stop() {
	s.mu.Lock()
	if !s.running {
		s.mu.Unlock()
		return
	}
	s.running = false
	lifecycle := s.lifecycle
	s.mu.Unlock()

	lifecycle.Stop()
	logger.Sched("NOTIFY").Info("通知重试调度器已停止")
}

func (s *NotificationRetryScheduler) run(ctx context.Context) {
	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()

	s.retryOnce(ctx)
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.retryOnce(ctx)
		}
	}
}

func (s *NotificationRetryScheduler) retryOnce(ctx context.Context) {
	if err := s.retryFunc(ctx); err != nil {
		logger.Sched("NOTIFY").Error("通知失败重试执行失败: %v", err)
	}
}
