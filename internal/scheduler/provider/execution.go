package provider

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/company/auto-healing/internal/database"
	"github.com/company/auto-healing/internal/model"
	"github.com/company/auto-healing/internal/pkg/logger"
	"github.com/company/auto-healing/internal/repository"
	executionService "github.com/company/auto-healing/internal/service/execution"
	scheduleService "github.com/company/auto-healing/internal/service/schedule"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

const executionClaimLease = 40 * time.Minute

// ExecutionScheduler 执行任务调度器
// 使用独立的 ExecutionSchedule 表管理定时任务
type ExecutionScheduler struct {
	execSvc               *executionService.Service
	scheduleSvc           *scheduleService.Service
	scheduleRepo          *repository.ScheduleRepository
	db                    *gorm.DB
	interval              time.Duration // 检查间隔
	lifecycle             *schedulerLifecycle
	running               bool
	mu                    sync.Mutex
	sem                   chan struct{} // 并发执行限制
	loadDueSchedules      func(context.Context) ([]model.ExecutionSchedule, error)
	runScheduledExecution func(context.Context, model.ExecutionSchedule)
	updateScheduleState   func(context.Context, uuid.UUID, map[string]interface{}) error
	updateScheduleLastRun func(context.Context, uuid.UUID) error
	updateScheduleNextRun func(context.Context, uuid.UUID, string) error
	markScheduleCompleted func(context.Context, uuid.UUID) error
	claimDueSchedule      func(context.Context, model.ExecutionSchedule) (bool, error)
}

// NewExecutionScheduler 创建执行任务调度器
func NewExecutionScheduler() *ExecutionScheduler {
	s := &ExecutionScheduler{
		execSvc:      executionService.NewService(),
		scheduleSvc:  scheduleService.NewService(),
		scheduleRepo: repository.NewScheduleRepository(),
		db:           database.DB,
		interval:     30 * time.Second, // 每30秒检查一次
		sem:          make(chan struct{}, 8),
	}
	s.loadDueSchedules = s.scheduleRepo.GetDueSchedules
	s.runScheduledExecution = s.executeSchedule
	s.updateScheduleState = s.persistScheduleState
	s.updateScheduleLastRun = s.scheduleRepo.UpdateLastRunAt
	s.updateScheduleNextRun = s.scheduleSvc.UpdateNextRunAt
	s.markScheduleCompleted = s.scheduleSvc.MarkCompleted
	s.claimDueSchedule = s.claimSchedule
	return s
}

func (s *ExecutionScheduler) persistScheduleState(ctx context.Context, id uuid.UUID, updates map[string]interface{}) error {
	return s.db.WithContext(ctx).Model(&model.ExecutionSchedule{}).Where("id = ?", id).Updates(updates).Error
}

// Start 启动调度器
func (s *ExecutionScheduler) Start() {
	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		return
	}
	s.running = true
	s.lifecycle = newSchedulerLifecycle()
	s.mu.Unlock()

	s.lifecycleGo(s.run)
	logger.Sched("TASK").Info("执行任务调度器已启动 (检查间隔: %v, 最大并发: %d)", s.interval, cap(s.sem))
}

// Stop 停止调度器
func (s *ExecutionScheduler) Stop() {
	s.mu.Lock()
	if !s.running {
		s.mu.Unlock()
		return
	}
	s.running = false
	lifecycle := s.lifecycle
	s.lifecycle = nil
	s.mu.Unlock()

	if lifecycle != nil {
		lifecycle.Stop()
	}
	logger.Sched("TASK").Info("执行任务调度器已停止")
}

// run 调度器主循环
func (s *ExecutionScheduler) run(ctx context.Context) {
	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()

	// 启动时立即检查一次
	s.checkAndExecute(ctx)

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.checkAndExecute(ctx)
		}
	}
}

