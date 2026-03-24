package provider

import (
	"context"
	"os"
	"sync"
	"time"

	"github.com/company/auto-healing/internal/database"
	"github.com/company/auto-healing/internal/notification"
	"github.com/company/auto-healing/internal/pkg/logger"
)

// NotificationRetryScheduler 通知失败重试调度器
type NotificationRetryScheduler struct {
	notifSvc *notification.Service
	interval time.Duration
	stopCh   chan struct{}
	wg       sync.WaitGroup
	running  bool
	mu       sync.Mutex
}

// NewNotificationRetryScheduler 创建通知重试调度器
func NewNotificationRetryScheduler() *NotificationRetryScheduler {
	interval := 30 * time.Second
	if value := os.Getenv("NOTIFICATION_RETRY_INTERVAL"); value != "" {
		if parsed, err := time.ParseDuration(value); err == nil && parsed > 0 {
			interval = parsed
		}
	}

	return &NotificationRetryScheduler{
		notifSvc: notification.NewService(database.DB, "Auto-Healing", "", "1.0.0"),
		interval: interval,
		stopCh:   make(chan struct{}),
	}
}

// Start 启动调度器
func (s *NotificationRetryScheduler) Start() {
	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		return
	}
	s.running = true
	s.mu.Unlock()

	s.wg.Add(1)
	go s.run()
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
	s.mu.Unlock()

	close(s.stopCh)
	s.wg.Wait()
	logger.Sched("NOTIFY").Info("通知重试调度器已停止")
}

func (s *NotificationRetryScheduler) run() {
	defer s.wg.Done()

	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()

	s.retryOnce()

	for {
		select {
		case <-s.stopCh:
			return
		case <-ticker.C:
			s.retryOnce()
		}
	}
}

func (s *NotificationRetryScheduler) retryOnce() {
	ctx := context.Background()
	if err := s.notifSvc.RetryFailed(ctx); err != nil {
		logger.Sched("NOTIFY").Error("通知失败重试执行失败: %v", err)
	}
}
