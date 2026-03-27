package execution

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/company/auto-healing/internal/modules/automation/engine/provider/ansible"
	"github.com/company/auto-healing/internal/model"
	"github.com/company/auto-healing/internal/pkg/logger"
	"github.com/google/uuid"
)

type runProcessResult struct {
	exitCode int
	stdout   string
	stderr   string
	stats    model.JSON
	duration time.Duration
}

// executeInBackground 后台执行任务
func (s *Service) executeInBackground(rootCtx context.Context, runID uuid.UUID, task *model.ExecutionTask, playbook *model.Playbook, gitRepo *model.GitRepository, params *executeParams) {
	ctx, cleanup := s.bindRunContext(rootCtx, runID)
	defer cleanup()
	defer s.recoverRunPanic(ctx, runID)

	if !s.startRun(ctx, runID) {
		return
	}
	stopWatching := s.watchRunStatus(ctx, runID)
	defer stopWatching()

	task.Playbook = playbook
	s.sendRunStartNotification(ctx, runID, task, params.skipNotification)

	workDir, releaseWorkspace, err := s.prepareRunWorkspace(ctx, runID, gitRepo.LocalPath)
	if err != nil {
		return
	}
	defer releaseWorkspace()

	if s.abortOnSecurityViolations(ctx, runID, task, workDir) {
		return
	}

	inventoryPath, err := s.prepareRunInventory(ctx, runID, task, workDir, params)
	if err != nil {
		return
	}

	executor := s.selectExecutor(task.ExecutorType)
	result, execErr := s.executePlaybook(ctx, runID, task, playbook, workDir, inventoryPath, params.extraVars, executor)
	if ctx.Err() != nil {
		s.interruptRunningRun(runID, context.WithoutCancel(ctx))
		logger.Exec("RUN").Warn("执行被取消，跳过结果更新: %s", runID)
		return
	}

	processResult := buildRunProcessResult(result)
	s.persistRunResult(ctx, runID, processResult.exitCode, processResult.stdout, processResult.stderr, processResult.stats)
	logRunOutput(runID, processResult.stdout, processResult.stderr)
	s.logRunOutcome(ctx, runID, processResult, execErr)
	logger.Exec("RUN").Info("完成: %s | 状态: %s | 退出码: %d", runID, getStatusFromExitCode(processResult.exitCode), processResult.exitCode)
	s.sendRunCompletionNotification(ctx, runID, task, playbook, params.skipNotification)
}

func (s *Service) bindRunContext(rootCtx context.Context, runID uuid.UUID) (context.Context, func()) {
	ctx, cancel := context.WithCancel(rootCtx)
	s.runningExecutions.Store(runID, cancel)
	return ctx, func() {
		s.runningExecutions.Delete(runID)
		cancel()
	}
}

func (s *Service) recoverRunPanic(ctx context.Context, runID uuid.UUID) {
	if rec := recover(); rec != nil {
		logger.Exec("RUN").Error("[%s] executeInBackground panic: %v", shortRunID(runID), rec)
		s.finalizeRunFailure(ctx, runID, fmt.Sprintf("内部错误: %v", rec), nil)
	}
}

func (s *Service) startRun(ctx context.Context, runID uuid.UUID) bool {
	started, err := s.repo.UpdateRunStarted(ctx, runID)
	if err != nil {
		logger.Exec("RUN").Error("[%s] 更新执行开始状态失败: %v", shortRunID(runID), err)
		return false
	}
	if !started {
		logger.Exec("RUN").Warn("[%s] 执行在启动前已取消，跳过后台执行", shortRunID(runID))
		return false
	}
	return true
}

func (s *Service) watchRunStatus(ctx context.Context, runID uuid.UUID) func() {
	return watchRunCancellation(context.WithoutCancel(ctx), time.Second, func(watchCtx context.Context) (string, error) {
		return s.repo.GetRunStatus(watchCtx, runID)
	}, func() {
		s.cancelRunningExecution(runID)
	})
}

func (s *Service) sendRunStartNotification(ctx context.Context, runID uuid.UUID, task *model.ExecutionTask, skipNotification bool) {
	if skipNotification {
		return
	}
	if task.NotificationConfig == nil || !task.NotificationConfig.Enabled {
		return
	}

	run, err := s.repo.GetRunByID(ctx, runID)
	if err != nil {
		logger.Exec("RUN").Warn("[%s] 加载执行记录失败，跳过开始通知: %v", shortRunID(runID), err)
		return
	}
	if logs, err := s.notificationSvc.SendOnStart(ctx, run, task); err != nil {
		s.appendLog(ctx, runID, "warn", "notification", fmt.Sprintf("发送开始通知失败: %v", err), nil)
	} else if len(logs) > 0 {
		s.appendLog(ctx, runID, "info", "notification", fmt.Sprintf("已发送开始通知: %d 条", len(logs)), nil)
	}
}

func (s *Service) prepareRunWorkspace(ctx context.Context, runID uuid.UUID, localPath string) (string, func(), error) {
	s.appendLog(ctx, runID, "info", "prepare", "开始准备执行环境", nil)
	workDir, cleanup, err := s.workspaceManager.PrepareWorkspace(runID, localPath)
	if err != nil {
		s.finalizeRunFailure(ctx, runID, err.Error(), nil)
		s.appendDetachedLog(ctx, runID, "error", "prepare", fmt.Sprintf("准备工作空间失败: %v", err), nil)
		return "", nil, err
	}
	s.appendLog(ctx, runID, "info", "prepare", fmt.Sprintf("工作空间已准备: %s", workDir), nil)
	return workDir, cleanup, nil
}

