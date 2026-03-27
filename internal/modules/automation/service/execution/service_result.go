package execution

import (
	"context"
	"fmt"
	"strings"

	"github.com/company/auto-healing/internal/modules/automation/engine/provider/ansible"
	"github.com/company/auto-healing/internal/modules/automation/model"
	integrationsmodel "github.com/company/auto-healing/internal/modules/integrations/model"
	"github.com/company/auto-healing/internal/pkg/logger"
	"github.com/google/uuid"
)

func buildRunProcessResult(result *ansible.ExecuteResult) runProcessResult {
	if result == nil {
		return runProcessResult{exitCode: -1}
	}

	processResult := runProcessResult{
		exitCode: result.ExitCode,
		stdout:   result.Stdout,
		stderr:   result.Stderr,
		duration: result.Duration,
	}
	if result.Stats != nil {
		processResult.stats = model.JSON{
			"ok":          result.Stats.Ok,
			"changed":     result.Stats.Changed,
			"unreachable": result.Stats.Unreachable,
			"failed":      result.Stats.Failed,
			"skipped":     result.Stats.Skipped,
			"rescued":     result.Stats.Rescued,
			"ignored":     result.Stats.Ignored,
		}
	}
	return processResult
}

func logRunOutput(runID uuid.UUID, stdout, stderr string) {
	for _, line := range strings.Split(stdout, "\n") {
		if line != "" {
			logger.Exec("ANSIBLE").Info("[%s] %s", shortRunID(runID), line)
		}
	}
	for _, line := range strings.Split(stderr, "\n") {
		if line != "" {
			logger.Exec("ANSIBLE").Warn("[%s] %s", shortRunID(runID), line)
		}
	}
}

func (s *Service) logRunOutcome(ctx context.Context, runID uuid.UUID, result runProcessResult, execErr error) {
	details := map[string]any{
		"exit_code": result.exitCode,
		"stats":     result.stats,
	}

	switch {
	case execErr != nil:
		s.appendLog(ctx, runID, "error", "execute", fmt.Sprintf("执行失败: %v", execErr), details)
	case result.exitCode == 0:
		s.appendLog(ctx, runID, "info", "execute", fmt.Sprintf("执行成功 (耗时: %v)", result.duration), details)
	default:
		s.appendLog(ctx, runID, "warn", "execute", fmt.Sprintf("执行完成但有错误 (退出码: %d)", result.exitCode), details)
	}
}

func (s *Service) sendRunCompletionNotification(ctx context.Context, runID uuid.UUID, task *model.ExecutionTask, playbook *integrationsmodel.Playbook, skipNotification bool) {
	if skipNotification {
		s.appendLog(ctx, runID, "info", "notification", "本次执行跳过通知", nil)
		return
	}
	if task.NotificationConfig == nil || !task.NotificationConfig.Enabled {
		return
	}

	run, err := s.repo.GetRunByID(ctx, runID)
	if err != nil {
		logger.Exec("RUN").Warn("[%s] 加载执行记录失败，跳过完成通知: %v", shortRunID(runID), err)
		return
	}
	task.Playbook = playbook
	logs, err := s.notificationSvc.SendFromExecution(ctx, toNotificationRun(run), toNotificationTask(task, playbook))
	if err != nil {
		s.appendLog(ctx, runID, "warn", "notification", fmt.Sprintf("发送通知失败: %v", err), nil)
		return
	}
	if len(logs) > 0 {
		s.appendLog(ctx, runID, "info", "notification", fmt.Sprintf("已发送 %d 条通知", len(logs)), nil)
	}
}
