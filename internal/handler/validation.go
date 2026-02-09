package handler

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/go-playground/validator/v10"
)

// 字段名中文映射
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

// FormatValidationError 格式化验证错误，提取具体字段名称
func FormatValidationError(err error) string {
	if err == nil {
		return ""
	}

	// 检查是否是 validator 错误
	if validationErrors, ok := err.(validator.ValidationErrors); ok {
		var msgs []string
		for _, fieldErr := range validationErrors {
			fieldName := getFieldName(fieldErr.Field())
			msg := formatFieldError(fieldName, fieldErr)
			msgs = append(msgs, msg)
		}
		return strings.Join(msgs, "; ")
	}

	// 检查是否是 UUID 解析错误
	errStr := err.Error()
	if strings.Contains(errStr, "invalid UUID") {
		// 尝试从错误中提取更多信息
		if strings.Contains(errStr, "length") {
			return "role_ids 字段格式错误: UUID 长度不正确，请使用完整的 UUID 格式 (例如: 12220598-8abf-4adf-9406-41f5b9cab04b)"
		}
		return "role_ids 字段格式错误: 无效的 UUID 格式"
	}

	// 检查 JSON 解析错误
	if strings.Contains(errStr, "cannot unmarshal") {
		if strings.Contains(errStr, "uuid") || strings.Contains(errStr, "UUID") {
			return "role_ids 字段类型错误: 需要有效的 UUID 数组"
		}
	}

	return errStr
}

// getFieldName 获取字段的中文名称
func getFieldName(field string) string {
	if name, ok := fieldNameMap[field]; ok {
		return fmt.Sprintf("%s (%s)", name, field)
	}
	return field
}

// formatFieldError 格式化单个字段的错误
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

// ToBusinessError 将技术错误转换为业务友好的错误信息
func ToBusinessError(err error) string {
	if err == nil {
		return ""
	}

	errStr := err.Error()

	// 常见错误映射
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

	// 检查是否匹配已知错误
	for key, msg := range errorMap {
		if strings.Contains(errStr, key) {
			return msg
		}
	}

	// 不暴露具体技术细节，返回通用错误
	if strings.Contains(errStr, "SQLSTATE") ||
		strings.Contains(errStr, "pq:") ||
		strings.Contains(errStr, "database") {
		return "操作失败，请稍后重试"
	}

	// 如果是我们自定义的中文错误，直接返回
	if isChineseError(errStr) {
		return errStr
	}

	return "操作失败，请稍后重试"
}

// isChineseError 检查是否是中文错误信息
func isChineseError(s string) bool {
	for _, r := range s {
		if r >= 0x4e00 && r <= 0x9fff {
			return true
		}
	}
	return false
}
