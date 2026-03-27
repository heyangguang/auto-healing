package healing

import (
	"fmt"
	"reflect"
	"strconv"
	"strings"
)

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

// joinArray 将数组用分隔符连接成字符串
func joinArray(arr interface{}, sep string) string {
	if arr == nil {
		return ""
	}

	val := reflect.ValueOf(arr)
	if !isArrayLike(val.Kind()) {
		return toStringValue(arr)
	}

	parts := make([]string, 0, val.Len())
	for i := 0; i < val.Len(); i++ {
		parts = append(parts, arrayItemDisplayValue(val.Index(i).Interface()))
	}
	return strings.Join(parts, sep)
}

func arrayItemDisplayValue(elem interface{}) string {
	m, ok := elem.(map[string]interface{})
	if !ok {
		return toStringValue(elem)
	}
	if ip, ok := m["ip_address"].(string); ok && ip != "" {
		return ip
	}
	if name, ok := m["name"].(string); ok {
		return name
	}
	if hostname, ok := m["hostname"].(string); ok {
		return hostname
	}
	return toStringValue(elem)
}

// pluckField 从数组中提取指定字段形成新数组
func pluckField(arr interface{}, field string) []interface{} {
	if arr == nil {
		return nil
	}

	val := reflect.ValueOf(arr)
	if !isArrayLike(val.Kind()) {
		return nil
	}

	results := make([]interface{}, 0, val.Len())
	for i := 0; i < val.Len(); i++ {
		m, ok := val.Index(i).Interface().(map[string]interface{})
		if !ok {
			continue
		}
		if value, exists := m[field]; exists {
			results = append(results, value)
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
	}
	return 0
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
	}
	return 0
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
