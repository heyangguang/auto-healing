package healing

import (
	"context"

	"github.com/company/auto-healing/internal/model"
	"github.com/company/auto-healing/internal/pkg/logger"
)

const schedulerShutdownMessage = "调度器停止，流程被中断"

func (s *Scheduler) interruptQueuedFlow(rootCtx context.Context, instance *model.FlowInstance) {
	s.interruptFlow(rootCtx, instance, []string{model.FlowInstanceStatusPending})
}

func (s *Scheduler) interruptActiveFlow(rootCtx context.Context, instance *model.FlowInstance) {
	s.interruptFlow(rootCtx, instance, []string{
		model.FlowInstanceStatusPending,
		model.FlowInstanceStatusRunning,
	})
}

func (s *Scheduler) interruptFlow(rootCtx context.Context, instance *model.FlowInstance, currentStatuses []string) {
	ctx := withTenantContext(detachContext(rootCtx), instance.TenantID)
	updated, err := s.instanceRepo.UpdateStatusWithIncidentSync(
		ctx,
		instance.ID,
		currentStatuses,
		model.FlowInstanceStatusFailed,
		schedulerShutdownMessage,
		instanceIncidentSyncOptions(instance, "failed"),
	)
	if err != nil {
		logger.Sched("HEAL").Error("中断流程实例失败 [%s]: %v", instance.ID.String()[:8], err)
		return
	}
	if !updated {
		logger.Sched("HEAL").Warn("流程实例已进入终态，跳过中断失败覆盖 [%s]", instance.ID.String()[:8])
		return
	}
}
