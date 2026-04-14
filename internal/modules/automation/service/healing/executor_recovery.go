package healing

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/company/auto-healing/internal/modules/automation/model"
	"github.com/google/uuid"
)

func (e *FlowExecutor) RecoverInstance(ctx context.Context, instanceID uuid.UUID, triggerSource string) (*model.FlowRecoveryAttempt, error) {
	if !startInstanceRecovery(instanceID) {
		return nil, ErrFlowInstanceRecoveryInProgress
	}
	defer finishInstanceRecovery(instanceID)

	instance, err := e.instanceRepo.GetByID(ctx, instanceID)
	if err != nil {
		return nil, err
	}

	nodes, edges, err := e.parseFlowSnapshot(instance)
	if err != nil {
		return nil, err
	}
	currentNode := findFlowNode(nodes, instance.CurrentNodeID)
	attempt, err := e.createRecoveryAttempt(ctx, instance, currentNode, triggerSource)
	if err != nil {
		return nil, err
	}
	if err := e.prepareInstanceForRecovery(ctx, instance); err != nil {
		result := failureRecovery(model.FlowRecoveryActionResumeDefault, "恢复前重置实例状态失败")
		if finishErr := e.finishRecoveryAttempt(ctx, attempt, result, err); finishErr != nil {
			return attempt, finishErr
		}
		return attempt, err
	}

	result, recoverErr := e.runRecoveryAttempt(ctx, attempt, instance, nodes, edges, currentNode)
	if finishErr := e.finishRecoveryAttempt(ctx, attempt, result, recoverErr); finishErr != nil {
		return attempt, finishErr
	}
	if recoverErr != nil {
		return attempt, recoverErr
	}
	return attempt, nil
}

func (e *FlowExecutor) prepareInstanceForRecovery(ctx context.Context, instance *model.FlowInstance) error {
	if instance.Status != model.FlowInstanceStatusFailed {
		return nil
	}
	updated, err := e.instanceRepo.UpdateStatusIfCurrent(ctx, instance.ID, []string{model.FlowInstanceStatusFailed}, model.FlowInstanceStatusRunning, "")
	if err != nil {
		return err
	}
	if !updated {
		return fmt.Errorf("流程实例状态不允许恢复")
	}
	instance.Status = model.FlowInstanceStatusRunning
	instance.ErrorMessage = ""
	return nil
}

func (e *FlowExecutor) ListRecoveryAttempts(ctx context.Context, instanceID uuid.UUID) ([]model.FlowRecoveryAttempt, error) {
	return e.recoveryRepo.ListByFlowInstanceID(ctx, instanceID)
}

func findFlowNode(nodes []model.FlowNode, nodeID string) *model.FlowNode {
	for i := range nodes {
		if nodes[i].ID == nodeID {
			return &nodes[i]
		}
	}
	return nil
}

type recoveryResult struct {
	Status       string
	Action       string
	DetectReason string
	Details      map[string]interface{}
}

func (e *FlowExecutor) runRecoveryAttempt(
	ctx context.Context,
	attempt *model.FlowRecoveryAttempt,
	instance *model.FlowInstance,
	nodes []model.FlowNode,
	edges []model.FlowEdge,
	currentNode *model.FlowNode,
) (recoveryResult, error) {
	if isNonRecoverableFlowStatus(instance.Status) {
		return skipRecovery("实例已处于终态", nil), nil
	}
	if currentNode == nil {
		return e.recoverWithoutCurrentNode(ctx, instance, nodes, edges)
	}
	switch currentNode.Type {
	case model.NodeTypeApproval:
		return e.recoverApprovalNode(ctx, instance, nodes, edges, currentNode)
	case model.NodeTypeExecution:
		return e.recoverExecutionNode(ctx, instance, nodes, edges, currentNode)
	case model.NodeTypeNotification:
		return e.recoverNotificationNode(ctx, instance, nodes, edges, currentNode)
	case model.NodeTypeEnd:
		return e.recoverEndNode(ctx, instance, currentNode)
	default:
		return e.recoverRegularNode(ctx, instance, nodes, edges, currentNode)
	}
}

