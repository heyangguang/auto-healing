package healing

import (
	"context"

	"github.com/company/auto-healing/internal/model"
	"github.com/company/auto-healing/internal/pkg/logger"
	"github.com/google/uuid"
)

type channelTemplateConfig struct {
	ChannelID  string
	TemplateID string
}

func (e *FlowExecutor) executeNotification(ctx context.Context, instance *model.FlowInstance, node *model.FlowNode) error {
	logger.Exec("NODE").Info("[%s] 执行通知节点", shortID(instance))

	configs := parseNotificationConfigs(node.Config)
	webhookURL := notificationWebhookURL(node.Config)
	if len(configs) == 0 && webhookURL == "" {
		logger.Exec("NODE").Warn("未配置通知渠道，跳过发送")
		e.logNode(ctx, instance.ID, node.ID, node.Type, model.LogLevelWarn, "未配置通知渠道", map[string]interface{}{
			"config": node.Config,
		})
		return nil
	}

	includeExecutionResult, includeIncidentInfo := notificationFlags(node.Config)
	variables := e.buildNotificationVariables(instance, includeExecutionResult, includeIncidentInfo)
	subject, body := buildNotificationMessage(instance)

	e.logNotificationDispatchStart(ctx, instance, node, configs, subject)
	if len(configs) > 0 && e.notificationSvc != nil {
		e.sendConfiguredNotifications(ctx, instance, node, configs, variables, subject, body)
		return nil
	}
	if webhookURL != "" {
		e.sendNotificationWebhook(ctx, instance, node, webhookURL, variables, subject, body)
		return nil
	}

	logger.Exec("NODE").Warn("通知服务未初始化，跳过发送")
	return nil
}

func parseUUID(s string) uuid.UUID {
	id, _ := uuid.Parse(s)
	return id
}
