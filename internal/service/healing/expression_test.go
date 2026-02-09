package healing

import (
	"testing"
)

// TestExpressionEvaluator_IncidentFieldAccess 测试工单字段访问
func TestExpressionEvaluator_IncidentFieldAccess(t *testing.T) {
	evaluator := NewExpressionEvaluator()

	// 模拟 incidentToMap 转换后的 incident 数据 (小写键名)
	incidentMap := map[string]interface{}{
		"id":               "550e8400-e29b-41d4-a716-446655440000",
		"title":            "服务器故障",
		"description":      "服务器 172.16.1.100 无法访问",
		"severity":         "critical",
		"priority":         "high",
		"status":           "open",
		"category":         "基础设施",
		"affected_ci":      "app-server-01",
		"affected_service": "订单服务",
		"assignee":         "张三",
		"reporter":         "李四",
		"raw_data": map[string]interface{}{
			"cmdb_ci":   "server-001",
			"alert_id":  "ALT-12345",
			"env":       "production",
			"host_list": []string{"192.168.1.1", "192.168.1.2"},
		},
	}

	context := map[string]interface{}{
		"incident": incidentMap,
	}

	tests := []struct {
		name       string
		expression string
		wantType   string // "string", "int", "bool", "array", "map"
		wantValue  interface{}
	}{
		// 基本字段访问
		{"访问 title", "incident.title", "string", "服务器故障"},
		{"访问 severity", "incident.severity", "string", "critical"},
		{"访问 affected_ci", "incident.affected_ci", "string", "app-server-01"},
		{"访问 assignee", "incident.assignee", "string", "张三"},

		// 嵌套字段访问 (raw_data)
		{"访问 raw_data.cmdb_ci", "incident.raw_data.cmdb_ci", "string", "server-001"},
		{"访问 raw_data.env", "incident.raw_data.env", "string", "production"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := evaluator.Evaluate(tt.expression, context)
			if err != nil {
				t.Errorf("表达式 %q 执行失败: %v", tt.expression, err)
				return
			}
			if result != tt.wantValue {
				t.Errorf("表达式 %q = %v, 期望 %v", tt.expression, result, tt.wantValue)
			}
		})
	}
}

// TestExpressionEvaluator_ArrayOperations 测试数组操作
func TestExpressionEvaluator_ArrayOperations(t *testing.T) {
	evaluator := NewExpressionEvaluator()

	// 模拟 validated_hosts 数据
	validatedHosts := []interface{}{
		map[string]interface{}{
			"original_name": "server-01",
			"cmdb_name":     "app-server-01",
			"ip":            "192.168.1.100",
			"status":        "active",
		},
		map[string]interface{}{
			"original_name": "server-02",
			"cmdb_name":     "app-server-02",
			"ip":            "192.168.1.101",
			"status":        "active",
		},
		map[string]interface{}{
			"original_name": "server-03",
			"cmdb_name":     "app-server-03",
			"ip":            "192.168.1.102",
			"status":        "maintenance",
		},
	}

	simpleHosts := []string{"host1", "host2", "host3"}

	context := map[string]interface{}{
		"validated_hosts": validatedHosts,
		"hosts":           simpleHosts,
	}

	tests := []struct {
		name       string
		expression string
	}{
		// 数组长度
		{"len(validated_hosts)", "len(validated_hosts)"},
		{"len(hosts)", "len(hosts)"},

		// 数组索引访问
		{"validated_hosts[0].ip", "validated_hosts[0].ip"},
		{"validated_hosts[1].cmdb_name", "validated_hosts[1].cmdb_name"},
		{"hosts[0]", "hosts[0]"},

		// first/last 函数
		{"first(validated_hosts).ip", "first(validated_hosts).ip"},
		{"last(validated_hosts).ip", "last(validated_hosts).ip"},

		// join 函数
		{"join(hosts, ',')", "join(hosts, ',')"},
		{"join(hosts, ' | ')", "join(hosts, ' | ')"},

		// pluck 函数 - 提取字段
		{"pluck(validated_hosts, 'ip')", "pluck(validated_hosts, 'ip')"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := evaluator.Evaluate(tt.expression, context)
			if err != nil {
				t.Errorf("表达式 %q 执行失败: %v", tt.expression, err)
				return
			}
			t.Logf("表达式 %q = %v", tt.expression, result)
		})
	}
}

