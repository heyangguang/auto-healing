package httpapi

import (
	"testing"

	"github.com/company/auto-healing/internal/modules/automation/model"
	"github.com/google/uuid"
)

func TestApplyNotificationNodeNamesSupportsStructuredConfigs(t *testing.T) {
	channelID := uuid.New()
	templateID := uuid.New()
	flow := model.HealingFlow{
		Nodes: model.JSONArray{
			map[string]interface{}{
				"id":   "notify_result",
				"type": "notification",
				"config": map[string]interface{}{
					"notification_configs": []interface{}{
						map[string]interface{}{
							"channel_id":  channelID.String(),
							"template_id": templateID.String(),
						},
					},
				},
			},
		},
	}

	channelIDs, templateIDs := collectNotificationNodeReferences([]model.HealingFlow{flow})
	if !channelIDs[channelID] {
		t.Fatalf("structured channel_id was not collected")
	}
	if !templateIDs[templateID] {
		t.Fatalf("structured template_id was not collected")
	}

	applyNotificationNodeNames(
		[]model.HealingFlow{flow},
		map[string]string{channelID.String(): "Demo Enterprise WeChat Bot"},
		map[string]string{templateID.String(): "Demo Notify Result"},
	)

	node := flow.Nodes[0].(map[string]interface{})
	config := node["config"].(map[string]interface{})
	channelNames, ok := config["channel_names"].(map[string]string)
	if !ok {
		t.Fatalf("channel_names was not populated")
	}
	if channelNames[channelID.String()] != "Demo Enterprise WeChat Bot" {
		t.Fatalf("channel_names[%s] = %q", channelID, channelNames[channelID.String()])
	}
	if config["template_name"] != "Demo Notify Result" {
		t.Fatalf("template_name = %v, want Demo Notify Result", config["template_name"])
	}
}
