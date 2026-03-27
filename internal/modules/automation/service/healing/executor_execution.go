package healing

import (
	"context"
	"fmt"
	"time"

	"github.com/company/auto-healing/internal/modules/automation/model"
	"github.com/company/auto-healing/internal/pkg/logger"
	"github.com/google/uuid"
)

func (e *FlowExecutor) executeExecution(ctx context.Context, instance *model.FlowInstance, node *model.FlowNode) error {
	logger.Exec("ANSIBLE").Info("[%s] 执行任务节点", shortID(instance))
	startTime := time.Now()

	prepared, err := e.prepareExecutionNode(ctx, instance, node)
	if err != nil {
		return err
	}

	if err := e.markExecutionNodeRunning(ctx, instance, node.ID, prepared, startTime); err != nil {
		return err
	}
	e.logExecutionNodeStart(ctx, instance, node, prepared)

	outcome := e.runPreparedExecution(ctx, instance, prepared)
	if err := e.recordExecutionOutcome(ctx, instance, node, prepared, outcome, startTime); err != nil {
		return err
	}

	logger.Exec("ANSIBLE").Info("[%s] 执行完成，状态: %s", shortID(instance), outcome.Status)
	if outcome.Status == "failed" || outcome.Status == "partial" {
		return fmt.Errorf("%s", outcome.Message)
	}
	return nil
}

// executeExecutionWithBranch 执行任务节点并根据结果选择分支
// 分支: success（全部成功）、partial（部分成功）、failed（全部失败或其他错误）
func (e *FlowExecutor) executeExecutionWithBranch(ctx context.Context, instance *model.FlowInstance, nodes []model.FlowNode, edges []model.FlowEdge, node *model.FlowNode) error {
	logger.Exec("ANSIBLE").Info("[%s] 执行任务节点（分支模式）", shortID(instance))
	if err := e.setNodeState(ctx, instance, node.ID, "running", ""); err != nil {
		return err
	}
	err := e.executeExecution(ctx, instance, node)
	outputHandle := resolveExecutionOutputHandle(instance, err)
	logger.Exec("ANSIBLE").Info("[%s] 执行节点结果分支: %s", shortID(instance), outputHandle)
	nodeStateStatus, nodeErrMsg := executionBranchState(outputHandle, err)
	if stateErr := e.setNodeState(ctx, instance, node.ID, nodeStateStatus, nodeErrMsg); stateErr != nil {
		return stateErr
	}
	if err := e.persistExecutionBranchState(ctx, instance, node.ID, outputHandle); err != nil {
		return err
	}
	e.publishExecutionBranchEvent(instance, node, outputHandle)
	return e.continueExecutionBranch(ctx, instance, nodes, edges, node, outputHandle, err)
}

func resolveExecutionOutputHandle(instance *model.FlowInstance, err error) string {
	status := executionResultStatus(instance)
	if status != "" {
		return mapExecutionOutputHandle(status)
	}
	if err != nil {
		return "failed"
	}
	return "success"
}

func executionResultStatus(instance *model.FlowInstance) string {
	if instance.Context == nil {
		return ""
	}
	execResult, ok := instance.Context["execution_result"].(map[string]interface{})
	if !ok {
		return ""
	}
	if runInfo, ok := execResult["run"].(map[string]interface{}); ok {
		if runStatus, ok := runInfo["status"].(string); ok {
			return runStatus
		}
	}
	if status, ok := execResult["status"].(string); ok {
		return status
	}
	return ""
}

func mapExecutionOutputHandle(status string) string {
	switch status {
	case "completed", "success":
		return "success"
	case "partial":
		return "partial"
	default:
		return "failed"
	}
}

func executionBranchState(outputHandle string, err error) (string, string) {
	switch outputHandle {
	case "failed":
		if err != nil {
			return "failed", err.Error()
		}
		return "failed", ""
	case "partial":
		return "partial", ""
	default:
		return "completed", ""
	}
}

