package healing

import (
	"context"
	"errors"
	"time"

	"github.com/company/auto-healing/internal/modules/automation/model"
	"github.com/company/auto-healing/internal/pkg/logger"
	platformrepo "github.com/company/auto-healing/internal/platform/repositoryx"
	"github.com/google/uuid"
)

const (
	stuckRecoveryThreshold = 2 * time.Minute
	orphanFailThreshold    = 30 * time.Minute
)

func (s *Scheduler) reconcileStuckInstances(ctx context.Context) {
	s.recoverResolvedApprovals(ctx)
	s.recoverStaleFlowInstances(ctx)
}

func (s *Scheduler) recoverResolvedApprovals(ctx context.Context) {
	rows, err := s.approvalRepo.ListResolvedWaitingFlowInstances(ctx, stuckRecoveryThreshold)
	if err != nil {
		logger.Sched("HEAL").Error("查询已审批但未恢复实例失败: %v", err)
		return
	}
	for _, row := range rows {
		s.scheduleInstanceRecovery(row.FlowInstanceID, row.TenantID, "审批已处理但实例仍停留在 waiting_approval")
	}
}

func (s *Scheduler) recoverStaleFlowInstances(ctx context.Context) {
	instances, err := s.instanceRepo.ListStaleByStatuses(ctx, []string{
		model.FlowInstanceStatusRunning,
		model.FlowInstanceStatusPending,
	}, stuckRecoveryThreshold)
	if err != nil {
		logger.Sched("HEAL").Error("查询停滞实例失败: %v", err)
		return
	}
	for _, instance := range instances {
		s.scheduleInstanceRecovery(instance.ID, instance.TenantID, "实例停滞超过恢复阈值")
	}
}

func (s *Scheduler) scheduleInstanceRecovery(instanceID uuid.UUID, tenantID *uuid.UUID, reason string) {
	lifecycle := s.ensureLifecycle()
	lifecycle.Go(func(rootCtx context.Context) {
		recoveryCtx := rootCtx
		if tenantID != nil {
			recoveryCtx = platformrepo.WithTenantID(rootCtx, *tenantID)
		}
		if _, err := s.executor.RecoverInstance(recoveryCtx, instanceID, model.FlowRecoveryTriggerScheduler); err != nil && !errorsIsRecoveryBusy(err) {
			logger.Sched("HEAL").Warn("自动恢复实例失败: instance=%s reason=%s error=%v", instanceID.String()[:8], reason, err)
		}
	})
}

func errorsIsRecoveryBusy(err error) bool {
	return errors.Is(err, ErrFlowInstanceRecoveryInProgress)
}
