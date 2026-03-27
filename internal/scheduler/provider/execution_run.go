package provider

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/company/auto-healing/internal/model"
	executionService "github.com/company/auto-healing/internal/modules/automation/service/execution"
	"github.com/company/auto-healing/internal/pkg/logger"
	platformrepo "github.com/company/auto-healing/internal/platform/repositoryx"
	"github.com/google/uuid"
)

func (s *ExecutionScheduler) executeSchedule(ctx context.Context, sched model.ExecutionSchedule) {
	shortID := sched.ID.String()[:8]
	tenantCtx := scheduleTenantContext(ctx, sched)
	runStarted := false

	defer func() {
		if rec := recover(); rec != nil {
			panicErr := fmt.Errorf("定时任务 panic: %v", rec)
			logger.Sched("TASK").Error("[%s] 定时任务 panic: %v", shortID, rec)
			if err := s.handleScheduledExecutionError(ctx, tenantCtx, sched, shortID, panicErr, runStarted); err != nil {
				logger.Sched("TASK").Error("[%s] panic 后状态更新失败: %v | state_err=%v", shortID, panicErr, err)
			}
		}
	}()

	logger.Sched("TASK").Info("[%s] 开始执行定时任务: %s", shortID, sched.Name)

	opts, err := buildExecutionOptions(sched)
	if err != nil {
		if stateErr := s.handleScheduledExecutionError(ctx, tenantCtx, sched, shortID, err, runStarted); stateErr != nil {
			logger.Sched("TASK").Error("[%s] 构造执行参数失败且状态更新失败: %v | state_err=%v", shortID, err, stateErr)
		}
		return
	}

	run, err := s.executeTask(tenantCtx, sched.TaskID, opts)
	if err == nil {
		runStarted = true
	}
	if err == nil {
		result, triggerErr := s.afterScheduleTriggered(tenantCtx, sched, run.ID)
		if triggerErr != nil {
			err = triggerErr
		} else if result == scheduleTriggerDetached {
			logger.Sched("TASK").Info("[%s] 调度器停止，已转为后台继续跟踪: %s", shortID, sched.Name)
			return
		}
	}
	if err != nil {
		if stateErr := s.handleScheduledExecutionError(ctx, tenantCtx, sched, shortID, err, runStarted); stateErr != nil {
			logger.Sched("TASK").Error("[%s] 执行失败且状态更新失败: %v | state_err=%v", shortID, err, stateErr)
		}
		return
	}

	if err := s.handleScheduledExecutionSuccess(ctx, tenantCtx, sched, shortID); err != nil {
		logger.Sched("TASK").Error("[%s] 执行成功但状态更新失败: %v", shortID, err)
	}
}

func scheduleTenantContext(ctx context.Context, sched model.ExecutionSchedule) context.Context {
	if sched.TenantID == nil {
		return ctx
	}
	return platformrepo.WithTenantID(ctx, *sched.TenantID)
}

func buildExecutionOptions(sched model.ExecutionSchedule) (*executionService.ExecuteOptions, error) {
	opts := &executionService.ExecuteOptions{
		TriggeredBy:      scheduleTriggerLabel(sched),
		TargetHosts:      sched.TargetHostsOverride,
		ExtraVars:        sched.ExtraVarsOverride,
		SkipNotification: sched.SkipNotification,
	}
	for _, idStr := range sched.SecretsSourceIDs {
		id, err := uuid.Parse(idStr)
		if err != nil {
			return nil, fmt.Errorf("无效的密钥源 ID: %s", idStr)
		}
		opts.SecretsSourceIDs = append(opts.SecretsSourceIDs, id)
	}
	return opts, nil
}

func scheduleTriggerLabel(sched model.ExecutionSchedule) string {
	if sched.IsCron() {
		return "scheduler:cron"
	}
	return "scheduler:once"
}

type scheduleTriggerResult int

const (
	scheduleTriggerCompleted scheduleTriggerResult = iota
	scheduleTriggerDetached
)

func (s *ExecutionScheduler) afterScheduleTriggered(ctx context.Context, sched model.ExecutionSchedule, runID uuid.UUID) (scheduleTriggerResult, error) {
	persistErr := s.markScheduleTriggered(ctx, sched)

	finalStatus, err := s.waitForRunTerminalStatus(ctx, runID)
	if errors.Is(err, errExecutionSchedulerStopped) {
		if persistErr != nil {
			return scheduleTriggerCompleted, persistErr
		}
		s.inFlight.Retain(sched.ID)
		if s.followRunAfterStop != nil {
			s.followRunAfterStop(context.WithoutCancel(ctx), sched, runID)
		}
		return scheduleTriggerDetached, nil
	}
	if err != nil {
		return scheduleTriggerCompleted, mergeScheduleTriggerError(err, persistErr)
	}
	if persistErr != nil {
		return scheduleTriggerCompleted, persistErr
	}
	if runStatusCountsAsSuccess(finalStatus) {
		return scheduleTriggerCompleted, nil
	}
	return scheduleTriggerCompleted, fmt.Errorf("执行结果状态为 %s", finalStatus)
}

