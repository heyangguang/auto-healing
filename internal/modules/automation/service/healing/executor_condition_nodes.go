package healing

import (
	"context"
	"fmt"
	"strings"

	"github.com/company/auto-healing/internal/modules/automation/model"
	"github.com/company/auto-healing/internal/pkg/logger"
)

// executeCondition 执行条件判断节点
func (e *FlowExecutor) executeCondition(ctx context.Context, instance *model.FlowInstance, nodes []model.FlowNode, edges []model.FlowEdge, node *model.FlowNode) error {
	logger.Exec("NODE").Debug("执行条件节点 %s", node.ID)
	if err := e.setNodeState(ctx, instance, node.ID, "running", ""); err != nil {
		return err
	}

	conditions, defaultTarget := parseConditionConfig(node.Config)
	flowContext := instance.Context
	if flowContext == nil {
		flowContext = make(model.JSON)
	}

	matchedTarget, matchedExpression := e.matchConditionBranch(ctx, instance, node, conditions, defaultTarget, flowContext)
	if matchedTarget == "" {
		errMsg := "条件节点没有匹配的分支且无默认目标"
		if err := e.setNodeState(ctx, instance, node.ID, "failed", errMsg); err != nil {
			return err
		}
		e.logNode(ctx, instance.ID, node.ID, node.Type, model.LogLevelError, "条件判断失败", map[string]interface{}{
			"error": errMsg,
		})
		return fmt.Errorf("%s", errMsg)
	}

	nextNode := findConditionTargetNode(nodes, matchedTarget)
	if nextNode == nil {
		errMsg := fmt.Sprintf("找不到目标节点: %s", matchedTarget)
		if err := e.setNodeState(ctx, instance, node.ID, "failed", errMsg); err != nil {
			return err
		}
		e.logNode(ctx, instance.ID, node.ID, node.Type, model.LogLevelError, "条件判断失败", map[string]interface{}{
			"error":  errMsg,
			"target": matchedTarget,
		})
		return fmt.Errorf("%s", errMsg)
	}

	if err := e.setNodeState(ctx, instance, node.ID, "completed", ""); err != nil {
		return err
	}
	if existing, ok := instance.NodeStates[node.ID].(map[string]interface{}); ok {
		existing["activated_branch"] = matchedTarget
		if matchedExpression != "" {
			existing["matched_expression"] = matchedExpression
		}
		instance.NodeStates[node.ID] = existing
		if err := e.persistNodeStates(ctx, instance, "持久化条件节点分支状态"); err != nil {
			return err
		}
	}

	logger.Exec("NODE").Info("条件节点跳转到: %s", matchedTarget)
	return e.executeNode(ctx, instance, nodes, edges, nextNode)
}

func parseConditionConfig(config map[string]interface{}) ([]map[string]interface{}, string) {
	var conditions []map[string]interface{}
	if rawConditions, ok := config["conditions"].([]interface{}); ok {
		for _, raw := range rawConditions {
			if condMap, ok := raw.(map[string]interface{}); ok {
				conditions = append(conditions, condMap)
			}
		}
	}

	defaultTarget, _ := config["default_target"].(string)
	return conditions, defaultTarget
}

func (e *FlowExecutor) matchConditionBranch(ctx context.Context, instance *model.FlowInstance, node *model.FlowNode, conditions []map[string]interface{}, defaultTarget string, flowContext model.JSON) (string, string) {
	for _, condition := range conditions {
		expression, _ := condition["expression"].(string)
		target, _ := condition["target"].(string)
		if expression == "" || target == "" {
			continue
		}

		result := e.evaluateCondition(expression, flowContext, instance.NodeStates)
		logger.Exec("NODE").Debug("条件求值: %s = %v", expression, result)
		if !result {
			continue
		}

		e.logNode(ctx, instance.ID, node.ID, node.Type, model.LogLevelInfo, "条件匹配", map[string]interface{}{
			"expression":     expression,
			"matched_target": target,
		})
		return target, expression
	}

	if defaultTarget != "" {
		e.logNode(ctx, instance.ID, node.ID, node.Type, model.LogLevelInfo, "使用默认分支", map[string]interface{}{
			"default_target": defaultTarget,
		})
	}
	return defaultTarget, ""
}

func findConditionTargetNode(nodes []model.FlowNode, targetID string) *model.FlowNode {
	for i := range nodes {
		if nodes[i].ID == targetID {
			return &nodes[i]
		}
	}
	return nil
}

// executeSetVariable 执行变量设置节点
func (e *FlowExecutor) executeSetVariable(ctx context.Context, instance *model.FlowInstance, node *model.FlowNode) error {
	logger.Exec("NODE").Debug("执行变量设置节点 %s", node.ID)

	variables, ok := node.Config["variables"].(map[string]interface{})
	if !ok {
		e.logNode(ctx, instance.ID, node.ID, node.Type, model.LogLevelWarn, "变量配置为空", nil)
		return nil
	}
	if instance.Context == nil {
		instance.Context = make(model.JSON)
	}

	setVars := make(map[string]interface{})
	for varName, varValue := range variables {
		finalValue := e.resolveVariableValue(instance, varValue)
		instance.Context[varName] = finalValue
		setVars[varName] = finalValue
		logger.Exec("NODE").Debug("设置变量: %s = %v", varName, finalValue)
	}

	if err := e.instanceRepo.Update(ctx, instance); err != nil {
		return err
	}

	e.logNode(ctx, instance.ID, node.ID, node.Type, model.LogLevelInfo, "变量设置完成", map[string]interface{}{
		"variables": setVars,
	})
	if instance.NodeStates == nil {
		instance.NodeStates = make(model.JSON)
	}
	if existing, ok := instance.NodeStates[node.ID].(map[string]interface{}); ok {
		existing["variables_set"] = setVars
		instance.NodeStates[node.ID] = existing
		if err := e.persistNodeStates(ctx, instance, "持久化变量设置节点状态"); err != nil {
			return err
		}
	}
	return nil
}

func (e *FlowExecutor) resolveVariableValue(instance *model.FlowInstance, raw interface{}) interface{} {
	value, ok := raw.(string)
	if !ok {
		return raw
	}
	if !looksLikeExpression(value) {
		return value
	}
	if isBooleanExpression(value) {
		return e.evaluateCondition(value, instance.Context, instance.NodeStates)
	}
	if resolved := e.resolveValue(value, instance.Context, instance.NodeStates); resolved != nil {
		return resolved
	}
	return value
}

func looksLikeExpression(value string) bool {
	return isBooleanExpression(value) || strings.Contains(value, ".")
}

func isBooleanExpression(value string) bool {
	return strings.Contains(value, "==") || strings.Contains(value, "!=") || strings.Contains(value, ">") || strings.Contains(value, "<")
}
