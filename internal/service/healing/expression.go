package healing

import (
	"fmt"
	"reflect"
	"strconv"
	"strings"

	"github.com/expr-lang/expr"
	"github.com/expr-lang/expr/vm"
)

// ExpressionEvaluator 表达式求值器
// 使用 expr 库执行动态表达式，支持访问 context 中的数据
type ExpressionEvaluator struct {
	// 预编译的表达式缓存，提升性能
	cache map[string]*vm.Program
}

// NewExpressionEvaluator 创建表达式求值器
func NewExpressionEvaluator() *ExpressionEvaluator {
	return &ExpressionEvaluator{
		cache: make(map[string]*vm.Program),
	}
}

// customFunctions 返回自定义函数列表
// 注意：当函数参数类型不匹配时，使用 panic 抛出错误信息
// expr 引擎会捕获 panic 并将其转换为执行错误，返回给用户
func (e *ExpressionEvaluator) customFunctions() map[string]interface{} {
	return map[string]interface{}{
		// 数组操作 - 要求参数必须是数组类型
		"join": func(arr interface{}, sep string) string {
			if arr == nil {
				return ""
			}
			val := reflect.ValueOf(arr)
			if val.Kind() != reflect.Slice && val.Kind() != reflect.Array {
				panic(fmt.Sprintf("join() 函数要求第一个参数是数组类型，但收到的是 %s 类型", getTypeName(arr)))
			}
			return joinArray(arr, sep)
		},
		"first": func(arr interface{}) interface{} {
			if arr == nil {
				panic("first() 函数的参数不能为 nil")
			}
			val := reflect.ValueOf(arr)
			if val.Kind() != reflect.Slice && val.Kind() != reflect.Array {
				panic(fmt.Sprintf("first() 函数要求参数是数组类型，但收到的是 %s 类型。如需访问对象字段，请直接使用 对象.字段名 的方式", getTypeName(arr)))
			}
			if val.Len() == 0 {
				panic("first() 函数的参数是空数组")
			}
			return val.Index(0).Interface()
		},
		"last": func(arr interface{}) interface{} {
			if arr == nil {
				panic("last() 函数的参数不能为 nil")
			}
			val := reflect.ValueOf(arr)
			if val.Kind() != reflect.Slice && val.Kind() != reflect.Array {
				panic(fmt.Sprintf("last() 函数要求参数是数组类型，但收到的是 %s 类型。如需访问对象字段，请直接使用 对象.字段名 的方式", getTypeName(arr)))
			}
			if val.Len() == 0 {
				panic("last() 函数的参数是空数组")
			}
			return val.Index(val.Len() - 1).Interface()
		},

		// 类型转换
		"toInt": func(v interface{}) int {
			return toIntValue(v)
		},
		"toFloat": func(v interface{}) float64 {
			return toFloatValue(v)
		},
		"toString": func(v interface{}) string {
			return toStringValue(v)
		},

		// 字符串操作 - 参数类型由 expr 引擎自动检查
		"upper": func(s string) string {
			return strings.ToUpper(s)
		},
		"lower": func(s string) string {
			return strings.ToLower(s)
		},
		"replace": func(s, old, new string) string {
			return strings.ReplaceAll(s, old, new)
		},
		"trim": func(s string) string {
			return strings.TrimSpace(s)
		},
		"split": func(s, sep string) []string {
			return strings.Split(s, sep)
		},
		// 字符串包含检查 - 使用 strContains 避免与 expr 内置 contains 中缀操作符冲突
		// expr 内置语法: "hello world" contains "world"  → true
		// 自定义函数语法: strContains("hello world", "world") → true
		"strContains": func(s, substr string) bool {
			return strings.Contains(s, substr)
		},
		"hasPrefix": func(s, prefix string) bool {
			return strings.HasPrefix(s, prefix)
		},
		"hasSuffix": func(s, suffix string) bool {
			return strings.HasSuffix(s, suffix)
		},

		// 数学操作 - 使用 interface{} 支持 int 和 float64
		"abs": func(n interface{}) float64 {
			f := toFloatValue(n)
			if f < 0 {
				return -f
			}
			return f
		},
		"max": func(a, b interface{}) float64 {
			fa, fb := toFloatValue(a), toFloatValue(b)
			if fa > fb {
				return fa
			}
			return fb
		},
		"min": func(a, b interface{}) float64 {
			fa, fb := toFloatValue(a), toFloatValue(b)
			if fa < fb {
				return fa
			}
			return fb
		},

		// 条件操作
		"default": func(val interface{}, defaultVal interface{}) interface{} {
			if val == nil || val == "" {
				return defaultVal
			}
			return val
		},

		// 提取字段（从数组中提取指定字段形成新数组）
		"pluck": func(arr interface{}, field string) []interface{} {
			if arr == nil {
				panic("pluck() 函数的参数不能为 nil")
			}
			val := reflect.ValueOf(arr)
			if val.Kind() != reflect.Slice && val.Kind() != reflect.Array {
				panic(fmt.Sprintf("pluck() 函数要求第一个参数是数组类型，但收到的是 %s 类型", getTypeName(arr)))
			}
			return pluckField(arr, field)
		},

		// 数组长度
		"len": func(arr interface{}) int {
			if arr == nil {
				return 0
			}
			val := reflect.ValueOf(arr)
			switch val.Kind() {
			case reflect.Slice, reflect.Array, reflect.Map, reflect.String:
				return val.Len()
			default:
				panic(fmt.Sprintf("len() 函数要求参数是数组、字符串或 map 类型，但收到的是 %s 类型", getTypeName(arr)))
			}
		},
	}
}

