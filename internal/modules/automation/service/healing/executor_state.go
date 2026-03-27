package healing

import (
	"context"
	"time"

	"github.com/company/auto-healing/internal/modules/automation/model"
	"github.com/company/auto-healing/internal/pkg/logger"
)

// setNodeState 更新节点状态到 instance.NodeStates 并持久化
func (e *FlowExecutor) setNodeState(ctx context.Context, instance *model.FlowInstance, nodeID string, status string, errorMsg string) error {
	if instance.NodeStates == nil {
		instance.NodeStates = make(model.JSON)
	}
	now := time.Now()
	state := map[string]interface{}{
		"status":     status,
		"updated_at": now.Format(time.RFC3339),
	}
	if status == "running" {
		state["started_at"] = now.Format(time.RFC3339)
	}
	if errorMsg != "" {
		state["error_message"] = errorMsg
	}
	if existing, ok := instance.NodeStates[nodeID].(map[string]interface{}); ok {
		for key, value := range state {
			existing[key] = value
		}
		if status != "running" {
			if startedStr, ok := existing["started_at"].(string); ok {
				if startedAt, err := time.Parse(time.RFC3339, startedStr); err == nil {
					existing["duration_ms"] = now.Sub(startedAt).Milliseconds()
				}
			}
		}
		instance.NodeStates[nodeID] = existing
	} else {
		if status != "running" {
			state["duration_ms"] = int64(0)
		}
		instance.NodeStates[nodeID] = state
	}
	return e.persistNodeStates(ctx, instance, "更新节点状态")
}

// complete 完成流程
func (e *FlowExecutor) complete(ctx context.Context, instance *model.FlowInstance) error {
	updated, err := e.instanceRepo.UpdateStatusWithIncidentSync(
		ctx,
		instance.ID,
		[]string{model.FlowInstanceStatusPending, model.FlowInstanceStatusRunning, model.FlowInstanceStatusWaitingApproval},
		model.FlowInstanceStatusCompleted,
		"",
		instanceIncidentSyncOptions(instance, "healed"),
	)
	if err != nil {
		return err
	}
	if !updated {
		logger.Exec("FLOW").Warn("[%s] 流程实例已进入终态，跳过完成状态覆盖", instance.ID.String()[:8])
		return nil
	}

	logger.Exec("FLOW").Info("[%s] 流程实例完成", instance.ID.String()[:8])
	e.eventBus.PublishFlowComplete(instance.ID, true, model.FlowInstanceStatusCompleted, "流程执行完成")
	return nil
}

// fail 失败流程
func (e *FlowExecutor) fail(ctx context.Context, instance *model.FlowInstance, errMsg string) {
	updated, err := e.instanceRepo.UpdateStatusWithIncidentSync(
		ctx,
		instance.ID,
		[]string{model.FlowInstanceStatusPending, model.FlowInstanceStatusRunning, model.FlowInstanceStatusWaitingApproval},
		model.FlowInstanceStatusFailed,
		errMsg,
		instanceIncidentSyncOptions(instance, "failed"),
	)
	if err != nil {
		logger.Exec("FLOW").Error("[%s] 更新失败状态异常: %v", instance.ID.String()[:8], err)
		return
	}
	if !updated {
		logger.Exec("FLOW").Warn("[%s] 流程实例已进入终态，跳过失败状态覆盖", instance.ID.String()[:8])
		return
	}

	logger.Exec("FLOW").Error("[%s] 流程实例失败: %s", instance.ID.String()[:8], errMsg)
	e.eventBus.PublishFlowComplete(instance.ID, false, model.FlowInstanceStatusFailed, errMsg)
}