func (s *ExecutionScheduler) markScheduleTriggered(ctx context.Context, sched model.ExecutionSchedule) error {
	if err := s.updateLastRun(ctx, sched.ID); err != nil {
		return fmt.Errorf("更新上次执行时间失败: %w", err)
	}
	if sched.IsCron() {
		if sched.ScheduleExpr == nil || *sched.ScheduleExpr == "" {
			return fmt.Errorf("cron 调度缺少 schedule_expr")
		}
		if err := s.updateNextRun(ctx, sched.ID, *sched.ScheduleExpr); err != nil {
			return fmt.Errorf("更新下次执行时间失败: %w", err)
		}
		return nil
	}
	return s.updateScheduleState(ctx, sched.ID, map[string]interface{}{
		"status":      model.ScheduleStatusRunning,
		"last_run_at": time.Now(),
		"next_run_at": nil,
	})
}

func mergeScheduleTriggerError(waitErr, persistErr error) error {
	if persistErr == nil {
		return waitErr
	}
	return fmt.Errorf("%w；触发状态持久化也失败: %v", waitErr, persistErr)
}

func (s *ExecutionScheduler) handleScheduledExecutionError(ctx, tenantCtx context.Context, sched model.ExecutionSchedule, shortID string, err error, runStarted bool) error {
	newCount := sched.ConsecutiveFailures + 1
	updates := map[string]interface{}{
		"consecutive_failures": newCount,
	}

	pausedCron := false
	switch {
	case !sched.IsCron() && !runStarted:
		updates["enabled"] = false
		updates["status"] = model.ScheduleStatusDisabled
		updates["next_run_at"] = nil
		updates["pause_reason"] = fmt.Sprintf("单次调度未启动: %s", truncateStr(err.Error(), 200))
		logger.Sched("TASK").Error("[%s] 单次调度未启动，已禁用: %s - %v", shortID, sched.Name, err)
	case sched.MaxFailures > 0 && newCount >= sched.MaxFailures && sched.IsCron():
		pausedCron = true
		updates["enabled"] = false
		updates["status"] = model.ScheduleStatusAutoPaused
		updates["next_run_at"] = nil
		updates["pause_reason"] = fmt.Sprintf("连续失败 %d 次后自动暂停 (最后错误: %s)", newCount, truncateStr(err.Error(), 200))
		logger.Sched("TASK").Warn("[%s] ⚠ 连续失败 %d/%d 次，已自动暂停: %s", shortID, newCount, sched.MaxFailures, sched.Name)
	case sched.MaxFailures > 0:
		logger.Sched("TASK").Error("[%s] 执行失败 (%d/%d): %s - %v", shortID, newCount, sched.MaxFailures, sched.Name, err)
	default:
		logger.Sched("TASK").Error("[%s] 执行失败 (第%d次): %s - %v", shortID, newCount, sched.Name, err)
	}

	if updateErr := s.updateScheduleState(ctx, sched.ID, updates); updateErr != nil {
		return fmt.Errorf("更新调度失败状态失败: %w", updateErr)
	}
	if sched.IsCron() && !runStarted && !pausedCron {
		if restoreErr := s.restoreCronNextRun(ctx, sched); restoreErr != nil {
			return fmt.Errorf("恢复 cron 下一次执行时间失败: %w", restoreErr)
		}
	}
	if !sched.IsCron() && runStarted {
		if completeErr := s.markCompletedSchedule(tenantCtx, sched.ID); completeErr != nil {
			return fmt.Errorf("标记单次调度完成失败: %w", completeErr)
		}
	}
	return nil
}

func (s *ExecutionScheduler) handleScheduledExecutionSuccess(ctx, tenantCtx context.Context, sched model.ExecutionSchedule, shortID string) error {
	if err := s.updateScheduleState(ctx, sched.ID, map[string]interface{}{
		"consecutive_failures": 0,
		"pause_reason":         "",
	}); err != nil {
		return fmt.Errorf("重置调度失败计数失败: %w", err)
	}

	if sched.ConsecutiveFailures > 0 {
		logger.Sched("TASK").Info("[%s] 执行成功: %s | 失败计数已重置 (之前: %d)", shortID, sched.Name, sched.ConsecutiveFailures)
	} else {
		logger.Sched("TASK").Info("[%s] 执行完成: %s", shortID, sched.Name)
	}

	if !sched.IsCron() {
		if err := s.markCompletedSchedule(tenantCtx, sched.ID); err != nil {
			return fmt.Errorf("标记单次调度完成失败: %w", err)
		}
	}
	return nil
}
