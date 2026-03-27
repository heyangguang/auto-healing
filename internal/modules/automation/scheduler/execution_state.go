package scheduler

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/company/auto-healing/internal/modules/automation/model"
	"github.com/company/auto-healing/internal/pkg/logger"
	platformsched "github.com/company/auto-healing/internal/platform/schedulerx"
	"github.com/google/uuid"
)

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

func (s *ExecutionScheduler) lifecycleSnapshot() *platformsched.Lifecycle {
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
