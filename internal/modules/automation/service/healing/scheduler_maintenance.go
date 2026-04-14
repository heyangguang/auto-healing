package healing

import (
	"context"
	"fmt"
	"time"

	"github.com/company/auto-healing/internal/modules/automation/model"
	"github.com/company/auto-healing/internal/pkg/logger"
	platformrepo "github.com/company/auto-healing/internal/platform/repositoryx"
	"github.com/google/uuid"
)

// processExpiredApprovals 处理超时的审批任务
func (s *Scheduler) processExpiredApprovals(ctx context.Context) {
	count, err := s.approvalRepo.ExpireTimedOut(ctx)
	if err != nil {
		logger.Sched("HEAL").Error("处理超时审批失败: %v", err)
		return
	}
	if count == 0 {
		return
	}

	expiredTasks, err := s.approvalRepo.ListRecentlyExpired(ctx, time.Now().Add(-time.Minute))
	if err != nil {
		logger.Sched("HEAL").Error("查询最近过期审批任务失败: %v", err)
		return
	}
	for _, task := range expiredTasks {
		s.failExpiredApprovalTask(ctx, task)
	}
	logger.Sched("HEAL").Info("已将 %d 个审批任务标记为超时", count)
}

func (s *Scheduler) failExpiredApprovalTask(ctx context.Context, task model.ApprovalTask) {
	taskCtx := ctx
	if task.TenantID != nil {
		taskCtx = platformrepo.WithTenantID(ctx, *task.TenantID)
	}
	opts := incidentFailureSyncOptions(taskCtx, s.instanceRepo, task.FlowInstanceID)
	updated, err := s.instanceRepo.UpdateStatusWithIncidentSync(
		taskCtx,
		task.FlowInstanceID,
		[]string{model.FlowInstanceStatusWaitingApproval},
		model.FlowInstanceStatusFailed,
		"审批超时",
		opts,
	)
	if err != nil || !updated {
		return
	}
	if opts != nil {
		logger.Sched("HEAL").Info("审批超时，工单 %s 状态已更新为 failed", opts.IncidentID.String()[:8])
	}
}

// TriggerManual 手动触发流程
func (s *Scheduler) TriggerManual(ctx context.Context, incidentID string, ruleID uuid.UUID) (*model.FlowInstance, error) {
	incident, err := s.incidentRepo.GetByID(ctx, parseUUID(incidentID))
	if err != nil {
		return nil, err
	}
	rule, err := s.ruleRepo.GetByID(ctx, ruleID)
	if err != nil {
		return nil, err
	}

	instance, err := s.createFlowInstance(ctx, incident, rule)
	if err != nil {
		return nil, err
	}
	s.scheduleManualFlowExecution(instance)
	return instance, nil
}

// recoverOrphanedInstances 恢复服务重启前遗留的 running/pending 实例
func (s *Scheduler) recoverOrphanedInstances(ctx context.Context) {
	instances, err := s.instanceRepo.ListStaleRunning(ctx, orphanFailThreshold)
	if err != nil {
		logger.Sched("HEAL").Error("查询停滞实例失败: %v", err)
		return
	}
	if len(instances) == 0 {
		return
	}

	logger.Sched("HEAL").Warn("发现 %d 个停滞的实例，开始恢复...", len(instances))
	for _, instance := range instances {
		s.recoverOrphanedInstance(ctx, instance, orphanFailThreshold)
	}
}

func (s *Scheduler) recoverOrphanedInstance(ctx context.Context, instance model.FlowInstance, staleThreshold time.Duration) {
	recoveryCtx := ctx
	if instance.TenantID != nil {
		recoveryCtx = platformrepo.WithTenantID(ctx, *instance.TenantID)
	}
	attempt, err := s.executor.RecoverInstance(recoveryCtx, instance.ID, model.FlowRecoveryTriggerScheduler)
	if err == nil && attempt != nil && attempt.Status == model.FlowRecoveryStatusSuccess {
		logger.Sched("HEAL").Info("已恢复孤儿实例 %s (%s) -> %s", instance.ID.String()[:8], instance.FlowName, attempt.RecoveryAction)
		return
	}

	errMsg := fmt.Sprintf("服务重启恢复: 实例已停滞超过 %v (上次更新: %s)", staleThreshold, instance.UpdatedAt.Format("2006-01-02 15:04:05"))
	updated, err := s.instanceRepo.UpdateStatusWithIncidentSync(
		recoveryCtx,
		instance.ID,
		[]string{model.FlowInstanceStatusRunning, model.FlowInstanceStatusPending},
		model.FlowInstanceStatusFailed,
		errMsg,
		instanceIncidentSyncOptions(&instance, "failed"),
	)
	if err != nil {
		logger.Sched("HEAL").Error("恢复实例 %s 失败: %v", instance.ID.String()[:8], err)
		return
	}
	if !updated {
		logger.Sched("HEAL").Warn("孤儿实例 %s 状态已变化，跳过失败覆盖", instance.ID.String()[:8])
		return
	}
	logger.Sched("HEAL").Warn("已恢复孤儿实例 %s (%s) -> failed", instance.ID.String()[:8], instance.FlowName)
}
