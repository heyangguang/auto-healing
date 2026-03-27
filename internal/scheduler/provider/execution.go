package provider

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/company/auto-healing/internal/database"
	"github.com/company/auto-healing/internal/model"
	automationrepo "github.com/company/auto-healing/internal/modules/automation/repository"
	executionService "github.com/company/auto-healing/internal/modules/automation/service/execution"
	scheduleService "github.com/company/auto-healing/internal/modules/automation/service/schedule"
	"github.com/company/auto-healing/internal/pkg/logger"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

const executionClaimLease = 40 * time.Minute

// ExecutionScheduler 执行任务调度器
// 使用独立的 ExecutionSchedule 表管理定时任务
type ExecutionScheduler struct {
	execSvc               *executionService.Service
	scheduleSvc           *scheduleService.Service
	scheduleRepo          *automationrepo.ScheduleRepository
	db                    *gorm.DB
	interval              time.Duration
	lifecycle             *schedulerLifecycle
	inFlight              *inFlightSet
	running               bool
	mu                    sync.Mutex
	sem                   chan struct{}
	loadDueSchedules      func(context.Context) ([]model.ExecutionSchedule, error)
	runScheduledExecution func(context.Context, model.ExecutionSchedule)
	executeTask           func(context.Context, uuid.UUID, *executionService.ExecuteOptions) (*model.ExecutionRun, error)
	getRun                func(context.Context, uuid.UUID) (*model.ExecutionRun, error)
	updateScheduleState   func(context.Context, uuid.UUID, map[string]interface{}) error
	updateScheduleLastRun func(context.Context, uuid.UUID) error
	updateScheduleNextRun func(context.Context, uuid.UUID, string) error
	markScheduleCompleted func(context.Context, uuid.UUID) error
	updateLastRunAt       func(context.Context, uuid.UUID) error
	updateNextRunAt       func(context.Context, uuid.UUID, string) error
	markCompleted         func(context.Context, uuid.UUID) error
	claimDueSchedule      func(context.Context, model.ExecutionSchedule) (bool, error)
	followRunAfterStop    func(context.Context, model.ExecutionSchedule, uuid.UUID)
}

var errExecutionSchedulerStopped = errors.New("execution scheduler stopped")

// NewExecutionScheduler 创建执行任务调度器
func NewExecutionScheduler() *ExecutionScheduler {
	s := &ExecutionScheduler{
		execSvc:      executionService.NewService(),
		scheduleSvc:  scheduleService.NewService(),
		scheduleRepo: automationrepo.NewScheduleRepository(),
		db:           database.DB,
		interval:     30 * time.Second,
		inFlight:     newInFlightSet(),
		sem:          make(chan struct{}, 8),
	}
	s.loadDueSchedules = s.scheduleRepo.GetDueSchedules
	s.runScheduledExecution = s.executeSchedule
	s.executeTask = s.execSvc.ExecuteTask
	s.getRun = s.execSvc.GetRun
	s.updateScheduleState = s.applyScheduleStateUpdate
	s.updateLastRunAt = s.scheduleRepo.UpdateLastRunAt
	s.updateNextRunAt = s.scheduleSvc.UpdateNextRunAt
	s.markCompleted = s.scheduleSvc.MarkCompleted
	s.claimDueSchedule = s.claimSchedule
	s.followRunAfterStop = s.detachScheduleCompletion
	return s
}

// Start 启动调度器
func (s *ExecutionScheduler) Start() {
	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		return
	}
	if s.lifecycle == nil || s.lifecycle.ctx.Err() != nil {
		s.lifecycle = newSchedulerLifecycle()
	}
	lifecycle := s.lifecycle
	s.running = true
	s.mu.Unlock()

	lifecycle.Go(s.run)
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

	lifecycle := s.lifecycleSnapshot()
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
			if !s.dispatchScheduledExecution(lifecycle, schedule) {
				s.rollbackExecutionClaim(ctx, schedule)
			}
		default:
			s.rollbackExecutionClaim(ctx, schedule)
			logger.Sched("TASK").Warn("执行调度器并发已满，延后调度: %s (%s)", schedule.Name, schedule.ID.String()[:8])
		}
	}
}

