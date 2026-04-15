package healing

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/company/auto-healing/internal/modules/automation/model"
	engagementmodel "github.com/company/auto-healing/internal/modules/engagement/model"
	notificationSvc "github.com/company/auto-healing/internal/modules/engagement/service/notification"
	"github.com/company/auto-healing/internal/pkg/logger"
	"github.com/google/uuid"
)

func parseNotificationConfigs(config map[string]interface{}) []channelTemplateConfig {
	configs := parseStructuredNotificationConfigs(config)
	if len(configs) > 0 {
		return configs
	}
	return parseLegacyNotificationConfigs(config)
}

func parseStructuredNotificationConfigs(config map[string]interface{}) []channelTemplateConfig {
	rawConfigs, ok := config["notification_configs"].([]interface{})
	if !ok || len(rawConfigs) == 0 {
		return nil
	}

	var configs []channelTemplateConfig
	for _, raw := range rawConfigs {
		cfgMap, ok := raw.(map[string]interface{})
		if !ok {
			continue
		}
		channelID, _ := cfgMap["channel_id"].(string)
		templateID, _ := cfgMap["template_id"].(string)
		if channelID == "" {
			continue
		}
		configs = append(configs, channelTemplateConfig{ChannelID: channelID, TemplateID: templateID})
	}
	return configs
}

func parseLegacyNotificationConfigs(config map[string]interface{}) []channelTemplateConfig {
	var channelIDs []string
	if channelID, ok := config["channel_id"].(string); ok && channelID != "" {
		channelIDs = append(channelIDs, channelID)
	}
	if rawIDs, ok := config["channel_ids"].([]interface{}); ok {
		for _, raw := range rawIDs {
			if channelID, ok := raw.(string); ok && channelID != "" {
				channelIDs = append(channelIDs, channelID)
			}
		}
	}

	templateID, _ := config["template_id"].(string)
	configs := make([]channelTemplateConfig, 0, len(channelIDs))
	for _, channelID := range channelIDs {
		configs = append(configs, channelTemplateConfig{ChannelID: channelID, TemplateID: templateID})
	}
	return configs
}

func notificationFlags(config map[string]interface{}) (bool, bool) {
	includeExecutionResult := true
	if value, ok := config["include_execution_result"].(bool); ok {
		includeExecutionResult = value
	}
	includeIncidentInfo := true
	if value, ok := config["include_incident_info"].(bool); ok {
		includeIncidentInfo = value
	}
	return includeExecutionResult, includeIncidentInfo
}

func notificationWebhookURL(config map[string]interface{}) string {
	webhookURL, _ := config["webhook_url"].(string)
	return webhookURL
}

func buildNotificationMessage(instance *model.FlowInstance) (string, string) {
	subject := fmt.Sprintf("[自愈系统] 流程实例 #%s 执行完成", instance.ID.String()[:8])
	body := fmt.Sprintf("流程实例 #%s 执行完成，状态：%s", instance.ID.String()[:8], instance.Status)

	if instance.Context == nil {
		return subject, body
	}
	execResult, ok := instance.Context["execution_result"].(map[string]interface{})
	if !ok {
		return subject, body
	}

	if status, ok := execResult["status"].(string); ok {
		if status == "completed" {
			subject = fmt.Sprintf("[自愈系统] 流程实例 #%s 执行成功", instance.ID.String()[:8])
		} else {
			subject = fmt.Sprintf("[自愈系统] 流程实例 #%s 执行失败", instance.ID.String()[:8])
		}
	}
	if message, ok := execResult["message"].(string); ok {
		body = message
	}
	return subject, body
}

func (e *FlowExecutor) logNotificationDispatchStart(ctx context.Context, instance *model.FlowInstance, node *model.FlowNode, configs []channelTemplateConfig, subject string) {
	processLogs := []string{fmt.Sprintf("共 %d 个通知配置", len(configs))}
	for i, cfg := range configs {
		processLogs = append(processLogs, fmt.Sprintf("  配置 %d: 渠道=%s, 模板=%s", i+1, cfg.ChannelID, cfg.TemplateID))
	}
	processLogs = append(processLogs, fmt.Sprintf("主题: %s", subject), "开始发送通知")

	e.logNode(ctx, instance.ID, node.ID, node.Type, model.LogLevelInfo, "发送通知", map[string]interface{}{
		"input": map[string]interface{}{
			"context":              instance.Context,
			"notification_configs": configs,
		},
		"process": processLogs,
	})
}

