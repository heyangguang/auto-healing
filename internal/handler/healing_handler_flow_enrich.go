package handler

import (
	"context"

	"github.com/company/auto-healing/internal/model"
	"github.com/google/uuid"
)

// enrichFlowNodes 填充 flow 中 notification 节点的 channel_names 和 template_name
func (h *HealingHandler) enrichFlowNodes(ctx context.Context, flows []model.HealingFlow) {
	channelIDs, templateIDs := collectNotificationNodeReferences(flows)
	if len(channelIDs) == 0 && len(templateIDs) == 0 {
		return
	}

	channelNameMap := h.loadNotificationChannelNames(ctx, channelIDs)
	templateNameMap := h.loadNotificationTemplateNames(ctx, templateIDs)
	applyNotificationNodeNames(flows, channelNameMap, templateNameMap)
}

func collectNotificationNodeReferences(flows []model.HealingFlow) (map[uuid.UUID]bool, map[uuid.UUID]bool) {
	channelIDs := make(map[uuid.UUID]bool)
	templateIDs := make(map[uuid.UUID]bool)
	for _, flow := range flows {
		for _, item := range flow.Nodes {
			node, ok := item.(map[string]interface{})
			if !ok || node["type"] != "notification" {
				continue
			}
			config, _ := node["config"].(map[string]interface{})
			for _, channelID := range notificationChannelIDs(config) {
				channelIDs[channelID] = true
			}
			if templateID, ok := notificationTemplateID(config); ok {
				templateIDs[templateID] = true
			}
		}
	}
	return channelIDs, templateIDs
}

func notificationChannelIDs(config map[string]interface{}) []uuid.UUID {
	rawIDs, _ := config["channel_ids"].([]interface{})
	ids := make([]uuid.UUID, 0, len(rawIDs))
	for _, rawID := range rawIDs {
		if text, ok := rawID.(string); ok {
			if parsed, err := uuid.Parse(text); err == nil {
				ids = append(ids, parsed)
			}
		}
	}
	return ids
}

func notificationTemplateID(config map[string]interface{}) (uuid.UUID, bool) {
	rawTemplateID, _ := config["template_id"].(string)
	if rawTemplateID == "" {
		return uuid.Nil, false
	}
	templateID, err := uuid.Parse(rawTemplateID)
	if err != nil {
		return uuid.Nil, false
	}
	return templateID, true
}

func (h *HealingHandler) loadNotificationChannelNames(ctx context.Context, ids map[uuid.UUID]bool) map[string]string {
	names := make(map[string]string)
	if len(ids) == 0 {
		return names
	}
	items := make([]uuid.UUID, 0, len(ids))
	for id := range ids {
		items = append(items, id)
	}
	if channels, err := h.notifRepo.GetChannelsByIDs(ctx, items); err == nil {
		for _, channel := range channels {
			names[channel.ID.String()] = channel.Name
		}
	}
	return names
}

func (h *HealingHandler) loadNotificationTemplateNames(ctx context.Context, ids map[uuid.UUID]bool) map[string]string {
	names := make(map[string]string)
	if len(ids) == 0 {
		return names
	}
	items := make([]uuid.UUID, 0, len(ids))
	for id := range ids {
		items = append(items, id)
	}
	if templates, err := h.notifRepo.GetTemplatesByIDs(ctx, items); err == nil {
		for _, template := range templates {
			names[template.ID.String()] = template.Name
		}
	}
	return names
}

func applyNotificationNodeNames(flows []model.HealingFlow, channelNameMap, templateNameMap map[string]string) {
	for _, flow := range flows {
		for _, item := range flow.Nodes {
			node, ok := item.(map[string]interface{})
			if !ok || node["type"] != "notification" {
				continue
			}
			config, _ := node["config"].(map[string]interface{})
			if config == nil {
				continue
			}
			applyNotificationChannelNames(config, channelNameMap)
			applyNotificationTemplateName(config, templateNameMap)
		}
	}
}

func applyNotificationChannelNames(config map[string]interface{}, channelNameMap map[string]string) {
	rawIDs, _ := config["channel_ids"].([]interface{})
	channelNames := make(map[string]string)
	for _, rawID := range rawIDs {
		if text, ok := rawID.(string); ok {
			if name, exists := channelNameMap[text]; exists {
				channelNames[text] = name
			}
		}
	}
	if len(channelNames) > 0 {
		config["channel_names"] = channelNames
	}
}

func applyNotificationTemplateName(config map[string]interface{}, templateNameMap map[string]string) {
	templateID, _ := config["template_id"].(string)
	if name, exists := templateNameMap[templateID]; exists {
		config["template_name"] = name
	}
}
