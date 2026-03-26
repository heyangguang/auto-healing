package execution

import (
	"context"
	"time"

	"github.com/company/auto-healing/internal/model"
	"github.com/company/auto-healing/internal/pkg/logger"
	"github.com/google/uuid"
)

// ==================== 内部方法 ====================

func (s *Service) appendLog(ctx context.Context, runID uuid.UUID, level, stage, message string, details map[string]any) {
	if err := s.appendLogErr(ctx, runID, level, stage, message, details); err != nil {
		logger.Exec("RUN").Error("追加执行日志失败: run=%s stage=%s err=%v", runID, stage, err)
	}
}

func (s *Service) appendLogErr(ctx context.Context, runID uuid.UUID, level, stage, message string, details map[string]any) error {
	seq, err := s.repo.GetNextLogSequence(ctx, runID)
	if err != nil {
		return err
	}

	var detailsJSON model.JSON
	if details != nil {
		detailsJSON = model.JSON(details)
	}

	return s.repo.AppendLog(ctx, &model.ExecutionLog{
		RunID:    runID,
		LogLevel: level,
		Stage:    stage,
		Message:  message,
		Details:  detailsJSON,
		Sequence: seq,
	})
}

func getStatusFromExitCode(exitCode int) string {
	if exitCode == 0 {
		return "success"
	}
	return "failed"
}

func uuidsToStrings(uuids []uuid.UUID) model.StringArray {
	result := make(model.StringArray, len(uuids))
	for i, u := range uuids {
		result[i] = u.String()
	}
	return result
}

func toJSON(m map[string]any) model.JSON {
	if m == nil {
		return nil
	}
	result := make(model.JSON, len(m))
	for k, v := range m {
		result[k] = v
	}
	return result
}

func (s *Service) persistRunResult(ctx context.Context, runID uuid.UUID, exitCode int, stdout, stderr string, stats model.JSON) {
	if err := s.repo.UpdateRunResult(ctx, runID, exitCode, stdout, stderr, stats); err != nil {
		logger.Exec("RUN").Error("更新执行结果失败: run=%s err=%v", runID, err)
	}
}

func watchRunCancellation(ctx context.Context, pollInterval time.Duration, runStatus func(context.Context) (string, error), cancel context.CancelFunc) func() {
	stopCtx, stop := context.WithCancel(ctx)

	go func() {
		ticker := time.NewTicker(pollInterval)
		defer ticker.Stop()

		for {
			select {
			case <-stopCtx.Done():
				return
			case <-ticker.C:
				status, err := runStatus(stopCtx)
				if err != nil {
					continue
				}
				switch status {
				case "cancelled":
					cancel()
					return
				case "success", "failed", "partial":
					return
				}
			}
		}
	}()

	return stop
}

func shortRunID(runID uuid.UUID) string {
	return runID.String()[:8]
}
