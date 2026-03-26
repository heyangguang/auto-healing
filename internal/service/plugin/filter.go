package plugin

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/company/auto-healing/internal/model"
)

// FilterCondition 过滤条件（支持嵌套）
type FilterCondition struct {
	// 逻辑操作符 (and/or)，有此字段时表示是组合条件
	Logic string `json:"logic,omitempty"`
	// 子规则列表
	Rules []FilterCondition `json:"rules,omitempty"`

	// 单个规则的字段（当 Logic 为空时使用）
	Field    string      `json:"field,omitempty"`
	Operator string      `json:"operator,omitempty"`
	Value    interface{} `json:"value,omitempty"`
}

// ParseSyncFilter 解析插件的同步过滤器配置
func ParseSyncFilter(syncFilter model.JSON) (*FilterCondition, error) {
	if len(syncFilter) == 0 {
		return nil, nil
	}

	data, err := json.Marshal(syncFilter)
	if err != nil {
		return nil, err
	}

	var condition FilterCondition
	if err := json.Unmarshal(data, &condition); err != nil {
		return nil, err
	}

	return &condition, nil
}

// ApplyFilter 应用过滤器到数据
// 返回 true 表示数据通过过滤（应该被同步）
func ApplyFilter(filter *FilterCondition, data map[string]interface{}) bool {
	if filter == nil {
		return true // 无过滤器，全部放行
	}
	matched, _ := evaluateConditionWithReason(filter, data)
	return matched
}

// ApplyFilterWithReason 应用过滤器并返回原因
// 返回 (是否通过, 不通过原因)
func ApplyFilterWithReason(filter *FilterCondition, data map[string]interface{}) (bool, string) {
	if filter == nil {
		return true, "" // 无过滤器，全部放行
	}
	return evaluateConditionWithReason(filter, data)
}

// evaluateConditionWithReason 递归评估条件并返回原因
func evaluateConditionWithReason(cond *FilterCondition, data map[string]interface{}) (bool, string) {
	// 组合条件
	if cond.Logic != "" && len(cond.Rules) > 0 {
		switch strings.ToLower(cond.Logic) {
		case "and":
			for _, rule := range cond.Rules {
				matched, reason := evaluateConditionWithReason(&rule, data)
				if !matched {
					return false, reason
				}
			}
			return true, ""
		case "or":
			reasons := []string{}
			for _, rule := range cond.Rules {
				matched, reason := evaluateConditionWithReason(&rule, data)
				if matched {
					return true, ""
				}
				reasons = append(reasons, reason)
			}
			return false, strings.Join(reasons, " 且 ")
		}
	}

	// 单个规则
	if cond.Field != "" && cond.Operator != "" {
		return evaluateRuleWithReason(cond.Field, cond.Operator, cond.Value, data)
	}

	return true, "" // 空条件放行
}

// evaluateRuleWithReason 评估单个规则并返回原因
func evaluateRuleWithReason(field, operator string, value interface{}, data map[string]interface{}) (bool, string) {
	fieldStr := fieldString(data, field)
	valueStr := toString(value)
	return evaluateNormalizedRule(field, strings.ToLower(operator), value, fieldStr, valueStr)
}

func fieldString(data map[string]interface{}, field string) string {
	fieldValue, ok := data[field]
	if !ok {
		fieldValue = ""
	}
	return toString(fieldValue)
}

func evaluateNormalizedRule(field, operator string, value interface{}, fieldStr, valueStr string) (bool, string) {
	makeReason := func() string {
		return fmt.Sprintf("%s=%s 不满足 %s %s", field, fieldStr, operator, valueStr)
	}

	switch operator {
	case "equals":
		if fieldStr == valueStr {
			return true, ""
		}
		return false, makeReason()
	case "not_equals":
		if fieldStr != valueStr {
			return true, ""
		}
		return false, makeReason()
	case "contains":
		return evaluateContains(fieldStr, valueStr, true, makeReason)
	case "not_contains":
		return evaluateContains(fieldStr, valueStr, false, makeReason)
	case "starts_with":
		return evaluatePrefix(fieldStr, valueStr, true, makeReason)
	case "ends_with":
		return evaluatePrefix(fieldStr, valueStr, false, makeReason)
	case "regex":
		matched, _ := regexp.MatchString(valueStr, fieldStr)
		if matched {
			return true, ""
		}
		return false, makeReason()
	case "in":
		if isInList(fieldStr, value) {
			return true, ""
		}
		return false, fmt.Sprintf("%s=%s 不在列表 %v 中", field, fieldStr, value)
	case "not_in":
		if !isInList(fieldStr, value) {
			return true, ""
		}
		return false, fmt.Sprintf("%s=%s 在排除列表 %v 中", field, fieldStr, value)
	default:
		return true, "" // 未知操作符放行
	}
}

func evaluateContains(fieldStr, valueStr string, shouldContain bool, makeReason func() string) (bool, string) {
	contains := strings.Contains(strings.ToLower(fieldStr), strings.ToLower(valueStr))
	if contains == shouldContain {
		return true, ""
	}
	return false, makeReason()
}

func evaluatePrefix(fieldStr, valueStr string, prefix bool, makeReason func() string) (bool, string) {
	left := strings.ToLower(fieldStr)
	right := strings.ToLower(valueStr)
	var matched bool
	if prefix {
		matched = strings.HasPrefix(left, right)
	} else {
		matched = strings.HasSuffix(left, right)
	}
	if matched {
		return true, ""
	}
	return false, makeReason()
}

// toString 将任意值转为字符串
func toString(v interface{}) string {
	if v == nil {
		return ""
	}
	switch val := v.(type) {
	case string:
		return val
	case float64:
		return strconv.FormatFloat(val, 'f', -1, 64)
	case float32:
		return strconv.FormatFloat(float64(val), 'f', -1, 32)
	case int:
		return strconv.Itoa(val)
	case int64:
		return strconv.FormatInt(val, 10)
	case bool:
		return strconv.FormatBool(val)
	default:
		data, _ := json.Marshal(v)
		return strings.Trim(string(data), "\"")
	}
}

// isInList 检查值是否在列表中
func isInList(value string, list interface{}) bool {
	switch l := list.(type) {
	case []interface{}:
		for _, item := range l {
			if toString(item) == value {
				return true
			}
		}
	case []string:
		for _, item := range l {
			if item == value {
				return true
			}
		}
	}
	return false
}
