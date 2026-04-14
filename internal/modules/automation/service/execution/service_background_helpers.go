package execution

import (
	"context"
	"fmt"
	"time"

	"github.com/company/auto-healing/internal/modules/automation/engine/provider/ansible"
	"github.com/company/auto-healing/internal/modules/automation/model"
	engagementmodel "github.com/company/auto-healing/internal/modules/engagement/model"
	projection "github.com/company/auto-healing/internal/modules/engagement/projection"
	integrationsmodel "github.com/company/auto-healing/internal/modules/integrations/model"
	"github.com/google/uuid"
)

func (s *Service) executePlaybook(ctx context.Context, runID uuid.UUID, task *model.ExecutionTask, playbook *integrationsmodel.Playbook, workDir, inventoryPath string, extraVars map[string]any, executor ansible.Executor) (*ansible.ExecuteResult, error) {
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

func toNotificationRun(run *model.ExecutionRun) *engagementmodel.ExecutionRun {
	if run == nil {
		return nil
	}
	return &engagementmodel.ExecutionRun{
		ID:          run.ID,
		TenantID:    run.TenantID,
		TaskID:      run.TaskID,
		Status:      run.Status,
		ExitCode:    run.ExitCode,
		Stats:       engagementmodel.JSON(run.Stats),
		Stdout:      run.Stdout,
		Stderr:      run.Stderr,
		TriggeredBy: run.TriggeredBy,
		StartedAt:   run.StartedAt,
		CompletedAt: run.CompletedAt,
		CreatedAt:   run.CreatedAt,
		RuntimeTargetHosts:      run.RuntimeTargetHosts,
		RuntimeSecretsSourceIDs: engagementmodel.StringArray(run.RuntimeSecretsSourceIDs),
		RuntimeExtraVars:        engagementmodel.JSON(run.RuntimeExtraVars),
		RuntimeSkipNotification: run.RuntimeSkipNotification,
	}
}

func toNotificationTask(task *model.ExecutionTask, playbook *integrationsmodel.Playbook) *engagementmodel.ExecutionTask {
	if task == nil {
		return nil
	}
	notifyTask := &engagementmodel.ExecutionTask{
		ID:                 task.ID,
		PlaybookID:         task.PlaybookID,
		Name:               task.Name,
		Description:        task.Description,
		TargetHosts:        task.TargetHosts,
		ExecutorType:       task.ExecutorType,
		NotificationConfig: toNotificationTaskConfig(task.NotificationConfig),
	}
	if playbook == nil {
		return notifyTask
	}
	notifyPlaybook := &engagementmodel.Playbook{
		ID:           playbook.ID,
		RepositoryID: playbook.RepositoryID,
		Name:         playbook.Name,
		Description:  playbook.Description,
		FilePath:     playbook.FilePath,
		Status:       playbook.Status,
	}
	if playbook.Repository != nil {
		notifyPlaybook.Repository = &engagementmodel.GitRepository{
			ID:            playbook.Repository.ID,
			Name:          playbook.Repository.Name,
			URL:           playbook.Repository.URL,
			DefaultBranch: playbook.Repository.DefaultBranch,
			Status:        playbook.Repository.Status,
			LastSyncAt:    playbook.Repository.LastSyncAt,
		}
	}
	notifyTask.Playbook = notifyPlaybook
	return notifyTask
}

func toNotificationTaskConfig(cfg *model.TaskNotificationConfig) *projection.TaskNotificationConfig {
	if cfg == nil {
		return nil
	}
	return &projection.TaskNotificationConfig{
		Enabled:   cfg.Enabled,
		OnStart:   toNotificationTriggerConfig(cfg.OnStart),
		OnSuccess: toNotificationTriggerConfig(cfg.OnSuccess),
		OnFailure: toNotificationTriggerConfig(cfg.OnFailure),
	}
}

func toNotificationTriggerConfig(cfg *model.NotificationTriggerConfig) *projection.NotificationTriggerConfig {
	if cfg == nil {
		return nil
	}
	channelIDs := append([]uuid.UUID(nil), cfg.ChannelIDs...)
	return &projection.NotificationTriggerConfig{
		Enabled:    cfg.Enabled,
		ChannelIDs: channelIDs,
		TemplateID: cfg.TemplateID,
	}
}