func isTerminalFlowStatus(status string) bool {
	return status == model.FlowInstanceStatusCompleted ||
		status == model.FlowInstanceStatusFailed ||
		status == model.FlowInstanceStatusCancelled
}

func isNonRecoverableFlowStatus(status string) bool {
	return status == model.FlowInstanceStatusCompleted || status == model.FlowInstanceStatusCancelled
}

func (e *FlowExecutor) recoverWithoutCurrentNode(
	ctx context.Context,
	instance *model.FlowInstance,
	nodes []model.FlowNode,
	edges []model.FlowEdge,
) (recoveryResult, error) {
	if instance.Status != model.FlowInstanceStatusPending {
		return skipRecovery("缺少当前节点信息，无法安全恢复", map[string]interface{}{
			"status": instance.Status,
		}), nil
	}
	startNode := e.findStartNode(nodes)
	if startNode == nil {
		return failureRecovery(model.FlowRecoveryActionFailInstance, "找不到起始节点"), errors.New("找不到起始节点")
	}
	if err := e.executeNode(ctx, instance, nodes, edges, startNode); err != nil {
		return failureRecovery(model.FlowRecoveryActionResumeFromStart, "从起始节点恢复失败"), err
	}
	return successRecovery(model.FlowRecoveryActionResumeFromStart, "从起始节点恢复执行", map[string]interface{}{
		"node_id": startNode.ID,
	}), nil
}

func skipRecovery(reason string, details map[string]interface{}) recoveryResult {
	return recoveryResult{
		Status:       model.FlowRecoveryStatusSkipped,
		Action:       model.FlowRecoveryActionWaitExternalRun,
		DetectReason: reason,
		Details:      ensureRecoveryDetails(details),
	}
}

func successRecovery(action, reason string, details map[string]interface{}) recoveryResult {
	return recoveryResult{
		Status:       model.FlowRecoveryStatusSuccess,
		Action:       action,
		DetectReason: reason,
		Details:      ensureRecoveryDetails(details),
	}
}

func failureRecovery(action, reason string) recoveryResult {
	return recoveryResult{
		Status:       model.FlowRecoveryStatusFailed,
		Action:       action,
		DetectReason: reason,
		Details:      model.JSON{},
	}
}

func ensureRecoveryDetails(details map[string]interface{}) map[string]interface{} {
	if details == nil {
		return map[string]interface{}{}
	}
	return details
}

func (e *FlowExecutor) createRecoveryAttempt(
	ctx context.Context,
	instance *model.FlowInstance,
	currentNode *model.FlowNode,
	triggerSource string,
) (*model.FlowRecoveryAttempt, error) {
	attempt := &model.FlowRecoveryAttempt{
		ID:             uuid.New(),
		FlowInstanceID: instance.ID,
		TriggerSource:  triggerSource,
		Status:         model.FlowRecoveryStatusStarted,
		Details:        model.JSON{},
		StartedAt:      time.Now().UTC(),
	}
	if currentNode != nil {
		attempt.CurrentNodeID = currentNode.ID
		attempt.CurrentNodeType = currentNode.Type
	}
	if err := e.recoveryRepo.Create(ctx, attempt); err != nil {
		return nil, fmt.Errorf("创建恢复尝试记录失败: %w", err)
	}
	return attempt, nil
}

func (e *FlowExecutor) finishRecoveryAttempt(
	ctx context.Context,
	attempt *model.FlowRecoveryAttempt,
	result recoveryResult,
	recoverErr error,
) error {
	finishedAt := time.Now().UTC()
	attempt.Status = result.Status
	attempt.RecoveryAction = result.Action
	attempt.DetectReason = result.DetectReason
	attempt.Details = result.Details
	attempt.FinishedAt = &finishedAt
	if recoverErr != nil {
		attempt.Status = model.FlowRecoveryStatusFailed
		attempt.ErrorMessage = recoverErr.Error()
	}
	return e.recoveryRepo.Update(ctx, attempt)
}
