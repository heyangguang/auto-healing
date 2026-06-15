package execution

import (
	"context"
	"fmt"

	"github.com/company/auto-healing/internal/modules/automation/model"
	"github.com/company/auto-healing/internal/modules/automation/service/secretsources"
	"github.com/google/uuid"
)

var allowedExecutorTypes = map[string]struct{}{
	"local":  {},
	"docker": {},
}

func (s *Service) validateTaskForSave(ctx context.Context, task *model.ExecutionTask) error {
	if task.ExecutorType == "" {
		task.ExecutorType = "local"
	}
	if _, ok := allowedExecutorTypes[task.ExecutorType]; !ok {
		return fmt.Errorf("executor_type 仅支持 local 或 docker")
	}
	if err := secretsources.ValidateStringArray(ctx, s.secretsRepo, task.SecretsSourceIDs, "secrets_source_ids"); err != nil {
		return err
	}
	return s.validateNotificationConfig(ctx, task.NotificationConfig)
}

func (s *Service) validateRuntimeSecrets(ctx context.Context, ids []uuid.UUID) error {
	return secretsources.ValidateActive(ctx, s.secretsRepo, ids)
}

func (s *Service) validateNotificationConfig(ctx context.Context, cfg *model.TaskNotificationConfig) error {
	if cfg == nil || !cfg.Enabled {
		return nil
	}
	triggers := []struct {
		name string
		cfg  *model.NotificationTriggerConfig
	}{
		{name: "on_start", cfg: cfg.OnStart},
		{name: "on_success", cfg: cfg.OnSuccess},
		{name: "on_failure", cfg: cfg.OnFailure},
	}
	for _, trigger := range triggers {
		if err := s.validateNotificationTrigger(ctx, trigger.name, trigger.cfg); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) validateNotificationTrigger(ctx context.Context, name string, cfg *model.NotificationTriggerConfig) error {
	if cfg == nil || !cfg.Enabled {
		return nil
	}
	if cfg.TemplateID == nil || *cfg.TemplateID == uuid.Nil {
		return fmt.Errorf("notification_config.%s.template_id 不能为空", name)
	}
	if len(cfg.ChannelIDs) == 0 {
		return fmt.Errorf("notification_config.%s.channel_ids 不能为空", name)
	}
	template, err := s.notificationSvc.GetTemplate(ctx, *cfg.TemplateID)
	if err != nil {
		return fmt.Errorf("notification_config.%s.template_id 引用了不存在的通知模板: %s", name, cfg.TemplateID.String())
	}
	if !template.IsActive {
		return fmt.Errorf("notification_config.%s.template_id 引用了未启用的通知模板: %s", name, template.Name)
	}
	seenChannels := make(map[uuid.UUID]struct{}, len(cfg.ChannelIDs))
	for _, channelID := range cfg.ChannelIDs {
		if channelID == uuid.Nil {
			return fmt.Errorf("notification_config.%s.channel_ids 包含空 ID", name)
		}
		if _, exists := seenChannels[channelID]; exists {
			return fmt.Errorf("notification_config.%s.channel_ids 包含重复渠道: %s", name, channelID)
		}
		seenChannels[channelID] = struct{}{}
		channel, err := s.notificationSvc.GetChannel(ctx, channelID)
		if err != nil {
			return fmt.Errorf("notification_config.%s.channel_ids 引用了不存在的通知渠道: %s", name, channelID)
		}
		if !channel.IsActive {
			return fmt.Errorf("notification_config.%s.channel_ids 引用了未启用的通知渠道: %s", name, channel.Name)
		}
	}
	return nil
}
