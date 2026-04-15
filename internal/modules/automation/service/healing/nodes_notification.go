package healing

import (
	"context"
	"fmt"

	"github.com/company/auto-healing/internal/modules/automation/model"
	notification "github.com/company/auto-healing/internal/modules/engagement/service/notification"
	"github.com/google/uuid"
)

// NotificationConfig 通知配置
type NotificationConfig struct {
	ChannelID  int64             `json:"channel_id"`
	TemplateID int64             `json:"template_id"`
	Recipients []string          `json:"recipients"`
	Variables  map[string]string `json:"variables"`
}

// ExecuteNotification 执行通知
func (e *NodeExecutors) ExecuteNotification(ctx context.Context, instance *model.FlowInstance, config map[string]interface{}) error {
	sendReq := e.buildNotificationSendRequest(instance, config)
	_, err := e.newNotificationService().Send(ctx, sendReq)
	return err
}

func (e *NodeExecutors) newNotificationService() *notification.Service {
	return e.notificationSvc
}

func (e *NodeExecutors) buildNotificationSendRequest(instance *model.FlowInstance, config map[string]interface{}) notification.SendNotificationRequest {
	sendReq := notification.SendNotificationRequest{
		Variables:          e.buildNotificationVariables(instance, config),
		ChannelIDs:         parseNotificationChannelIDs(config),
		TemplateID:         parseNotificationTemplateID(config),
		WorkflowInstanceID: &instance.ID,
		IncidentID:         instance.IncidentID,
	}
	if subject, ok := config["subject"].(string); ok {
		sendReq.Subject = subject
	}
	if body, ok := config["body"].(string); ok {
		sendReq.Body = body
	}
	return sendReq
}

func (e *NodeExecutors) buildNotificationVariables(instance *model.FlowInstance, config map[string]interface{}) map[string]interface{} {
	variables := make(map[string]interface{})
	incident := e.getIncidentFromContext(instance)
	if incident != nil {
		variables["incident_id"] = incident.ID.String()
		variables["incident_title"] = incident.Title
		variables["incident_severity"] = incident.Severity
		variables["incident_status"] = incident.Status
	}
	if configVars, ok := config["variables"].(map[string]interface{}); ok {
		for key, value := range configVars {
			variables[key] = toString(value)
		}
	}
	return variables
}

func parseNotificationChannelIDs(config map[string]interface{}) []uuid.UUID {
	var channelIDs []uuid.UUID
	if idStrs, ok := config["channel_ids"].([]interface{}); ok {
		for _, raw := range idStrs {
			if id, err := uuid.Parse(fmt.Sprint(raw)); err == nil {
				channelIDs = append(channelIDs, id)
			}
		}
		return channelIDs
	}
	if idStr, ok := config["channel_id"].(string); ok {
		if id, err := uuid.Parse(idStr); err == nil {
			channelIDs = append(channelIDs, id)
		}
	}
	return channelIDs
}

func parseNotificationTemplateID(config map[string]interface{}) *uuid.UUID {
	tmplStr, ok := config["template_id"].(string)
	if !ok {
		return nil
	}
	id, err := uuid.Parse(tmplStr)
	if err != nil {
		return nil
	}
	return &id
}
