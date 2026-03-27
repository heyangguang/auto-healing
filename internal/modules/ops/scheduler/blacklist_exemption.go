package scheduler

import (
	"context"
	"os"
	"sync"
	"time"

	opsservice "github.com/company/auto-healing/internal/modules/ops/service"
	"github.com/company/auto-healing/internal/pkg/logger"
	schedulerx "github.com/company/auto-healing/internal/platform/schedulerx"
)

// BlacklistExemptionScheduler 过期豁免清理调度器
type BlacklistExemptionScheduler struct {
	svc        *opsservice.BlacklistExemptionService
	interval   time.Duration
	lifecycle  *schedulerx.Lifecycle
	running    bool
	mu         sync.Mutex
	expireFunc func(context.Context) (int64, error)
}

type BlacklistExemptionSchedulerDeps struct {
	Service    *opsservice.BlacklistExemptionService
	Interval   time.Duration
	Lifecycle  *schedulerx.Lifecycle
	ExpireFunc func(context.Context) (int64, error)
}

// NewBlacklistExemptionScheduler 创建过期豁免清理调度器
func NewBlacklistExemptionScheduler() *BlacklistExemptionScheduler {
	interval := time.Minute
	if value := os.Getenv("BLACKLIST_EXEMPTION_INTERVAL"); value != "" {
		if parsed, err := time.ParseDuration(value); err == nil && parsed > 0 {
			interval = parsed
		}
	}
	service := opsservice.NewBlacklistExemptionService()
	return NewBlacklistExemptionSchedulerWithDeps(BlacklistExemptionSchedulerDeps{
		Service:    service,
		Interval:   interval,
		Lifecycle:  schedulerx.NewLifecycle(),
		ExpireFunc: service.ExpireOverdue,
	})
}

func NewBlacklistExemptionSchedulerWithDeps(deps BlacklistExemptionSchedulerDeps) *BlacklistExemptionScheduler {
	if deps.Service == nil {
		panic("blacklist exemption scheduler requires service")
	}
	if deps.Interval == 0 {
		deps.Interval = time.Minute
		if value := os.Getenv("BLACKLIST_EXEMPTION_INTERVAL"); value != "" {
			if parsed, err := time.ParseDuration(value); err == nil && parsed > 0 {
				deps.Interval = parsed
			}
		}
	}
	if deps.Lifecycle == nil {
		deps.Lifecycle = schedulerx.NewLifecycle()
	}
	if deps.ExpireFunc == nil {
		deps.ExpireFunc = deps.Service.ExpireOverdue
	}
	return &BlacklistExemptionScheduler{
		svc:        deps.Service,
		interval:   deps.Interval,
		lifecycle:  deps.Lifecycle,
		expireFunc: deps.ExpireFunc,
	}
}

// Start 启动调度器
func (s *BlacklistExemptionScheduler) Start() {
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
	lifecycle := s.lifecycle
	s.mu.Unlock()

	lifecycle.Stop()
	logger.Sched("BLACKLIST").Info("黑名单豁免过期调度器已停止")
}

func (s *BlacklistExemptionScheduler) run(ctx context.Context) {
	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()

	s.expireOnce(ctx)
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.expireOnce(ctx)
		}
	}
}

func (s *BlacklistExemptionScheduler) expireOnce(ctx context.Context) {
	affected, err := s.expireFunc(ctx)
	if err != nil {
		logger.Sched("BLACKLIST").Error("黑名单豁免过期清理失败: %v", err)
		return
	}
	if affected > 0 {
		logger.Sched("BLACKLIST").Info("已过期黑名单豁免: %d", affected)
	}
}
