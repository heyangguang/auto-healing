package healing

import (
	"context"

	"github.com/company/auto-healing/internal/modules/automation/model"
	automationrepo "github.com/company/auto-healing/internal/modules/automation/repository"
	"github.com/google/uuid"
)

func nodeState(instance *model.FlowInstance, nodeID string) map[string]interface{} {
	if instance == nil || instance.NodeStates == nil {
		return nil
	}
	state, _ := instance.NodeStates[nodeID].(map[string]interface{})
	return state
}

func nodeStateStatus(instance *model.FlowInstance, nodeID string) string {
	state := nodeState(instance, nodeID)
	if state == nil {
		return ""
	}
	status, _ := state["status"].(string)
	return status
}

func hasNotificationSentLog(ctx context.Context, repo *automationrepo.FlowLogRepository, instanceID uuid.UUID, nodeID string) bool {
	logs, err := repo.GetByInstanceAndNode(ctx, instanceID, nodeID)
	if err != nil {
		return false
	}
	for _, entry := range logs {
		if entry.Message == "通知已发送" {
			return true
		}
	}
	return false
}
