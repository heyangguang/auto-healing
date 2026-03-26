package healing

import (
	"reflect"
	"testing"
)

type expressionValueCase struct {
	name       string
	expression string
	want       interface{}
}

type expressionSuccessCase struct {
	name       string
	expression string
}

func assertExpressionValue(t *testing.T, evaluator *ExpressionEvaluator, context map[string]interface{}, tc expressionValueCase) {
	t.Helper()

	result, err := evaluator.Evaluate(tc.expression, context)
	if err != nil {
		t.Fatalf("表达式 %q 执行失败: %v", tc.expression, err)
	}
	if !reflect.DeepEqual(result, tc.want) {
		t.Fatalf("表达式 %q = %#v, 期望 %#v", tc.expression, result, tc.want)
	}
}

func logExpressionValue(t *testing.T, evaluator *ExpressionEvaluator, context map[string]interface{}, tc expressionSuccessCase) {
	t.Helper()

	result, err := evaluator.Evaluate(tc.expression, context)
	if err != nil {
		t.Fatalf("表达式 %q 执行失败: %v", tc.expression, err)
	}
	t.Logf("表达式 %q = %v", tc.expression, result)
}

func runExpressionValueCases(t *testing.T, context map[string]interface{}, cases []expressionValueCase) {
	t.Helper()

	evaluator := NewExpressionEvaluator()
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assertExpressionValue(t, evaluator, context, tc)
		})
	}
}

func runExpressionSuccessCases(t *testing.T, context map[string]interface{}, cases []expressionSuccessCase) {
	t.Helper()

	evaluator := NewExpressionEvaluator()
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			logExpressionValue(t, evaluator, context, tc)
		})
	}
}

func incidentExpressionContext() map[string]interface{} {
	return map[string]interface{}{
		"incident": map[string]interface{}{
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
		},
	}
}

func arrayExpressionContext() map[string]interface{} {
	return map[string]interface{}{
		"validated_hosts": []interface{}{
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
		},
		"hosts": []string{"host1", "host2", "host3"},
	}
}