func (s *Service) abortOnSecurityViolations(ctx context.Context, runID uuid.UUID, task *model.ExecutionTask, workDir string) bool {
	s.appendLog(ctx, runID, "info", "security", "开始安全扫描...", nil)
	violations, err := s.blacklistSvc.ScanWorkspace(ctx, workDir)
	if err != nil {
		s.appendLog(ctx, runID, "warn", "security", fmt.Sprintf("安全扫描异常: %v", err), nil)
	}
	violations = s.filterExemptedViolations(ctx, runID, task.ID, violations)
	if len(violations) == 0 {
		s.appendLog(ctx, runID, "info", "security", "安全扫描通过", nil)
		return false
	}

	s.logSecurityViolations(ctx, runID, violations)
	s.finalizeRunFailure(ctx, runID, fmt.Sprintf("安全拦截：检测到 %d 个高危指令", len(violations)), nil)
	logger.Exec("SECURITY").Warn("[%s] 检测到 %d 个高危指令，执行已拦截", shortRunID(runID), len(violations))
	return true
}

func (s *Service) filterExemptedViolations(ctx context.Context, runID, taskID uuid.UUID, violations []model.CommandBlacklistViolation) []model.CommandBlacklistViolation {
	if len(violations) == 0 {
		return violations
	}

	approvedExemptions, err := s.exemptionSvc.GetApprovedByTaskID(ctx, taskID)
	if err != nil {
		s.appendLog(ctx, runID, "warn", "security", fmt.Sprintf("加载豁免规则失败: %v", err), nil)
		return violations
	}
	if len(approvedExemptions) == 0 {
		return violations
	}

	exemptedRuleIDs := make(map[uuid.UUID]bool, len(approvedExemptions))
	for _, item := range approvedExemptions {
		exemptedRuleIDs[item.RuleID] = true
		logger.Exec("SECURITY").Info("[%s] 豁免规则: id=%s name=%s pattern=%s", shortRunID(runID), item.RuleID, item.RuleName, item.RulePattern)
	}

	filtered := violations[:0]
	exemptedCount := 0
	for _, violation := range violations {
		logger.Exec("SECURITY").Info("[%s] 违规命中: id=%s name=%s pattern=%s", shortRunID(runID), violation.RuleID, violation.RuleName, violation.Pattern)
		if exemptedRuleIDs[violation.RuleID] {
			exemptedCount++
			continue
		}
		filtered = append(filtered, violation)
	}
	if exemptedCount > 0 {
		s.appendLog(ctx, runID, "info", "security", fmt.Sprintf("已应用 %d 条豁免规则，跳过对应安全拦截", exemptedCount), nil)
	}
	return filtered
}

func (s *Service) logSecurityViolations(ctx context.Context, runID uuid.UUID, violations []model.CommandBlacklistViolation) {
	violationList := make([]map[string]any, 0, len(violations))
	var msg strings.Builder
	msg.WriteString(fmt.Sprintf("检测到 %d 个高危指令，执行已拦截:\n", len(violations)))

	for i, violation := range violations {
		violationList = append(violationList, map[string]any{
			"rule_id":   violation.RuleID,
			"file":      violation.File,
			"line":      violation.Line,
			"content":   violation.Content,
			"rule_name": violation.RuleName,
			"pattern":   violation.Pattern,
			"severity":  violation.Severity,
		})
		msg.WriteString(fmt.Sprintf("  %d. [%s] %s (文件: %s, 行: %d)\n", i+1, violation.Severity, violation.RuleName, violation.File, violation.Line))
	}

	s.appendLog(ctx, runID, "error", "security", msg.String(), map[string]any{
		"violations": violationList,
		"count":      len(violations),
	})
}

func (s *Service) executePlaybook(ctx context.Context, runID uuid.UUID, task *model.ExecutionTask, playbook *model.Playbook, workDir, inventoryPath string, extraVars map[string]any, executor ansible.Executor) (*ansible.ExecuteResult, error) {
	s.appendLog(ctx, runID, "info", "execute", fmt.Sprintf("开始执行 Playbook (执行器: %s)", executor.Name()), nil)
	return executor.Execute(ctx, &ansible.ExecuteRequest{
		PlaybookPath: playbook.FilePath,
		WorkDir:      workDir,
		Inventory:    inventoryPath,
		ExtraVars:    mergeExecutionVars(task.ExtraVars, extraVars),
		Timeout:      30 * time.Minute,
		LogCallback: func(level, stage, message string) {
			s.appendLog(ctx, runID, level, stage, message, nil)
		},
	})
}

func mergeExecutionVars(taskVars model.JSON, runtimeVars map[string]any) map[string]any {
	merged := make(map[string]any, len(taskVars)+len(runtimeVars))
	for k, v := range taskVars {
		merged[k] = v
	}
	for k, v := range runtimeVars {
		merged[k] = v
	}
	return merged
}

func (s *Service) selectExecutor(executorType string) ansible.Executor {
	if executorType == "docker" {
		return s.dockerExecutor
	}
	return s.localExecutor
}
