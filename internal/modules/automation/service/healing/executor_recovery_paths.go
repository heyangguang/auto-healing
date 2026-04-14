package healing

import (
	"context"
	"fmt"

	"github.com/company/auto-healing/internal/modules/automation/model"
)

func (e *FlowExecutor) recoverEndNode(ctx context.Context, instance *model.FlowInstance, currentNode *model.FlowNode) (recoveryResult, error) {
	if err := e.complete(ctx, instance); err != nil {
		return failureRecovery(model.FlowRecoveryActionCompleteInstance, "结束节点收口失败"), err
	}
	return successRecovery(model.FlowRecoveryActionCompleteInstance, "结束节点已补充完成收口", map[string]interface{}{
		"node_id": currentNode.ID,
	}), nil
}

func (e *FlowExecutor) recoverRegularNode(
	ctx context.Context,
	instance *model.FlowInstance,
	nodes []model.FlowNode,
	edges []model.FlowEdge,
	currentNode *model.FlowNode,
) (recoveryResult, error) {
	if isCompletedNodeState(instance, currentNode.ID) {
		return e.resumeFromHandle(ctx, instance, nodes, edges, currentNode, "default", model.FlowRecoveryActionResumeDefault, "普通节点已完成，继续 default 分支")
	}
	if err := e.executeNode(ctx, instance, nodes, edges, currentNode); err != nil {
		return failureRecovery(model.FlowRecoveryActionRerunCurrentNode, "重新执行当前节点失败"), err
	}
	return successRecovery(model.FlowRecoveryActionRerunCurrentNode, "已重新执行当前节点", map[string]interface{}{
		"node_id":   currentNode.ID,
		"node_type": currentNode.Type,
	}), nil
}

func (e *FlowExecutor) recoverNotificationNode(
	ctx context.Context,
	instance *model.FlowInstance,
	nodes []model.FlowNode,
	edges []model.FlowEdge,
	currentNode *model.FlowNode,
) (recoveryResult, error) {
	if hasNotificationSentLog(ctx, e.flowLogRepo, instance.ID, currentNode.ID) || isCompletedNodeState(instance, currentNode.ID) {
		return e.resumeFromHandle(ctx, instance, nodes, edges, currentNode, "default", model.FlowRecoveryActionResumeDefault, "通知节点已完成，继续 default 分支")
	}
	if err := e.executeNode(ctx, instance, nodes, edges, currentNode); err != nil {
		return failureRecovery(model.FlowRecoveryActionRerunCurrentNode, "通知节点重试失败"), err
	}
	return successRecovery(model.FlowRecoveryActionRerunCurrentNode, "已重新执行通知节点", map[string]interface{}{
		"node_id": currentNode.ID,
	}), nil
}

func (e *FlowExecutor) resumeFromHandle(
	ctx context.Context,
	instance *model.FlowInstance,
	nodes []model.FlowNode,
	edges []model.FlowEdge,
	currentNode *model.FlowNode,
	handle string,
	action string,
	reason string,
) (recoveryResult, error) {
	nextNode := e.findNextNodeByHandle(nodes, edges, currentNode.ID, handle)
	if nextNode == nil {
		return e.finishRecoveredBranch(ctx, instance, handle, action, reason)
	}
	if err := e.executeNode(ctx, instance, nodes, edges, nextNode); err != nil {
		return failureRecovery(action, reason), err
	}
	return successRecovery(action, reason, map[string]interface{}{
		"node_id":       currentNode.ID,
		"output_handle": handle,
		"next_node_id":  nextNode.ID,
	}), nil
}

func (e *FlowExecutor) finishRecoveredBranch(
	ctx context.Context,
	instance *model.FlowInstance,
	handle string,
	action string,
	reason string,
) (recoveryResult, error) {
	if handle == "failed" || handle == "partial" {
		err := fmt.Errorf("执行结果 %s 且缺少对应分支", handle)
		e.fail(ctx, instance, err.Error())
		return failureRecovery(model.FlowRecoveryActionFailInstance, reason), err
	}
	if err := e.complete(ctx, instance); err != nil {
		return failureRecovery(model.FlowRecoveryActionCompleteInstance, reason), err
	}
	return successRecovery(action, reason, map[string]interface{}{
		"output_handle": handle,
		"completed":     true,
	}), nil
}

func isCompletedNodeState(instance *model.FlowInstance, nodeID string) bool {
	status := nodeStateStatus(instance, nodeID)
	return status == "completed" || status == "approved" || status == "rejected"
}
