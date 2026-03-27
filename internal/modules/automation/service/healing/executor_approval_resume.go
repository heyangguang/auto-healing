package healing

import (
	"context"
	"fmt"

	"github.com/company/auto-healing/internal/modules/automation/model"
	"github.com/company/auto-healing/internal/pkg/logger"
	"github.com/google/uuid"
)

// ResumeAfterApproval 审批后恢复执行
func (e *FlowExecutor) ResumeAfterApproval(ctx context.Context, instanceID uuid.UUID, approved bool) (err error) {
	instance, err := e.instanceRepo.GetByID(ctx, instanceID)
	if err != nil {
		return err
	}
	shouldFailOnError := true
	defer func() {
		if err == nil || !shouldFailOnError {
			return
		}
		e.handleApprovalResumeError(detachContext(ctx), instance, err)
	}()

	nodes, edges, err := e.parseFlowSnapshot(instance)
	if err != nil {
		return err
	}
	currentNode := currentApprovalNode(nodes, instance.CurrentNodeID)
	if currentNode == nil {
		shouldFailOnError = false
		return e.failApprovalResume(ctx, instance, instanceID, "找不到当前节点")
	}
	if currentNode.Type != model.NodeTypeApproval {
		return fmt.Errorf("当前节点不是审批节点: %s", currentNode.Type)
	}
	if err := e.markApprovalResumeRunning(ctx, instanceID); err != nil {
		return err
	}
	runCtx, done := startApprovalResumeContext(ctx, instance.ID)
	defer done()
	instance.Status = model.FlowInstanceStatusRunning
	outputHandle := approvalOutputHandle(approved)
	if err := e.recordApprovalResume(runCtx, instance, currentNode, approved); err != nil {
		return err
	}
	nextNode := e.findNextNodeByHandle(nodes, edges, currentNode.ID, outputHandle)
	if nextNode == nil {
		if !approved {
			shouldFailOnError = false
			return e.failApprovalResume(ctx, instance, instanceID, "审批被拒绝")
		}
		return e.complete(runCtx, instance)
	}
	return e.executeNode(runCtx, instance, nodes, edges, nextNode)
}

func (e *FlowExecutor) markApprovalResumeRunning(ctx context.Context, instanceID uuid.UUID) error {
	updated, err := e.instanceRepo.UpdateStatusIfCurrent(ctx, instanceID, []string{model.FlowInstanceStatusWaitingApproval}, model.FlowInstanceStatusRunning, "")
	if err != nil {
		return err
	}
	if !updated {
		return fmt.Errorf("流程实例不处于待审批状态")
	}
	return nil
}

func startApprovalResumeContext(ctx context.Context, instanceID uuid.UUID) (context.Context, func()) {
	runCtx, cancel := context.WithCancel(ctx)
	runningFlowCancels.Store(instanceID, cancel)
	return runCtx, func() {
		runningFlowCancels.Delete(instanceID)
		cancel()
	}
}

func (e *FlowExecutor) failApprovalResume(ctx context.Context, instance *model.FlowInstance, instanceID uuid.UUID, message string) error {
	updated, err := e.instanceRepo.UpdateStatusWithIncidentSync(
		ctx,
		instanceID,
		[]string{model.FlowInstanceStatusWaitingApproval, model.FlowInstanceStatusRunning},
		model.FlowInstanceStatusFailed,
		message,
		instanceIncidentSyncOptions(instance, "failed"),
	)
	if err != nil {
		return err
	}
	if !updated {
		return nil
	}
	return nil
}

func (e *FlowExecutor) handleApprovalResumeError(ctx context.Context, instance *model.FlowInstance, resumeErr error) {
	status, err := e.flowInstanceStatus(ctx, instance.ID)
	if err != nil {
		logger.Exec("FLOW").Error("[%s] 查询审批恢复状态失败: %v", instance.ID.String()[:8], err)
		return
	}
	if status != model.FlowInstanceStatusWaitingApproval && status != model.FlowInstanceStatusRunning {
		return
	}
	if err := e.failApprovalResume(ctx, instance, instance.ID, "审批恢复失败: "+resumeErr.Error()); err != nil {
		logger.Exec("FLOW").Error("[%s] 审批恢复失败后更新状态异常: %v", instance.ID.String()[:8], err)
	}
}

func currentApprovalNode(nodes []model.FlowNode, currentNodeID string) *model.FlowNode {
	for i := range nodes {
		if nodes[i].ID == currentNodeID {
			return &nodes[i]
		}
	}
	return nil
}

func approvalOutputHandle(approved bool) string {
	if approved {
		return "approved"
	}
	return "rejected"
}

func (e *FlowExecutor) recordApprovalResume(ctx context.Context, instance *model.FlowInstance, currentNode *model.FlowNode, approved bool) error {
	outputHandle := approvalOutputHandle(approved)
	if approved {
		logger.Exec("FLOW").Info("[%s] 审批通过，走 approved 分支", instance.ID.String()[:8])
		return e.setNodeState(ctx, instance, currentNode.ID, "approved", "")
	}
	logger.Exec("FLOW").Info("[%s] 审批拒绝，走 %s 分支", instance.ID.String()[:8], outputHandle)
	return e.setNodeState(ctx, instance, currentNode.ID, "rejected", "审批被拒绝")
}
