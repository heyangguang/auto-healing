package healing

import (
	"context"
	"time"

	"github.com/company/auto-healing/internal/modules/automation/model"
	"github.com/google/uuid"
)

func (e *FlowExecutor) recoverExecutionNode(
	ctx context.Context,
	instance *model.FlowInstance,
	nodes []model.FlowNode,
	edges []model.FlowEdge,
	currentNode *model.FlowNode,
) (recoveryResult, error) {
	handle, details, ok, err := e.resolveExecutionRecoveryHandle(ctx, instance, currentNode.ID)
	if err != nil {
		return failureRecovery(model.FlowRecoveryActionResumeExecution, "解析执行节点恢复状态失败"), err
	}
	if ok {
		return e.resumeFromHandle(ctx, instance, nodes, edges, currentNode, handle, model.FlowRecoveryActionResumeExecution, "执行节点已完成，继续结果分支")
	}
	if hasExecutionRunID(instance, currentNode.ID) {
		return recoveryResult{
			Status:       model.FlowRecoveryStatusSkipped,
			Action:       model.FlowRecoveryActionWaitExternalRun,
			DetectReason: "执行记录仍处于进行中，暂不重复触发",
			Details:      details,
		}, nil
	}
	if err := e.executeExecutionWithBranch(ctx, instance, nodes, edges, currentNode); err != nil {
		return failureRecovery(model.FlowRecoveryActionRerunCurrentNode, "重新执行任务节点失败"), err
	}
	return successRecovery(model.FlowRecoveryActionRerunCurrentNode, "已重新执行任务节点", details), nil
}

func (e *FlowExecutor) resolveExecutionRecoveryHandle(
	ctx context.Context,
	instance *model.FlowInstance,
	nodeID string,
) (string, map[string]interface{}, bool, error) {
	if outputHandle := executionNodeOutputHandle(instance, nodeID); outputHandle != "" {
		return outputHandle, map[string]interface{}{"output_handle": outputHandle}, true, nil
	}
	runID := executionRunID(instance, nodeID)
	if runID == nil {
		return "", map[string]interface{}{}, false, nil
	}
	run, err := e.executionRepo.GetRunByID(ctx, *runID)
	if err != nil {
		return "", map[string]interface{}{"run_id": runID.String()}, false, err
	}
	if !isTerminalRunStatus(run.Status) {
		return "", map[string]interface{}{
			"run_id":     run.ID.String(),
			"run_status": run.Status,
		}, false, nil
	}
	if err := e.persistRecoveredExecutionRun(ctx, instance, nodeID, run); err != nil {
		return "", map[string]interface{}{"run_id": run.ID.String()}, false, err
	}
	outputHandle := mapExecutionOutputHandle(run.Status)
	return outputHandle, map[string]interface{}{
		"run_id":        run.ID.String(),
		"run_status":    run.Status,
		"output_handle": outputHandle,
	}, true, nil
}

func executionNodeOutputHandle(instance *model.FlowInstance, nodeID string) string {
	state := nodeState(instance, nodeID)
	if state != nil {
		if outputHandle, ok := state["output_handle"].(string); ok && outputHandle != "" {
			return outputHandle
		}
	}
	return resolveExecutionOutputHandle(instance, nil)
}

func executionRunID(instance *model.FlowInstance, nodeID string) *uuid.UUID {
	state := nodeState(instance, nodeID)
	if state != nil {
		if id := extractRunID(state["run"]); id != nil {
			return id
		}
	}
	if instance != nil && instance.Context != nil {
		if execResult, ok := instance.Context["execution_result"].(map[string]interface{}); ok {
			if id := extractRunID(execResult["run"]); id != nil {
				return id
			}
		}
	}
	return nil
}

func extractRunID(raw interface{}) *uuid.UUID {
	run, ok := raw.(map[string]interface{})
	if !ok {
		return nil
	}
	runID, ok := run["run_id"].(string)
	if !ok || runID == "" {
		return nil
	}
	id, err := uuid.Parse(runID)
	if err != nil {
		return nil
	}
	return &id
}

func hasExecutionRunID(instance *model.FlowInstance, nodeID string) bool {
	return executionRunID(instance, nodeID) != nil
}

func isTerminalRunStatus(status string) bool {
	return status == "success" || status == "failed" || status == "partial" || status == "cancelled"
}

func (e *FlowExecutor) persistRecoveredExecutionRun(ctx context.Context, instance *model.FlowInstance, nodeID string, run *model.ExecutionRun) error {
	executionResult := recoveredExecutionResult(run, nodeID)
	if instance.Context == nil {
		instance.Context = make(model.JSON)
	}
	instance.Context["execution_result"] = executionResult
	instance.NodeStates[nodeID] = recoveredExecutionState(executionResult)
	if err := e.persistNodeStates(ctx, instance, "持久化恢复后的执行节点状态"); err != nil {
		return err
	}
	return e.persistInstance(ctx, instance, "持久化恢复后的执行上下文")
}

func recoveredExecutionResult(run *model.ExecutionRun, nodeID string) map[string]interface{} {
	result := map[string]interface{}{
		"status":       run.Status,
		"message":      recoveredExecutionMessage(run),
		"task_id":      run.TaskID.String(),
		"target_hosts": run.RuntimeTargetHosts,
		"run": map[string]interface{}{
			"run_id":    run.ID.String(),
			"status":    run.Status,
			"exit_code": run.ExitCode,
			"stats":     run.Stats,
		},
	}
	if run.StartedAt != nil {
		result["started_at"] = run.StartedAt.Format(time.RFC3339)
	}
	if run.CompletedAt != nil {
		result["finished_at"] = run.CompletedAt.Format(time.RFC3339)
	}
	if run.StartedAt != nil && run.CompletedAt != nil {
		result["duration_ms"] = run.CompletedAt.Sub(*run.StartedAt).Milliseconds()
	}
	return result
}

func recoveredExecutionMessage(run *model.ExecutionRun) string {
	switch run.Status {
	case "partial":
		return "任务部分成功（恢复自执行记录）"
	case "failed", "cancelled":
		return "任务执行失败（恢复自执行记录）"
	default:
		return "执行成功（恢复自执行记录）"
	}
}

func recoveredExecutionState(result map[string]interface{}) map[string]interface{} {
	status := result["status"].(string)
	nodeStatus, _ := executionBranchState(mapExecutionOutputHandle(status), nil)
	state := map[string]interface{}{
		"status":        nodeStatus,
		"message":       result["message"],
		"task_id":       result["task_id"],
		"target_hosts":  result["target_hosts"],
		"run":           result["run"],
		"output_handle": mapExecutionOutputHandle(status),
	}
	if startedAt, ok := result["started_at"]; ok {
		state["started_at"] = startedAt
	}
	if finishedAt, ok := result["finished_at"]; ok {
		state["finished_at"] = finishedAt
	}
	if duration, ok := result["duration_ms"]; ok {
		state["duration_ms"] = duration
	}
	return state
}
