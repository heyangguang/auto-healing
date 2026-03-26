package healing

import (
	"context"
	"errors"
	"fmt"

	"github.com/company/auto-healing/internal/model"
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
	case <-lifecycle.ctx.Done():
		return
	}
	s.startTrackedFlowWorker(lifecycle, instance)
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
			logger.Sched("HEAL").Error("流程执行 panic [%s]: %v", instance.ID.String()[:8], fmt.Sprintf("%v", r))
			s.instanceRepo.UpdateStatus(execCtx, instance.ID,
				model.FlowInstanceStatusFailed,
				fmt.Sprintf("执行异常(panic): %v", r))
		}
	}()
	if err := s.runFlow(execCtx, instance); shouldLogTrackedFlowError(rootCtx, err) {
		logger.Sched("HEAL").Error("流程执行返回错误 [%s]: %v", instance.ID.String()[:8], err)
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
