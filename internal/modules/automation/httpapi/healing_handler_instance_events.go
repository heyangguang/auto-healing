package httpapi

import (
	"context"

	"github.com/company/auto-healing/internal/modules/automation/model"
	healing "github.com/company/auto-healing/internal/modules/automation/service/healing"
)

func writeInitialInstanceEvents(sseWriter *SSEWriter, instance *model.FlowInstance) bool {
	_ = sseWriter.WriteEvent("connected", map[string]interface{}{
		"instance_id": instance.ID.String(),
		"status":      instance.Status,
	})
	if !isTerminalFlowStatus(instance.Status) {
		return false
	}
	_ = sseWriter.WriteEvent(string(healing.EventFlowComplete), map[string]interface{}{
		"success": instance.Status == model.FlowInstanceStatusCompleted,
		"status":  instance.Status,
		"message": terminalFlowMessage(instance),
	})
	return true
}

func streamInstanceEvents(ctx context.Context, sseWriter *SSEWriter, eventCh <-chan healing.Event) {
	for {
		select {
		case <-ctx.Done():
			return
		case event, ok := <-eventCh:
			if !ok {
				return
			}
			_ = sseWriter.WriteEvent(string(event.Type), event.Data)
			if event.Type == healing.EventFlowComplete {
				return
			}
		}
	}
}

func isTerminalFlowStatus(status string) bool {
	return status == model.FlowInstanceStatusCompleted ||
		status == model.FlowInstanceStatusFailed ||
		status == model.FlowInstanceStatusCancelled
}

func terminalFlowMessage(instance *model.FlowInstance) string {
	if instance.ErrorMessage != "" {
		return instance.ErrorMessage
	}
	return "流程已结束"
}
