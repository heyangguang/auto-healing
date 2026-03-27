package healing

import (
	"context"
	"fmt"

	"github.com/company/auto-healing/internal/model"
)

type dryRunEmit func(eventType string, data map[string]interface{})

func dryRunEmitter(callback NodeCallback) dryRunEmit {
	return func(eventType string, data map[string]interface{}) {
		if callback != nil {
			callback(eventType, data)
		}
	}
}

func (e *DryRunExecutor) prepareDryRunStart(flow *model.HealingFlow, mockIncident *MockIncident, fromNodeID string, initialContext map[string]interface{}, mockApprovals map[string]string, nodes []model.FlowNode, result *DryRunResult, emit dryRunEmit) (*model.FlowNode, map[string]interface{}, bool) {
	if len(nodes) == 0 {
		result.Success = false
		result.Message = "流程没有定义节点"
		emit(model.SSEEventFlowComplete, map[string]interface{}{"success": false, "message": result.Message})
		return nil, nil, false
	}

	flowContext := e.initialDryRunContext(mockIncident, initialContext, mockApprovals)
	startNode := e.resolveDryRunStartNode(nodes, fromNodeID, result, emit)
	if startNode == nil {
		return nil, nil, false
	}
	return startNode, flowContext, true
}

func (e *DryRunExecutor) initialDryRunContext(mockIncident *MockIncident, initialContext map[string]interface{}, mockApprovals map[string]string) map[string]interface{} {
	flowContext := map[string]interface{}{
		"incident": incidentToMap(mockIncident.ToIncident()),
	}
	if mockApprovals != nil {
		flowContext["_mock_approvals"] = mockApprovals
	}
	for key, value := range initialContext {
		flowContext[key] = value
	}
	return flowContext
}

func (e *DryRunExecutor) resolveDryRunStartNode(nodes []model.FlowNode, fromNodeID string, result *DryRunResult, emit dryRunEmit) *model.FlowNode {
	if fromNodeID != "" {
		startNode := e.findNodeByID(nodes, fromNodeID)
		if startNode != nil {
			return startNode
		}
		result.Success = false
		result.Message = fmt.Sprintf("指定的节点 %s 不存在", fromNodeID)
		emit(model.SSEEventFlowComplete, map[string]interface{}{"success": false, "message": result.Message})
		return nil
	}

	startNode := e.findNodeByType(nodes, model.NodeTypeStart)
	if startNode != nil {
		return startNode
	}
	result.Success = false
	result.Message = "流程缺少起始节点"
	emit(model.SSEEventFlowComplete, map[string]interface{}{"success": false, "message": result.Message})
	return nil
}

func (e *DryRunExecutor) runDryRunLoop(ctx context.Context, nodes []model.FlowNode, edges []model.FlowEdge, startNode *model.FlowNode, flowContext map[string]interface{}, result *DryRunResult, emit dryRunEmit) {
	currentNode := startNode
	visited := make(map[string]bool)

	for currentNode != nil {
		if visited[currentNode.ID] {
			break
		}
		visited[currentNode.ID] = true

		e.emitDryRunNodeStart(currentNode, emit)
		nodeResult := e.executeNode(ctx, currentNode, flowContext)
		result.Nodes = append(result.Nodes, nodeResult)
		e.emitDryRunNodeComplete(nodeResult, emit)

		if e.handleDryRunNodeError(currentNode, nodes, edges, visited, result, nodeResult, emit) {
			break
		}
		if currentNode.Type == model.NodeTypeEnd {
			break
		}
		currentNode = e.nextDryRunNode(currentNode, nodeResult, nodes, edges, visited, result, emit)
	}
}

func (e *DryRunExecutor) emitDryRunNodeStart(node *model.FlowNode, emit dryRunEmit) {
	config := e.getNodeConfig(node)
	nodeName := node.Name
	if nodeName == "" {
		if label, ok := config["label"].(string); ok {
			nodeName = label
		}
	}
	emit(model.SSEEventNodeStart, map[string]interface{}{
		"node_id":   node.ID,
		"node_type": node.Type,
		"node_name": nodeName,
		"status":    model.NodeStatusRunning,
	})
}

func (e *DryRunExecutor) emitDryRunNodeComplete(nodeResult DryRunNodeResult, emit dryRunEmit) {
	outputHandle := ""
	if nodeResult.Output != nil {
		if value, ok := nodeResult.Output["output_handle"].(string); ok {
			outputHandle = value
		}
	}
	emit(model.SSEEventNodeComplete, map[string]interface{}{
		"node_id":       nodeResult.NodeID,
		"node_type":     nodeResult.NodeType,
		"node_name":     nodeResult.NodeName,
		"status":        nodeResult.Status,
		"message":       nodeResult.Message,
		"input":         nodeResult.Input,
		"process":       nodeResult.Process,
		"output":        nodeResult.Output,
		"output_handle": outputHandle,
	})
}

