package provider

import (
	"context"
	"fmt"
	"time"

	"github.com/company/auto-healing/internal/model"
	"github.com/company/auto-healing/internal/pkg/logger"
	"github.com/company/auto-healing/internal/repository"
	executionService "github.com/company/auto-healing/internal/service/execution"
	"github.com/google/uuid"
)

func (s *ExecutionScheduler) executeSchedule(ctx context.Context, sched model.ExecutionSchedule) {
	shortID := sched.ID.String()[:8]
	tenantCtx := scheduleTenantContext(ctx, sched)
	triggered := false

	defer func() {
		if rec := recover(); rec != nil {
			panicErr := fmt.Errorf("panic: %v", rec)
			logger.Sched("TASK").Error("[%s] 定时任务 panic: %v", shortID, rec)
			if stateErr := s.handleScheduledExecutionError(ctx, tenantCtx, sched, shortID, panicErr, triggered); stateErr != nil {
				logger.Sched("TASK").Error("[%s] panic 后状态更新失败: %v | state_err=%v", shortID, panicErr, stateErr)
			}
		}
	}()

	logger.Sched("TASK").Info("[%s] 开始执行定时任务: %s", shortID, sched.Name)
	run, err := s.execSvc.ExecuteTask(tenantCtx, sched.TaskID, buildExecutionOptions(sched))
	if err == nil {
		triggered = true
		err = s.afterScheduleTriggered(tenantCtx, sched, run.ID)
	}
	if err != nil {
		if stateErr := s.handleScheduledExecutionError(ctx, tenantCtx, sched, shortID, err, triggered); stateErr != nil {
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
	return repository.WithTenantID(ctx, *sched.TenantID)
}

func buildExecutionOptions(sched model.ExecutionSchedule) *executionService.ExecuteOptions {
	opts := &executionService.ExecuteOptions{
		TriggeredBy:      scheduleTriggerLabel(sched),
		TargetHosts:      sched.TargetHostsOverride,
		ExtraVars:        sched.ExtraVarsOverride,
		SkipNotification: sched.SkipNotification,
	}
	for _, idStr := range sched.SecretsSourceIDs {
		if id, err := uuid.Parse(idStr); err == nil {
			opts.SecretsSourceIDs = append(opts.SecretsSourceIDs, id)
		}
	}
	return opts
}

func scheduleTriggerLabel(sched model.ExecutionSchedule) string {
	if sched.IsCron() {
		return "scheduler:cron"
	}
	return "scheduler:once"
}

func (s *ExecutionScheduler) afterScheduleTriggered(ctx context.Context, sched model.ExecutionSchedule, runID uuid.UUID) error {
	if err := s.updateScheduleLastRun(ctx, sched.ID); err != nil {
		return fmt.Errorf("更新调度 last_run_at 失败: %w", err)
	}
	if sched.IsCron() {
		if err := s.updateScheduleNextRun(ctx, sched.ID, *sched.ScheduleExpr); err != nil {
			return fmt.Errorf("更新调度 next_run_at 失败: %w", err)
		}
	} else {
		if err := s.updateScheduleState(ctx, sched.ID, map[string]interface{}{
			"status":      model.ScheduleStatusRunning,
			"last_run_at": time.Now(),
			"next_run_at": nil,
		}); err != nil {
			return fmt.Errorf("更新单次调度运行状态失败: %w", err)
		}
	}

	finalStatus, err := s.waitForRunTerminalStatus(ctx, runID)
	if err != nil {
		return err
	}
	if runStatusCountsAsSuccess(finalStatus) {
		return nil
	}
	return fmt.Errorf("执行结果状态为 %s", finalStatus)
}

func (s *ExecutionScheduler) handleScheduledExecutionError(ctx, tenantCtx context.Context, sched model.ExecutionSchedule, shortID string, err error, triggered bool) error {
	newCount := sched.ConsecutiveFailures + 1
	updates := map[string]interface{}{
		"consecutive_failures": newCount,
	}
	pausedCron := sched.MaxFailures > 0 && newCount >= sched.MaxFailures && sched.IsCron()
	if pausedCron {
		updates["enabled"] = false
		updates["status"] = model.ScheduleStatusAutoPaused
		updates["next_run_at"] = nil
		updates["pause_reason"] = fmt.Sprintf("连续失败 %d 次后自动暂停 (最后错误: %s)", newCount, truncateStr(err.Error(), 200))
		logger.Sched("TASK").Warn("[%s] ⚠ 连续失败 %d/%d 次，已自动暂停: %s", shortID, newCount, sched.MaxFailures, sched.Name)
	} else if sched.MaxFailures > 0 {
		logger.Sched("TASK").Error("[%s] 执行失败 (%d/%d): %s - %v", shortID, newCount, sched.MaxFailures, sched.Name, err)
	} else {
		logger.Sched("TASK").Error("[%s] 执行失败 (第%d次): %s - %v", shortID, newCount, sched.Name, err)
	}

	if updateErr := s.updateScheduleState(ctx, sched.ID, updates); updateErr != nil {
		return fmt.Errorf("更新调度失败状态失败: %w", updateErr)
	}
	if sched.IsCron() && !triggered && !pausedCron {
		if err := s.restoreCronNextRun(ctx, sched); err != nil {
			return fmt.Errorf("恢复 cron 下一次执行时间失败: %w", err)
		}
	}
	if !sched.IsCron() {
		if completeErr := s.markScheduleCompleted(tenantCtx, sched.ID); completeErr != nil {
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
		if err := s.markScheduleCompleted(tenantCtx, sched.ID); err != nil {
			return fmt.Errorf("标记单次调度完成失败: %w", err)
		}
	}
	return nil
}
