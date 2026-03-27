package healing

import (
	"context"
	"errors"
	"fmt"

	"github.com/company/auto-healing/internal/modules/automation/model"
	"github.com/company/auto-healing/internal/pkg/logger"
	"github.com/google/uuid"
)

func (s *Scheduler) ensureLifecycle() *asyncLifecycle {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.lifecycle == nil || s.lifecycle.ctx.Err() != nil {
		s.lifecycle = newAsyncLifecycle()
	}
	return s.lifecycle
}

func (s *Scheduler) scheduleAutoFlowExecution(instance *model.FlowInstance, incidentID uuid.UUID) {
	lifecycle := s.ensureLifecycle()
	select {
	case s.sem <- struct{}{}:
		s.startTrackedFlowWorker(lifecycle, instance)
	default:
		logger.Sched("HEAL").Warn("并发执行达到上限，工单 %s 延迟执行", incidentID)
		lifecycle.Go(func(rootCtx context.Context) {
			select {
			case s.sem <- struct{}{}:
			case <-rootCtx.Done():
				s.interruptQueuedFlow(rootCtx, instance)
				return
			}
			s.executeTrackedFlow(rootCtx, instance)
		})
	}
}

func (s *Scheduler) scheduleManualFlowExecution(instance *model.FlowInstance) {
	lifecycle := s.ensureLifecycle()
	select {
	case s.sem <- struct{}{}:
		s.startTrackedFlowWorker(lifecycle, instance)
		return
	default:
		lifecycle.Go(func(rootCtx context.Context) {
			select {
			case s.sem <- struct{}{}:
			case <-rootCtx.Done():
				s.interruptQueuedFlow(rootCtx, instance)
				return
			}
			s.executeTrackedFlow(rootCtx, instance)
		})
	}
}

func (s *Scheduler) startTrackedFlowWorker(lifecycle *asyncLifecycle, instance *model.FlowInstance) {
	lifecycle.Go(func(rootCtx context.Context) {
		s.executeTrackedFlow(rootCtx, instance)
	})
}

func (s *Scheduler) executeTrackedFlow(rootCtx context.Context, instance *model.FlowInstance) {
	defer func() { <-s.sem }()

	execCtx := withTenantContext(rootCtx, instance.TenantID)
	defer func() {
		if r := recover(); r != nil {
			errMsg := fmt.Sprintf("执行异常(panic): %v", r)
			logger.Sched("HEAL").Error("流程执行 panic [%s]: %v", instance.ID.String()[:8], errMsg)
			s.failTrackedFlow(execCtx, instance, errMsg)
		}
	}()
	if err := s.runFlow(execCtx, instance); shouldLogTrackedFlowError(rootCtx, err) {
		logger.Sched("HEAL").Error("流程执行返回错误 [%s]: %v", instance.ID.String()[:8], err)
	}
	if rootCtx.Err() != nil {
		s.interruptActiveFlow(rootCtx, instance)
	}
}

func (s *Scheduler) failTrackedFlow(ctx context.Context, instance *model.FlowInstance, errMsg string) {
	updated, err := s.instanceRepo.UpdateStatusWithIncidentSync(
		ctx,
		instance.ID,
		[]string{
			model.FlowInstanceStatusPending,
			model.FlowInstanceStatusRunning,
			model.FlowInstanceStatusWaitingApproval,
		},
		model.FlowInstanceStatusFailed,
		errMsg,
		instanceIncidentSyncOptions(instance, "failed"),
	)
	if err != nil {
		logger.Sched("HEAL").Error("更新流程实例失败 [%s]: %v", instance.ID.String()[:8], err)
		return
	}
	if !updated {
		logger.Sched("HEAL").Warn("流程实例已进入终态，跳过失败状态覆盖 [%s]", instance.ID.String()[:8])
	}
}

func shouldLogTrackedFlowError(rootCtx context.Context, err error) bool {
	if err == nil {
		return false
	}
	if rootErr := rootCtx.Err(); rootErr != nil && errors.Is(err, rootErr) {
		return false
	}
	return true
}