// getTypeName 返回用户友好的类型名称
func getTypeName(v interface{}) string {
	if v == nil {
		return "nil"
	}
	val := reflect.ValueOf(v)
	switch val.Kind() {
	case reflect.Slice, reflect.Array:
		return "数组"
	case reflect.Map:
		return "对象/字典"
	case reflect.Ptr:
		return "指针（内部类型：" + val.Elem().Type().String() + "）"
	case reflect.Struct:
		return "结构体"
	case reflect.String:
		return "字符串"
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return "整数"
	case reflect.Float32, reflect.Float64:
		return "浮点数"
	case reflect.Bool:
		return "布尔值"
	default:
		return val.Type().String()
	}
}

// Evaluate 执行表达式并返回结果
// expression: 表达式字符串，如 "validated_hosts[0].ip_address" 或 "join(hosts, ',')"
// context: 上下文数据，包含 incident、hosts、validated_hosts 等
func (e *ExpressionEvaluator) Evaluate(expression string, context map[string]interface{}) (interface{}, error) {
	if expression == "" {
		return nil, fmt.Errorf("表达式为空")
	}

	// 合并上下文和自定义函数
	env := make(map[string]interface{})
	for k, v := range context {
		env[k] = v
	}
	for k, v := range e.customFunctions() {
		env[k] = v
	}

	// 编译表达式
	program, err := expr.Compile(expression, expr.Env(env))
	if err != nil {
		return nil, fmt.Errorf("编译表达式失败: %w", err)
	}

	// 执行表达式
	result, err := expr.Run(program, env)
	if err != nil {
		return nil, fmt.Errorf("执行表达式失败: %w", err)
	}

	return result, nil
}

// EvaluateToString 执行表达式并返回字符串结果
func (e *ExpressionEvaluator) EvaluateToString(expression string, context map[string]interface{}) (string, error) {
	result, err := e.Evaluate(expression, context)
	if err != nil {
		return "", err
	}
	return toStringValue(result), nil
}

// EvaluateMultiple 批量执行多个表达式
// expressions: map[output_key] = expression
// 返回: map[output_key] = result
func (e *ExpressionEvaluator) EvaluateMultiple(expressions map[string]string, context map[string]interface{}) (map[string]interface{}, []error) {
	results := make(map[string]interface{})
	var errors []error

	for outputKey, expression := range expressions {
		result, err := e.Evaluate(expression, context)
		if err != nil {
			errors = append(errors, fmt.Errorf("计算 %s 失败: %w", outputKey, err))
			continue
		}
		results[outputKey] = result
	}

	return results, errors
}

