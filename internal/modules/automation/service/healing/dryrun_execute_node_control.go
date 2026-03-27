package healing

import (
	"fmt"
)

func (e *DryRunExecutor) executeStartNodeDryRun(result *DryRunNodeResult, flowContext map[string]interface{}) {
	result.Process = append(result.Process, "初始化流程上下文")
	result.Message = "流程开始"
	if incident, ok := flowContext["incident"]; ok {
		result.Output["incident"] = incident
		result.Process = append(result.Process, "输出 incident 到下游")
	}
}

func (e *DryRunExecutor) executeEndNodeDryRun(result *DryRunNodeResult) {
	result.Process = append(result.Process, "流程执行完毕")
	result.Message = "流程结束"
}

func (e *DryRunExecutor) executeApprovalNodeDryRun(result *DryRunNodeResult, flowContext map[string]interface{}, config map[string]interface{}) {
	title, hasTitle := config["title"].(string)
	result.Process = append(result.Process, fmt.Sprintf("读取配置 title: %s", title))
	if !hasTitle || title == "" {
		result.Status = "error"
		result.Message = "审批节点配置错误: 未设置审批标题"
		result.Process = append(result.Process, "错误: 未设置审批标题")
		return
	}

	mockResult := e.mockApprovalResult(result, flowContext)
	if mockResult == "rejected" {
		result.Message = fmt.Sprintf("审批节点「%s」(模拟拒绝)", title)
		result.Output["approval_result"] = "rejected"
		result.Output["output_handle"] = "rejected"
		result.Process = append(result.Process, "模拟结果: 拒绝，走 rejected 分支")
		return
	}

	result.Message = fmt.Sprintf("审批节点「%s」(模拟通过)", title)
	result.Output["approval_result"] = "approved"
	result.Output["output_handle"] = "approved"
	result.Process = append(result.Process, "模拟结果: 通过，走 approved 分支")
}

func (e *DryRunExecutor) mockApprovalResult(result *DryRunNodeResult, flowContext map[string]interface{}) string {
	if mockApprovals, ok := flowContext["_mock_approvals"].(map[string]string); ok {
		if specified, exists := mockApprovals[result.NodeID]; exists {
			result.Process = append(result.Process, fmt.Sprintf("使用 mock_approvals 配置: %s", specified))
			return specified
		}
	}
	result.Process = append(result.Process, "未指定模拟结果，默认 approved")
	return "approved"
}

func (e *DryRunExecutor) executeConditionNodeDryRun(result *DryRunNodeResult, config map[string]interface{}) {
	result.Message = "条件判断(Dry-Run 默认走 true 分支)"
	if condition, ok := config["condition"].(string); ok {
		result.Output["condition"] = condition
	}
	result.Output["result"] = true
	result.Output["output_handle"] = "true"
}

func (e *DryRunExecutor) executeSetVariableNodeDryRun(result *DryRunNodeResult, flowContext map[string]interface{}, config map[string]interface{}) {
	key, _ := config["key"].(string)
	value := config["value"]
	result.Process = append(result.Process, fmt.Sprintf("读取配置 key: %s", key))
	result.Process = append(result.Process, fmt.Sprintf("读取配置 value: %v", value))
	result.Message = fmt.Sprintf("设置变量 %s = %v", key, value)
	if key != "" {
		flowContext[key] = value
		result.Process = append(result.Process, fmt.Sprintf("写入上下文 %s", key))
	}
	result.Output["key"] = key
	result.Output["value"] = value
}

func (e *DryRunExecutor) executeComputeNodeDryRun(result *DryRunNodeResult, flowContext map[string]interface{}, config map[string]interface{}) {
	operations, ok := config["operations"].([]interface{})
	if !ok || len(operations) == 0 {
		result.Process = append(result.Process, "计算节点配置为空")
		result.Message = "计算节点无操作"
		return
	}

	result.Process = append(result.Process, fmt.Sprintf("读取 %d 个计算操作", len(operations)))
	computedVars := make(map[string]interface{})
	evaluator := NewExpressionEvaluator()

	for i, opRaw := range operations {
		e.executeComputeOperation(result, flowContext, evaluator, computedVars, i, opRaw)
	}

	result.Status = "success"
	result.Message = fmt.Sprintf("计算完成: %d 个变量", len(computedVars))
	result.Output = computedVars
}

func (e *DryRunExecutor) executeComputeOperation(result *DryRunNodeResult, flowContext map[string]interface{}, evaluator *ExpressionEvaluator, computedVars map[string]interface{}, index int, opRaw interface{}) {
	op, ok := opRaw.(map[string]interface{})
	if !ok {
		result.Process = append(result.Process, fmt.Sprintf("操作 %d: 格式无效，跳过", index+1))
		return
	}

	outputKey, _ := op["output_key"].(string)
	expression, _ := op["expression"].(string)
	if outputKey == "" || expression == "" {
		result.Process = append(result.Process, fmt.Sprintf("操作 %d: output_key 或 expression 为空，跳过", index+1))
		return
	}

	result.Process = append(result.Process, fmt.Sprintf("计算 %s = %s", outputKey, expression))
	computeResult, err := evaluator.Evaluate(expression, flowContext)
	if err != nil {
		result.Process = append(result.Process, fmt.Sprintf("  → 错误: %v", err))
		return
	}

	flowContext[outputKey] = computeResult
	computedVars[outputKey] = computeResult
	result.Process = append(result.Process, fmt.Sprintf("  → %s = %v", outputKey, computeResult))
}
