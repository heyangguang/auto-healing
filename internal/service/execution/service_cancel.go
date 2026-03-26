package execution

import (
	"context"
	"fmt"

	"github.com/company/auto-healing/internal/pkg/logger"
	"github.com/google/uuid"
)

// CancelRun 取消执行
func (s *Service) CancelRun(ctx context.Context, id uuid.UUID) error {
	run, err := s.repo.GetRunByID(ctx, id)
	if err != nil {
		return err
	}
	if run.Status != "pending" && run.Status != "running" {
		return fmt.Errorf("执行状态不允许取消: %s", run.Status)
	}

	updated, err := s.repo.UpdateRunStatus(ctx, id, "cancelled")
	if err != nil {
		return err
	}
	if !updated {
		return fmt.Errorf("执行状态不允许取消: %s", s.runStatusForCancelError(ctx, id))
	}

	s.cancelRunningExecution(id)
	s.appendLog(ctx, id, "warn", "control", "执行已被取消", nil)
	logger.Exec("RUN").Warn("已取消: %s", id)
	return nil
}

func (s *Service) runStatusForCancelError(ctx context.Context, id uuid.UUID) string {
	status, err := s.repo.GetRunStatus(ctx, id)
	if err != nil || status == "" {
		return "unknown"
	}
	return status
}

func (s *Service) cancelRunningExecution(id uuid.UUID) {
	cancelFunc, ok := s.runningExecutions.Load(id)
	if !ok {
		return
	}
	if cancel, ok := cancelFunc.(context.CancelFunc); ok {
		cancel()
		logger.Exec("RUN").Warn("已发送取消信号: %s", id)
	}
	s.runningExecutions.Delete(id)
}