func (s *ExecutionScheduler) dispatchScheduledExecution(lifecycle *schedulerLifecycle, schedule model.ExecutionSchedule) bool {
	if lifecycle == nil {
		s.releaseSemaphoreToken()
		return false
	}
	if !s.inFlight.Start(schedule.ID) {
		s.releaseSemaphoreToken()
		return false
	}

	sched := schedule
	started := lifecycle.Go(func(rootCtx context.Context) {
		defer s.inFlight.Finish(sched.ID)
		defer s.releaseSemaphoreToken()
		s.runScheduledExecution(rootCtx, sched)
	})
	if !started {
		s.inFlight.Finish(sched.ID)
		s.releaseSemaphoreToken()
	}
	return started
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
	return s.updateNextRun(ctx, schedule.ID, *schedule.ScheduleExpr)
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
		if ctx.Err() != nil {
			return "", errExecutionSchedulerStopped
		}
		run, err := s.getRun(ctx, runID)
		if err != nil {
			if ctx.Err() != nil || errors.Is(err, context.Canceled) {
				return "", errExecutionSchedulerStopped
			}
			return "", err
		}
		switch run.Status {
		case "success", "failed", "partial", "cancelled", "timeout":
			return run.Status, nil
		}

		select {
		case <-ticker.C:
		case <-timeout.C:
			return "", fmt.Errorf("等待执行结果超时")
		case <-ctx.Done():
			return "", errExecutionSchedulerStopped
		}
	}
}

func (s *ExecutionScheduler) lifecycleSnapshot() *schedulerLifecycle {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.lifecycle
}

func (s *ExecutionScheduler) applyScheduleStateUpdate(ctx context.Context, id uuid.UUID, updates map[string]interface{}) error {
	return s.db.WithContext(ctx).
		Model(&model.ExecutionSchedule{}).
		Where("id = ?", id).
		Updates(updates).Error
}

func (s *ExecutionScheduler) detachScheduleCompletion(ctx context.Context, sched model.ExecutionSchedule, runID uuid.UUID) {
	go func() {
		defer s.inFlight.Finish(sched.ID)
		shortID := sched.ID.String()[:8]
		tenantCtx := scheduleTenantContext(ctx, sched)

		finalStatus, err := s.waitForRunTerminalStatus(ctx, runID)
		if err != nil {
			logger.Sched("TASK").Error("[%s] 调度器停止后继续跟踪执行结果失败: %v", shortID, err)
			return
		}
		if runStatusCountsAsSuccess(finalStatus) {
			if err := s.handleScheduledExecutionSuccess(ctx, tenantCtx, sched, shortID); err != nil {
				logger.Sched("TASK").Error("[%s] 后台跟踪成功但状态更新失败: %v", shortID, err)
			}
			return
		}
		if err := s.handleScheduledExecutionError(ctx, tenantCtx, sched, shortID, fmt.Errorf("执行结果状态为 %s", finalStatus), true); err != nil {
			logger.Sched("TASK").Error("[%s] 后台跟踪失败且状态更新失败: %v", shortID, err)
		}
	}()
}

func (s *ExecutionScheduler) updateLastRun(ctx context.Context, id uuid.UUID) error {
	if s.updateScheduleLastRun != nil {
		return s.updateScheduleLastRun(ctx, id)
	}
	if s.updateLastRunAt != nil {
		return s.updateLastRunAt(ctx, id)
	}
	return nil
}

func (s *ExecutionScheduler) updateNextRun(ctx context.Context, id uuid.UUID, expr string) error {
	if s.updateScheduleNextRun != nil {
		return s.updateScheduleNextRun(ctx, id, expr)
	}
	if s.updateNextRunAt != nil {
		return s.updateNextRunAt(ctx, id, expr)
	}
	return nil
}

func (s *ExecutionScheduler) markCompletedSchedule(ctx context.Context, id uuid.UUID) error {
	if s.markScheduleCompleted != nil {
		return s.markScheduleCompleted(ctx, id)
	}
	if s.markCompleted != nil {
		return s.markCompleted(ctx, id)
	}
	return nil
}

func (s *ExecutionScheduler) releaseSemaphoreToken() {
	select {
	case <-s.sem:
	default:
	}
}