// ============= 辅助函数 =============

// joinArray 将数组用分隔符连接成字符串
func joinArray(arr interface{}, sep string) string {
	if arr == nil {
		return ""
	}

	val := reflect.ValueOf(arr)
	if val.Kind() != reflect.Slice && val.Kind() != reflect.Array {
		return toStringValue(arr)
	}

	var parts []string
	for i := 0; i < val.Len(); i++ {
		elem := val.Index(i).Interface()
		// 如果是 map，尝试提取 ip_address 或 name
		if m, ok := elem.(map[string]interface{}); ok {
			if ip, ok := m["ip_address"].(string); ok && ip != "" {
				parts = append(parts, ip)
			} else if name, ok := m["name"].(string); ok {
				parts = append(parts, name)
			} else if hostname, ok := m["hostname"].(string); ok {
				parts = append(parts, hostname)
			} else {
				parts = append(parts, toStringValue(elem))
			}
		} else {
			parts = append(parts, toStringValue(elem))
		}
	}
	return strings.Join(parts, sep)
}

// firstElement 返回数组第一个元素
func firstElement(arr interface{}) interface{} {
	if arr == nil {
		return nil
	}

	val := reflect.ValueOf(arr)
	if val.Kind() != reflect.Slice && val.Kind() != reflect.Array {
		return arr
	}

	if val.Len() == 0 {
		return nil
	}

	return val.Index(0).Interface()
}

// lastElement 返回数组最后一个元素
func lastElement(arr interface{}) interface{} {
	if arr == nil {
		return nil
	}

	val := reflect.ValueOf(arr)
	if val.Kind() != reflect.Slice && val.Kind() != reflect.Array {
		return arr
	}

	if val.Len() == 0 {
		return nil
	}

	return val.Index(val.Len() - 1).Interface()
}

// pluckField 从数组中提取指定字段形成新数组
func pluckField(arr interface{}, field string) []interface{} {
	if arr == nil {
		return nil
	}

	val := reflect.ValueOf(arr)
	if val.Kind() != reflect.Slice && val.Kind() != reflect.Array {
		return nil
	}

	var results []interface{}
	for i := 0; i < val.Len(); i++ {
		elem := val.Index(i).Interface()
		if m, ok := elem.(map[string]interface{}); ok {
			if v, exists := m[field]; exists {
				results = append(results, v)
			}
		}
	}
	return results
}

// toIntValue 将任意值转换为 int
func toIntValue(v interface{}) int {
	switch val := v.(type) {
	case int:
		return val
	case int64:
		return int(val)
	case int32:
		return int(val)
	case float64:
		return int(val)
	case float32:
		return int(val)
	case string:
		if i, err := strconv.Atoi(val); err == nil {
			return i
		}
		return 0
	default:
		return 0
	}
}

// toFloatValue 将任意值转换为 float64
func toFloatValue(v interface{}) float64 {
	switch val := v.(type) {
	case float64:
		return val
	case float32:
		return float64(val)
	case int:
		return float64(val)
	case int64:
		return float64(val)
	case int32:
		return float64(val)
	case string:
		if f, err := strconv.ParseFloat(val, 64); err == nil {
			return f
		}
		return 0
	default:
		return 0
	}
}

// toStringValue 将任意值转换为 string
func toStringValue(v interface{}) string {
	if v == nil {
		return ""
	}
	switch val := v.(type) {
	case string:
		return val
	case int:
		return strconv.Itoa(val)
	case int64:
		return strconv.FormatInt(val, 10)
	case float64:
		return strconv.FormatFloat(val, 'f', -1, 64)
	case bool:
		return strconv.FormatBool(val)
	default:
		return fmt.Sprintf("%v", val)
	}
}