// TestExpressionEvaluator_StringOperations 测试字符串操作
func TestExpressionEvaluator_StringOperations(t *testing.T) {
	evaluator := NewExpressionEvaluator()

	context := map[string]interface{}{
		"title":       "服务器故障报告",
		"hostname":    "  app-server-01  ",
		"description": "IP: 192.168.1.100, Host: server-01",
	}

	tests := []struct {
		name       string
		expression string
		wantValue  interface{}
	}{
		{"upper", "upper(title)", "服务器故障报告"}, // 中文不变
		{"lower - ASCII", "lower('HELLO')", "hello"},
		{"trim", "trim(hostname)", "app-server-01"},
		{"replace", "replace('hello world', 'world', 'go')", "hello go"},
		{"contains - true (中缀语法)", "description contains '192.168'", true},
		{"contains - false (中缀语法)", "description contains 'not-exist'", false},
		{"split", "split('a,b,c', ',')", []string{"a", "b", "c"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := evaluator.Evaluate(tt.expression, context)
			if err != nil {
				t.Errorf("表达式 %q 执行失败: %v", tt.expression, err)
				return
			}
			t.Logf("表达式 %q = %v", tt.expression, result)
		})
	}
}

// TestExpressionEvaluator_TypeConversion 测试类型转换
func TestExpressionEvaluator_TypeConversion(t *testing.T) {
	evaluator := NewExpressionEvaluator()

	context := map[string]interface{}{
		"str_num":   "123",
		"float_num": 3.14,
		"int_num":   42,
	}

	tests := []struct {
		name       string
		expression string
	}{
		{"toInt from string", "toInt(str_num)"},
		{"toFloat from int", "toFloat(int_num)"},
		{"toString from int", "toString(int_num)"},
		{"toString from float", "toString(float_num)"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := evaluator.Evaluate(tt.expression, context)
			if err != nil {
				t.Errorf("表达式 %q 执行失败: %v", tt.expression, err)
				return
			}
			t.Logf("表达式 %q = %v (type: %T)", tt.expression, result, result)
		})
	}
}

// TestExpressionEvaluator_Arithmetic 测试算术运算
func TestExpressionEvaluator_Arithmetic(t *testing.T) {
	evaluator := NewExpressionEvaluator()

	validatedHosts := []interface{}{
		map[string]interface{}{"ip": "192.168.1.1"},
		map[string]interface{}{"ip": "192.168.1.2"},
	}

	context := map[string]interface{}{
		"a":               10,
		"b":               3,
		"price":           99.9,
		"validated_hosts": validatedHosts,
	}

	tests := []struct {
		name       string
		expression string
		wantValue  interface{}
	}{
		{"加法", "a + b", 13},
		{"减法", "a - b", 7},
		{"乘法", "a * b", 30},
		{"除法", "a / b", float64(10) / float64(3)},
		{"取模", "a % b", 1},
		{"abs", "abs(-5)", float64(5)},
		{"max", "max(a, b)", float64(10)},
		{"min", "min(a, b)", float64(3)},
		{"len 用于计算", "len(validated_hosts) * 2", 4},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := evaluator.Evaluate(tt.expression, context)
			if err != nil {
				t.Errorf("表达式 %q 执行失败: %v", tt.expression, err)
				return
			}
			t.Logf("表达式 %q = %v", tt.expression, result)
		})
	}
}

