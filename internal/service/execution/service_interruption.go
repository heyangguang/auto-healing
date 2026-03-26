package execution

import (
	"context"

	"github.com/company/auto-healing/internal/model"
	"github.com/company/auto-healing/internal/pkg/logger"
	"github.com/company/auto-healing/internal/repository"
	"github.com/google/uuid"
)

const serviceShutdownMessage = "服务停止，执行被中断"

func (s *Service) markPendingRunInterrupted(runID uuid.UUID, ctx context.Context) {
	detached := detachedExecutionContext(ctx)
	updated, err := s.repo.UpdateRunResultIfCurrent(detached, runID, []string{"pending"}, -1, "", serviceShutdownMessage, nil)
	if err != nil {
		logger.Exec("RUN").Error("回写未启动执行终态失败: run=%s err=%v", runID, err)
		return
	}
	if !updated {
		return
	}
	s.appendLog(detached, runID, "warn", "control", serviceShutdownMessage, nil)
}

func (s *Service) interruptRunningRun(runID uuid.UUID, ctx context.Context) {
	detached := detachedExecutionContext(ctx)
	updated, err := s.repo.UpdateRunResultIfCurrent(detached, runID, []string{"running"}, -1, "", serviceShutdownMessage, nil)
	if err != nil {
		logger.Exec("RUN").Error("回写中断执行终态失败: run=%s err=%v", runID, err)
		return
	}
	if !updated {
		return
	}
	s.appendLog(detached, runID, "warn", "control", serviceShutdownMessage, nil)
}

func (s *Service) appendDetachedLog(ctx context.Context, runID uuid.UUID, level, stage, message string, details map[string]any) {
	s.appendLog(detachedExecutionContext(ctx), runID, level, stage, message, details)
}

func detachedExecutionContext(ctx context.Context) context.Context {
	detached := context.WithoutCancel(ctx)
	tenantID, ok := repository.TenantIDFromContextOK(ctx)
	if !ok {
		return detached
	}
	return repository.WithTenantID(detached, tenantID)
}

func (s *Service) finalizeRunFailure(ctx context.Context, runID uuid.UUID, message string, stats model.JSON) {
	detached := detachedExecutionContext(ctx)
	if err := s.repo.UpdateRunResult(detached, runID, -1, "", message, stats); err != nil {
		logger.Exec("RUN").Error("回写执行失败终态失败: run=%s err=%v", runID, err)
	}
}
