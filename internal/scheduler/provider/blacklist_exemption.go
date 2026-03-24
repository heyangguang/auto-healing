package provider

import (
	"context"
	"os"
	"sync"
	"time"

	"github.com/company/auto-healing/internal/pkg/logger"
	"github.com/company/auto-healing/internal/service"
)

// BlacklistExemptionScheduler 过期豁免清理调度器
type BlacklistExemptionScheduler struct {
	svc      *service.BlacklistExemptionService
	interval time.Duration
	stopCh   chan struct{}
	wg       sync.WaitGroup
	running  bool
	mu       sync.Mutex
}

// NewBlacklistExemptionScheduler 创建过期豁免清理调度器
func NewBlacklistExemptionScheduler() *BlacklistExemptionScheduler {
	interval := time.Minute
	if value := os.Getenv("BLACKLIST_EXEMPTION_INTERVAL"); value != "" {
		if parsed, err := time.ParseDuration(value); err == nil && parsed > 0 {
			interval = parsed
		}
	}

	return &BlacklistExemptionScheduler{
		svc:      service.NewBlacklistExemptionService(),
		interval: interval,
		stopCh:   make(chan struct{}),
	}
}

// Start 启动调度器
func (s *BlacklistExemptionScheduler) Start() {
	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		return
	}
	s.running = true
	s.mu.Unlock()

	s.wg.Add(1)
	go s.run()
	logger.Sched("BLACKLIST").Info("黑名单豁免过期调度器已启动 (检查间隔: %v)", s.interval)
}

// Stop 停止调度器
func (s *BlacklistExemptionScheduler) Stop() {
	s.mu.Lock()
	if !s.running {
		s.mu.Unlock()
		return
	}
	s.running = false
	s.mu.Unlock()

	close(s.stopCh)
	s.wg.Wait()
	logger.Sched("BLACKLIST").Info("黑名单豁免过期调度器已停止")
}

func (s *BlacklistExemptionScheduler) run() {
	defer s.wg.Done()

	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()

	s.expireOnce()

	for {
		select {
		case <-s.stopCh:
			return
		case <-ticker.C:
			s.expireOnce()
		}
	}
}

func (s *BlacklistExemptionScheduler) expireOnce() {
	ctx := context.Background()
	affected, err := s.svc.ExpireOverdue(ctx)
	if err != nil {
		logger.Sched("BLACKLIST").Error("黑名单豁免过期清理失败: %v", err)
		return
	}
	if affected > 0 {
		logger.Sched("BLACKLIST").Info("已过期黑名单豁免: %d", affected)
	}
}
