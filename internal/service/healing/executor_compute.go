package healing

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/company/auto-healing/internal/model"
	"github.com/company/auto-healing/internal/pkg/logger"
)

// executeCompute 执行计算节点
func (e *FlowExecutor) executeCompute(ctx context.Context, instance *model.FlowInstance, node *model.FlowNode) error {
	logger.Exec("NODE").Info("[%s] 执行计算节点", shortID(instance))

	operations, ok := node.Config["operations"].([]interface{})
	if !ok || len(operations) == 0 {
		e.logNode(ctx, instance.ID, node.ID, node.Type, model.LogLevelWarn, "计算节点配置为空", nil)
		return nil
	}

	if instance.Context == nil {
		instance.Context = make(model.JSON)
	}

	processLogs := []string{fmt.Sprintf("读取 %d 个计算操作", len(operations))}
	results := make(map[string]interface{})
	var errors []string
	evaluator := NewExpressionEvaluator()

	for i, opRaw := range operations {
		e.evaluateComputeOperation(instance, evaluator, i, opRaw, results, &errors, &processLogs)
	}

	if err := e.persistInstance(ctx, instance, "持久化计算节点上下文"); err != nil {
		return err
	}

	logDetails := map[string]interface{}{
		"input": map[string]interface{}{
			"operations": operations,
			"context":    instance.Context,
		},
		"process": processLogs,
		"output":  results,
	}
	if len(errors) > 0 {
		logDetails["errors"] = errors
	}

	if len(results) == 0 && len(errors) > 0 {
		e.logNode(ctx, instance.ID, node.ID, node.Type, model.LogLevelError, "计算节点执行失败", logDetails)
		return fmt.Errorf("所有计算操作均失败: %v", errors)
	}

	logLevel, message := computeLogStatus(results, errors)
	e.logNode(ctx, instance.ID, node.ID, node.Type, logLevel, message, logDetails)
	if err := e.storeComputeNodeState(ctx, instance, node.ID, results, errors); err != nil {
		return err
	}
	logger.Exec("NODE").Info("[%s] 计算节点完成: %d 个变量已写入 context", shortID(instance), len(results))
	return nil
}

func (e *FlowExecutor) evaluateComputeOperation(instance *model.FlowInstance, evaluator *ExpressionEvaluator, index int, opRaw interface{}, results map[string]interface{}, errors *[]string, processLogs *[]string) {
	op, ok := opRaw.(map[string]interface{})
	if !ok {
		*errors = append(*errors, fmt.Sprintf("操作 %d: 格式无效", index+1))
		return
	}

	outputKey, _ := op["output_key"].(string)
	expression, _ := op["expression"].(string)
	if outputKey == "" {
		*errors = append(*errors, fmt.Sprintf("操作 %d: output_key 为空", index+1))
		return
	}
	if expression == "" {
		*errors = append(*errors, fmt.Sprintf("操作 %d: expression 为空", index+1))
		return
	}

	*processLogs = append(*processLogs, fmt.Sprintf("计算 %s = %s", outputKey, expression))
	result, err := evaluator.Evaluate(expression, instance.Context)
	if err != nil {
		errMsg := fmt.Sprintf("操作 %d (%s): %v", index+1, outputKey, err)
		*errors = append(*errors, errMsg)
		logger.Exec("NODE").Warn("[%s] 表达式计算失败: %s", shortID(instance), errMsg)
		return
	}

	instance.Context[outputKey] = result
	results[outputKey] = result
	resultJSON, _ := json.Marshal(result)
	*processLogs = append(*processLogs, fmt.Sprintf("  → %s = %s", outputKey, string(resultJSON)))
	logger.Exec("NODE").Debug("[%s] 计算结果: %s = %s", shortID(instance), outputKey, string(resultJSON))
}

func computeLogStatus(results map[string]interface{}, errors []string) (string, string) {
	if len(errors) > 0 {
		return model.LogLevelWarn, fmt.Sprintf("计算完成: %d 成功, %d 失败", len(results), len(errors))
	}
	return model.LogLevelInfo, fmt.Sprintf("计算完成: %d 个变量", len(results))
}

func (e *FlowExecutor) storeComputeNodeState(ctx context.Context, instance *model.FlowInstance, nodeID string, results map[string]interface{}, errors []string) error {
	if instance.NodeStates == nil {
		instance.NodeStates = make(model.JSON)
	}
	if existing, ok := instance.NodeStates[nodeID].(map[string]interface{}); ok {
		existing["computed_results"] = results
			if len(errors) > 0 {
				existing["errors"] = errors
			}
			instance.NodeStates[nodeID] = existing
			return e.persistNodeStates(ctx, instance, "持久化计算节点状态")
		}
	return nil
}
