package healing

import "testing"

// TestExpressionEvaluator_Arithmetic 测试算术运算
func TestExpressionEvaluator_Arithmetic(t *testing.T) {
	context := map[string]interface{}{
		"a":     10,
		"b":     3,
		"price": 99.9,
		"validated_hosts": []interface{}{
			map[string]interface{}{"ip": "192.168.1.1"},
			map[string]interface{}{"ip": "192.168.1.2"},
		},
	}

	runExpressionValueCases(t, context, []expressionValueCase{
		{name: "加法", expression: "a + b", want: 13},
		{name: "减法", expression: "a - b", want: 7},
		{name: "乘法", expression: "a * b", want: 30},
		{name: "除法", expression: "a / b", want: float64(10) / float64(3)},
		{name: "取模", expression: "a % b", want: 1},
		{name: "abs", expression: "abs(-5)", want: float64(5)},
		{name: "max", expression: "max(a, b)", want: float64(10)},
		{name: "min", expression: "min(a, b)", want: float64(3)},
		{name: "len 用于计算", expression: "len(validated_hosts) * 2", want: 4},
	})
}

// TestExpressionEvaluator_Comparison 测试比较运算
func TestExpressionEvaluator_Comparison(t *testing.T) {
	context := map[string]interface{}{
		"count":  5,
		"status": "active",
		"validated_hosts": []interface{}{
			map[string]interface{}{"ip": "192.168.1.1"},
		},
	}

	runExpressionValueCases(t, context, []expressionValueCase{
		{name: "大于", expression: "count > 3", want: true},
		{name: "大于等于", expression: "count >= 5", want: true},
		{name: "小于", expression: "count < 10", want: true},
		{name: "小于等于", expression: "count <= 5", want: true},
		{name: "等于 - 数字", expression: "count == 5", want: true},
		{name: "等于 - 字符串", expression: "status == 'active'", want: true},
		{name: "不等于", expression: "status != 'inactive'", want: true},
		{name: "逻辑与", expression: "count > 0 && status == 'active'", want: true},
		{name: "逻辑或", expression: "count < 0 || status == 'active'", want: true},
		{name: "逻辑非", expression: "!(count < 0)", want: true},
		{name: "数组长度比较", expression: "len(validated_hosts) > 0", want: true},
	})
}

// TestExpressionEvaluator_DefaultFunction 测试 default 函数
func TestExpressionEvaluator_DefaultFunction(t *testing.T) {
	context := map[string]interface{}{
		"existing_value": "hello",
		"empty_value":    "",
	}

	runExpressionValueCases(t, context, []expressionValueCase{
		{name: "有值不使用默认", expression: "default(existing_value, 'fallback')", want: "hello"},
		{name: "空值使用默认", expression: "default(empty_value, 'fallback')", want: "fallback"},
	})
}

// TestExpressionEvaluator_ComplexExpressions 测试复合表达式
func TestExpressionEvaluator_ComplexExpressions(t *testing.T) {
	context := map[string]interface{}{
		"incident": map[string]interface{}{
			"title":    "服务器故障",
			"severity": "critical",
			"raw_data": map[string]interface{}{
				"host": "192.168.1.100",
			},
		},
		"validated_hosts": []interface{}{
			map[string]interface{}{"ip": "192.168.1.1", "name": "server-01"},
			map[string]interface{}{"ip": "192.168.1.2", "name": "server-02"},
		},
	}

	runExpressionSuccessCases(t, context, []expressionSuccessCase{
		{name: "标题拼接", expression: "incident.title + ' - ' + incident.severity"},
		{name: "条件判断 + 数组操作", expression: "len(validated_hosts) > 0 ? first(validated_hosts).ip : 'no hosts'"},
		{name: "join IP 地址", expression: "join(pluck(validated_hosts, 'ip'), ',')"},
	})
}

// TestExpressionEvaluator_GoStructVsMap 测试 Go 结构体 vs Map 的字段访问差异
func TestExpressionEvaluator_GoStructVsMap(t *testing.T) {
	context := map[string]interface{}{
		"incident": map[string]interface{}{
			"title":       "测试工单",
			"severity":    "high",
			"affected_ci": "server-01",
		},
	}

	evaluator := NewExpressionEvaluator()
	result, err := evaluator.Evaluate("incident.title", context)
	if err != nil {
		t.Fatalf("Map 场景下 incident.title 失败: %v", err)
	}
	t.Logf("Map 场景 incident.title = %v ✓", result)
}