// checkAndExecute 检查并执行定时任务
func (s *ExecutionScheduler) checkAndExecute(ctx context.Context) {
	schedules, err := s.loadDueSchedules(ctx)
	if err != nil {
		logger.Sched("TASK").Error("查询待执行调度失败: %v", err)
		return
	}

	if len(schedules) == 0 {
		return
	}

	logger.Sched("TASK").Info("发现 %d 个定时任务需要执行", len(schedules))

	for _, schedule := range schedules {
		claimed, err := s.claimDueSchedule(ctx, schedule)
		if err != nil {
			logger.Sched("TASK").Error("认领定时任务失败: %s (%s) - %v", schedule.Name, schedule.ID.String()[:8], err)
			continue
		}
		if !claimed {
			continue
		}

		select {
		case <-ctx.Done():
			s.rollbackExecutionClaim(ctx, schedule)
			return
		case s.sem <- struct{}{}:
			if !s.dispatchScheduledExecution(schedule) {
				<-s.sem
				s.rollbackExecutionClaim(ctx, schedule)
			}
		default:
			s.rollbackExecutionClaim(ctx, schedule)
			logger.Sched("TASK").Warn("执行调度器并发已满，延后调度: %s (%s)", schedule.Name, schedule.ID.String()[:8])
		}
	}
}

func (s *ExecutionScheduler) dispatchScheduledExecution(schedule model.ExecutionSchedule) bool {
	sched := schedule
	return s.lifecycleGo(func(rootCtx context.Context) {
		defer func() { <-s.sem }()
		s.runScheduledExecution(rootCtx, sched)
	})
}

func (s *ExecutionScheduler) lifecycleGo(fn func(context.Context)) bool {
	s.mu.Lock()
	lifecycle := s.lifecycle
	running := s.running
	s.mu.Unlock()
	if !running || lifecycle == nil {
		return false
	}
	return lifecycle.Go(fn)
}

func (s *ExecutionScheduler) claimSchedule(ctx context.Context, schedule model.ExecutionSchedule) (bool, error) {
	if schedule.NextRunAt == nil {
		return false, nil
	}
	now := time.Now()
	claimUntil := now.Add(executionClaimLease)
	result := s.db.WithContext(ctx).
		Model(&model.ExecutionSchedule{}).
		Where("id = ? AND enabled = ? AND next_run_at IS NOT NULL AND next_run_at <= ?", schedule.ID, true, now).
		Update("next_run_at", claimUntil)
	if result.Error != nil {
		return false, result.Error
	}
	return result.RowsAffected == 1, nil
}

func (s *ExecutionScheduler) restoreCronNextRun(ctx context.Context, schedule model.ExecutionSchedule) error {
	if schedule.ScheduleExpr == nil || *schedule.ScheduleExpr == "" {
		return nil
	}
	return s.updateScheduleNextRun(ctx, schedule.ID, *schedule.ScheduleExpr)
}

func (s *ExecutionScheduler) rollbackExecutionClaim(ctx context.Context, schedule model.ExecutionSchedule) {
	if schedule.NextRunAt == nil {
		return
	}
	if err := s.updateScheduleState(ctx, schedule.ID, map[string]interface{}{
		"next_run_at": schedule.NextRunAt,
	}); err != nil {
		logger.Sched("TASK").Warn("回滚定时任务认领失败: %s (%s) - %v", schedule.Name, schedule.ID.String()[:8], err)
	}
}

func runStatusCountsAsSuccess(status string) bool {
	switch status {
	case "success", "partial":
		return true
	default:
		return false
	}
}

func (s *ExecutionScheduler) waitForRunTerminalStatus(ctx context.Context, runID uuid.UUID) (string, error) {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	timeout := time.NewTimer(35 * time.Minute)
	defer timeout.Stop()

	for {
		run, err := s.execSvc.GetRun(ctx, runID)
		if err != nil {
			return "", err
		}
		switch run.Status {
		case "success", "failed", "partial", "cancelled":
			return run.Status, nil
		}

		select {
		case <-ticker.C:
		case <-timeout.C:
			return "", fmt.Errorf("等待执行结果超时")
		case <-ctx.Done():
			return "", fmt.Errorf("调度器已停止")
		}
	}
}