func (e *DryRunExecutor) handleDryRunNodeError(currentNode *model.FlowNode, nodes []model.FlowNode, edges []model.FlowEdge, visited map[string]bool, result *DryRunResult, nodeResult DryRunNodeResult, emit dryRunEmit) bool {
	if nodeResult.Status != "error" {
		return false
	}
	result.Success = false
	result.Message = fmt.Sprintf("节点 %s 执行失败: %s", currentNode.ID, nodeResult.Message)
	remainingNodes := make(map[string]bool)
	e.collectAllDownstreamNodes(nodes, edges, currentNode.ID, remainingNodes)
	e.emitSkippedNodes(remainingNodes, visited, nodes, result, emit, fmt.Sprintf("上游节点 %s 执行失败，跳过执行", currentNode.ID))
	return true
}

func (e *DryRunExecutor) nextDryRunNode(currentNode *model.FlowNode, nodeResult DryRunNodeResult, nodes []model.FlowNode, edges []model.FlowEdge, visited map[string]bool, result *DryRunResult, emit dryRunEmit) *model.FlowNode {
	switch currentNode.Type {
	case model.NodeTypeCondition:
		e.emitBranchSkippedNodes(currentNode, nodes, edges, visited, result, emit, "true", "条件分支未选中，跳过执行")
		if trueTarget, ok := e.getNodeConfig(currentNode)["true_target"].(string); ok {
			return e.findNodeByID(nodes, trueTarget)
		}
	case model.NodeTypeApproval:
		outputHandle := dryRunOutputHandle(nodeResult, "approved")
		e.emitBranchSkippedNodes(currentNode, nodes, edges, visited, result, emit, outputHandle, fmt.Sprintf("审批分支 %s 未选中，跳过执行", outputHandle))
		if nextNode := e.findNextNodeByHandle(nodes, edges, currentNode.ID, outputHandle); nextNode != nil {
			return nextNode
		}
	case model.NodeTypeExecution:
		outputHandle := dryRunOutputHandle(nodeResult, "success")
		e.emitBranchSkippedNodes(currentNode, nodes, edges, visited, result, emit, outputHandle, fmt.Sprintf("执行分支 %s 未选中，跳过执行", outputHandle))
		if nextNode := e.findNextNodeByHandle(nodes, edges, currentNode.ID, outputHandle); nextNode != nil {
			return nextNode
		}
	}
	return e.findNextNode(nodes, edges, currentNode.ID)
}

func dryRunOutputHandle(nodeResult DryRunNodeResult, defaultHandle string) string {
	if nodeResult.Output != nil {
		if handle, ok := nodeResult.Output["output_handle"].(string); ok {
			return handle
		}
	}
	return defaultHandle
}

func (e *DryRunExecutor) emitBranchSkippedNodes(currentNode *model.FlowNode, nodes []model.FlowNode, edges []model.FlowEdge, visited map[string]bool, result *DryRunResult, emit dryRunEmit, chosenHandle string, message string) {
	skippedNodeIDs := e.getSkippedBranchNodes(nodes, edges, currentNode.ID, chosenHandle, visited)
	e.emitSkippedNodeIDs(skippedNodeIDs, visited, nodes, result, emit, message)
}

func (e *DryRunExecutor) emitSkippedNodes(remainingNodes map[string]bool, visited map[string]bool, nodes []model.FlowNode, result *DryRunResult, emit dryRunEmit, message string) {
	var skippedIDs []string
	for nodeID := range remainingNodes {
		skippedIDs = append(skippedIDs, nodeID)
	}
	e.emitSkippedNodeIDs(skippedIDs, visited, nodes, result, emit, message)
}

func (e *DryRunExecutor) emitSkippedNodeIDs(skippedNodeIDs []string, visited map[string]bool, nodes []model.FlowNode, result *DryRunResult, emit dryRunEmit, message string) {
	for _, skippedID := range skippedNodeIDs {
		if visited[skippedID] {
			continue
		}
		skippedNode := e.findNodeByID(nodes, skippedID)
		if skippedNode == nil {
			continue
		}
		emit(model.SSEEventNodeComplete, map[string]interface{}{
			"node_id":   skippedNode.ID,
			"node_type": skippedNode.Type,
			"node_name": skippedNode.Name,
			"status":    "skipped",
			"message":   message,
			"input":     nil,
			"process":   []string{"分支未选中"},
			"output":    nil,
		})
		result.Nodes = append(result.Nodes, DryRunNodeResult{
			NodeID:   skippedNode.ID,
			NodeType: skippedNode.Type,
			NodeName: skippedNode.Name,
			Status:   "skipped",
			Message:  message,
		})
		visited[skippedID] = true
	}
}

func (e *DryRunExecutor) finishDryRunResult(result *DryRunResult, emit dryRunEmit) {
	if result.Success {
		result.Message = fmt.Sprintf("Dry-Run 完成，共执行 %d 个节点", len(result.Nodes))
	}
	emit(model.SSEEventFlowComplete, map[string]interface{}{
		"success": result.Success,
		"message": result.Message,
	})
}
