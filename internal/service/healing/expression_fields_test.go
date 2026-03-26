package healing

import "testing"

// TestExpressionEvaluator_IncidentFieldAccess 测试工单字段访问
func TestExpressionEvaluator_IncidentFieldAccess(t *testing.T) {
	runExpressionValueCases(t, incidentExpressionContext(), []expressionValueCase{
		{name: "访问 title", expression: "incident.title", want: "服务器故障"},
		{name: "访问 severity", expression: "incident.severity", want: "critical"},
		{name: "访问 affected_ci", expression: "incident.affected_ci", want: "app-server-01"},
		{name: "访问 assignee", expression: "incident.assignee", want: "张三"},
		{name: "访问 raw_data.cmdb_ci", expression: "incident.raw_data.cmdb_ci", want: "server-001"},
		{name: "访问 raw_data.env", expression: "incident.raw_data.env", want: "production"},
	})
}

// TestExpressionEvaluator_ArrayOperations 测试数组操作
func TestExpressionEvaluator_ArrayOperations(t *testing.T) {
	runExpressionSuccessCases(t, arrayExpressionContext(), []expressionSuccessCase{
		{name: "len(validated_hosts)", expression: "len(validated_hosts)"},
		{name: "len(hosts)", expression: "len(hosts)"},
		{name: "validated_hosts[0].ip", expression: "validated_hosts[0].ip"},
		{name: "validated_hosts[1].cmdb_name", expression: "validated_hosts[1].cmdb_name"},
		{name: "hosts[0]", expression: "hosts[0]"},
		{name: "first(validated_hosts).ip", expression: "first(validated_hosts).ip"},
		{name: "last(validated_hosts).ip", expression: "last(validated_hosts).ip"},
		{name: "join(hosts, ',')", expression: "join(hosts, ',')"},
		{name: "join(hosts, ' | ')", expression: "join(hosts, ' | ')"},
		{name: "pluck(validated_hosts, 'ip')", expression: "pluck(validated_hosts, 'ip')"},
	})
}

// TestExpressionEvaluator_StringOperations 测试字符串操作
func TestExpressionEvaluator_StringOperations(t *testing.T) {
	context := map[string]interface{}{
		"title":       "服务器故障报告",
		"hostname":    "  app-server-01  ",
		"description": "IP: 192.168.1.100, Host: server-01",
	}

	runExpressionValueCases(t, context, []expressionValueCase{
		{name: "upper", expression: "upper(title)", want: "服务器故障报告"},
		{name: "lower - ASCII", expression: "lower('HELLO')", want: "hello"},
		{name: "trim", expression: "trim(hostname)", want: "app-server-01"},
		{name: "replace", expression: "replace('hello world', 'world', 'go')", want: "hello go"},
		{name: "contains - true (中缀语法)", expression: "description contains '192.168'", want: true},
		{name: "contains - false (中缀语法)", expression: "description contains 'not-exist'", want: false},
		{name: "split", expression: "split('a,b,c', ',')", want: []string{"a", "b", "c"}},
	})
}

// TestExpressionEvaluator_TypeConversion 测试类型转换
func TestExpressionEvaluator_TypeConversion(t *testing.T) {
	context := map[string]interface{}{
		"str_num":   "123",
		"float_num": 3.14,
		"int_num":   42,
	}

	runExpressionSuccessCases(t, context, []expressionSuccessCase{
		{name: "toInt from string", expression: "toInt(str_num)"},
		{name: "toFloat from int", expression: "toFloat(int_num)"},
		{name: "toString from int", expression: "toString(int_num)"},
		{name: "toString from float", expression: "toString(float_num)"},
	})
}
