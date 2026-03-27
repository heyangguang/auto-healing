package healing

import (
	"fmt"
	"strings"

	"github.com/company/auto-healing/internal/modules/automation/model"
	"github.com/company/auto-healing/internal/pkg/logger"
)

// evaluateCondition 求值条件表达式
func (e *FlowExecutor) evaluateCondition(expression string, flowContext model.JSON, nodeStates model.JSON) bool {
	expression = strings.TrimSpace(expression)

	left, operator, right := parseConditionExpression(expression)
	if operator == "" {
		logger.Exec("NODE").Warn("无法解析条件表达式: %s", expression)
		return false
	}

	leftValue := e.resolveValue(left, flowContext, nodeStates)
	rightValue := e.parseRightValue(right)
	logger.Exec("NODE").Debug("比较: %v %s %v", leftValue, operator, rightValue)
	return e.compare(leftValue, operator, rightValue)
}

func parseConditionExpression(expression string) (string, string, string) {
	operators := []string{"==", "!=", ">=", "<=", ">", "<"}
	for _, operator := range operators {
		if parts := strings.SplitN(expression, operator, 2); len(parts) == 2 {
			return strings.TrimSpace(parts[0]), operator, strings.TrimSpace(parts[1])
		}
	}
	return "", "", ""
}

// resolveValue 从 context 或 nodeStates 解析变量值
func (e *FlowExecutor) resolveValue(path string, flowContext model.JSON, nodeStates model.JSON) interface{} {
	parts := strings.Split(path, ".")
	if len(parts) > 0 {
		if value := resolveExecutionValue(parts, flowContext, nodeStates); value != nil {
			return value
		}
		if value, ok := flowContext[parts[0]]; ok {
			if len(parts) == 1 {
				return value
			}
			if mapVal, ok := value.(map[string]interface{}); ok {
				return e.getNestedValue(mapVal, parts[1:])
			}
		}
	}
	return nil
}

func resolveExecutionValue(parts []string, flowContext model.JSON, nodeStates model.JSON) interface{} {
	if len(parts) == 0 || (parts[0] != "execution_result" && parts[0] != "execution") {
		return nil
	}
	if execResult, ok := flowContext["execution_result"].(map[string]interface{}); ok {
		if len(parts) > 1 {
			return getNestedExecutionValue(execResult, parts[1:])
		}
		return execResult
	}
	for nodeID, stateRaw := range nodeStates {
		state, ok := stateRaw.(map[string]interface{})
		if !ok {
			continue
		}
		if strings.HasPrefix(nodeID, "exec_") || nodeID == "execution" {
			if len(parts) > 1 {
				return getNestedExecutionValue(state, parts[1:])
			}
			return state
		}
	}
	return nil
}

func getNestedExecutionValue(data map[string]interface{}, path []string) interface{} {
	if len(path) == 0 {
		return data
	}
	key := path[0]
	if key == "length" {
		return len(data)
	}

	value, ok := data[key]
	if !ok {
		return nil
	}
	if len(path) == 1 {
		return value
	}
	if mapVal, ok := value.(map[string]interface{}); ok {
		return getNestedExecutionValue(mapVal, path[1:])
	}
	return nil
}

// getNestedValue 获取嵌套的值
func (e *FlowExecutor) getNestedValue(data map[string]interface{}, path []string) interface{} {
	return getNestedExecutionValue(data, path)
}

// parseRightValue 解析右值（支持字面量）
func (e *FlowExecutor) parseRightValue(s string) interface{} {
	s = strings.TrimSpace(s)
	if s == "true" {
		return true
	}
	if s == "false" {
		return false
	}
	if (strings.HasPrefix(s, "\"") && strings.HasSuffix(s, "\"")) || (strings.HasPrefix(s, "'") && strings.HasSuffix(s, "'")) {
		return s[1 : len(s)-1]
	}
	if strings.Contains(s, ".") {
		var value float64
		if _, err := fmt.Sscanf(s, "%f", &value); err == nil {
			return value
		}
	} else {
		var value int64
		if _, err := fmt.Sscanf(s, "%d", &value); err == nil {
			return value
		}
	}
	return s
}

// compare 执行比较操作
func (e *FlowExecutor) compare(left interface{}, op string, right interface{}) bool {
	switch op {
	case "==":
		return compareEqual(left, right)
	case "!=":
		return !compareEqual(left, right)
	case ">":
		return toFloat(left) > toFloat(right)
	case "<":
		return toFloat(left) < toFloat(right)
	case ">=":
		return toFloat(left) >= toFloat(right)
	case "<=":
		return toFloat(left) <= toFloat(right)
	default:
		return false
	}
}

func compareEqual(left interface{}, right interface{}) bool {
	if leftBool, ok := left.(bool); ok {
		if rightBool, ok := right.(bool); ok {
			return leftBool == rightBool
		}
	}

	leftString := fmt.Sprintf("%v", left)
	rightString := fmt.Sprintf("%v", right)
	if leftString == rightString {
		return true
	}

	leftNum := toFloat(left)
	rightNum := toFloat(right)
	if leftNum != 0 || rightNum != 0 {
		return leftNum == rightNum
	}
	return false
}
