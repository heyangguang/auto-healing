package healing

import (
	"context"
	"encoding/json"
	"regexp"
	"strings"

	"github.com/company/auto-healing/internal/model"
)

// RuleMatcher 规则匹配器
type RuleMatcher struct{}

// NewRuleMatcher 创建规则匹配器
func NewRuleMatcher() *RuleMatcher {
	return &RuleMatcher{}
}

// Match 判断工单是否匹配规则
func (m *RuleMatcher) Match(ctx context.Context, incident *model.Incident, rule *model.HealingRule) bool {
	// 解析规则条件
	conditions, err := m.parseConditions(rule.Conditions)
	if err != nil || len(conditions) == 0 {
		return false
	}

	// 使用递归评估（支持嵌套条件组）
	return m.evaluateConditions(incident, conditions, rule.MatchMode)
}

// parseConditions 解析规则条件
func (m *RuleMatcher) parseConditions(conditionsJSON model.JSONArray) ([]model.RuleCondition, error) {
	// 将 JSONArray 转换为条件数组
	data, err := json.Marshal(conditionsJSON)
	if err != nil {
		return nil, err
	}

	var conditions []model.RuleCondition
	if err := json.Unmarshal(data, &conditions); err != nil {
		return nil, err
	}
	return conditions, nil
}

// evaluateConditions 递归评估条件列表
// matchMode: "all" (AND) 或 "any" (OR)
func (m *RuleMatcher) evaluateConditions(incident *model.Incident, conditions []model.RuleCondition, matchMode string) bool {
	if len(conditions) == 0 {
		return true
	}

	// 根据匹配模式决定逻辑
	isAnd := matchMode == model.MatchModeAll

	for _, cond := range conditions {
		var result bool

		if cond.IsGroup() {
			// 条件组：递归评估
			groupLogic := cond.Logic
			if groupLogic == "" {
				groupLogic = "AND" // 默认AND
			}
			// 将 "AND"/"OR" 转换为 "all"/"any"
			groupMatchMode := model.MatchModeAll
			if groupLogic == "OR" {
				groupMatchMode = model.MatchModeAny
			}
			result = m.evaluateConditions(incident, cond.Conditions, groupMatchMode)
		} else {
			// 单个条件：直接评估
			result = m.evaluateCondition(incident, cond)
		}

		// 短路逻辑
		if isAnd && !result {
			return false // AND模式下，任一条件失败则整体失败
		}
		if !isAnd && result {
			return true // OR模式下，任一条件成功则整体成功
		}
	}

	// 如果走到这里：
	// - AND模式：所有条件都通过了
	// - OR模式：所有条件都失败了
	return isAnd
}

// evaluateCondition 评估单个条件
func (m *RuleMatcher) evaluateCondition(incident *model.Incident, cond model.RuleCondition) bool {
	// 获取工单字段值
	fieldValue := m.getFieldValue(incident, cond.Field)

	// 根据操作符进行匹配
	switch cond.Operator {
	case model.OperatorEquals:
		return m.equals(fieldValue, cond.Value)
	case model.OperatorContains:
		return m.contains(fieldValue, cond.Value)
	case model.OperatorIn:
		return m.in(fieldValue, cond.Value)
	case model.OperatorRegex:
		return m.regex(fieldValue, cond.Value)
	case model.OperatorGt:
		return m.gt(fieldValue, cond.Value)
	case model.OperatorLt:
		return m.lt(fieldValue, cond.Value)
	case model.OperatorGte:
		return m.gte(fieldValue, cond.Value)
	case model.OperatorLte:
		return m.lte(fieldValue, cond.Value)
	default:
		return false
	}
}

// getFieldValue 获取工单字段值
func (m *RuleMatcher) getFieldValue(incident *model.Incident, field string) interface{} {
	switch field {
	case "title":
		return incident.Title
	case "description":
		return incident.Description
	case "severity":
		return incident.Severity
	case "priority":
		return incident.Priority
	case "status":
		return incident.Status
	case "category":
		return incident.Category
	case "affected_ci":
		return incident.AffectedCI
	case "affected_service":
		return incident.AffectedService
	case "assignee":
		return incident.Assignee
	case "reporter":
		return incident.Reporter
	case "source_plugin_name":
		return incident.SourcePluginName
	default:
		// 尝试从 raw_data 中获取
		if incident.RawData != nil {
			if val, ok := incident.RawData[field]; ok {
				return val
			}
		}
		return nil
	}
}

// equals 等于判断
func (m *RuleMatcher) equals(fieldValue, condValue interface{}) bool {
	if fieldValue == nil || condValue == nil {
		return fieldValue == condValue
	}
	return toString(fieldValue) == toString(condValue)
}

// contains 包含判断
func (m *RuleMatcher) contains(fieldValue, condValue interface{}) bool {
	fieldStr := toString(fieldValue)
	condStr := toString(condValue)
	return strings.Contains(strings.ToLower(fieldStr), strings.ToLower(condStr))
}

// in 在列表中判断
func (m *RuleMatcher) in(fieldValue, condValue interface{}) bool {
	fieldStr := strings.ToLower(toString(fieldValue))

	// condValue 应该是数组
	switch v := condValue.(type) {
	case []interface{}:
		for _, item := range v {
			if strings.ToLower(toString(item)) == fieldStr {
				return true
			}
		}
	case []string:
		for _, item := range v {
			if strings.ToLower(item) == fieldStr {
				return true
			}
		}
	}
	return false
}

// regex 正则匹配
func (m *RuleMatcher) regex(fieldValue, condValue interface{}) bool {
	fieldStr := toString(fieldValue)
	pattern := toString(condValue)

	re, err := regexp.Compile(pattern)
	if err != nil {
		return false
	}
	return re.MatchString(fieldStr)
}

// gt 大于判断
func (m *RuleMatcher) gt(fieldValue, condValue interface{}) bool {
	fieldNum := toFloat64(fieldValue)
	condNum := toFloat64(condValue)
	return fieldNum > condNum
}

// lt 小于判断
func (m *RuleMatcher) lt(fieldValue, condValue interface{}) bool {
	fieldNum := toFloat64(fieldValue)
	condNum := toFloat64(condValue)
	return fieldNum < condNum
}

// gte 大于等于判断
func (m *RuleMatcher) gte(fieldValue, condValue interface{}) bool {
	fieldNum := toFloat64(fieldValue)
	condNum := toFloat64(condValue)
	return fieldNum >= condNum
}

// lte 小于等于判断
func (m *RuleMatcher) lte(fieldValue, condValue interface{}) bool {
	fieldNum := toFloat64(fieldValue)
	condNum := toFloat64(condValue)
	return fieldNum <= condNum
}

// toString 转换为字符串
func toString(v interface{}) string {
	if v == nil {
		return ""
	}
	switch val := v.(type) {
	case string:
		return val
	case float64:
		return strings.TrimRight(strings.TrimRight(
			strings.Replace(
				string(rune(int(val))),
				"\x00", "", -1),
			"0"), ".")
	default:
		data, _ := json.Marshal(val)
		return string(data)
	}
}

// toFloat64 转换为浮点数
func toFloat64(v interface{}) float64 {
	if v == nil {
		return 0
	}
	switch val := v.(type) {
	case float64:
		return val
	case int:
		return float64(val)
	case int64:
		return float64(val)
	case string:
		var f float64
		json.Unmarshal([]byte(val), &f)
		return f
	default:
		return 0
	}
}