// TestExpressionEvaluator_Comparison 测试比较运算
func TestExpressionEvaluator_Comparison(t *testing.T) {
	evaluator := NewExpressionEvaluator()

	validatedHosts := []interface{}{
		map[string]interface{}{"ip": "192.168.1.1"},
	}

	context := map[string]interface{}{
		"count":           5,
		"status":          "active",
		"validated_hosts": validatedHosts,
	}

	tests := []struct {
		name       string
		expression string
		wantValue  bool
	}{
		{"大于", "count > 3", true},
		{"大于等于", "count >= 5", true},
		{"小于", "count < 10", true},
		{"小于等于", "count <= 5", true},
		{"等于 - 数字", "count == 5", true},
		{"等于 - 字符串", "status == 'active'", true},
		{"不等于", "status != 'inactive'", true},
		{"逻辑与", "count > 0 && status == 'active'", true},
		{"逻辑或", "count < 0 || status == 'active'", true},
		{"逻辑非", "!(count < 0)", true},
		{"数组长度比较", "len(validated_hosts) > 0", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := evaluator.Evaluate(tt.expression, context)
			if err != nil {
				t.Errorf("表达式 %q 执行失败: %v", tt.expression, err)
				return
			}
			if result != tt.wantValue {
				t.Errorf("表达式 %q = %v, 期望 %v", tt.expression, result, tt.wantValue)
			}
		})
	}
}

// TestExpressionEvaluator_DefaultFunction 测试 default 函数
func TestExpressionEvaluator_DefaultFunction(t *testing.T) {
	evaluator := NewExpressionEvaluator()

	context := map[string]interface{}{
		"existing_value": "hello",
		"empty_value":    "",
		// nil_value 不存在
	}

	tests := []struct {
		name       string
		expression string
		wantValue  interface{}
	}{
		{"有值不使用默认", "default(existing_value, 'fallback')", "hello"},
		{"空值使用默认", "default(empty_value, 'fallback')", "fallback"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := evaluator.Evaluate(tt.expression, context)
			if err != nil {
				t.Errorf("表达式 %q 执行失败: %v", tt.expression, err)
				return
			}
			if result != tt.wantValue {
				t.Errorf("表达式 %q = %v, 期望 %v", tt.expression, result, tt.wantValue)
			}
		})
	}
}

// TestExpressionEvaluator_ComplexExpressions 测试复合表达式
func TestExpressionEvaluator_ComplexExpressions(t *testing.T) {
	evaluator := NewExpressionEvaluator()

	incidentMap := map[string]interface{}{
		"title":    "服务器故障",
		"severity": "critical",
		"raw_data": map[string]interface{}{
			"host": "192.168.1.100",
		},
	}

	validatedHosts := []interface{}{
		map[string]interface{}{"ip": "192.168.1.1", "name": "server-01"},
		map[string]interface{}{"ip": "192.168.1.2", "name": "server-02"},
	}

	context := map[string]interface{}{
		"incident":        incidentMap,
		"validated_hosts": validatedHosts,
	}

	tests := []struct {
		name       string
		expression string
	}{
		// 复合表达式
		{"标题拼接", "incident.title + ' - ' + incident.severity"},
		{"条件判断 + 数组操作", "len(validated_hosts) > 0 ? first(validated_hosts).ip : 'no hosts'"},
		{"join IP 地址", "join(pluck(validated_hosts, 'ip'), ',')"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := evaluator.Evaluate(tt.expression, context)
			if err != nil {
				t.Errorf("表达式 %q 执行失败: %v", tt.expression, err)
				return
			}
			t.Logf("表达式 %q = %v", tt.expression, result)
		})
	}
}

// TestExpressionEvaluator_GoStructVsMap 测试 Go 结构体 vs Map 的字段访问差异
// 这是导致 incident.title 错误的根本原因测试
func TestExpressionEvaluator_GoStructVsMap(t *testing.T) {
	evaluator := NewExpressionEvaluator()

	// 情况1: 使用 map (正确做法，字段名小写)
	incidentAsMap := map[string]interface{}{
		"title":       "测试工单",
		"severity":    "high",
		"affected_ci": "server-01",
	}

	mapContext := map[string]interface{}{
		"incident": incidentAsMap,
	}

	// 测试小写字段名访问 (map)
	result, err := evaluator.Evaluate("incident.title", mapContext)
	if err != nil {
		t.Errorf("Map 场景下 incident.title 失败: %v", err)
	} else {
		t.Logf("Map 场景 incident.title = %v ✓", result)
	}

	// 注意: 如果直接使用 Go 结构体，需要用大写字段名
	// type Incident struct { Title string }
	// 这种情况下表达式应该是 incident.Title
	// 但我们已经通过 incidentToMap 统一转换为 map，所以使用小写即可
}
