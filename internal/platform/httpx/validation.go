package httpx

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/go-playground/validator/v10"
)

var fieldNameMap = map[string]string{
	"Username":    "用户名",
	"Email":       "邮箱",
	"Password":    "密码",
	"DisplayName": "显示名称",
	"RoleIDs":     "角色ID列表",
	"OldPassword": "原密码",
	"NewPassword": "新密码",
	"Name":        "名称",
	"Type":        "类型",
	"Config":      "配置",
}

func FormatValidationError(err error) string {
	if err == nil {
		return ""
	}
	if validationErrors, ok := err.(validator.ValidationErrors); ok {
		var msgs []string
		for _, fieldErr := range validationErrors {
			fieldName := getFieldName(fieldErr.Field())
			msgs = append(msgs, formatFieldError(fieldName, fieldErr))
		}
		return strings.Join(msgs, "; ")
	}

	errStr := err.Error()
	if strings.Contains(errStr, "invalid UUID") {
		if strings.Contains(errStr, "length") {
			return "role_ids 字段格式错误: UUID 长度不正确，请使用完整的 UUID 格式 (例如: 12220598-8abf-4adf-9406-41f5b9cab04b)"
		}
		return "role_ids 字段格式错误: 无效的 UUID 格式"
	}
	if strings.Contains(errStr, "cannot unmarshal") {
		if strings.Contains(errStr, "uuid") || strings.Contains(errStr, "UUID") {
			return "role_ids 字段类型错误: 需要有效的 UUID 数组"
		}
	}
	return errStr
}

func ToBusinessError(err error) string {
	if err == nil {
		return ""
	}

	errStr := err.Error()
	errorMap := map[string]string{
		"用户名或密码错误":           "用户名或密码错误",
		"账户已锁定":              "账户已锁定，请稍后再试",
		"账户已禁用":              "账户已被禁用，请联系管理员",
		"原密码错误":              "原密码错误",
		"用户名已存在":             "用户名已被占用",
		"邮箱已存在":              "邮箱已被占用",
		"选择的角色不存在":           "选择的角色不存在",
		"record not found":   "数据不存在",
		"duplicate key":      "数据已存在",
		"foreign key":        "关联数据不存在",
		"connection refused": "服务暂时不可用，请稍后重试",
		"context deadline":   "请求超时，请稍后重试",
		"invalid UUID":       "ID格式不正确",
	}

	for key, msg := range errorMap {
		if strings.Contains(errStr, key) {
			return msg
		}
	}

	if strings.Contains(errStr, "SQLSTATE") ||
		strings.Contains(errStr, "pq:") ||
		strings.Contains(errStr, "database") {
		return "操作失败，请稍后重试"
	}
	if isChineseError(errStr) {
		return errStr
	}
	return "操作失败，请稍后重试"
}

func getFieldName(field string) string {
	if name, ok := fieldNameMap[field]; ok {
		return fmt.Sprintf("%s (%s)", name, field)
	}
	return field
}

func formatFieldError(fieldName string, fe validator.FieldError) string {
	switch fe.Tag() {
	case "required":
		return fmt.Sprintf("%s 不能为空", fieldName)
	case "min":
		if fe.Type().Kind() == reflect.String {
			return fmt.Sprintf("%s 长度不能少于 %s 个字符", fieldName, fe.Param())
		}
		return fmt.Sprintf("%s 不能小于 %s", fieldName, fe.Param())
	case "max":
		if fe.Type().Kind() == reflect.String {
			return fmt.Sprintf("%s 长度不能超过 %s 个字符", fieldName, fe.Param())
		}
		return fmt.Sprintf("%s 不能大于 %s", fieldName, fe.Param())
	case "email":
		return fmt.Sprintf("%s 格式不正确", fieldName)
	case "uuid":
		return fmt.Sprintf("%s 必须是有效的 UUID 格式", fieldName)
	default:
		return fmt.Sprintf("%s 验证失败 (%s)", fieldName, fe.Tag())
	}
}

func isChineseError(s string) bool {
	for _, r := range s {
		if r >= 0x4e00 && r <= 0x9fff {
			return true
		}
	}
	return false
}
