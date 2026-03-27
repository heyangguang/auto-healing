package healing

import (
	"context"
	"fmt"

	"github.com/company/auto-healing/internal/modules/automation/model"
)

// executeNode 执行单个节点
func (e *DryRunExecutor) executeNode(ctx context.Context, node *model.FlowNode, flowContext map[string]interface{}) DryRunNodeResult {
	result := e.newDryRunNodeResult(node, flowContext)
	config := e.getNodeConfig(node)
	result.Config = config

	switch node.Type {
	case model.NodeTypeStart:
		e.executeStartNodeDryRun(&result, flowContext)
	case model.NodeTypeEnd:
		e.executeEndNodeDryRun(&result)
	case model.NodeTypeHostExtractor:
		e.executeHostExtractorNodeDryRun(&result, flowContext, config)
	case model.NodeTypeCMDBValidator:
		e.executeCMDBValidatorNodeDryRun(ctx, &result, flowContext, config)
	case model.NodeTypeApproval:
		e.executeApprovalNodeDryRun(&result, flowContext, config)
	case model.NodeTypeExecution:
		e.executeExecutionNodeDryRun(ctx, &result, flowContext, config)
	case model.NodeTypeNotification:
		e.executeNotificationNodeDryRun(ctx, &result, config)
	case model.NodeTypeCondition:
		e.executeConditionNodeDryRun(&result, config)
	case model.NodeTypeSetVariable:
		e.executeSetVariableNodeDryRun(&result, flowContext, config)
	case model.NodeTypeCompute:
		e.executeComputeNodeDryRun(&result, flowContext, config)
	default:
		result.Status = "error"
		result.Message = fmt.Sprintf("未知节点类型: %s", node.Type)
	}

	return result
}

func (e *DryRunExecutor) newDryRunNodeResult(node *model.FlowNode, flowContext map[string]interface{}) DryRunNodeResult {
	result := DryRunNodeResult{
		NodeID:   node.ID,
		NodeType: node.Type,
		NodeName: node.Name,
		Status:   "success",
		Input:    make(map[string]interface{}),
		Process:  []string{},
		Output:   make(map[string]interface{}),
	}
	for key, value := range flowContext {
		if key != "_mock_approvals" {
			result.Input[key] = value
		}
	}
	return result
}