func (e *FlowExecutor) sendConfiguredNotifications(ctx context.Context, instance *model.FlowInstance, node *model.FlowNode, configs []channelTemplateConfig, variables map[string]interface{}, subject, body string) {
	var allLogs []interface{}
	var lastErr error

	for _, cfg := range configs {
		logs, err := e.sendNotificationToChannel(ctx, instance, cfg, variables, subject, body)
		if err != nil {
			logger.Exec("NODE").Error("通知服务发送失败 (渠道=%s): %v", cfg.ChannelID, err)
			lastErr = err
			continue
		}
		for _, notifLog := range logs {
			allLogs = append(allLogs, notifLog)
			e.logNode(ctx, instance.ID, node.ID, node.Type, model.LogLevelInfo, "通知已发送", map[string]interface{}{
				"log_id":      notifLog.ID,
				"status":      notifLog.Status,
				"channel_id":  notifLog.ChannelID,
				"template_id": cfg.TemplateID,
				"subject":     notifLog.Subject,
			})
		}
	}

	if lastErr != nil && len(allLogs) == 0 {
		e.logNode(ctx, instance.ID, node.ID, node.Type, model.LogLevelError, "通知发送失败", map[string]interface{}{
			"error": lastErr.Error(),
		})
		return
	}
	logger.Exec("NODE").Info("[%s] 通知发送完成, 共发送 %d 条通知", shortID(instance), len(allLogs))
}

func (e *FlowExecutor) sendNotificationToChannel(ctx context.Context, instance *model.FlowInstance, cfg channelTemplateConfig, variables map[string]interface{}, subject, body string) ([]*engagementmodel.NotificationLog, error) {
	channelUUID, err := uuid.Parse(cfg.ChannelID)
	if err != nil {
		return nil, err
	}

	var templateUUID *uuid.UUID
	if cfg.TemplateID != "" {
		if value, err := uuid.Parse(cfg.TemplateID); err == nil {
			templateUUID = &value
		}
	}

	return e.notificationSvc.Send(ctx, notificationSvc.SendNotificationRequest{
		TemplateID:         templateUUID,
		ChannelIDs:         []uuid.UUID{channelUUID},
		Variables:          variables,
		Subject:            subject,
		Body:               body,
		Format:             "markdown",
		WorkflowInstanceID: &instance.ID,
		IncidentID:         instance.IncidentID,
	})
}

func (e *FlowExecutor) sendNotificationWebhook(ctx context.Context, instance *model.FlowInstance, node *model.FlowNode, webhookURL string, variables map[string]interface{}, subject, body string) {
	payloadBytes, _ := json.Marshal(map[string]interface{}{
		"subject":   subject,
		"body":      body,
		"variables": variables,
		"timestamp": time.Now().Format(time.RFC3339),
	})

	httpClient := &http.Client{Timeout: 30 * time.Second}
	resp, err := httpClient.Post(webhookURL, "application/json", bytes.NewReader(payloadBytes))
	if err != nil {
		logger.Exec("NODE").Error("Webhook 通知发送失败: %v", err)
		e.logNode(ctx, instance.ID, node.ID, node.Type, model.LogLevelError, "Webhook 通知发送失败", map[string]interface{}{
			"error":       err.Error(),
			"webhook_url": webhookURL,
		})
		return
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	logger.Exec("NODE").Info("Webhook 通知发送成功: HTTP %d, 响应: %s", resp.StatusCode, string(respBody))
	e.logNode(ctx, instance.ID, node.ID, node.Type, model.LogLevelInfo, "Webhook 通知发送成功", map[string]interface{}{
		"status_code":    resp.StatusCode,
		"webhook_url":    webhookURL,
		"variable_count": len(variables),
	})
}