func (e *FlowExecutor) persistExecutionBranchState(ctx context.Context, instance *model.FlowInstance, nodeID string, outputHandle string) error {
	if instance.NodeStates == nil {
		instance.NodeStates = make(model.JSON)
	}
	state := executionBranchDetails(instance, nodeID, outputHandle)
	instance.NodeStates[nodeID] = state
	if err := e.instanceRepo.UpdateNodeStates(ctx, instance.ID, instance.NodeStates); err != nil {
		return fmt.Errorf("更新执行节点结果失败: %w", err)
	}
	return nil
}

func executionBranchDetails(instance *model.FlowInstance, nodeID string, outputHandle string) map[string]interface{} {
	state, _ := instance.NodeStates[nodeID].(map[string]interface{})
	if state == nil {
		state = make(map[string]interface{})
	}
	if instance.Context == nil {
		state["output_handle"] = outputHandle
		return state
	}
	if execResult, ok := instance.Context["execution_result"].(map[string]interface{}); ok {
		if run, ok := execResult["run"]; ok {
			state["run"] = run
		}
		if taskID, ok := execResult["task_id"]; ok {
			state["task_id"] = taskID
		}
		if targetHosts, ok := execResult["target_hosts"]; ok {
			state["target_hosts"] = targetHosts
		}
	}
	state["output_handle"] = outputHandle
	return state
}

func (e *FlowExecutor) publishExecutionBranchEvent(instance *model.FlowInstance, node *model.FlowNode, outputHandle string) {
	nodeStatus := model.NodeStatusSuccess
	switch outputHandle {
	case "failed":
		nodeStatus = model.NodeStatusFailed
	case "partial":
		nodeStatus = model.NodeStatusPartial
	}
	e.eventBus.PublishNodeComplete(instance.ID, node.ID, node.Type, nodeStatus, nil, nil, nil, outputHandle)
}

func (e *FlowExecutor) continueExecutionBranch(ctx context.Context, instance *model.FlowInstance, nodes []model.FlowNode, edges []model.FlowEdge, node *model.FlowNode, outputHandle string, execErr error) error {
	nextNode := e.findNextNodeByHandle(nodes, edges, node.ID, outputHandle)
	if nextNode != nil {
		return e.executeNode(ctx, instance, nodes, edges, nextNode)
	}
	if outputHandle == "success" {
		return e.complete(ctx, instance)
	}
	return e.failExecutionBranch(ctx, instance, outputHandle, execErr)
}

func (e *FlowExecutor) failExecutionBranch(ctx context.Context, instance *model.FlowInstance, outputHandle string, execErr error) error {
	errMsg := fmt.Sprintf("执行结果 %s 但无 %s 分支", outputHandle, outputHandle)
	if execErr != nil {
		errMsg += ": " + execErr.Error()
	}
	e.fail(ctx, instance, errMsg)
	return fmt.Errorf("%s", errMsg)
}

// waitForRunCompletion 等待执行完成
func (e *FlowExecutor) waitForRunCompletion(ctx context.Context, instanceID, runID uuid.UUID, timeout time.Duration) (*model.ExecutionRun, error) {
	deadline := time.Now().Add(timeout)
	pollInterval := 2 * time.Second

	for {
		if time.Now().After(deadline) {
			return nil, fmt.Errorf("等待执行完成超时")
		}
		if inst, err := e.instanceRepo.GetByID(ctx, instanceID); err == nil && inst.Status == model.FlowInstanceStatusCancelled {
			cancelCtx := detachContext(ctx)
			_ = e.executionSvc.CancelRun(cancelCtx, runID)
			return nil, fmt.Errorf("流程实例已取消")
		}

		run, err := e.executionRepo.GetRunByID(ctx, runID)
		if err != nil {
			return nil, err
		}

		if run.Status == "success" || run.Status == "failed" || run.Status == "cancelled" || run.Status == "partial" {
			return run, nil
		}

		select {
		case <-ctx.Done():
			cancelCtx := detachContext(ctx)
			_ = e.executionSvc.CancelRun(cancelCtx, runID)
			return nil, ctx.Err()
		case <-time.After(pollInterval):
		}
	}
}

// executeNotification 执行通知节点
// 复用 notification 服务发送通知
