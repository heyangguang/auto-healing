package healing

import (
	"context"
	"time"

	"github.com/company/auto-healing/internal/pkg/logger"
)

// Start 启动调度器
func (s *Scheduler) Start() {
	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		return
	}
	if s.lifecycle == nil || s.lifecycle.ctx.Err() != nil {
		s.lifecycle = newAsyncLifecycle()
	}
	lifecycle := s.lifecycle
	s.running = true
	s.mu.Unlock()

	logger.Sched("HEAL").Info("调度器启动，间隔: %v", s.interval)
	s.recoverOrphans(lifecycle.ctx)
	lifecycle.Go(s.run)
}

// Stop 停止调度器
func (s *Scheduler) Stop() {
	s.mu.Lock()
	if !s.running {
		s.mu.Unlock()
		return
	}
	s.running = false
	lifecycle := s.lifecycle
	s.mu.Unlock()

	lifecycle.Stop()
	s.executor.Shutdown()
	logger.Sched("HEAL").Info("调度器已停止")
}

// IsRunning 检查是否运行中
func (s *Scheduler) IsRunning() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.running
}

// run 调度循环
func (s *Scheduler) run(ctx context.Context) {
	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()

	s.scanNow(ctx)
	for {
		select {
		case <-ticker.C:
			s.scanNow(ctx)
		case <-ctx.Done():
			return
		}
	}
}
