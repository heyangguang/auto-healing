package healing

import (
	"context"

	"github.com/company/auto-healing/internal/modules/automation/model"
	automationrepo "github.com/company/auto-healing/internal/modules/automation/repository"
)

func (e *FlowExecutor) recoverApprovalNode(
	ctx context.Context,
	instance *model.FlowInstance,
	nodes []model.FlowNode,
	edges []model.FlowEdge,
	currentNode *model.FlowNode,
) (recoveryResult, error) {
	task, err := e.approvalRepo.GetByFlowInstanceAndNode(ctx, instance.ID, currentNode.ID)
	if err == automationrepo.ErrApprovalTaskNotFound {
		if err := e.executeApproval(ctx, instance, currentNode); err != nil {
			return failureRecovery(model.FlowRecoveryActionRerunCurrentNode, "审批节点重建失败"), err
		}
		return successRecovery(model.FlowRecoveryActionRerunCurrentNode, "已重新创建审批节点", map[string]interface{}{
			"node_id": currentNode.ID,
		}), nil
	}
	if err != nil {
		return failureRecovery(model.FlowRecoveryActionResumeApproval, "读取审批任务失败"), err
	}
	switch task.Status {
	case model.ApprovalTaskStatusPending:
		return recoveryResult{
			Status:       model.FlowRecoveryStatusSkipped,
			Action:       model.FlowRecoveryActionWaitApproval,
			DetectReason: "审批任务仍在等待人工处理",
			Details: map[string]interface{}{
				"approval_task_id": task.ID.String(),
				"approval_status":  task.Status,
			},
		}, nil
	case model.ApprovalTaskStatusApproved:
		return e.resumeResolvedApproval(ctx, instance, nodes, edges, currentNode, true, task)
	case model.ApprovalTaskStatusRejected:
		return e.resumeResolvedApproval(ctx, instance, nodes, edges, currentNode, false, task)
	default:
		return skipRecovery("审批任务处于不可恢复状态", map[string]interface{}{
			"approval_task_id": task.ID.String(),
			"approval_status":  task.Status,
		}), nil
	}
}

func (e *FlowExecutor) resumeResolvedApproval(
	ctx context.Context,
	instance *model.FlowInstance,
	nodes []model.FlowNode,
	edges []model.FlowEdge,
	currentNode *model.FlowNode,
	approved bool,
	task *model.ApprovalTask,
) (recoveryResult, error) {
	if instance.Status == model.FlowInstanceStatusWaitingApproval {
		if err := e.ResumeAfterApproval(ctx, instance.ID, approved); err != nil {
			return failureRecovery(model.FlowRecoveryActionResumeApproval, "审批恢复失败"), err
		}
		return successRecovery(model.FlowRecoveryActionResumeApproval, "已根据审批结果恢复流程", map[string]interface{}{
			"approval_task_id": task.ID.String(),
			"approved":         approved,
		}), nil
	}
	handle := approvalOutputHandle(approved)
	if err := e.recordApprovalResume(ctx, instance, currentNode, approved); err != nil {
		return failureRecovery(model.FlowRecoveryActionResumeApproval, "审批结果回填失败"), err
	}
	return e.resumeFromHandle(ctx, instance, nodes, edges, currentNode, handle, model.FlowRecoveryActionResumeApproval, "审批已处理，继续后续分支")
}
