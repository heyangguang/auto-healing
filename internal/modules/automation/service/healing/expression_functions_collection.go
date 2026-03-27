package healing

import (
	"fmt"
	"reflect"
)

const objectFieldAccessHint = "。如需访问对象字段，请直接使用 对象.字段名 的方式"

func sequenceFunctions() map[string]interface{} {
	return map[string]interface{}{
		"join":  joinFunction(),
		"first": firstFunction(),
		"last":  lastFunction(),
	}
}

func collectionFunctions() map[string]interface{} {
	return map[string]interface{}{
		"default": defaultFunction(),
		"pluck":   pluckFunction(),
		"len":     lengthFunction(),
	}
}

func joinFunction() interface{} {
	return func(arr interface{}, sep string) string {
		if arr == nil {
			return ""
		}
		if !isArrayLike(reflect.ValueOf(arr).Kind()) {
			panic(fmt.Sprintf("join() 函数要求第一个参数是数组类型，但收到的是 %s 类型", getTypeName(arr)))
		}
		return joinArray(arr, sep)
	}
}

func firstFunction() interface{} {
	return func(arr interface{}) interface{} {
		val := requireNonEmptyArray("first", arr, objectFieldAccessHint)
		return val.Index(0).Interface()
	}
}

func lastFunction() interface{} {
	return func(arr interface{}) interface{} {
		val := requireNonEmptyArray("last", arr, objectFieldAccessHint)
		return val.Index(val.Len() - 1).Interface()
	}
}

func defaultFunction() interface{} {
	return func(val interface{}, defaultVal interface{}) interface{} {
		if val == nil || val == "" {
			return defaultVal
		}
		return val
	}
}

func pluckFunction() interface{} {
	return func(arr interface{}, field string) []interface{} {
		requireArray("pluck", arr, "第一个参数", "")
		return pluckField(arr, field)
	}
}

func lengthFunction() interface{} {
	return func(arr interface{}) int {
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
	}
}

func requireNonEmptyArray(name string, value interface{}, hint string) reflect.Value {
	val := requireArray(name, value, "参数", hint)
	if val.Len() == 0 {
		panic(fmt.Sprintf("%s() 函数的参数是空数组", name))
	}
	return val
}

func requireArray(name string, value interface{}, argLabel, hint string) reflect.Value {
	if value == nil {
		panic(fmt.Sprintf("%s() 函数的参数不能为 nil", name))
	}
	val := reflect.ValueOf(value)
	if isArrayLike(val.Kind()) {
		return val
	}

	message := fmt.Sprintf("%s() 函数要求%s是数组类型，但收到的是 %s 类型", name, argLabel, getTypeName(value))
	if hint != "" {
		message += hint
	}
	panic(message)
}

func isArrayLike(kind reflect.Kind) bool {
	return kind == reflect.Slice || kind == reflect.Array
}
